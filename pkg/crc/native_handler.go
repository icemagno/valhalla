package crc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

// Tipos de Mensagem do Protocolo Nativo Valhalla (HLA-4)
const (
	NativeMsgCreateFederation       = common.NativeMsgCreateFederation
	NativeMsgDestroyFederation      = common.NativeMsgDestroyFederation
	NativeMsgJoin                   = common.NativeMsgJoin
	NativeMsgJoinSuccess            = common.NativeMsgJoinSuccess
	NativeMsgRegisterSyncPoint      = common.NativeMsgRegisterSyncPoint
	NativeMsgAnnounceSyncPoint      = common.NativeMsgAnnounceSyncPoint
	NativeMsgAchieveSyncPoint       = common.NativeMsgAchieveSyncPoint
	NativeMsgSyncPointAchieved      = common.NativeMsgSyncPointAchieved
	NativeMsgFederationSynchronized = common.NativeMsgFederationSynchronized
	NativeMsgPublishObjectClass          = common.NativeMsgPublishObjectClass
	NativeMsgUnpublishObjectClass        = common.NativeMsgUnpublishObjectClass
	NativeMsgPublishInteractionClass     = common.NativeMsgPublishInteractionClass
	NativeMsgUnpublishInteractionClass   = common.NativeMsgUnpublishInteractionClass
	NativeMsgSubscribeObjectClassAttrs   = common.NativeMsgSubscribeObjectClassAttrs
	NativeMsgUnsubscribeObjectClass      = common.NativeMsgUnsubscribeObjectClass
	NativeMsgSubscribeInteractionClass   = common.NativeMsgSubscribeInteractionClass
	NativeMsgUnsubscribeInteractionClass = common.NativeMsgUnsubscribeInteractionClass
	NativeMsgRegisterObject         = common.NativeMsgRegisterObject
	NativeMsgDiscoverObject         = common.NativeMsgDiscoverObject
	NativeMsgDeleteObjectInstance   = common.NativeMsgDeleteObjectInstance
	NativeMsgRemoveObjectInstance   = common.NativeMsgRemoveObjectInstance
	NativeMsgUpdateAttributes       = common.NativeMsgUpdateAttributes
	NativeMsgReflectAttributes      = common.NativeMsgReflectAttributes
	NativeMsgSendInteraction        = common.NativeMsgSendInteraction
	NativeMsgReceiveInteraction     = common.NativeMsgReceiveInteraction
	NativeMsgEnableTimeRegulation    = common.NativeMsgEnableTimeRegulation
	NativeMsgTimeRegulationEnabled   = common.NativeMsgTimeRegulationEnabled
	NativeMsgDisableTimeRegulation   = common.NativeMsgDisableTimeRegulation
	NativeMsgEnableTimeConstrained   = common.NativeMsgEnableTimeConstrained
	NativeMsgTimeConstrainedEnabled  = common.NativeMsgTimeConstrainedEnabled
	NativeMsgDisableTimeConstrained  = common.NativeMsgDisableTimeConstrained
	NativeMsgTimeAdvanceRequest      = common.NativeMsgTimeAdvanceRequest
	NativeMsgTimeAdvanceGrant        = common.NativeMsgTimeAdvanceGrant
	NativeMsgNextMessageRequest      = common.NativeMsgNextMessageRequest
	NativeMsgFlushQueueRequest       = common.NativeMsgFlushQueueRequest
	NativeMsgDivestOwnership         = common.NativeMsgDivestOwnership
	NativeMsgAcquireOwnership        = common.NativeMsgAcquireOwnership
	NativeMsgQueryAttributeOwnership = common.NativeMsgQueryAttributeOwnership
	NativeMsgAttributeOwnershipResponse = common.NativeMsgAttributeOwnershipResponse
	NativeMsgCreateRegion            = common.NativeMsgCreateRegion
	NativeMsgRegionCreated           = common.NativeMsgRegionCreated
	NativeMsgSetRangeBounds          = common.NativeMsgSetRangeBounds
	NativeMsgSetMetricBounds         = common.NativeMsgSetMetricBounds
	NativeMsgSubscribeWithRegion     = common.NativeMsgSubscribeWithRegion
	NativeMsgUpdateAttributesSpatial = common.NativeMsgUpdateAttributesSpatial
	NativeMsgResign                  = common.NativeMsgResign
	NativeMsgSuccessOK               = common.NativeMsgSuccessOK
	NativeMsgError                   = common.NativeMsgError
)

