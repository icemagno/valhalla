package crc

import (
	"encoding/binary"
	"errors"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
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

		respData, _ := common.JoinFederationResponsePayload{
			FederateHandle:   common.FederateHandle(ctx.federateID),
			FederationHandle: ctx.currentFed.Handle,
		}.MarshalBinary()

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

// handleDeclarationManagement handles IEEE 1516-2025 Clause 5 services
func (ctx *nativeContext) handleDeclarationManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgPublishObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var pubReq common.PublishObjectClassPayload
		if err := pubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid publish object class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		err := ctx.currentFed.Router().PublishObjectClass(common.FederateHandle(ctx.federateID), pubReq.ClassHandle, pubReq.AttributeHandles)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		return true, nil

	case NativeMsgUnpublishObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var unpubReq common.PublishObjectClassPayload
		if err := unpubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid unpublish object class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.Router().UnpublishObjectClass(common.FederateHandle(ctx.federateID), unpubReq.ClassHandle); err != nil {
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
				return true, sendErr
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
		return true, nil

	case NativeMsgSubscribeObjectClassAttrs:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var subReq common.SubscribeObjectClassPayload
		if err := subReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid subscribe object class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		err := ctx.currentFed.Router().SubscribeObjectClass(common.FederateHandle(ctx.federateID), subReq.ClassHandle, subReq.AttributeHandles, subReq.Active)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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

		// Catch-up Discovery: notify the subscriber about all existing instances matching the hierarchy
		if subReq.Active {
			for _, obj := range ctx.currentFed.GetAllObjects() {
				if ctx.currentFed.OMT().IsSubclassOfObject(common.ObjectClassHandle(obj.ClassHandle), subReq.ClassHandle) {
					discPayload, _ := common.DiscoverObjectInstancePayload{
						ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
						ClassHandle:  common.ObjectClassHandle(obj.ClassHandle),
						ObjectName:   obj.Name,
					}.MarshalBinary()
					if err := ctx.session.SendNotification(NativeMsgDiscoverObject, discPayload); err != nil {
						logCRCNative.Warn("failed to send DiscoverObject to federate %d: %v", ctx.federateID, err)
						return true, err
					}
				}
			}
		}
		return true, nil

	case NativeMsgUnsubscribeObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var unsubReq common.PublishObjectClassPayload
		if err := unsubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid unsubscribe object class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.Router().UnsubscribeObjectClass(common.FederateHandle(ctx.federateID), unsubReq.ClassHandle); err != nil {
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
				return true, sendErr
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
		return true, nil

	case NativeMsgPublishInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var pubReq common.InteractionClassPayload
		if err := pubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid publish interaction class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		err := ctx.currentFed.Router().PublishInteractionClass(common.FederateHandle(ctx.federateID), pubReq.ClassHandle)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		return true, nil

	case NativeMsgUnpublishInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var unpubReq common.InteractionClassPayload
		if err := unpubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid unpublish interaction class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.Router().UnpublishInteractionClass(common.FederateHandle(ctx.federateID), unpubReq.ClassHandle); err != nil {
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
				return true, sendErr
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
		return true, nil

	case NativeMsgSubscribeInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var subReq common.SubscribeInteractionClassPayload
		if err := subReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid subscribe interaction class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		err := ctx.currentFed.Router().SubscribeInteractionClass(common.FederateHandle(ctx.federateID), subReq.ClassHandle, subReq.Active)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		return true, nil

	case NativeMsgUnsubscribeInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var unsubReq common.InteractionClassPayload
		if err := unsubReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid unsubscribe interaction class payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.Router().UnsubscribeInteractionClass(common.FederateHandle(ctx.federateID), unsubReq.ClassHandle); err != nil {
			if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
				return true, sendErr
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
		return true, nil
	}
	return false, nil
}

// handleObjectManagement handles IEEE 1516-2025 Clause 6 services
func (ctx *nativeContext) handleObjectManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgRegisterObject:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var regReq common.RegisterObjectInstancePayload
		if err := regReq.UnmarshalBinary(payload); err != nil || regReq.ClassHandle == 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid register object request")); err != nil {
				return true, err
			}
			return true, nil
		}
		if _, exists := ctx.currentFed.OMT().GetObjectClass(regReq.ClassHandle); !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, common.ErrObjectClassNotDefined); err != nil {
				return true, err
			}
			return true, nil
		}
		obj, err := ctx.currentFed.RegisterAndBroadcastObject(int32(regReq.ClassHandle), regReq.ObjectName, ctx.federateID)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 409, err); err != nil {
				return true, err
			}
			return true, nil
		}

		var respPayload [4]byte
		binary.LittleEndian.PutUint32(respPayload[0:4], uint32(obj.Handle))

		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			PayloadLen: 4,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload[:], ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Object registered: handle=%d, class=%d, name=%s", obj.Handle, regReq.ClassHandle, obj.Name)
		return true, nil

	case NativeMsgDeleteObjectInstance:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var delReq common.DeleteObjectInstancePayload
		if err := delReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid delete object instance payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		obj, err := ctx.currentFed.DeleteAndBroadcastObject(int32(delReq.ObjectHandle), ctx.federateID, delReq.Tag)
		if err != nil {
			if errors.Is(err, common.ErrObjectInstanceNotKnown) {
				if err := sendNativeError(ctx.session, hdr.ReqID, 404, err); err != nil {
					return true, err
				}
			} else if errors.Is(err, common.ErrDeletePrivilegeNotHeld) {
				if err := sendNativeError(ctx.session, hdr.ReqID, 403, err); err != nil {
					return true, err
				}
			} else {
				if err := sendNativeError(ctx.session, hdr.ReqID, 500, err); err != nil {
					return true, err
				}
			}
			return true, nil
		}

		var globalRoot [32]byte
		respFlags := common.FlagNativeSSZ
		if ctx.currentFed.IsMerkleAuditEnabled() {
			globalRoot = ctx.currentFed.StateTree().GetGlobalRoot()
			respFlags |= common.FlagNativeMerkle
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      respFlags,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			MerkleRoot: globalRoot,
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Object [%d] '%s' deleted by federate [%d]", obj.Handle, obj.Name, ctx.federateID)
		return true, nil

	case NativeMsgUpdateAttributes:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var uavReq common.UpdateAttributesPayload
		if err := uavReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid update attributes payload")); err != nil {
				return true, err
			}
			return true, nil
		}

		globalRoot, err := ctx.currentFed.UpdateAndRouteAttributes(
			ctx.federateID,
			uavReq.ObjectHandle,
			uavReq.Attributes,
			uavReq.Tag,
			uavReq.HasTime,
			uavReq.Time,
			nil,
			hdr.MerkleRoot,
		)
		if err != nil {
			if errors.Is(err, common.ErrObjectInstanceNotKnown) {
				if sendErr := sendNativeError(ctx.session, hdr.ReqID, 404, err); sendErr != nil {
					return true, sendErr
				}
			} else if errors.Is(err, core.ErrNotAttributeOwner) {
				if sendErr := sendNativeError(ctx.session, hdr.ReqID, 403, err); sendErr != nil {
					return true, sendErr
				}
			} else {
				logCRCNative.Warn("failed to update attributes for object %d: %v", uavReq.ObjectHandle, err)
				if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
					return true, sendErr
				}
			}
			return true, nil
		}

		respFlags := common.FlagNativeSSZ
		if ctx.currentFed.IsMerkleAuditEnabled() {
			respFlags |= common.FlagNativeMerkle
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      respFlags,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			MerkleRoot: globalRoot,
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case NativeMsgSendInteraction:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var interReq common.SendInteractionPayload
		if err := interReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid send interaction payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if _, exists := ctx.currentFed.OMT().GetInteractionClass(interReq.ClassHandle); !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, common.ErrInteractionClassNotDefined); err != nil {
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

		if err := ctx.currentFed.RouteInteraction(ctx.federateID, interReq.ClassHandle, interReq.Parameters, interReq.Tag, interReq.HasTime, interReq.Time); err != nil {
			logCRCNative.Warn("failed to route interaction %d: %v", interReq.ClassHandle, err)
		}
		return true, nil
	}
	return false, nil
}

