package lrc

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
)

// EnableTimeRegulation enables time regulation with the specified lookahead
func (a *RTIambassadorImpl) EnableTimeRegulation(lookahead float64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}
	if math.IsNaN(lookahead) || math.IsInf(lookahead, 0) || lookahead < 0 {
		return common.ErrInvalidLookahead
	}

	timeReq := common.TimeFloat64Payload{Time: lookahead}
	payload, err := timeReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgEnableTimeRegulation, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
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

	a.lookahead = lookahead
	return nil
}

// DisableTimeRegulation disables time regulation for the federate
func (a *RTIambassadorImpl) DisableTimeRegulation() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	resp, err := a.client.SendRequest(common.NativeMsgDisableTimeRegulation, uint32(a.federationID), uint32(a.federateID), nil, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// EnableTimeConstrained enables time constrained state for the federate
func (a *RTIambassadorImpl) EnableTimeConstrained() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	resp, err := a.client.SendRequest(common.NativeMsgEnableTimeConstrained, uint32(a.federationID), uint32(a.federateID), nil, 5*time.Second)
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
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	resp, err := a.client.SendRequest(common.NativeMsgDisableTimeConstrained, uint32(a.federationID), uint32(a.federateID), nil, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// TimeAdvanceRequest requests advance of logical simulation time
func (a *RTIambassadorImpl) TimeAdvanceRequest(targetTime float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	if math.IsNaN(targetTime) || math.IsInf(targetTime, 0) {
		return common.ErrInvalidLogicalTime
	}

	tarReq := common.TimeFloat64Payload{Time: targetTime}
	payload, err := tarReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgTimeAdvanceRequest, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// NextMessageRequest requests advance to the next message timestamp or targetTime
func (a *RTIambassadorImpl) NextMessageRequest(targetTime float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	if math.IsNaN(targetTime) || math.IsInf(targetTime, 0) {
		return common.ErrInvalidLogicalTime
	}

	nmrReq := common.TimeFloat64Payload{Time: targetTime}
	payload, err := nmrReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgNextMessageRequest, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// FlushQueueRequest requests advance with delivery of all queued TSO messages
func (a *RTIambassadorImpl) FlushQueueRequest(targetTime float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	if math.IsNaN(targetTime) || math.IsInf(targetTime, 0) {
		return common.ErrInvalidLogicalTime
	}

	flushReq := common.TimeFloat64Payload{Time: targetTime}
	payload, err := flushReq.MarshalBinary()
	if err != nil {
		return err
	}

	resp, err := a.client.SendRequest(common.NativeMsgFlushQueueRequest, uint32(a.federationID), uint32(a.federateID), payload, 5*time.Second)
	if err != nil {
		return err
	}
	return checkNativeResponse(resp)
}

// Heartbeat notifies the CRC of federate vitality to prevent anti-stall eviction/quarantine
func (a *RTIambassadorImpl) Heartbeat() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.client == nil || a.client.IsClosed() {
		return ErrNotConnected
	}
	if a.federateID == 0 {
		return ErrFederateNotExecutionMember
	}

	resp, err := a.client.SendRequest(common.NativeMsgHeartbeat, uint32(a.federationID), uint32(a.federateID), nil, 5*time.Second)
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

	if client == nil || client.IsClosed() {
		return false, ErrNotConnected
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
	deadline := time.Now().Add(maxTimeout)
	for time.Now().Before(deadline) {
		evoked, err := a.EvokeCallback(minTimeout)
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
		if err := succPayload.UnmarshalBinary(cb.Payload); err == nil {
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
			}
		}

	case common.NativeMsgAnnounceSyncPoint:
		var annPayload common.RegisterSyncPointPayload
		if err := annPayload.UnmarshalBinary(cb.Payload); err == nil {
			fedAmb.AnnounceSynchronizationPoint(annPayload.Label, annPayload.Tag)
		}

	case common.NativeMsgFederationSynchronized:
		var syncPayload common.SyncPointLabelPayload
		if err := syncPayload.UnmarshalBinary(cb.Payload); err == nil {
			fedAmb.FederationSynchronized(syncPayload.Label)
		}

	case common.NativeMsgDiscoverObject:
		var discPayload common.DiscoverObjectInstancePayload
		if err := discPayload.UnmarshalBinary(cb.Payload); err == nil {
			a.stateTree.UpdateObjectState(discPayload.ObjectHandle, [32]byte{})
			fedAmb.DiscoverObjectInstance(int32(discPayload.ObjectHandle), int32(discPayload.ClassHandle), discPayload.ObjectName)
		}

	case common.NativeMsgRemoveObjectInstance:
		var remPayload common.RemoveObjectInstancePayload
		if err := remPayload.UnmarshalBinary(cb.Payload); err == nil {
			a.stateTree.RemoveObjectState(remPayload.ObjectHandle)
			fedAmb.RemoveObjectInstance(int32(remPayload.ObjectHandle), remPayload.Tag)
		}

	case common.NativeMsgReflectAttributes:
		var uavPayload common.UpdateAttributesPayload
		if err := uavPayload.UnmarshalBinary(cb.Payload); err == nil {
			attrs := make(map[int32][]byte, len(uavPayload.Attributes))
			for k, v := range uavPayload.Attributes {
				attrs[int32(k)] = v
				a.stateTree.UpdateEntityAttribute(uavPayload.ObjectHandle, k, v)
			}
			fedAmb.ReflectAttributeValues(int32(uavPayload.ObjectHandle), attrs, uavPayload.Tag)

			if cb.HasMerkle {
				localRoot := a.stateTree.GetGlobalRoot()
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
		if err := interPayload.UnmarshalBinary(cb.Payload); err == nil {
			params := make(map[int32][]byte, len(interPayload.Parameters))
			for k, v := range interPayload.Parameters {
				params[int32(k)] = v
			}
			fedAmb.ReceiveInteraction(int32(interPayload.ClassHandle), params, interPayload.Tag)
		}

	case common.NativeMsgTimeAdvanceGrant:
		var timePayload common.TimeFloat64Payload
		if err := timePayload.UnmarshalBinary(cb.Payload); err == nil {
			fedAmb.TimeAdvanceGrant(timePayload.Time)
		}
	}
}

