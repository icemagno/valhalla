package lrc

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/logger"
)

var logLRC = logger.NewLogger("LRC")

func (a *RTIambassadorImpl) snapshotMemberSession() (*Client, int32, int32, error) {
	a.mu.RLock()
	client := a.client
	fedExecID := a.federationID
	fedID := a.federateID
	a.mu.RUnlock()

	if client == nil || client.IsClosed() {
		return nil, 0, 0, ErrNotConnected
	}
	if fedID == 0 {
		return nil, 0, 0, ErrFederateNotExecutionMember
	}
	return client, fedExecID, fedID, nil
}

// EnableTimeRegulation enables time regulation with the specified lookahead
func (a *RTIambassadorImpl) EnableTimeRegulation(lookahead float64) error {
	client, fedExecID, fedID, err := a.snapshotMemberSession()
	if err != nil {
		return err
	}
	if math.IsNaN(lookahead) || math.IsInf(lookahead, 0) || lookahead < 0 {
		return common.ErrInvalidLookahead
	}

	timeReq := common.TimeFloat64Payload{Time: lookahead}
	payload, err := timeReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgEnableTimeRegulation, uint32(fedExecID), uint32(fedID), payload, 5*time.Second)
	if err != nil {
		return err
	}

	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return common.AsCanonicalError(errPayload.Message)
		}
		return ErrInvalidResponse
	}

	if resp.Header.MsgType != common.NativeMsgTimeRegulationEnabled {
		return ErrInvalidResponse
	}

	a.mu.Lock()
	a.lookahead = lookahead
	a.mu.Unlock()
	return nil
}

// DisableTimeRegulation disables time regulation for the federate
func (a *RTIambassadorImpl) DisableTimeRegulation() error {
	client, fedExecID, fedID, err := a.snapshotMemberSession()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgDisableTimeRegulation, uint32(fedExecID), uint32(fedID), nil, 5*time.Second)
	if err != nil {
		return err
	}
	if err := checkNativeResponse(resp); err != nil {
		return err
	}
	a.mu.Lock()
	a.lookahead = 0
	a.mu.Unlock()
	return nil
}

// EnableTimeConstrained enables time constrained state for the federate
func (a *RTIambassadorImpl) EnableTimeConstrained() error {
	client, fedExecID, fedID, err := a.snapshotMemberSession()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgEnableTimeConstrained, uint32(fedExecID), uint32(fedID), nil, 5*time.Second)
	if err != nil {
		return err
	}

	if resp.Header.MsgType == common.NativeMsgError {
		var errPayload common.ErrorResponsePayload
		if err := errPayload.UnmarshalBinary(resp.Payload); err == nil {
			return common.AsCanonicalError(errPayload.Message)
		}
		return ErrInvalidResponse
	}

	if resp.Header.MsgType != common.NativeMsgTimeConstrainedEnabled {
		return ErrInvalidResponse
	}

	return nil
}

