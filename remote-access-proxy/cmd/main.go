package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	chserver "github.com/jpillora/chisel/server"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	remoteaccessv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/remoteaccess/v1"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"golang.org/x/crypto/ssh"
)

type RemoteAccessProxyHandler struct {
	invClient inv_client.TenantAwareInventoryClient
	invEvents chan *inv_client.WatchEvents
	filter    Filter
	wg        *sync.WaitGroup
	sigTerm   chan bool
}

func NewRemoteAccessProxyHandler(
	invClient inv_client.TenantAwareInventoryClient,
	events chan *inv_client.WatchEvents,
	wg *sync.WaitGroup,
) *RemoteAccessProxyHandler {
	cache := NewRemoteAccessStateCache()
	filter := NewFilterForRAP(cache, time.Now)

	return &RemoteAccessProxyHandler{
		invClient: invClient,
		invEvents: events,
		filter:    filter,
		wg:        wg,
		sigTerm:   make(chan bool, 1),
	}
}

//func (h *RemoteAccessProxyHandler) Run() {
//	defer h.wg.Done()
//
//	for {
//		select {
//		case <-h.sigTerm:
//			return
//		case we := <-h.invEvents:
//			ev := we.Event // zależy jak wygląda WatchEvents – dopasuj
//			if h.filter(ev) {
//				h.handleEvent(ev)
//			}
//		}
//	}
//}

type Filter func(event *inv_v1.SubscribeEventsResponse) bool

// ---- WS message schema (MVP) ----
type wsMsg struct {
	Type string `json:"type"`           // "stdio" | "resize"
	Data string `json:"data,omitempty"` // for stdio
	Cols int    `json:"cols,omitempty"` // for resize
	Rows int    `json:"rows,omitempty"` // for resize
}