// handleOwnershipManagement handles IEEE 1516-2025 Clause 7 services
func (ctx *nativeContext) handleOwnershipManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgDivestOwnership:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.DivestOwnershipPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid divest ownership payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		attrs := convertToAttrHandles(req.Attributes)
		if err := ctx.currentFed.OwnershipManager().UnconditionalDivestiture(common.ObjectInstanceHandle(req.ObjectHandle), attrs, common.FederateHandle(ctx.federateID)); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		return true, nil

	case NativeMsgAcquireOwnership:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.AcquireOwnershipPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid acquire ownership payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		attrs := convertToAttrHandles(req.Attributes)
		if err := ctx.currentFed.OwnershipManager().AcquireAttributeOwnership(common.ObjectInstanceHandle(req.ObjectHandle), attrs, common.FederateHandle(ctx.federateID)); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		return true, nil

	case NativeMsgQueryAttributeOwnership:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.QueryOwnershipPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid query ownership payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		owner, isOwned := ctx.currentFed.OwnershipManager().QueryAttributeOwnership(common.ObjectInstanceHandle(req.ObjectHandle), common.AttributeHandle(req.AttributeHandle))
		respPayload, _ := common.QueryOwnershipResponsePayload{
			ObjectHandle:    req.ObjectHandle,
			AttributeHandle: req.AttributeHandle,
			OwnerHandle:     uint32(owner),
			IsOwned:         isOwned,
		}.MarshalBinary()

		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			PayloadLen: uint32(len(respPayload)),
			MsgType:    NativeMsgAttributeOwnershipResponse,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

