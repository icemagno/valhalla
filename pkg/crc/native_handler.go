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

// Message types for the native Valhalla HLA-4 protocol
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
	NativeMsgHeartbeat               = common.NativeMsgHeartbeat
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
	errPayload, marshalErr := common.ErrorResponsePayload{
		ErrorCode: code,
		Message:   err.Error(),
	}.MarshalBinary()
	if marshalErr != nil {
		logCRCNative.Error("failed to marshal ErrorResponsePayload: %v", marshalErr)
		return marshalErr
	}
	respHdr := NativeHeader{
		Magic:      NativeHeaderMagic,
		Version:    NativeProtocolVer,
		Flags:      0x01,
		PayloadLen: uint32(len(errPayload)),
		MsgType:    NativeMsgError,
		ReqID:      reqID,
	}
	if sendErr := session.SendMessage(respHdr, errPayload); sendErr != nil {
		logCRCNative.Warn("failed to send error response (reqID=%d, code=%d): %v", reqID, code, sendErr)
		return sendErr
	}
	return nil
}

func sendNativeResponse(session *NativeSession, hdr NativeHeader, payload []byte, federateID int32) error {
	if err := session.SendMessage(hdr, payload); err != nil {
		logCRCNative.Warn("failed to send response (msgType=%d, reqID=%d) to federate %d: %v", hdr.MsgType, hdr.ReqID, federateID, err)
		return err
	}
	return nil
}

func dispatchTimeGrants(currentFed *FederationExecution, grants []core.TimeGrantEvent) {
	for _, grant := range grants {
		fed, ok := currentFed.GetFederate(int32(grant.Federate))
		if !ok || fed.Session == nil {
			continue
		}
		// Deliver any queued TSO messages first in strictly non-decreasing time order
		for _, msg := range grant.Messages {
			if err := fed.Session.SendNotificationWithMerkle(msg.MsgType, msg.Payload, msg.MerkleRoot); err != nil {
				logCRCNative.Warn("failed to deliver TSO message %d to federate %d: %v", msg.MsgType, grant.Federate, err)
			}
		}
		// Deliver TimeAdvanceGrant notification
		grantPayload, err := common.TimeFloat64Payload{Time: grant.Time}.MarshalBinary()
		if err != nil {
			logCRCNative.Error("failed to marshal TimeFloat64Payload: %v", err)
			continue
		}
		if err := fed.Session.SendNotification(NativeMsgTimeAdvanceGrant, grantPayload); err != nil {
			logCRCNative.Warn("failed to deliver TimeAdvanceGrant to federate %d: %v", grant.Federate, err)
		}
	}
}

func convertToAttrHandles(attrs []uint32) []common.AttributeHandle {
	res := make([]common.AttributeHandle, len(attrs))
	for i, a := range attrs {
		res[i] = common.AttributeHandle(a)
	}
	return res
}