// DisableTimeConstrained disables time constrained state for the federate
func (a *RTIambassadorImpl) DisableTimeConstrained() error {
	client, fedExecID, fedID, err := a.snapshotMemberSession()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgDisableTimeConstrained, uint32(fedExecID), uint32(fedID), nil, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// TimeAdvanceRequest requests advance of logical simulation time
func (a *RTIambassadorImpl) TimeAdvanceRequest(targetTime float64) error {
	client, fedExecID, fedID, err := a.snapshotMemberSession()
	if err != nil {
		return err
	}

	if math.IsNaN(targetTime) || math.IsInf(targetTime, 0) {
		return common.ErrInvalidLogicalTime
	}

	tarReq := common.TimeFloat64Payload{Time: targetTime}
	payload, err := tarReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgTimeAdvanceRequest, uint32(fedExecID), uint32(fedID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// NextMessageRequest requests advance to the next message timestamp or targetTime
func (a *RTIambassadorImpl) NextMessageRequest(targetTime float64) error {
	client, fedExecID, fedID, err := a.snapshotMemberSession()
	if err != nil {
		return err
	}

	if math.IsNaN(targetTime) || math.IsInf(targetTime, 0) {
		return common.ErrInvalidLogicalTime
	}

	nmrReq := common.TimeFloat64Payload{Time: targetTime}
	payload, err := nmrReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgNextMessageRequest, uint32(fedExecID), uint32(fedID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// FlushQueueRequest requests advance with delivery of all queued TSO messages
func (a *RTIambassadorImpl) FlushQueueRequest(targetTime float64) error {
	client, fedExecID, fedID, err := a.snapshotMemberSession()
	if err != nil {
		return err
	}

	if math.IsNaN(targetTime) || math.IsInf(targetTime, 0) {
		return common.ErrInvalidLogicalTime
	}

	flushReq := common.TimeFloat64Payload{Time: targetTime}
	payload, err := flushReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgFlushQueueRequest, uint32(fedExecID), uint32(fedID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// Heartbeat notifies the CRC of federate vitality to prevent anti-stall eviction/quarantine
func (a *RTIambassadorImpl) Heartbeat() error {
	client, fedExecID, fedID, err := a.snapshotMemberSession()
	if err != nil {
		return err
	}

	resp, err := client.SendRequest(common.NativeMsgHeartbeat, uint32(fedExecID), uint32(fedID), nil, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

var timerPool = sync.Pool{
	New: func() interface{} {
		t := time.NewTimer(time.Hour)
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
		return t
	},
}

func acquireTimer(d time.Duration) *time.Timer {
	v := timerPool.Get()
	if v == nil {
		return time.NewTimer(d)
	}
	t := v.(*time.Timer)
	t.Reset(d)
	return t
}

func releaseTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	timerPool.Put(t)
}

// EvokeCallback delivers one pending callback to the FederateAmbassador if available.
// If the callback queue has reached saturation, it returns ErrCallbackQueueSaturated
// to explicitly alert the federate to message loss.
func (a *RTIambassadorImpl) EvokeCallback(timeout time.Duration) (bool, error) {
	a.mu.RLock()
	client := a.client
	fedAmb := a.fedAmb
	a.mu.RUnlock()

	if client == nil {
		return false, ErrNotConnected
	}

	// If the connection is already closed, drain any already-queued callbacks first.
	if client.IsClosed() {
		select {
		case cb := <-client.callbacks:
			a.dispatchCallback(cb, fedAmb)
			if client.IsQueueSaturated() {
				return true, ErrCallbackQueueSaturated
			}
			return true, nil
		default:
			return false, ErrConnectionClosed
		}
	}

	// 1. Non-blocking fast path when timeout <= 0
	if timeout <= 0 {
		select {
		case cb := <-client.callbacks:
			a.dispatchCallback(cb, fedAmb)
			if client.IsQueueSaturated() {
				return true, ErrCallbackQueueSaturated
			}
			return true, nil
		case <-client.closedChan:
			select {
			case cb := <-client.callbacks:
				a.dispatchCallback(cb, fedAmb)
				if client.IsQueueSaturated() {
					return true, ErrCallbackQueueSaturated
				}
				return true, nil
			default:
				return false, ErrConnectionClosed
			}
		default:
			if client.IsQueueSaturated() {
				return false, ErrCallbackQueueSaturated
			}
			return false, nil
		}
	}

	// 2. Immediate dispatch fast path: avoid acquiring any timer if a callback is already available
	select {
	case cb := <-client.callbacks:
		a.dispatchCallback(cb, fedAmb)
		if client.IsQueueSaturated() {
			return true, ErrCallbackQueueSaturated
		}
		return true, nil
	default:
	}

	// 3. Timed wait using pooled reusable timer (zero heap allocations)
	timer := acquireTimer(timeout)
	defer releaseTimer(timer)

	select {
	case cb := <-client.callbacks:
		a.dispatchCallback(cb, fedAmb)
		if client.IsQueueSaturated() {
			return true, ErrCallbackQueueSaturated
		}
		return true, nil
	case <-timer.C:
		if client.IsQueueSaturated() {
			return false, ErrCallbackQueueSaturated
		}
		return false, nil
	case <-client.closedChan:
		// Check if any drained callback remains available in callbacks channel
		select {
		case cb := <-client.callbacks:
			a.dispatchCallback(cb, fedAmb)
			if client.IsQueueSaturated() {
				return true, ErrCallbackQueueSaturated
			}
			return true, nil
		default:
			return false, ErrConnectionClosed
		}
	}
}

// GetCallbackQueueStatus reports the current callback backlog, saturation state, and dropped count
func (a *RTIambassadorImpl) GetCallbackQueueStatus() (length int, saturated bool, dropped uint64) {
	a.mu.RLock()
	client := a.client
	a.mu.RUnlock()

	if client == nil {
		return 0, false, 0
	}
	return client.QueueLength(), client.IsQueueSaturated(), client.DroppedCallbackCount()
}

// ResetCallbackQueueSaturation clears the saturation flag and dropped count on the client queue
func (a *RTIambassadorImpl) ResetCallbackQueueSaturation() {
	a.mu.RLock()
	client := a.client
	a.mu.RUnlock()

	if client != nil {
		client.ResetQueueSaturation()
	}
}

// SetCallbackQueueCapacity dynamically configures the maximum queue limit
func (a *RTIambassadorImpl) SetCallbackQueueCapacity(capacity int) {
	a.mu.RLock()
	client := a.client
	a.mu.RUnlock()

	if client != nil {
		client.SetMaxQueueCapacity(capacity)
	}
}

// EvokeMultipleCallbacks repeatedly processes available callbacks up to maxTimeout
func (a *RTIambassadorImpl) EvokeMultipleCallbacks(minTimeout, maxTimeout time.Duration) error {
	if maxTimeout < 0 {
		maxTimeout = 0
	}
	if minTimeout < 0 {
		minTimeout = 0
	}
	deadline := time.Now().Add(maxTimeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		wait := minTimeout
		if wait > remaining {
			wait = remaining
		}
		evoked, err := a.EvokeCallback(wait)
		if err != nil {
			return err
		}
		if !evoked {
			break
		}
	}
	return nil
}

func (a *RTIambassadorImpl) dispatchCallback(cb callbackItem, fedAmb FederateAmbassador) {
	if fedAmb == nil {
		return
	}

	switch cb.MsgType {
	case common.NativeMsgSyncPointRegSucceeded:
		var succPayload common.SyncPointLabelPayload
		if err := succPayload.UnmarshalBinary(cb.Payload); err != nil {
			logLRC.Warn("failed to decode SyncPointRegSucceeded callback: %v", err)
		} else {
			fedAmb.SynchronizationPointRegistrationSucceeded(succPayload.Label)
		}

	case common.NativeMsgSyncPointRegFailed:
		var failPayload common.SyncPointFailurePayload
		if err := failPayload.UnmarshalBinary(cb.Payload); err == nil {
			fedAmb.SynchronizationPointRegistrationFailed(failPayload.Label, failPayload.Reason)
		} else {
			var labelPayload common.SyncPointLabelPayload
			if err2 := labelPayload.UnmarshalBinary(cb.Payload); err2 == nil {
				fedAmb.SynchronizationPointRegistrationFailed(labelPayload.Label, "synchronization point registration failed")
			} else {
				logLRC.Warn("failed to decode SyncPointRegFailed callback: %v", err)
			}
		}

	case common.NativeMsgAnnounceSyncPoint:
		var annPayload common.RegisterSyncPointPayload
		if err := annPayload.UnmarshalBinary(cb.Payload); err != nil {
			logLRC.Warn("failed to decode AnnounceSyncPoint callback: %v", err)
		} else {
			fedAmb.AnnounceSynchronizationPoint(annPayload.Label, annPayload.Tag)
		}

	case common.NativeMsgFederationSynchronized:
		var syncPayload common.SyncPointLabelPayload
		if err := syncPayload.UnmarshalBinary(cb.Payload); err != nil {
			logLRC.Warn("failed to decode FederationSynchronized callback: %v", err)
		} else {
			fedAmb.FederationSynchronized(syncPayload.Label)
		}

	case common.NativeMsgDiscoverObject:
		var discPayload common.DiscoverObjectInstancePayload
		if err := discPayload.UnmarshalBinary(cb.Payload); err != nil {
			logLRC.Warn("failed to decode DiscoverObject callback: %v", err)
		} else {
			a.getStateTree().UpdateObjectState(discPayload.ObjectHandle, [32]byte{})
			fedAmb.DiscoverObjectInstance(int32(discPayload.ObjectHandle), int32(discPayload.ClassHandle), discPayload.ObjectName)
		}

	case common.NativeMsgRemoveObjectInstance:
		var remPayload common.RemoveObjectInstancePayload
		if err := remPayload.UnmarshalBinary(cb.Payload); err != nil {
			logLRC.Warn("failed to decode RemoveObjectInstance callback: %v", err)
		} else {
			a.getStateTree().RemoveObjectState(remPayload.ObjectHandle)
			fedAmb.RemoveObjectInstance(int32(remPayload.ObjectHandle), remPayload.Tag)
		}

	case common.NativeMsgReflectAttributes:
		var uavPayload common.UpdateAttributesPayload
		if err := uavPayload.UnmarshalBinary(cb.Payload); err != nil {
			logLRC.Warn("failed to decode ReflectAttributes callback: %v", err)
		} else {
			st := a.getStateTree()
			attrs := make(map[int32][]byte, len(uavPayload.Attributes))
			for k, v := range uavPayload.Attributes {
				attrs[int32(k)] = v
				st.UpdateEntityAttribute(uavPayload.ObjectHandle, k, v)
			}
			fedAmb.ReflectAttributeValues(int32(uavPayload.ObjectHandle), attrs, uavPayload.Tag)

			if cb.HasMerkle {
				localRoot := st.GetGlobalRoot()
				if localRoot != cb.MerkleRoot {
					desyncErr := fmt.Errorf("%w: CRC root %x != local root %x", ErrStateDesyncDetected, cb.MerkleRoot, localRoot)
					a.desyncMu.Lock()
					a.lastDesyncErr = desyncErr
					a.desyncMu.Unlock()
					if desyncAmb, ok := fedAmb.(DesyncAwareFederateAmbassador); ok {
						desyncAmb.OnStateDesyncDetected(cb.MerkleRoot, localRoot)
					}
				}
			}
		}

	case common.NativeMsgReceiveInteraction:
		var interPayload common.SendInteractionPayload
		if err := interPayload.UnmarshalBinary(cb.Payload); err != nil {
			logLRC.Warn("failed to decode ReceiveInteraction callback: %v", err)
		} else {
			params := make(map[int32][]byte, len(interPayload.Parameters))
			for k, v := range interPayload.Parameters {
				params[int32(k)] = v
			}
			fedAmb.ReceiveInteraction(int32(interPayload.ClassHandle), params, interPayload.Tag)
		}

	case common.NativeMsgTimeAdvanceGrant:
		var timePayload common.TimeFloat64Payload
		if err := timePayload.UnmarshalBinary(cb.Payload); err != nil {
			logLRC.Warn("failed to decode TimeAdvanceGrant callback: %v", err)
		} else {
			fedAmb.TimeAdvanceGrant(timePayload.Time)
		}
	}
}