// handleTimeManagement handles IEEE 1516-2025 Clause 8 services
func (ctx *nativeContext) handleTimeManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgEnableTimeRegulation:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var timeReq common.TimeFloat64Payload
		if err := timeReq.UnmarshalBinary(payload); err != nil || timeReq.Time < 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, common.ErrInvalidLookahead); err != nil {
				return true, err
			}
			return true, nil
		}
		fed, exists := ctx.currentFed.GetFederate(ctx.federateID)
		if !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, common.ErrFederateNotFound); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.TimeManager().EnableTimeRegulation(common.FederateHandle(ctx.federateID), timeReq.Time); err != nil {
			if errors.Is(err, common.ErrTimeRegulationAlreadyEnabled) {
				if err := sendNativeError(ctx.session, hdr.ReqID, 409, err); err != nil {
					return true, err
				}
			} else {
				if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
					return true, err
				}
			}
			return true, nil
		}
		fed.IsRegulating = true
		fed.Lookahead = timeReq.Time

		status, _ := ctx.currentFed.TimeManager().GetStatus(common.FederateHandle(ctx.federateID))
		respData, _ := common.TimeFloat64Payload{Time: status.CurrentTime}.MarshalBinary()
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			PayloadLen: uint32(len(respData)),
			MsgType:    NativeMsgTimeRegulationEnabled,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respData, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Time regulation enabled for federate [%d], lookahead=%f", ctx.federateID, fed.Lookahead)
		return true, nil

	case NativeMsgDisableTimeRegulation:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		grants, err := ctx.currentFed.TimeManager().DisableTimeRegulation(common.FederateHandle(ctx.federateID))
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
				return true, err
			}
			return true, nil
		}
		if fed, exists := ctx.currentFed.GetFederate(ctx.federateID); exists {
			fed.IsRegulating = false
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
		dispatchTimeGrants(ctx.currentFed, grants)
		return true, nil

	case NativeMsgEnableTimeConstrained:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		fed, exists := ctx.currentFed.GetFederate(ctx.federateID)
		if !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, common.ErrFederateNotFound); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.TimeManager().EnableTimeConstrained(common.FederateHandle(ctx.federateID)); err != nil {
			if errors.Is(err, common.ErrTimeConstrainedAlreadyEnabled) {
				if err := sendNativeError(ctx.session, hdr.ReqID, 409, err); err != nil {
					return true, err
				}
			} else {
				if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
					return true, err
				}
			}
			return true, nil
		}
		fed.IsConstrained = true

		status, _ := ctx.currentFed.TimeManager().GetStatus(common.FederateHandle(ctx.federateID))
		respData, _ := common.TimeFloat64Payload{Time: status.CurrentTime}.MarshalBinary()
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			PayloadLen: uint32(len(respData)),
			MsgType:    NativeMsgTimeConstrainedEnabled,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respData, ctx.federateID); err != nil {
			return true, err
		}
		logCRCNative.Info("Time constrained enabled for federate [%d]", ctx.federateID)
		return true, nil

	case NativeMsgDisableTimeConstrained:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		grants, err := ctx.currentFed.TimeManager().DisableTimeConstrained(common.FederateHandle(ctx.federateID))
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
				return true, err
			}
			return true, nil
		}
		if fed, exists := ctx.currentFed.GetFederate(ctx.federateID); exists {
			fed.IsConstrained = false
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
		dispatchTimeGrants(ctx.currentFed, grants)
		return true, nil

	case NativeMsgTimeAdvanceRequest:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var tarReq common.TimeFloat64Payload
		if err := tarReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid time advance request")); err != nil {
				return true, err
			}
			return true, nil
		}
		grants, err := ctx.currentFed.TimeManager().TimeAdvanceRequest(common.FederateHandle(ctx.federateID), tarReq.Time)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		dispatchTimeGrants(ctx.currentFed, grants)
		return true, nil

	case NativeMsgNextMessageRequest:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var nmrReq common.TimeFloat64Payload
		if err := nmrReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid next message request")); err != nil {
				return true, err
			}
			return true, nil
		}
		grants, err := ctx.currentFed.TimeManager().NextMessageRequest(common.FederateHandle(ctx.federateID), nmrReq.Time)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		dispatchTimeGrants(ctx.currentFed, grants)
		return true, nil

	case NativeMsgFlushQueueRequest:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var flushReq common.TimeFloat64Payload
		if err := flushReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid flush queue request")); err != nil {
				return true, err
			}
			return true, nil
		}
		grants, err := ctx.currentFed.TimeManager().FlushQueueRequest(common.FederateHandle(ctx.federateID), flushReq.Time)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		dispatchTimeGrants(ctx.currentFed, grants)
		return true, nil
	}
	return false, nil
}