func sendNativeError(session *NativeSession, reqID uint16, code uint16, err error) error {
	errPayload, _ := common.ErrorResponsePayload{
		ErrorCode: code,
		Message:   err.Error(),
	}.MarshalBinary()
	respHdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      0x01,
		PayloadLen: uint32(len(errPayload)),
		MsgType:    NativeMsgError,
		ReqID:      reqID,
	}
	return session.SendMessage(respHdr, errPayload)
}

func dispatchTimeGrants(currentFed *FederationExecution, grants []core.TimeGrantEvent) {
	for _, grant := range grants {
		fed, ok := currentFed.GetFederate(int32(grant.Federate))
		if !ok || fed.Session == nil {
			continue
		}
		// Deliver any queued TSO messages first in strictly non-decreasing time order
		for _, msg := range grant.Messages {
			_ = fed.Session.SendNotification(msg.MsgType, msg.Payload)
		}
		// Deliver TimeAdvanceGrant notification
		grantPayload, _ := common.TimeFloat64Payload{Time: grant.Time}.MarshalBinary()
		_ = fed.Session.SendNotification(NativeMsgTimeAdvanceGrant, grantPayload)
	}
}

func convertToAttrHandles(attrs []uint32) []common.AttributeHandle {
	res := make([]common.AttributeHandle, len(attrs))
	for i, a := range attrs {
		res[i] = common.AttributeHandle(a)
	}
	return res
}

