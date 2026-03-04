// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package alivemgr implements the availability manager for tracking host heartbeats.
package alivemgr

import (
	"flag"
	"sync"
	"time"

	"google.golang.org/grpc/codes"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_util "github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	util "github.com/open-edge-platform/infra-managers/host/pkg/utils"
)

var zlog = logging.GetLogger("HostManagerAlvMgr")

const (
	defaultBaseTimeDuration  = 10
	defaultTimeoutTimes      = 3
	defaultLoseConnHostsChan = 10
)

var (
	baseTimeDuration = flag.Int64(
		"baseTimeDuration",
		defaultBaseTimeDuration,
		"Flag to set default base time out value of node agent heartbeat, the unit is seconds.",
	)
	timeoutTimes = flag.Int(
		"timeoutTimes",
		defaultTimeoutTimes,
		"Flag to set default time out times of node agent heartbeat.",
	)
	dynamicTimeOut = flag.Bool(
		"dynamicTimeOut",
		false,
		"Flag to enable dynamic time out based on runtime heartbeat duration.",
	)
	onceOverflowChecker sync.Once
	aliveTimeout        int64
)

var alvMgr = aliverMgr{
	hostHeartbeatMap:  sync.Map{},
	loseConnHostsChan: make(chan util.TenantIDResourceIDTuple, defaultLoseConnHostsChan),
}

type aliverMgr struct {
	hostHeartbeatMap  sync.Map
	loseConnHostsChan chan util.TenantIDResourceIDTuple
}

// StartAlvMgr starts the availability manager for tracking host heartbeats.
func StartAlvMgr(termChan chan bool) chan util.TenantIDResourceIDTuple {
	go func(termChan chan bool) {
		zlog.InfraSec().Info().Msg("Start Availability Manager")
		initAlvMgr(termChan)
	}(termChan)

	return alvMgr.loseConnHostsChan
}

func initAlvMgr(termChan chan bool) {
	zlog.InfraSec().Info().Msg("initial Availability Manager.")
	for {
		zlog.Debug().Msgf("Alv Mgr will checkout timers after %s.", time.Duration(*baseTimeDuration)*time.Second)
		select {
		case <-termChan:
			return
		default:
			alvMgr.hostHeartbeatMap.Range(func(key any, value any) bool {
				tenantRes, ok := key.(util.TenantIDResourceIDTuple)
				if !ok {
					zlog.Error().Msgf("unexpected type for TenantIDResourceIDTuple: %T", key)
					return true
				}
				hostHb, ok := value.(*heartbeat)
				if !ok {
					zlog.Error().Msgf("unexpected type for heartbeat: %T", value)
					return true
				}
				go checkHeartbeat(tenantRes, hostHb, alvMgr.loseConnHostsChan)

				return true
			})
		}

		time.Sleep(time.Duration(*baseTimeDuration) * time.Second)
	}
}

type heartbeat struct {
	lastTimestamp int64
	baseTimeOut   int64
	timeout       time.Duration // int64
	timer         *time.Timer
	lock          sync.Mutex
}

func newheartbeat() *heartbeat {
	// Avoid logging overflow error multiple times for the alive timeout
	onceOverflowChecker.Do(func() {
		err := error(nil)
		aliveTimeout, err = inv_util.MulInt64(int64(*timeoutTimes), *baseTimeDuration)
		if err != nil {
			zlog.InfraSec().Err(err).Msgf("continuing with default timeout values")
			// We assume default values do not cause overflow
			aliveTimeout = defaultTimeoutTimes * defaultBaseTimeDuration
		}
	})

	timeoutDuration := time.Duration(aliveTimeout) * time.Second
	return &heartbeat{
		lastTimestamp: time.Now().UTC().Unix(),
		baseTimeOut:   *baseTimeDuration,
		timeout:       timeoutDuration,
		timer:         time.NewTimer(timeoutDuration),
		lock:          sync.Mutex{},
	}
}

func checkHeartbeat(key util.TenantIDResourceIDTuple, hostHb *heartbeat, timeoutChan chan util.TenantIDResourceIDTuple) {
	if hostHb == nil {
		return
	}
	// Avoid data race between the check heartbeat and the reset timer
	hostHb.lock.Lock()
	defer hostHb.lock.Unlock()
	if hostHb.timer == nil {
		return
	}
	select {
	default:
		zlog.Debug().Msgf("Check heartbeat of %s, the timer is not time out.", key)
	case <-hostHb.timer.C:
		zlog.Info().Msgf("%s lost heartbeat!", key)
		hostHb.timer = nil
		timeoutChan <- key
	}
}