// handleDDMManagement handles IEEE 1516-2025 Clause 9 services
func (ctx *nativeContext) handleDDMManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgCreateRegion:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.CreateRegionPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid create region payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		regionHandle := ctx.currentFed.CreateRegion()
		respData, _ := common.CreateRegionResponsePayload{
			RegionHandle: regionHandle,
		}.MarshalBinary()

		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			PayloadLen: uint32(len(respData)),
			MsgType:    NativeMsgRegionCreated,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respData, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case NativeMsgSetRangeBounds:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.SetRangeBoundsPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid set range bounds payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.SetRegionRangeBounds(req.RegionHandle, req.DimensionHandle, req.LowerBound, req.UpperBound); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		return true, nil

	case NativeMsgSetMetricBounds:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.SetMetricBoundsPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid set metric bounds payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.currentFed.SetRegionMetricBounds(req.RegionHandle, req.MinX, req.MaxX, req.MinY, req.MaxY, req.MinZ, req.MaxZ); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		return true, nil

	case NativeMsgSubscribeWithRegion:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.SubscribeWithRegionPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid subscribe with region payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		region, exists := ctx.currentFed.GetRegion(req.RegionHandle)
		if !exists {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, errors.New("region handle not found")); err != nil {
				return true, err
			}
			return true, nil
		}
		attrs := convertToAttrHandles(req.Attributes)
		if err := ctx.currentFed.Router().SubscribeObjectClassWithRegion(common.FederateHandle(ctx.federateID), common.ObjectClassHandle(req.ObjectClass), attrs, region); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, err); err != nil {
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
		return true, nil

	case NativeMsgUpdateAttributesSpatial:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var req common.UpdateAttributesSpatialPayload
		if err := req.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid spatial update attributes payload")); err != nil {
				return true, err
			}
			return true, nil
		}

		morton := core.EncodeECEF(req.X, req.Y, req.Z)
		globalRoot, err := ctx.currentFed.UpdateAndRouteAttributes(
			ctx.federateID,
			req.ObjectHandle,
			req.Attributes,
			req.Tag,
			false,
			0,
			&morton,
			hdr.MerkleRoot,
		)
		if err != nil {
			if errors.Is(err, common.ErrObjectInstanceNotKnown) {
				if sendErr := sendNativeError(ctx.session, hdr.ReqID, 404, err); sendErr != nil {
					return true, sendErr
				}
			} else if errors.Is(err, core.ErrNotAttributeOwner) {
				if sendErr := sendNativeError(ctx.session, hdr.ReqID, 403, err); sendErr != nil {
					return true, sendErr
				}
			} else {
				logCRCNative.Warn("failed to update attributes with region for object %d: %v", req.ObjectHandle, err)
				if sendErr := sendNativeError(ctx.session, hdr.ReqID, 400, err); sendErr != nil {
					return true, sendErr
				}
			}
			return true, nil
		}

		respFlags := common.FlagNativeSSZ
		if ctx.currentFed.IsMerkleAuditEnabled() {
			respFlags |= common.FlagNativeMerkle
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      respFlags,
			MsgType:    NativeMsgSuccessOK,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			MerkleRoot: globalRoot,
		}
		if err := sendNativeResponse(ctx.session, respHdr, nil, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

// handleMetadataManagement handles IEEE 1516-2025 Clause 10 / Valhalla support services
func (ctx *nativeContext) handleMetadataManagement(hdr NativeHeader, payload []byte) (bool, error) {
	switch hdr.MsgType {
	case NativeMsgHeartbeat:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		ctx.currentFed.TimeManager().Heartbeat(common.FederateHandle(ctx.federateID))
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
		return true, nil

	case common.NativeMsgGetObjectClassHandle:
		if ctx.currentFed == nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var qReq common.GetObjectClassHandlePayload
		if err := qReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid get object class handle payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		cls, ok := ctx.currentFed.OMT().GetObjectClassByName(qReq.ClassName)
		if !ok {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, core.ErrClassNotFound); err != nil {
				return true, err
			}
			return true, nil
		}
		respPayload, _ := common.HandleResponsePayload{Handle: int32(cls.Handle)}.MarshalBinary()
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    common.NativeMsgGetObjectClassHandleResp,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			PayloadLen: uint32(len(respPayload)),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case common.NativeMsgGetAttributeHandle:
		if ctx.currentFed == nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var qReq common.GetAttributeHandlePayload
		if err := qReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid get attribute handle payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		attr, err := ctx.currentFed.OMT().ResolveAttributeByName(common.ObjectClassHandle(qReq.ClassHandle), qReq.AttributeName)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respPayload, _ := common.HandleResponsePayload{Handle: int32(attr.Handle)}.MarshalBinary()
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    common.NativeMsgGetAttributeHandleResp,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			PayloadLen: uint32(len(respPayload)),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case common.NativeMsgGetDimensionHandle:
		if ctx.currentFed == nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var qReq common.GetDimensionHandlePayload
		if err := qReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid get dimension handle payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		dim, ok := ctx.currentFed.OMT().GetDimensionByName(qReq.DimensionName)
		if !ok {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, errors.New("dimension not found")); err != nil {
				return true, err
			}
			return true, nil
		}
		respPayload, _ := common.HandleResponsePayload{Handle: int32(dim.Handle)}.MarshalBinary()
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      0x01,
			MsgType:    common.NativeMsgGetDimensionHandleResp,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			PayloadLen: uint32(len(respPayload)),
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil

	case common.NativeMsgGetMerkleProof:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := sendNativeError(ctx.session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
				return true, err
			}
			return true, nil
		}
		var qReq common.GetMerkleProofPayload
		if err := qReq.UnmarshalBinary(payload); err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 400, errors.New("invalid get merkle proof payload")); err != nil {
				return true, err
			}
			return true, nil
		}
		proof, err := ctx.currentFed.StateTree().GenerateProof(qReq.ObjectHandle)
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 404, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respPayload, err := common.MerkleProofResponsePayload{
			ObjectHandle: qReq.ObjectHandle,
			LeafIndex:    proof.LeafIndex,
			GlobalRoot:   proof.GlobalRoot,
			LeafHash:     proof.LeafHash,
			AuditPath:    proof.AuditPath,
		}.MarshalBinary()
		if err != nil {
			if err := sendNativeError(ctx.session, hdr.ReqID, 500, err); err != nil {
				return true, err
			}
			return true, nil
		}
		respHdr := NativeHeader{
			Magic:      NativeHeaderMagic,
			Version:    NativeProtocolVer,
			Flags:      common.FlagNativeSSZ | common.FlagNativeMerkle,
			MsgType:    common.NativeMsgGetMerkleProofResp,
			ReqID:      hdr.ReqID,
			FedID:      uint32(ctx.currentFed.Handle),
			FederateID: uint32(ctx.federateID),
			PayloadLen: uint32(len(respPayload)),
			MerkleRoot: proof.GlobalRoot,
		}
		if err := sendNativeResponse(ctx.session, respHdr, respPayload, ctx.federateID); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}
