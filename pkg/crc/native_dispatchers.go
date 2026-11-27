package crc

import (
	"errors"

	"github.com/icemagno/valhalla/pkg/common"
)

type nativeContext struct {
	session    *NativeSession
	fedManager *FederationManager
	currentFed *FederationExecution
	federateID int32
}

// handleFederationManagement handles IEEE 1516-2025 Clause 4 services
func (ctx *nativeContext) handleFederationManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgCreateFederation:
		var req common.FederationExecutionPayload
		if err := req.UnmarshalBinary(payload); err != nil || req.FederationName == "" {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid federation name")); err != nil {
				return true, err
			}
			return true, nil
		}
		_, err := ctx.fedManager.CreateFederationWithFOMs(req.FederationName, req.FOMModules)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 409, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respHdr := NativeHeader{
			Magic:   NativeHeaderMagic,
			Version: NativeProtocolVer,
			Flags:   0x01,
			MsgType: NativeMsgSuccessOK,
			ReqID:   hdr.ReqID,
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Federation [%s] created successfully (foms=%v)", req.FederationName, req.FOMModules)
		return true, nil

	case NativeMsgDestroyFederation:
		var req common.FederationExecutionPayload
		if err := req.UnmarshalBinary(payload); err != nil || req.FederationName == "" {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid federation name")); err != nil {
				return true, err
			}
			return true, nil
		}
		err := ctx.fedManager.DestroyFederation(req.FederationName)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 409, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respHdr := NativeHeader{
			Magic:   NativeHeaderMagic,
			Version: NativeProtocolVer,
			Flags:   0x01,
			MsgType: NativeMsgSuccessOK,
			ReqID:   hdr.ReqID,
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Federation [%s] destroyed successfully", req.FederationName)
		return true, nil

	case NativeMsgJoin:
		var joinReq common.JoinFederationPayload
		if err := joinReq.UnmarshalBinary(payload); err != nil || joinReq.FederateName == "" {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid join request: federateName is mandatory")); err != nil {
				return true, err
			}
			return true, nil
		}

		targetFedName := joinReq.FederationName
		var fedExec *FederationExecution
		var exists bool

		if targetFedName == "" || targetFedName == "*" || targetFedName == "auto" {
			fedExec, exists = ctx.fedManager.GetFirstFederation()
		} else {
			fedExec, exists = ctx.fedManager.GetFederation(targetFedName)
		}

		if !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, common.ErrFederationExecutionDoesNotExist); err != nil {
				return true, err
			}
			return true, nil
		}

		if ctx.currentFed != nil && ctx.federateID > 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 409, common.ErrFederateAlreadyExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}

		fed, err := fedExec.Join(joinReq.FederateName, joinReq.FederateType, ProtocolNative, ctx.session)
		if err != nil {
			logCRCNative.Error("Join error for %s: %v", joinReq.FederateName, err)
			if err := sendNativeError(ctx.session, hdr.ReqID, 409, err); err != nil {
				return true, err
			}
			return true, nil
		}

		ctx.currentFed = fedExec
		ctx.federateID = fed.ID
		ctx.session.SetFederateID(ctx.federateID)
		ctx.session.SetFederationID(ctx.currentFed.Handle)

		respData, err := common.JoinFederationResponsePayload{
			FederateHandle:   common.FederateHandle(ctx.federateID),
			FederationHandle: ctx.currentFed.Handle,
		}.MarshalBinary()
		if err != nil {
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, 500, err); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}

		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			PayloadLen: uint32(len(respData)),
			MsgType:    NativeMsgJoinSuccess,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respData, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Federate [%s] joined federation [%s] with handle [%d]", joinReq.FederateName, joinReq.FederationName, ctx.federateID)
		return true, nil

	case NativeMsgResign:
		var fedHandle uint32
		if ctx.currentFed != nil {
			fedHandle = uint32(ctx.currentFed.Handle)
			if ctx.federateID > 0 {
				if _, err := ctx.currentFed.Resign(ctx.federateID); err != nil {
					logCRCNative.Warn("failed to resign federate %d: %v", ctx.federateID, err)
				}
			}
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      fedHandle,
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Federate [%d] resigned gracefully", ctx.federateID)
		ctx.currentFed = nil
		ctx.federateID = -1
		return true, nil

	case NativeMsgRegisterSyncPoint:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var syncReq common.RegisterSyncPointPayload
		if err := syncReq.UnmarshalBinary(payload); err != nil || syncReq.Label == "" {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid synchronization point label")); err != nil {
				return true, err
			}
			return true, nil
		}
		_, err := ctx.currentFed.RegisterSyncPoint(syncReq.Label, syncReq.Tag, ctx.federateID)
		if err != nil {
			if failPayload, merr := (common.SyncPointFailurePayload{Label: syncReq.Label, Reason: err.Error()}).MarshalBinary(); merr == nil {
				_ = ctx.session.SendNotification(common.NativeMsgSyncPointRegFailed, failPayload)
			}
			if err := sendNativeError(ctx.session, hdr.ReqID, 409, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		if succPayload, merr := (common.SyncPointLabelPayload{Label: syncReq.Label}).MarshalBinary(); merr == nil {
			if err := ctx.session.SendNotification(common.NativeMsgSyncPointRegSucceeded, succPayload); err != nil {
				logCRCNative.Warn("failed to send SyncPointRegSucceeded to federate %d: %v", ctx.federateID, err)
			}
		}
		logCRCNative.Info("Federate [%d] registered synchronization point '%s'", ctx.federateID, syncReq.Label)
		if err := ctx.currentFed.BroadcastSyncPointAnnounce(syncReq.Label, syncReq.Tag); err != nil {
			logCRCNative.Warn("failed to broadcast sync point announce: %v", err)
		}
		return true, nil

	case NativeMsgAchieveSyncPoint:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var labelReq common.SyncPointLabelPayload
		if err := labelReq.UnmarshalBinary(payload); err != nil || labelReq.Label == "" {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid synchronization point label")); err != nil {
				return true, err
			}
			return true, nil
		}
		isSynced, err := ctx.currentFed.AchieveSyncPoint(labelReq.Label, ctx.federateID)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Federate [%d] achieved synchronization point '%s'", ctx.federateID, labelReq.Label)

		if isSynced {
			logCRCNative.Info(">>> FEDERATION SYNCHRONIZED at point '%s'!", labelReq.Label)
			if err := ctx.currentFed.BroadcastFederationSynchronized(labelReq.Label); err != nil {
				logCRCNative.Warn("failed to broadcast federation synchronized: %v", err)
			}
		}
		return true, nil
	}
	return false, nil
}