// ---- SSH dialer to reverse port ----
func dialSSH(rows, cols int, term string) (*ssh.Client, *ssh.Session, io.WriteCloser, *bufio.Reader, error) {
	sshCfg := &ssh.ClientConfig{
		User:            "ubuntu",
		Auth:            []ssh.AuthMethod{ssh.Password("zaq12wsx")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	cli, err := ssh.Dial("tcp", "127.0.0.1:8000", sshCfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("ssh dial: %w", err)
	}
	sess, err := cli.NewSession()
	if err != nil {
		_ = cli.Close()
		return nil, nil, nil, nil, fmt.Errorf("ssh new session: %w", err)
	}

	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}
	if term == "" {
		term = "xterm-256color"
	}

	if err := sess.RequestPty(term, rows, cols, ssh.TerminalModes{}); err != nil {
		_ = sess.Close()
		_ = cli.Close()
		return nil, nil, nil, nil, fmt.Errorf("pty: %w", err)
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		_ = sess.Close()
		_ = cli.Close()
		return nil, nil, nil, nil, fmt.Errorf("stdin: %w", err)
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		_ = sess.Close()
		_ = cli.Close()
		return nil, nil, nil, nil, fmt.Errorf("stdout: %w", err)
	}
	// dorzucamy stderr do strumienia
	stderr, _ := sess.StderrPipe()
	reader := bufio.NewReader(io.MultiReader(stdout, stderr))

	if err := sess.Shell(); err != nil {
		_ = sess.Close()
		_ = cli.Close()
		return nil, nil, nil, nil, fmt.Errorf("shell: %w", err)
	}

	return cli, sess, stdin, reader, nil
}

// ---- WS handler: /term ----
var upgrader = websocket.Upgrader{
	ReadBufferSize:  32 * 1024,
	WriteBufferSize: 32 * 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func termHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade: %v", err)
		return
	}
	defer conn.Close()

	if err := waitPort("127.0.0.1:8000", 10*time.Second); err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"stdio","data":"[RAP] reverse not ready\n"}`))
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var initRows, initCols int
	var termName string
	{
		mt, payload, err := conn.ReadMessage()
		if err == nil && mt == websocket.TextMessage {
			var m wsMsg
			if json.Unmarshal(payload, &m) == nil && m.Type == "resize" {
				initRows, initCols = m.Rows, m.Cols
			}
			termName = r.URL.Query().Get("term")
		}
	}
	_ = conn.SetReadDeadline(time.Time{})

	sshClient, sess, stdin, stdout, err := dialSSH(initRows, initCols, termName)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"stdio","data":"[RAP] ssh error: %v\n"}`, err)))
		return
	}
	defer func() { _ = sess.Close(); _ = sshClient.Close() }()

	conn.SetPongHandler(func(string) error { return nil })
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second))
		}
	}()

	var writeMu sync.Mutex
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				out := wsMsg{Type: "stdio", Data: string(buf[:n])}
				b, _ := json.Marshal(out)
				writeMu.Lock()
				_ = conn.WriteMessage(websocket.TextMessage, b)
				writeMu.Unlock()
			}
			if err != nil {
				writeMu.Lock()
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"stdio","data":"[RAP] stream closed\n"}`))
				writeMu.Unlock()
				return
			}
		}
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WS read: %v", err)
			return
		}
		var m wsMsg
		if err := json.Unmarshal(payload, &m); err != nil {
			continue
		}
		switch m.Type {
		case "stdio":
			if _, err := io.WriteString(stdin, m.Data); err != nil {
				return
			}
		case "resize":
			if m.Cols > 0 && m.Rows > 0 {
				_ = sess.WindowChange(m.Rows, m.Cols)
			}
		}
	}
}

// ---- utils ----
func waitPort(addr string, max time.Duration) error {
	deadline := time.Now().Add(max)
	for {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("port %s not ready after %s", addr, max)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func main() {
	wsAddr := envOrDefault("WS_ADDR", "127.0.0.1:50052")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := &chserver.Config{
		KeySeed:   "edge-demo-seed",
		Auth:      "admin:secret", // MVP: hardcoded GA
		Reverse:   true,
		KeepAlive: 25 * time.Second,
	}
	chisrv, err := chserver.NewServer(cfg)
	if err != nil {
		log.Fatalf("chisel: %v", err)
	}
	go func() {
		log.Println("RAP: chisel listening on 0.0.0.0:8080")
		if err := chisrv.StartContext(ctx, "0.0.0.0", "8080"); err != nil {
			log.Printf("chisel err: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/term", termHandler)

	wsSrv := &http.Server{
		Addr:              wsAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("RAP: WS terminal on ws://%s/term\n", wsAddr)
		if err := wsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("ws srv err: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("RAP: shutting down...")
	_ = wsSrv.Shutdown(context.Background())
	_ = chisrv.Close()
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

type RemoteAccessStateCache struct {
	active map[string]bool // resource_id -> czy RAP uważał to za aktywne
}

func NewRemoteAccessStateCache() *RemoteAccessStateCache {
	return &RemoteAccessStateCache{
		active: make(map[string]bool),
	}
}

func NewFilterForRAP(cache *RemoteAccessStateCache, now func() time.Time) Filter {
	return func(ev *inv_v1.SubscribeEventsResponse) bool {
		rac, ok := getRemoteAccessConf(ev)
		if !ok {
			return false
		}

		resourceID := rac.GetResourceId()
		if resourceID == "" {
			// Bez ID nie ma sensu nic robić
			return false
		}

		switch ev.GetEventKind() {

		case inv_v1.SubscribeEventsResponse_EVENT_KIND_CREATED,
			inv_v1.SubscribeEventsResponse_EVENT_KIND_UPDATED:

			activeNow := isActiveForRAP(rac, now)
			activeBefore := cache.active[resourceID]

			// Aktualizujemy cache
			cache.active[resourceID] = activeNow

			// Reagujemy tylko, jeśli zmienił się status z punktu widzenia RAP:
			// nieaktywne -> aktywne lub aktywne -> nieaktywne.
			return activeNow != activeBefore

		case inv_v1.SubscribeEventsResponse_EVENT_KIND_DELETED:
			// Rekord usunięty – jeśli RAP coś o nim wiedział, trzeba posprzątać
			_, wasKnown := cache.active[resourceID]
			delete(cache.active, resourceID)
			return wasKnown

		default:
			return false
		}
	}
}

func isActiveForRAP(rac *remoteaccessv1.RemoteAccessConfiguration, now func() time.Time) bool {
	if rac == nil {
		return false
	}

	// 1. Stan logiczny
	if rac.GetCurrentState() != remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_ENABLED {
		return false
	}

	// 2. Ważność (expiration timestamp)
	if rac.GetExpirationTimestamp() != 0 &&
		rac.GetExpirationTimestamp() <= uint64(now().Unix()) {
		return false
	}

	// 3. Pola techniczne potrzebne do zestawienia tunelu/połączenia
	if rac.GetLocalPort() == 0 ||
		rac.GetProxyHost() == "" ||
		rac.GetSessionToken() == "" {
		return false
	}

	if rac.GetTargetHost() == "" || rac.GetTargetPort() == 0 {
		return false
	}

	return true
}

func getRemoteAccessConf(ev *inv_v1.SubscribeEventsResponse) (*remoteaccessv1.RemoteAccessConfiguration, bool) {
	if ev == nil || ev.GetResource() == nil {
		return nil, false
	}
	rac := ev.GetResource().GetRemoteAccess()
	if rac == nil {
		return nil, false
	}
	return rac, true
}