// HandleNativeConnection manages a connected native client using the Valhalla binary protocol
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
					if _, err := currentFed.Resign(federateID); err != nil {
						logCRCNative.Warn("failed to resign federate %d on disconnect: %v", federateID, err)
					}
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

		if (hdr.Flags & common.FlagNativePQC) != 0 {
			if err := sendNativeError(session, hdr.ReqID, 400, errors.New("post-quantum cryptography (PQC) framing is not supported in this version; see Roadmap")); err != nil {
				return err
			}
			continue
		}

		if currentFed != nil && federateID > 0 {
			currentFed.TimeManager().Heartbeat(common.FederateHandle(federateID))
		}

		switch hdr.MsgType {
		case NativeMsgCreateFederation:
			var req common.FederationExecutionPayload
			if err := req.UnmarshalBinary(payload); err != nil || req.FederationName == "" {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid federation name")); err != nil {
					return err
				}
				continue
			}
			_, err := fedManager.CreateFederationWithFOMs(req.FederationName, req.FOMModules)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 409, err); err != nil {
					return err
				}
				continue
			}
			respHdr := NativeHeader{
				Magic:   NativeHeaderMagic,
				Version: NativeProtocolVer,
				Flags:   0x01,
				MsgType: NativeMsgSuccessOK,
				ReqID:   hdr.ReqID,
			}
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			logCRCNative.Info("Federation [%s] created successfully (foms=%v)", req.FederationName, req.FOMModules)

		case NativeMsgDestroyFederation:
			var req common.FederationExecutionPayload
			if err := req.UnmarshalBinary(payload); err != nil || req.FederationName == "" {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid federation name")); err != nil {
					return err
				}
				continue
			}
			err := fedManager.DestroyFederation(req.FederationName)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 409, err); err != nil {
					return err
				}
				continue
			}
			respHdr := NativeHeader{
				Magic:   NativeHeaderMagic,
				Version: NativeProtocolVer,
				Flags:   0x01,
				MsgType: NativeMsgSuccessOK,
				ReqID:   hdr.ReqID,
			}
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			logCRCNative.Info("Federation [%s] destroyed successfully", req.FederationName)

		case NativeMsgJoin:
			var joinReq common.JoinFederationPayload
			if err := joinReq.UnmarshalBinary(payload); err != nil || joinReq.FederateName == "" {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid join request: federateName is mandatory")); err != nil {
					return err
				}
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
				if err := sendNativeError(session, hdr.ReqID, 404, common.ErrFederationExecutionDoesNotExist); err != nil {
					return err
				}
				continue
			}

			fed, err := fedExec.Join(joinReq.FederateName, joinReq.FederateType, ProtocolNative, session)
			if err != nil {
				logCRCNative.Error("Join error for %s: %v", joinReq.FederateName, err)
				if err := sendNativeError(session, hdr.ReqID, 409, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, respData, federateID); err != nil {
				return err
			}
			logCRCNative.Info("Federate [%s] joined federation [%s] with handle [%d]", joinReq.FederateName, joinReq.FederationName, federateID)

		case NativeMsgRegisterSyncPoint:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var syncReq common.RegisterSyncPointPayload
			if err := syncReq.UnmarshalBinary(payload); err != nil || syncReq.Label == "" {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid synchronization point label")); err != nil {
					return err
				}
				continue
			}
			_, err := currentFed.RegisterSyncPoint(syncReq.Label, syncReq.Tag, federateID)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 409, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			logCRCNative.Info("Federate [%d] registered synchronization point '%s'", federateID, syncReq.Label)

			announcePayload, _ := common.RegisterSyncPointPayload{Label: syncReq.Label, Tag: syncReq.Tag}.MarshalBinary()
			for _, f := range currentFed.GetAllFederates() {
				if f.Session != nil {
					if err := f.Session.SendNotification(NativeMsgAnnounceSyncPoint, announcePayload); err != nil {
						logCRCNative.Warn("failed to dispatch AnnounceSyncPoint to federate %d: %v", f.ID, err)
					}
				}
			}

		case NativeMsgAchieveSyncPoint:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var labelReq common.SyncPointLabelPayload
			if err := labelReq.UnmarshalBinary(payload); err != nil || labelReq.Label == "" {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid synchronization point label")); err != nil {
					return err
				}
				continue
			}
			isSynced, err := currentFed.AchieveSyncPoint(labelReq.Label, federateID)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 404, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			logCRCNative.Info("Federate [%d] achieved synchronization point '%s'", federateID, labelReq.Label)

			if isSynced {
				logCRCNative.Info(">>> FEDERATION SYNCHRONIZED at point '%s'!", labelReq.Label)
				syncNotif, _ := common.SyncPointLabelPayload{Label: labelReq.Label}.MarshalBinary()
				for _, f := range currentFed.GetAllFederates() {
					if f.Session != nil {
						if err := f.Session.SendNotification(NativeMsgFederationSynchronized, syncNotif); err != nil {
							logCRCNative.Warn("failed to dispatch FederationSynchronized to federate %d: %v", f.ID, err)
						}
					}
				}
			}

		case NativeMsgPublishObjectClass:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var pubReq common.PublishObjectClassPayload
			if err := pubReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid publish object class payload")); err != nil {
					return err
				}
				continue
			}
			err := currentFed.Router().PublishObjectClass(common.FederateHandle(federateID), pubReq.ClassHandle, pubReq.AttributeHandles)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgUnpublishObjectClass:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var unpubReq common.PublishObjectClassPayload
			if err := unpubReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid unpublish object class payload")); err != nil {
					return err
				}
				continue
			}
			if err := currentFed.Router().UnpublishObjectClass(common.FederateHandle(federateID), unpubReq.ClassHandle); err != nil {
				if sendErr := sendNativeError(session, hdr.ReqID, 400, err); sendErr != nil {
					return sendErr
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgSubscribeObjectClassAttrs:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var subReq common.SubscribeObjectClassPayload
			if err := subReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid subscribe object class payload")); err != nil {
					return err
				}
				continue
			}
			err := currentFed.Router().SubscribeObjectClass(common.FederateHandle(federateID), subReq.ClassHandle, subReq.AttributeHandles, subReq.Active)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

			// Catch-up Discovery: notify the subscriber about all existing instances matching the hierarchy
			if subReq.Active {
				for _, obj := range currentFed.GetAllObjects() {
					if currentFed.OMT().IsSubclassOfObject(common.ObjectClassHandle(obj.ClassHandle), subReq.ClassHandle) {
						discPayload, _ := common.DiscoverObjectInstancePayload{
							ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
							ClassHandle:  common.ObjectClassHandle(obj.ClassHandle),
							ObjectName:   obj.Name,
						}.MarshalBinary()
						if err := session.SendNotification(NativeMsgDiscoverObject, discPayload); err != nil {
							logCRCNative.Warn("failed to send DiscoverObject to federate %d: %v", federateID, err)
							return err
						}
					}
				}
			}

		case NativeMsgUnsubscribeObjectClass:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var unsubReq common.PublishObjectClassPayload
			if err := unsubReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid unsubscribe object class payload")); err != nil {
					return err
				}
				continue
			}
			if err := currentFed.Router().UnsubscribeObjectClass(common.FederateHandle(federateID), unsubReq.ClassHandle); err != nil {
				if sendErr := sendNativeError(session, hdr.ReqID, 400, err); sendErr != nil {
					return sendErr
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgPublishInteractionClass:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var pubReq common.InteractionClassPayload
			if err := pubReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid publish interaction class payload")); err != nil {
					return err
				}
				continue
			}
			err := currentFed.Router().PublishInteractionClass(common.FederateHandle(federateID), pubReq.ClassHandle)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgUnpublishInteractionClass:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var unpubReq common.InteractionClassPayload
			if err := unpubReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid unpublish interaction class payload")); err != nil {
					return err
				}
				continue
			}
			if err := currentFed.Router().UnpublishInteractionClass(common.FederateHandle(federateID), unpubReq.ClassHandle); err != nil {
				if sendErr := sendNativeError(session, hdr.ReqID, 400, err); sendErr != nil {
					return sendErr
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgSubscribeInteractionClass:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var subReq common.SubscribeInteractionClassPayload
			if err := subReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid subscribe interaction class payload")); err != nil {
					return err
				}
				continue
			}
			err := currentFed.Router().SubscribeInteractionClass(common.FederateHandle(federateID), subReq.ClassHandle, subReq.Active)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgUnsubscribeInteractionClass:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var unsubReq common.InteractionClassPayload
			if err := unsubReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid unsubscribe interaction class payload")); err != nil {
					return err
				}
				continue
			}
			if err := currentFed.Router().UnsubscribeInteractionClass(common.FederateHandle(federateID), unsubReq.ClassHandle); err != nil {
				if sendErr := sendNativeError(session, hdr.ReqID, 400, err); sendErr != nil {
					return sendErr
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgRegisterObject:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var regReq common.RegisterObjectInstancePayload
			if err := regReq.UnmarshalBinary(payload); err != nil || regReq.ClassHandle == 0 {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid register object request")); err != nil {
					return err
				}
				continue
			}
			if _, exists := currentFed.OMT().GetObjectClass(regReq.ClassHandle); !exists {
				if err := sendNativeError(session, hdr.ReqID, 404, common.ErrObjectClassNotDefined); err != nil {
					return err
				}
				continue
			}
			obj, err := currentFed.RegisterObjectInstance(int32(regReq.ClassHandle), regReq.ObjectName, federateID)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 409, err); err != nil {
					return err
				}
				continue
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
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			if err := sendNativeResponse(session, respHdr, respPayload[:], federateID); err != nil {
				return err
			}
			logCRCNative.Info("Object registered: handle=%d, class=%d, name=%s", obj.Handle, regReq.ClassHandle, obj.Name)

			discPayload, _ := common.DiscoverObjectInstancePayload{
				ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
				ClassHandle:  regReq.ClassHandle,
				ObjectName:   obj.Name,
			}.MarshalBinary()

			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					if err := f.Session.SendNotification(NativeMsgDiscoverObject, discPayload); err != nil {
						logCRCNative.Warn("failed to dispatch DiscoverObject to federate %d: %v", f.ID, err)
					}
				}
			}

		case NativeMsgDeleteObjectInstance:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var delReq common.DeleteObjectInstancePayload
			if err := delReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid delete object instance payload")); err != nil {
					return err
				}
				continue
			}
			obj, err := currentFed.DeleteObjectInstance(int32(delReq.ObjectHandle), federateID, delReq.Tag)
			if err != nil {
				if errors.Is(err, common.ErrObjectInstanceNotKnown) {
					if err := sendNativeError(session, hdr.ReqID, 404, err); err != nil {
						return err
					}
				} else if errors.Is(err, common.ErrDeletePrivilegeNotHeld) {
					if err := sendNativeError(session, hdr.ReqID, 403, err); err != nil {
						return err
					}
				} else {
					if err := sendNativeError(session, hdr.ReqID, 500, err); err != nil {
						return err
					}
				}
				continue
			}

			var globalRoot [32]byte
			respFlags := common.FlagNativeSSZ
			if currentFed.IsMerkleAuditEnabled() {
				globalRoot = currentFed.StateTree().GetGlobalRoot()
				respFlags |= common.FlagNativeMerkle
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      respFlags,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
				MerkleRoot: globalRoot,
			}
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			logCRCNative.Info("Object [%d] '%s' deleted by federate [%d]", obj.Handle, obj.Name, federateID)

			removePayload, _ := common.RemoveObjectInstancePayload{
				ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
				Tag:          delReq.Tag,
			}.MarshalBinary()

			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					if err := f.Session.SendNotification(NativeMsgRemoveObjectInstance, removePayload); err != nil {
						logCRCNative.Warn("failed to dispatch RemoveObjectInstance to federate %d: %v", f.ID, err)
					}
				}
			}

		case NativeMsgUpdateAttributes:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var uavReq common.UpdateAttributesPayload
			if err := uavReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid update attributes payload")); err != nil {
					return err
				}
				continue
			}
			obj, exists := currentFed.GetObjectInstance(int32(uavReq.ObjectHandle))
			if !exists {
				if err := sendNativeError(session, hdr.ReqID, 404, common.ErrObjectInstanceNotKnown); err != nil {
					return err
				}
				continue
			}

			attrsToStore := make(map[int32][]byte, len(uavReq.Attributes))
			for k, v := range uavReq.Attributes {
				attrsToStore[int32(k)] = v
			}
			if err := currentFed.UpdateObjectAttributes(int32(uavReq.ObjectHandle), attrsToStore, hdr.MerkleRoot); err != nil {
				logCRCNative.Warn("failed to update attributes for object %d: %v", uavReq.ObjectHandle, err)
				if sendErr := sendNativeError(session, hdr.ReqID, 400, err); sendErr != nil {
					return sendErr
				}
				continue
			}

			var globalRoot [32]byte
			respFlags := common.FlagNativeSSZ
			if currentFed.IsMerkleAuditEnabled() {
				globalRoot = currentFed.StateTree().GetGlobalRoot()
				respFlags |= common.FlagNativeMerkle
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      respFlags,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
				MerkleRoot: globalRoot,
			}
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

			// Intelligent subscriber routing via Router (InterestMatcher)
			targetSubs := make(map[common.FederateHandle]bool)
			for attrHandle := range uavReq.Attributes {
				subs := currentFed.Router().GetAttributeSubscribers(common.ObjectClassHandle(obj.ClassHandle), attrHandle)
				for _, sub := range subs {
					if sub != common.FederateHandle(federateID) {
						targetSubs[sub] = true
					}
				}
			}

			// Dispatch reflections to federates with active subscriptions (TSO or RO)
			for subID := range targetSubs {
				if f, exists := currentFed.GetFederate(int32(subID)); exists && f.Session != nil {
					if uavReq.HasTime && f.IsConstrained {
						grants, err := currentFed.TimeManager().EnqueueTSOMessage(core.TSOMessage{
							Sender:     common.FederateHandle(federateID),
							Receiver:   subID,
							Timestamp:  uavReq.Time,
							MsgType:    NativeMsgReflectAttributes,
							Payload:    payload,
							MerkleRoot: globalRoot,
						})
						if err == nil && len(grants) > 0 {
							dispatchTimeGrants(currentFed, grants)
						}
					} else {
						if err := f.Session.SendNotificationWithMerkle(NativeMsgReflectAttributes, payload, globalRoot); err != nil {
							logCRCNative.Warn("failed to dispatch ReflectAttributes to federate %d: %v", f.ID, err)
						}
					}
				}
			}

		case NativeMsgSendInteraction:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var interReq common.SendInteractionPayload
			if err := interReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid send interaction payload")); err != nil {
					return err
				}
				continue
			}
			if _, exists := currentFed.OMT().GetInteractionClass(interReq.ClassHandle); !exists {
				if err := sendNativeError(session, hdr.ReqID, 404, common.ErrInteractionClassNotDefined); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

			// Polymorphic interaction routing via Router (TSO or RO)
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
							if err := f.Session.SendNotification(NativeMsgReceiveInteraction, payload); err != nil {
								logCRCNative.Warn("failed to dispatch ReceiveInteraction to federate %d: %v", f.ID, err)
							}
						}
					}
				}
			}

		case NativeMsgEnableTimeRegulation:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var timeReq common.TimeFloat64Payload
			if err := timeReq.UnmarshalBinary(payload); err != nil || timeReq.Time < 0 {
				if err := sendNativeError(session, hdr.ReqID, 400, common.ErrInvalidLookahead); err != nil {
					return err
				}
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				if err := sendNativeError(session, hdr.ReqID, 404, common.ErrFederateNotFound); err != nil {
					return err
				}
				continue
			}
			if err := currentFed.TimeManager().EnableTimeRegulation(common.FederateHandle(federateID), timeReq.Time); err != nil {
				if errors.Is(err, common.ErrTimeRegulationAlreadyEnabled) {
					if err := sendNativeError(session, hdr.ReqID, 409, err); err != nil {
						return err
					}
				} else {
					if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
						return err
					}
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
			if err := sendNativeResponse(session, respHdr, respData, federateID); err != nil {
				return err
			}
			logCRCNative.Info("Time regulation enabled for federate [%d], lookahead=%f", federateID, fed.Lookahead)

		case NativeMsgDisableTimeRegulation:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			grants, err := currentFed.TimeManager().DisableTimeRegulation(common.FederateHandle(federateID))
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			dispatchTimeGrants(currentFed, grants)

		case NativeMsgEnableTimeConstrained:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				if err := sendNativeError(session, hdr.ReqID, 404, common.ErrFederateNotFound); err != nil {
					return err
				}
				continue
			}
			if err := currentFed.TimeManager().EnableTimeConstrained(common.FederateHandle(federateID)); err != nil {
				if errors.Is(err, common.ErrTimeConstrainedAlreadyEnabled) {
					if err := sendNativeError(session, hdr.ReqID, 409, err); err != nil {
						return err
					}
				} else {
					if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
						return err
					}
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
			if err := sendNativeResponse(session, respHdr, respData, federateID); err != nil {
				return err
			}
			logCRCNative.Info("Time constrained enabled for federate [%d]", federateID)

		case NativeMsgDisableTimeConstrained:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			grants, err := currentFed.TimeManager().DisableTimeConstrained(common.FederateHandle(federateID))
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			dispatchTimeGrants(currentFed, grants)

		case NativeMsgTimeAdvanceRequest:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var tarReq common.TimeFloat64Payload
			if err := tarReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid time advance request")); err != nil {
					return err
				}
				continue
			}
			grants, err := currentFed.TimeManager().TimeAdvanceRequest(common.FederateHandle(federateID), tarReq.Time)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			dispatchTimeGrants(currentFed, grants)

		case NativeMsgNextMessageRequest:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var nmrReq common.TimeFloat64Payload
			if err := nmrReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid next message request")); err != nil {
					return err
				}
				continue
			}
			grants, err := currentFed.TimeManager().NextMessageRequest(common.FederateHandle(federateID), nmrReq.Time)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			dispatchTimeGrants(currentFed, grants)

		case NativeMsgFlushQueueRequest:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var flushReq common.TimeFloat64Payload
			if err := flushReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid flush queue request")); err != nil {
					return err
				}
				continue
			}
			grants, err := currentFed.TimeManager().FlushQueueRequest(common.FederateHandle(federateID), flushReq.Time)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			dispatchTimeGrants(currentFed, grants)

		case NativeMsgHeartbeat:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			currentFed.TimeManager().Heartbeat(common.FederateHandle(federateID))
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgDivestOwnership:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var req common.DivestOwnershipPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid divest ownership payload")); err != nil {
					return err
				}
				continue
			}
			attrs := convertToAttrHandles(req.Attributes)
			if err := currentFed.OwnershipManager().UnconditionalDivestiture(common.ObjectInstanceHandle(req.ObjectHandle), attrs, common.FederateHandle(federateID)); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgAcquireOwnership:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var req common.AcquireOwnershipPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid acquire ownership payload")); err != nil {
					return err
				}
				continue
			}
			attrs := convertToAttrHandles(req.Attributes)
			if err := currentFed.OwnershipManager().AcquireAttributeOwnership(common.ObjectInstanceHandle(req.ObjectHandle), attrs, common.FederateHandle(federateID)); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgQueryAttributeOwnership:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var req common.QueryOwnershipPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid query ownership payload")); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, respPayload, federateID); err != nil {
				return err
			}

		case NativeMsgCreateRegion:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var req common.CreateRegionPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid create region payload")); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, respData, federateID); err != nil {
				return err
			}

		case NativeMsgSetRangeBounds:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var req common.SetRangeBoundsPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid set range bounds payload")); err != nil {
					return err
				}
				continue
			}
			if err := currentFed.SetRegionRangeBounds(req.RegionHandle, req.DimensionHandle, req.LowerBound, req.UpperBound); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgSetMetricBounds:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var req common.SetMetricBoundsPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid set metric bounds payload")); err != nil {
					return err
				}
				continue
			}
			if err := currentFed.SetRegionMetricBounds(req.RegionHandle, req.MinX, req.MaxX, req.MinY, req.MaxY, req.MinZ, req.MaxZ); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgSubscribeWithRegion:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var req common.SubscribeWithRegionPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid subscribe with region payload")); err != nil {
					return err
				}
				continue
			}
			region, exists := currentFed.GetRegion(req.RegionHandle)
			if !exists {
				if err := sendNativeError(session, hdr.ReqID, 404, errors.New("region handle not found")); err != nil {
					return err
				}
				continue
			}
			attrs := convertToAttrHandles(req.Attributes)
			if err := currentFed.Router().SubscribeObjectClassWithRegion(common.FederateHandle(federateID), common.ObjectClassHandle(req.ObjectClass), attrs, region); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, err); err != nil {
					return err
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

		case NativeMsgUpdateAttributesSpatial:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var req common.UpdateAttributesSpatialPayload
			if err := req.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid spatial update attributes payload")); err != nil {
					return err
				}
				continue
			}
			obj, exists := currentFed.GetObjectInstance(int32(req.ObjectHandle))
			if !exists {
				if err := sendNativeError(session, hdr.ReqID, 404, common.ErrObjectInstanceNotKnown); err != nil {
					return err
				}
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
				if err := sendNativeError(session, hdr.ReqID, 403, core.ErrNotAttributeOwner); err != nil {
					return err
				}
				continue
			}

			attrsToStore := make(map[int32][]byte, len(req.Attributes))
			for k, v := range req.Attributes {
				attrsToStore[int32(k)] = v
			}
			if err := currentFed.UpdateObjectAttributes(int32(req.ObjectHandle), attrsToStore, hdr.MerkleRoot); err != nil {
				logCRCNative.Warn("failed to update attributes with region for object %d: %v", req.ObjectHandle, err)
				if sendErr := sendNativeError(session, hdr.ReqID, 400, err); sendErr != nil {
					return sendErr
				}
				continue
			}

			var globalRoot [32]byte
			respFlags := common.FlagNativeSSZ
			if currentFed.IsMerkleAuditEnabled() {
				globalRoot = currentFed.StateTree().GetGlobalRoot()
				respFlags |= common.FlagNativeMerkle
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      respFlags,
				MsgType:    NativeMsgSuccessOK,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
				MerkleRoot: globalRoot,
			}
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}

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
					if err := f.Session.SendNotificationWithMerkle(NativeMsgReflectAttributes, ravPayload, globalRoot); err != nil {
						logCRCNative.Warn("failed to dispatch spatial ReflectAttributes to federate %d: %v", f.ID, err)
					}
				}
			}

		case common.NativeMsgGetObjectClassHandle:
			if currentFed == nil {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var qReq common.GetObjectClassHandlePayload
			if err := qReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid get object class handle payload")); err != nil {
					return err
				}
				continue
			}
			cls, ok := currentFed.OMT().GetObjectClassByName(qReq.ClassName)
			if !ok {
				if err := sendNativeError(session, hdr.ReqID, 404, core.ErrClassNotFound); err != nil {
					return err
				}
				continue
			}
			respPayload, _ := common.HandleResponsePayload{Handle: int32(cls.Handle)}.MarshalBinary()
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    common.NativeMsgGetObjectClassHandleResp,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
				PayloadLen: uint32(len(respPayload)),
			}
			if err := sendNativeResponse(session, respHdr, respPayload, federateID); err != nil {
				return err
			}

		case common.NativeMsgGetAttributeHandle:
			if currentFed == nil {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var qReq common.GetAttributeHandlePayload
			if err := qReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid get attribute handle payload")); err != nil {
					return err
				}
				continue
			}
			attr, err := currentFed.OMT().ResolveAttributeByName(common.ObjectClassHandle(qReq.ClassHandle), qReq.AttributeName)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 404, err); err != nil {
					return err
				}
				continue
			}
			respPayload, _ := common.HandleResponsePayload{Handle: int32(attr.Handle)}.MarshalBinary()
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    common.NativeMsgGetAttributeHandleResp,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
				PayloadLen: uint32(len(respPayload)),
			}
			if err := sendNativeResponse(session, respHdr, respPayload, federateID); err != nil {
				return err
			}

		case common.NativeMsgGetDimensionHandle:
			if currentFed == nil {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var qReq common.GetDimensionHandlePayload
			if err := qReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid get dimension handle payload")); err != nil {
					return err
				}
				continue
			}
			dim, ok := currentFed.OMT().GetDimensionByName(qReq.DimensionName)
			if !ok {
				if err := sendNativeError(session, hdr.ReqID, 404, errors.New("dimension not found")); err != nil {
					return err
				}
				continue
			}
			respPayload, _ := common.HandleResponsePayload{Handle: int32(dim.Handle)}.MarshalBinary()
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				MsgType:    common.NativeMsgGetDimensionHandleResp,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
				PayloadLen: uint32(len(respPayload)),
			}
			if err := sendNativeResponse(session, respHdr, respPayload, federateID); err != nil {
				return err
			}

		case common.NativeMsgGetMerkleProof:
			if currentFed == nil || federateID <= 0 {
				if err := sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember); err != nil {
					return err
				}
				continue
			}
			var qReq common.GetMerkleProofPayload
			if err := qReq.UnmarshalBinary(payload); err != nil {
				if err := sendNativeError(session, hdr.ReqID, 400, errors.New("invalid get merkle proof payload")); err != nil {
					return err
				}
				continue
			}
			proof, err := currentFed.StateTree().GenerateProof(qReq.ObjectHandle)
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 404, err); err != nil {
					return err
				}
				continue
			}
			respPayload, err := common.MerkleProofResponsePayload{
				ObjectHandle: qReq.ObjectHandle,
				LeafIndex:    proof.LeafIndex,
				GlobalRoot:   proof.GlobalRoot,
				LeafHash:     proof.LeafHash,
				AuditPath:    proof.AuditPath,
			}.MarshalBinary()
			if err != nil {
				if err := sendNativeError(session, hdr.ReqID, 500, err); err != nil {
					return err
				}
				continue
			}
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      common.FlagNativeSSZ | common.FlagNativeMerkle,
				MsgType:    common.NativeMsgGetMerkleProofResp,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
				PayloadLen: uint32(len(respPayload)),
				MerkleRoot: proof.GlobalRoot,
			}
			if err := sendNativeResponse(session, respHdr, respPayload, federateID); err != nil {
				return err
			}

		case NativeMsgResign:
			if currentFed != nil && federateID > 0 {
				if _, err := currentFed.Resign(federateID); err != nil {
					logCRCNative.Warn("failed to resign federate %d: %v", federateID, err)
				}
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
			if err := sendNativeResponse(session, respHdr, nil, federateID); err != nil {
				return err
			}
			logCRCNative.Info("Federate [%d] resigned gracefully", federateID)
			currentFed = nil
			federateID = -1
		}
	}
}
