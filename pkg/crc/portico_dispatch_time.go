package crc

import (
	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

// handleSyncPoints handles Portico synchronization points.
func (ctx *porticoContext) handleSyncPoints(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgRegisterSyncPoint:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		var label string
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if l, ok := obj.Get("label").(string); ok && l != "" {
				label = l
			}
		}
		if label == "" {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: empty sync point label"); err != nil {
				return true, err
			}
			return true, nil
		}

		_, err := ctx.currentFed.RegisterSyncPoint(label, nil, ctx.federateID)
		if err != nil {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
				return true, err
			}
			return true, nil
		}

		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Synchronization point '%s' registered by federate [%d]", label, ctx.federateID)

		if err := ctx.currentFed.BroadcastSyncPointAnnounce(label, nil); err != nil {
			logCRCPortico.Warn("failed to broadcast sync point announce: %v", err)
		}
		return true, nil

	case wire.MsgAchieveSyncPoint:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		var label string
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if l, ok := obj.Get("label").(string); ok && l != "" {
				label = l
			}
		}
		if label == "" {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: empty sync point label"); err != nil {
				return true, err
			}
			return true, nil
		}

		isSynced, err := ctx.currentFed.AchieveSyncPoint(label, ctx.federateID)
		if err != nil {
			logCRCPortico.Error("AchieveSyncPoint error: %v", err)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
				return true, err
			}
			return true, nil
		}

		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] achieved synchronization point '%s'", ctx.federateID, label)

		if isSynced {
			logCRCPortico.Info(">>> FEDERATION SYNCHRONIZED at point '%s'!", label)
			if err := ctx.currentFed.BroadcastFederationSynchronized(label); err != nil {
				logCRCPortico.Warn("failed to broadcast federation synchronized: %v", err)
			}
		}
		return true, nil
	}
	return false, nil
}

// handleTimeManagement handles Portico time advancement services via core.TimeManager.
func (ctx *porticoContext) handleTimeManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgEnableTimeRegulation:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		lookahead := 0.0
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if la, ok := obj.Get("lookahead").(float64); ok {
				lookahead = la
			}
		}
		if err := ctx.currentFed.TimeManager().EnableTimeRegulation(common.FederateHandle(ctx.federateID), lookahead); err != nil {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		fedStatus, _ := ctx.currentFed.TimeManager().GetStatus(common.FederateHandle(ctx.federateID))
		respPayload := wire.BuildDoubleResponsePayload(fedStatus.CurrentTime)
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] enabled time regulation at time %f (lookahead=%f)", ctx.federateID, fedStatus.CurrentTime, lookahead)
		return true, nil

	case wire.MsgEnableTimeConstrained:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.TimeManager().EnableTimeConstrained(common.FederateHandle(ctx.federateID)); err != nil {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] enabled time constrained", ctx.federateID)
		return true, nil

	case wire.MsgTimeAdvanceRequest, wire.MsgNextEventRequest, wire.MsgFlushQueueRequest:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		var requestedTime float64
		hasTime := false
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if t, ok := obj.Get("time").(float64); ok {
				requestedTime = t
				hasTime = true
			}
		}
		if !hasTime {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InvalidLogicalTime"); err != nil {
				return true, err
			}
			return true, nil
		}

		var (
			grants []core.TimeGrantEvent
			err    error
		)
		tm := ctx.currentFed.TimeManager()
		fedHandle := common.FederateHandle(ctx.federateID)
		switch hdr.MessageType {
		case wire.MsgTimeAdvanceRequest:
			grants, err = tm.TimeAdvanceRequest(fedHandle, requestedTime)
		case wire.MsgNextEventRequest:
			grants, err = tm.NextMessageRequest(fedHandle, requestedTime)
		case wire.MsgFlushQueueRequest:
			grants, err = tm.FlushQueueRequest(fedHandle, requestedTime)
		}
		if err != nil {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}

		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		if len(grants) > 0 {
			dispatchTimeGrants(ctx.currentFed, grants)
		}
		return true, nil
	}
	return false, nil
}

// handleSaveRestoreManagement explicitly rejects Portico save/restore until persistence is implemented.
// NOT IMPLEMENTED: Planned for future release.
func (ctx *porticoContext) handleSaveRestoreManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgSaveRequest, wire.MsgSaveBegun, wire.MsgSaveComplete, wire.MsgRestoreRequest, wire.MsgRestoreComplete:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "NOT IMPLEMENTED: Federation Save/Restore is not implemented in this release"); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}