// HandleNativeConnection gerencia um cliente nativo conectado usando o protocolo Valhalla
func HandleNativeConnection(conn net.Conn, reader io.Reader, fedManager *FederationManager) error {
	session := NewNativeSession(conn)
	defer session.Close()

	var currentFed *FederationExecution
	var federateID int32 = -1

	for {
		hdr, err := ReadNativeHeader(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				if currentFed != nil && federateID > 0 {
					_, _ = currentFed.Resign(federateID)
				}
				return nil
			}
			return fmt.Errorf("failed to read native header: %w", err)
		}

		var payload []byte
		if hdr.PayloadLen > 0 {
			payload = make([]byte, hdr.PayloadLen)
			if _, err := io.ReadFull(reader, payload); err != nil {
				return fmt.Errorf("failed to read native payload (%d bytes): %w", hdr.PayloadLen, err)
			}
		}

		switch hdr.MsgType {
		case NativeMsgCreateFederation:
			var req common.FederationExecutionPayload
			if err := req.UnmarshalBinary(payload); err != nil || req.FederationName == "" {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid federation name"))
				continue
			}
			_, err := fedManager.CreateFederation(req.FederationName)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 409, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:   NativeHeaderMagic,
				Version: NativeProtocolVer,
				Flags:   0x01,
				MsgType: NativeMsgSuccessOK,
				ReqID:   hdr.ReqID,
			}
			_ = session.SendMessage(respHdr, nil)
			logCRCNative.Info("Federation [%s] created successfully", req.FederationName)

		case NativeMsgDestroyFederation:
			var req common.FederationExecutionPayload
			if err := req.UnmarshalBinary(payload); err != nil || req.FederationName == "" {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid federation name"))
				continue
			}
			err := fedManager.DestroyFederation(req.FederationName)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 409, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:   NativeHeaderMagic,
				Version: NativeProtocolVer,
				Flags:   0x01,
				MsgType: NativeMsgSuccessOK,
				ReqID:   hdr.ReqID,
			}
			_ = session.SendMessage(respHdr, nil)
			logCRCNative.Info("Federation [%s] destroyed successfully", req.FederationName)

		case NativeMsgJoin:
			var joinReq common.JoinFederationPayload
			if err := joinReq.UnmarshalBinary(payload); err != nil || joinReq.FederateName == "" {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid join request: federateName is mandatory"))
				continue
			}

			targetFedName := joinReq.FederationName
			var fedExec *FederationExecution
			var exists bool

			if targetFedName == "" || targetFedName == "*" || targetFedName == "auto" {
				fedExec, exists = fedManager.GetFirstFederation()
			} else {
				fedExec, exists = fedManager.GetFederation(targetFedName)
			}

			if !exists {
				_ = sendNativeError(session, hdr.ReqID, 404, common.ErrFederationExecutionDoesNotExist)
				continue
			}

			fed, err := fedExec.Join(joinReq.FederateName, joinReq.FederateType, ProtocolNative, session)
			if err != nil {
				logCRCNative.Error("Join error for %s: %v", joinReq.FederateName, err)
				_ = sendNativeError(session, hdr.ReqID, 409, err)
				continue
			}

			currentFed = fedExec
			federateID = fed.ID
			session.SetFederateID(federateID)
			session.SetFederationID(currentFed.Handle)

			respData, _ := common.JoinFederationResponsePayload{
				FederateHandle:   common.FederateHandle(federateID),
				FederationHandle: currentFed.Handle,
			}.MarshalBinary()

			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				PayloadLen: uint32(len(respData)),
				MsgType:    NativeMsgJoinSuccess,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, respData)
			logCRCNative.Info("Federate [%s] joined federation [%s] with handle [%d]", joinReq.FederateName, joinReq.FederationName, federateID)

		case NativeMsgRegisterSyncPoint:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var syncReq common.RegisterSyncPointPayload
			if err := syncReq.UnmarshalBinary(payload); err != nil || syncReq.Label == "" {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid synchronization point label"))
				continue
			}
			_, err := currentFed.RegisterSyncPoint(syncReq.Label, syncReq.Tag, federateID)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 409, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)
			logCRCNative.Info("Federate [%d] registered synchronization point '%s'", federateID, syncReq.Label)

			announcePayload, _ := common.RegisterSyncPointPayload{Label: syncReq.Label, Tag: syncReq.Tag}.MarshalBinary()
			for _, f := range currentFed.GetAllFederates() {
				if f.Session != nil {
					_ = f.Session.SendNotification(NativeMsgAnnounceSyncPoint, announcePayload)
				}
			}

		case NativeMsgAchieveSyncPoint:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var labelReq common.SyncPointLabelPayload
			if err := labelReq.UnmarshalBinary(payload); err != nil || labelReq.Label == "" {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid synchronization point label"))
				continue
			}
			isSynced, err := currentFed.AchieveSyncPoint(labelReq.Label, federateID)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 404, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)
			logCRCNative.Info("Federate [%d] achieved synchronization point '%s'", federateID, labelReq.Label)

			if isSynced {
				logCRCNative.Info(">>> FEDERATION SYNCHRONIZED at point '%s'!", labelReq.Label)
				syncNotif, _ := common.SyncPointLabelPayload{Label: labelReq.Label}.MarshalBinary()
				for _, f := range currentFed.GetAllFederates() {
					if f.Session != nil {
						_ = f.Session.SendNotification(NativeMsgFederationSynchronized, syncNotif)
					}
				}
			}

		case NativeMsgPublishObjectClass:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var pubReq common.PublishObjectClassPayload
			if err := pubReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid publish object class payload"))
				continue
			}
			err := currentFed.Router().PublishObjectClass(common.FederateHandle(federateID), pubReq.ClassHandle, pubReq.AttributeHandles)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgUnpublishObjectClass:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var unpubReq common.PublishObjectClassPayload
			if err := unpubReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid unpublish object class payload"))
				continue
			}
			_ = currentFed.Router().UnpublishObjectClass(common.FederateHandle(federateID), unpubReq.ClassHandle)
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgSubscribeObjectClassAttrs:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var subReq common.SubscribeObjectClassPayload
			if err := subReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid subscribe object class payload"))
				continue
			}
			err := currentFed.Router().SubscribeObjectClass(common.FederateHandle(federateID), subReq.ClassHandle, subReq.AttributeHandles, subReq.Active)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

			// Catch-up Discovery: notificar o assinante sobre todas as instâncias existentes que casam com a hierarquia
			if subReq.Active {
				for _, obj := range currentFed.GetAllObjects() {
					if currentFed.OMT().IsSubclassOfObject(common.ObjectClassHandle(obj.ClassHandle), subReq.ClassHandle) {
						discPayload, _ := common.DiscoverObjectInstancePayload{
							ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
							ClassHandle:  common.ObjectClassHandle(obj.ClassHandle),
							ObjectName:   obj.Name,
						}.MarshalBinary()
						_ = session.SendNotification(NativeMsgDiscoverObject, discPayload)
					}
				}
			}

		case NativeMsgUnsubscribeObjectClass:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var unsubReq common.PublishObjectClassPayload
			if err := unsubReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid unsubscribe object class payload"))
				continue
			}
			_ = currentFed.Router().UnsubscribeObjectClass(common.FederateHandle(federateID), unsubReq.ClassHandle)
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgPublishInteractionClass:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var pubReq common.InteractionClassPayload
			if err := pubReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid publish interaction class payload"))
				continue
			}
			err := currentFed.Router().PublishInteractionClass(common.FederateHandle(federateID), pubReq.ClassHandle)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgUnpublishInteractionClass:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var unpubReq common.InteractionClassPayload
			if err := unpubReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid unpublish interaction class payload"))
				continue
			}
			_ = currentFed.Router().UnpublishInteractionClass(common.FederateHandle(federateID), unpubReq.ClassHandle)
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgSubscribeInteractionClass:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var subReq common.SubscribeInteractionClassPayload
			if err := subReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid subscribe interaction class payload"))
				continue
			}
			err := currentFed.Router().SubscribeInteractionClass(common.FederateHandle(federateID), subReq.ClassHandle, subReq.Active)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgUnsubscribeInteractionClass:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var unsubReq common.InteractionClassPayload
			if err := unsubReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid unsubscribe interaction class payload"))
				continue
			}
			_ = currentFed.Router().UnsubscribeInteractionClass(common.FederateHandle(federateID), unsubReq.ClassHandle)
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgRegisterObject:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var regReq common.RegisterObjectInstancePayload
			if err := regReq.UnmarshalBinary(payload); err != nil || regReq.ClassHandle == 0 {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid register object request"))
				continue
			}
			if _, exists := currentFed.OMT().GetObjectClass(regReq.ClassHandle); !exists {
				_ = sendNativeError(session, hdr.ReqID, 404, common.ErrObjectClassNotDefined)
				continue
			}
			obj, err := currentFed.RegisterObjectInstance(int32(regReq.ClassHandle), regReq.ObjectName, federateID)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 409, err)
				continue
			}

			var respPayload [4]byte
			binary.BigEndian.PutUint32(respPayload[0:4], uint32(obj.Handle))

			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				PayloadLen: 4,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, respPayload[:])
			logCRCNative.Info("Object registered: handle=%d, class=%d, name=%s", obj.Handle, regReq.ClassHandle, obj.Name)

			discPayload, _ := common.DiscoverObjectInstancePayload{
				ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
				ClassHandle:  regReq.ClassHandle,
				ObjectName:   obj.Name,
			}.MarshalBinary()

			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					_ = f.Session.SendNotification(NativeMsgDiscoverObject, discPayload)
				}
			}

		case NativeMsgDeleteObjectInstance:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var delReq common.DeleteObjectInstancePayload
			if err := delReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid delete object instance payload"))
				continue
			}
			obj, err := currentFed.DeleteObjectInstance(int32(delReq.ObjectHandle), federateID, delReq.Tag)
			if err != nil {
				if errors.Is(err, common.ErrObjectInstanceNotKnown) {
					_ = sendNativeError(session, hdr.ReqID, 404, err)
				} else if errors.Is(err, common.ErrDeletePrivilegeNotHeld) {
					_ = sendNativeError(session, hdr.ReqID, 403, err)
				} else {
					_ = sendNativeError(session, hdr.ReqID, 500, err)
				}
				continue
			}

			globalRoot := currentFed.StateTree().GetGlobalRoot()
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      common.FlagNativeSSZ | common.FlagNativeMerkle,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
				MerkleRoot: globalRoot,
			}
			_ = session.SendMessage(respHdr, nil)
			logCRCNative.Info("Object [%d] '%s' deleted by federate [%d]", obj.Handle, obj.Name, federateID)

			removePayload, _ := common.RemoveObjectInstancePayload{
				ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
				Tag:          delReq.Tag,
			}.MarshalBinary()

			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					_ = f.Session.SendNotification(NativeMsgRemoveObjectInstance, removePayload)
				}
			}

		case NativeMsgUpdateAttributes:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var uavReq common.UpdateAttributesPayload
			if err := uavReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid update attributes payload"))
				continue
			}
			obj, exists := currentFed.GetObjectInstance(int32(uavReq.ObjectHandle))
			if !exists {
				_ = sendNativeError(session, hdr.ReqID, 404, common.ErrObjectInstanceNotKnown)
				continue
			}

			attrsToStore := make(map[int32][]byte, len(uavReq.Attributes))
			for k, v := range uavReq.Attributes {
				attrsToStore[int32(k)] = v
			}
			_ = currentFed.UpdateObjectAttributes(int32(uavReq.ObjectHandle), attrsToStore, hdr.MerkleRoot)

			globalRoot := currentFed.StateTree().GetGlobalRoot()
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      common.FlagNativeSSZ | common.FlagNativeMerkle,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
				MerkleRoot: globalRoot,
			}
			_ = session.SendMessage(respHdr, nil)

			// Roteamento inteligente de assinantes via Router (InterestMatcher)
			targetSubs := make(map[common.FederateHandle]bool)
			for attrHandle := range uavReq.Attributes {
				subs := currentFed.Router().GetAttributeSubscribers(common.ObjectClassHandle(obj.ClassHandle), attrHandle)
				for _, sub := range subs {
					if sub != common.FederateHandle(federateID) {
						targetSubs[sub] = true
					}
				}
			}

			// Despacha reflexões para federados com assinatura ativa (TSO ou RO)
			for subID := range targetSubs {
				if f, exists := currentFed.GetFederate(int32(subID)); exists && f.Session != nil {
					if uavReq.HasTime && f.IsConstrained {
						grants, err := currentFed.TimeManager().EnqueueTSOMessage(core.TSOMessage{
							Sender:    common.FederateHandle(federateID),
							Receiver:  subID,
							Timestamp: uavReq.Time,
							MsgType:   NativeMsgReflectAttributes,
							Payload:   payload,
						})
						if err == nil && len(grants) > 0 {
							dispatchTimeGrants(currentFed, grants)
						}
					} else {
						_ = f.Session.SendNotification(NativeMsgReflectAttributes, payload)
					}
				}
			}

		case NativeMsgSendInteraction:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var interReq common.SendInteractionPayload
			if err := interReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid send interaction payload"))
				continue
			}
			if _, exists := currentFed.OMT().GetInteractionClass(interReq.ClassHandle); !exists {
				_ = sendNativeError(session, hdr.ReqID, 404, common.ErrInteractionClassNotDefined)
				continue
			}

			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

			// Roteamento polimórfico de interações via Router (TSO ou RO)
			subs := currentFed.Router().GetInteractionSubscribers(interReq.ClassHandle)
			for _, subID := range subs {
				if subID != common.FederateHandle(federateID) {
					if f, exists := currentFed.GetFederate(int32(subID)); exists && f.Session != nil {
						if interReq.HasTime && f.IsConstrained {
							grants, err := currentFed.TimeManager().EnqueueTSOMessage(core.TSOMessage{
								Sender:    common.FederateHandle(federateID),
								Receiver:  subID,
								Timestamp: interReq.Time,
								MsgType:   NativeMsgReceiveInteraction,
								Payload:   payload,
							})
							if err == nil && len(grants) > 0 {
								dispatchTimeGrants(currentFed, grants)
							}
						} else {
							_ = f.Session.SendNotification(NativeMsgReceiveInteraction, payload)
						}
					}
				}
			}

		case NativeMsgEnableTimeRegulation:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var timeReq common.TimeFloat64Payload
			if err := timeReq.UnmarshalBinary(payload); err != nil || timeReq.Time < 0 {
				_ = sendNativeError(session, hdr.ReqID, 400, common.ErrInvalidLookahead)
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				_ = sendNativeError(session, hdr.ReqID, 404, common.ErrFederateNotFound)
				continue
			}
			if err := currentFed.TimeManager().EnableTimeRegulation(common.FederateHandle(federateID), timeReq.Time); err != nil {
				if errors.Is(err, common.ErrTimeRegulationAlreadyEnabled) {
					_ = sendNativeError(session, hdr.ReqID, 409, err)
				} else {
					_ = sendNativeError(session, hdr.ReqID, 400, err)
				}
				continue
			}
			fed.IsRegulating = true
			fed.Lookahead = timeReq.Time

			status, _ := currentFed.TimeManager().GetStatus(common.FederateHandle(federateID))
			respData, _ := common.TimeFloat64Payload{Time: status.CurrentTime}.MarshalBinary()
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				PayloadLen: uint32(len(respData)),
				MsgType:    NativeMsgTimeRegulationEnabled,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, respData)
			logCRCNative.Info("Time regulation enabled for federate [%d], lookahead=%f", federateID, fed.Lookahead)

		case NativeMsgDisableTimeRegulation:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			grants, err := currentFed.TimeManager().DisableTimeRegulation(common.FederateHandle(federateID))
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			if fed, exists := currentFed.GetFederate(federateID); exists {
				fed.IsRegulating = false
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)
			dispatchTimeGrants(currentFed, grants)

		case NativeMsgEnableTimeConstrained:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				_ = sendNativeError(session, hdr.ReqID, 404, common.ErrFederateNotFound)
				continue
			}
			if err := currentFed.TimeManager().EnableTimeConstrained(common.FederateHandle(federateID)); err != nil {
				if errors.Is(err, common.ErrTimeConstrainedAlreadyEnabled) {
					_ = sendNativeError(session, hdr.ReqID, 409, err)
				} else {
					_ = sendNativeError(session, hdr.ReqID, 400, err)
				}
				continue
			}
			fed.IsConstrained = true

			status, _ := currentFed.TimeManager().GetStatus(common.FederateHandle(federateID))
			respData, _ := common.TimeFloat64Payload{Time: status.CurrentTime}.MarshalBinary()
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				PayloadLen: uint32(len(respData)),
				MsgType:    NativeMsgTimeConstrainedEnabled,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, respData)
			logCRCNative.Info("Time constrained enabled for federate [%d]", federateID)

		case NativeMsgDisableTimeConstrained:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			grants, err := currentFed.TimeManager().DisableTimeConstrained(common.FederateHandle(federateID))
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			if fed, exists := currentFed.GetFederate(federateID); exists {
				fed.IsConstrained = false
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)
			dispatchTimeGrants(currentFed, grants)

		case NativeMsgTimeAdvanceRequest:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var tarReq common.TimeFloat64Payload
			if err := tarReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid time advance request"))
				continue
			}
			grants, err := currentFed.TimeManager().TimeAdvanceRequest(common.FederateHandle(federateID), tarReq.Time)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}

			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)
			dispatchTimeGrants(currentFed, grants)

		case NativeMsgNextMessageRequest:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var nmrReq common.TimeFloat64Payload
			if err := nmrReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid next message request"))
				continue
			}
			grants, err := currentFed.TimeManager().NextMessageRequest(common.FederateHandle(federateID), nmrReq.Time)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}

			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)
			dispatchTimeGrants(currentFed, grants)

		case NativeMsgFlushQueueRequest:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var flushReq common.TimeFloat64Payload
			if err := flushReq.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid flush queue request"))
				continue
			}
			grants, err := currentFed.TimeManager().FlushQueueRequest(common.FederateHandle(federateID), flushReq.Time)
			if err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}

			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)
			dispatchTimeGrants(currentFed, grants)

		case NativeMsgDivestOwnership:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var req common.DivestOwnershipPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid divest ownership payload"))
				continue
			}
			attrs := convertToAttrHandles(req.Attributes)
			if err := currentFed.OwnershipManager().UnconditionalDivestiture(common.ObjectInstanceHandle(req.ObjectHandle), attrs, common.FederateHandle(federateID)); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgAcquireOwnership:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var req common.AcquireOwnershipPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid acquire ownership payload"))
				continue
			}
			attrs := convertToAttrHandles(req.Attributes)
			if err := currentFed.OwnershipManager().AcquireAttributeOwnership(common.ObjectInstanceHandle(req.ObjectHandle), attrs, common.FederateHandle(federateID)); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgQueryAttributeOwnership:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var req common.QueryOwnershipPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid query ownership payload"))
				continue
			}
			owner, isOwned := currentFed.OwnershipManager().QueryAttributeOwnership(common.ObjectInstanceHandle(req.ObjectHandle), common.AttributeHandle(req.AttributeHandle))
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
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, respPayload)

		case NativeMsgCreateRegion:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var req common.CreateRegionPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid create region payload"))
				continue
			}
			regionHandle := currentFed.CreateRegion()
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
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, respData)

		case NativeMsgSetRangeBounds:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var req common.SetRangeBoundsPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid set range bounds payload"))
				continue
			}
			if err := currentFed.SetRegionRangeBounds(req.RegionHandle, req.DimensionHandle, req.LowerBound, req.UpperBound); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgSetMetricBounds:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var req common.SetMetricBoundsPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid set metric bounds payload"))
				continue
			}
			if err := currentFed.SetRegionMetricBounds(req.RegionHandle, req.MinX, req.MaxX, req.MinY, req.MaxY, req.MinZ, req.MaxZ); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgSubscribeWithRegion:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var req common.SubscribeWithRegionPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid subscribe with region payload"))
				continue
			}
			region, exists := currentFed.GetRegion(req.RegionHandle)
			if !exists {
				_ = sendNativeError(session, hdr.ReqID, 404, errors.New("region handle not found"))
				continue
			}
			attrs := convertToAttrHandles(req.Attributes)
			if err := currentFed.Router().SubscribeObjectClassWithRegion(common.FederateHandle(federateID), common.ObjectClassHandle(req.ObjectClass), attrs, region); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, err)
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)

		case NativeMsgUpdateAttributesSpatial:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
				continue
			}
			var req common.UpdateAttributesSpatialPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid spatial update attributes payload"))
				continue
			}
			obj, exists := currentFed.GetObjectInstance(int32(req.ObjectHandle))
			if !exists {
				_ = sendNativeError(session, hdr.ReqID, 404, common.ErrObjectInstanceNotKnown)
				continue
			}

			// Validate attribute ownership if ownership is held by another federate
			ownershipViolation := false
			for attrHandle := range req.Attributes {
				if owner, isOwned := currentFed.OwnershipManager().QueryAttributeOwnership(req.ObjectHandle, attrHandle); isOwned && owner != common.FederateHandle(federateID) {
					ownershipViolation = true
					break
				}
			}
			if ownershipViolation {
				_ = sendNativeError(session, hdr.ReqID, 403, core.ErrNotAttributeOwner)
				continue
			}

			attrsToStore := make(map[int32][]byte, len(req.Attributes))
			for k, v := range req.Attributes {
				attrsToStore[int32(k)] = v
			}
			_ = currentFed.UpdateObjectAttributes(int32(req.ObjectHandle), attrsToStore, hdr.MerkleRoot)

			globalRoot := currentFed.StateTree().GetGlobalRoot()
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      common.FlagNativeSSZ | common.FlagNativeMerkle,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
				MerkleRoot: globalRoot,
			}
			_ = session.SendMessage(respHdr, nil)

			// Compute 3D Morton code from metric coordinates
			mortonCode := core.EncodeECEF(req.X, req.Y, req.Z)

			// Route spatial reflections using Morton 3D spatial containment
			targetSubs := make(map[common.FederateHandle]bool)
			for attrHandle := range req.Attributes {
				subs := currentFed.Router().GetAttributeSubscribersInRegion(common.ObjectClassHandle(obj.ClassHandle), attrHandle, mortonCode)
				for _, sub := range subs {
					if sub != common.FederateHandle(federateID) {
						targetSubs[sub] = true
					}
				}
			}

			// Prepare standard RAV payload for all matched subscribers
			ravPayload, _ := common.UpdateAttributesPayload{
				ObjectHandle: req.ObjectHandle,
				Attributes:   req.Attributes,
				Tag:          req.Tag,
			}.MarshalBinary()

			for subID := range targetSubs {
				if f, exists := currentFed.GetFederate(int32(subID)); exists && f.Session != nil {
					_ = f.Session.SendNotification(NativeMsgReflectAttributes, ravPayload)
				}
			}

		case NativeMsgResign:
			if currentFed != nil && federateID > 0 {
				_, _ = currentFed.Resign(federateID)
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			_ = session.SendMessage(respHdr, nil)
			logCRCNative.Info("Federate [%d] resigned gracefully", federateID)
			currentFed = nil
			federateID = -1
		}
	}
}