func (hb *heartbeat) resetTimer() {
	hb.updateDuration()

	// Avoid data race between the reset timer and the check heartbeat
	hb.lock.Lock()
	defer hb.lock.Unlock()
	zlog.Debug().Msgf("Debug resetTimer %v, %s.", hb.timer, hb.timeout)
	if hb.timer == nil {
		zlog.Debug().Msgf("Create new timer with timeout %s.", hb.timeout)
		hb.timer = time.NewTimer(hb.timeout)
	} else {
		zlog.Debug().Msgf("Reset timer %v with timeout %s.", hb.timer, hb.timeout)
		hb.timer.Reset(hb.timeout)
	}
}

func (hb *heartbeat) updateTimeStamp() {
	hb.lastTimestamp = time.Now().UTC().Unix()
}

func (hb *heartbeat) updateDuration() {
	// Placeholder for dynamic timeout
	if *dynamicTimeOut {
		zlog.Debug().Msg("Dynamic timeout is unimplemented")
	}
}

// UpdateHostHeartBeat updates the heartbeat timestamp for a host.
func UpdateHostHeartBeat(host *computev1.HostResource) error {
	hbk := util.NewTenantIDResourceIDTupleFromHost(host)
	value, has := alvMgr.hostHeartbeatMap.Load(hbk)

	if has {
		// Based on the mechanism of sync.Map.Load,
		// hostHeartbeat will be an un-nil heartbeat pointer in this condition.
		hostHeartbeat, ok := value.(*heartbeat)
		if !ok {
			zlog.InfraSec().InfraError("casting heartbeat pointer has failed").Msg("UpdateHostHeartBeat")
			return errors.Errorfc(codes.Internal, "casting heartbeat pointer has failed")
		}

		hostHeartbeat.resetTimer()
		hostHeartbeat.updateTimeStamp()
	} else {
		alvMgr.hostHeartbeatMap.Store(hbk, newheartbeat())
	}

	return nil
}

// ForgetHost removes a host from the heartbeat tracking map.
func ForgetHost(host *computev1.HostResource) {
	hbk := util.NewTenantIDResourceIDTupleFromHost(host)
	alvMgr.hostHeartbeatMap.Delete(hbk)
	zlog.Debug().Msgf("Host %s has been removed from the heartbeat list", hbk)
}

// SyncHosts receives a desired list of hosts and removes hosts that are not in the desired list from the heartbeat map.
func SyncHosts(desiredHostsList []util.TenantIDResourceIDTuple) {
	zlog.Debug().Msgf("Syncing desired host list with heartbeat map state")

	// converting to map should guarantee O(n) instead of O(n^2)
	hbksMap := map[util.TenantIDResourceIDTuple]struct{}{}
	for _, host := range desiredHostsList {
		hbksMap[host] = struct{}{}
	}

	hbksToRemove := make([]util.TenantIDResourceIDTuple, 0)
	alvMgr.hostHeartbeatMap.Range(func(key, _ any) bool {
		zlog.Debug().Msgf("host %s", key)
		hbk, ok := key.(util.TenantIDResourceIDTuple)
		if !ok {
			return true
		}
		if _, exists := hbksMap[hbk]; !exists {
			zlog.Debug().Msgf("Host %s doesn't exist in desired host lists, removing from heartbeat map",
				hbk)
			hbksToRemove = append(hbksToRemove, hbk)
		}
		return true
	})

	for _, hbk := range hbksToRemove {
		alvMgr.hostHeartbeatMap.Delete(hbk)
	}
}

// IsHostTracked checks if host is tracked by availability manager. Currently, used for testing only.
func IsHostTracked(host *computev1.HostResource) bool {
	_, exists := alvMgr.hostHeartbeatMap.Load(util.NewTenantIDResourceIDTupleFromHost(host))
	return exists
}

// GetHostHeartBeat retrieves the heartbeat information for a host.
func GetHostHeartBeat(host *computev1.HostResource) (bool, error) {
	hbk := util.NewTenantIDResourceIDTupleFromHost(host)
	value, has := alvMgr.hostHeartbeatMap.Load(hbk)

	if has {
		// Based on the mechanism of sync.Map.Load,
		// hostHeartbeat will be an un-nil heartbeat pointer in this condition.
		hostHeartbeat, ok := value.(*heartbeat)
		if !ok {
			zlog.InfraSec().InfraError("casting heartbeat pointer has failed").Msg("UpdateHostHeartBeat")
			return false, errors.Errorfc(codes.Internal, "casting heartbeat pointer has failed")
		}

		if hostHeartbeat.timer != nil {
			zlog.Debug().Msgf("Check heartbeat of %s, the timer is not time out.", hbk)
			return true, nil
		}
		return false, nil
	}

	return false, errors.Errorfc(codes.Internal, "host has no heartbeat")
}
