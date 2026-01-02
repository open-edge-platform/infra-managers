package rmtaccessconfmgr

import (
	"strings"
	"time"

	remoteaccessv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/remoteaccess/v1"
)

type readiness int

const (
	readinessReady readiness = iota
	readinessPending
	readinessInvalid
)

func evaluateReadiness(ra *remoteaccessv1.RemoteAccessConfiguration, now time.Time) (readiness, string) {
	if ra == nil {
		return readinessInvalid, "nil_config"
	}

	// Fatal: desired is unspecified (admin bug / invalid config)
	if ra.GetDesiredState() == remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_UNSPECIFIED {
		return readinessInvalid, "desired_state=UNSPECIFIED"
	}

	// Expiration: if set and already expired => fatal
	if ts := ra.GetExpirationTimestamp(); ts != 0 && int64(ts) <= now.Unix() {
		return readinessInvalid, "expired"
	}
	// If expiration not set yet => pending (depends on your flow)
	if ra.GetExpirationTimestamp() == 0 {
		return readinessPending, "expiration_timestamp_missing"
	}

	// Pending until RAP allocates port + endpoint is set
	if ra.GetLocalPort() == 0 {
		return readinessPending, "local_port_missing"
	}
	if strings.TrimSpace(ra.GetProxyHost()) == "" {
		return readinessPending, "proxy_host_missing"
	}

	// Pending until manager fills agent target + auth
	if strings.TrimSpace(ra.GetTargetHost()) == "" || ra.GetTargetPort() == 0 {
		return readinessPending, "target_missing"
	}
	if strings.TrimSpace(ra.GetUser()) == "" {
		return readinessPending, "user_missing"
	}
	if strings.TrimSpace(ra.GetSessionToken()) == "" {
		return readinessPending, "session_token_missing"
	}

	return readinessReady, ""
}

func isNotFoundErr(conf *remoteaccessv1.RemoteAccessConfiguration, err error) bool {
	if conf == nil {
		return true
	}
	return false
}
