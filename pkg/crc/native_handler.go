package crc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/icemagno/valhalla/pkg/common"
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
	NativeMsgRegisterObject         = common.NativeMsgRegisterObject
	NativeMsgDiscoverObject         = common.NativeMsgDiscoverObject
	NativeMsgUpdateAttributes       = common.NativeMsgUpdateAttributes
	NativeMsgReflectAttributes      = common.NativeMsgReflectAttributes
	NativeMsgSendInteraction        = common.NativeMsgSendInteraction
	NativeMsgReceiveInteraction     = common.NativeMsgReceiveInteraction
	NativeMsgEnableTimeRegulation   = common.NativeMsgEnableTimeRegulation
	NativeMsgTimeRegulationEnabled  = common.NativeMsgTimeRegulationEnabled
	NativeMsgEnableTimeConstrained  = common.NativeMsgEnableTimeConstrained
	NativeMsgTimeConstrainedEnabled = common.NativeMsgTimeConstrainedEnabled
	NativeMsgTimeAdvanceRequest     = common.NativeMsgTimeAdvanceRequest
	NativeMsgTimeAdvanceGrant       = common.NativeMsgTimeAdvanceGrant
	NativeMsgResign                 = common.NativeMsgResign
	NativeMsgSuccessOK              = common.NativeMsgSuccessOK
	NativeMsgError                  = common.NativeMsgError
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
			log.Printf("[CRC-Native] Federation [%s] created successfully", req.FederationName)

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
			log.Printf("[CRC-Native] Federation [%s] destroyed successfully", req.FederationName)

		case NativeMsgJoin:
			var joinReq common.JoinFederationPayload
			if err := joinReq.UnmarshalBinary(payload); err != nil || joinReq.FederateName == "" || joinReq.FederationName == "" {
				_ = sendNativeError(session, hdr.ReqID, 400, errors.New("invalid join request: federateName and federationName are mandatory"))
				continue
			}

			fedExec, exists := fedManager.GetFederation(joinReq.FederationName)
			if !exists {
				_ = sendNativeError(session, hdr.ReqID, 404, common.ErrFederationExecutionDoesNotExist)
				continue
			}

			fed, err := fedExec.Join(joinReq.FederateName, joinReq.FederateType, ProtocolNative, session)
			if err != nil {
				log.Printf("[CRC-Native] Join error for %s: %v", joinReq.FederateName, err)
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
			log.Printf("[CRC-Native] Federate [%s] joined federation [%s] with handle [%d]", joinReq.FederateName, joinReq.FederationName, federateID)

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
			log.Printf("[CRC-Native] Federate [%d] registered synchronization point '%s'", federateID, syncReq.Label)

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
			log.Printf("[CRC-Native] Federate [%d] achieved synchronization point '%s'", federateID, labelReq.Label)

			if isSynced {
				log.Printf("[CRC-Native] >>> FEDERATION SYNCHRONIZED at point '%s'!", labelReq.Label)
				syncNotif, _ := common.SyncPointLabelPayload{Label: labelReq.Label}.MarshalBinary()
				for _, f := range currentFed.GetAllFederates() {
					if f.Session != nil {
						_ = f.Session.SendNotification(NativeMsgFederationSynchronized, syncNotif)
					}
				}
			}

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
			log.Printf("[CRC-Native] Object registered: handle=%d, class=%d, name=%s", obj.Handle, regReq.ClassHandle, obj.Name)

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

		case NativeMsgUpdateAttributes:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
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

			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					_ = f.Session.SendNotification(NativeMsgReflectAttributes, payload)
				}
			}

		case NativeMsgSendInteraction:
			if currentFed == nil || federateID <= 0 {
				_ = sendNativeError(session, hdr.ReqID, 403, common.ErrFederateNotExecutionMember)
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

			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					_ = f.Session.SendNotification(NativeMsgReceiveInteraction, payload)
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
			if fed.IsRegulating {
				_ = sendNativeError(session, hdr.ReqID, 409, common.ErrTimeRegulationAlreadyEnabled)
				continue
			}
			fed.IsRegulating = true
			fed.Lookahead = timeReq.Time

			respData, _ := common.TimeFloat64Payload{Time: fed.CurrentTime}.MarshalBinary()
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
			log.Printf("[CRC-Native] Time regulation enabled for federate [%d], lookahead=%f", federateID, fed.Lookahead)

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
			if fed.IsConstrained {
				_ = sendNativeError(session, hdr.ReqID, 409, common.ErrTimeConstrainedAlreadyEnabled)
				continue
			}
			fed.IsConstrained = true

			respData, _ := common.TimeFloat64Payload{Time: fed.CurrentTime}.MarshalBinary()
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
			log.Printf("[CRC-Native] Time constrained enabled for federate [%d]", federateID)

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
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				_ = sendNativeError(session, hdr.ReqID, 404, common.ErrFederateNotFound)
				continue
			}
			if tarReq.Time < fed.CurrentTime {
				_ = sendNativeError(session, hdr.ReqID, 400, common.ErrInvalidLogicalTime)
				continue
			}
			fed.IsAdvancing = true
			fed.TargetTime = tarReq.Time

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

			// Concede o avanço do federado
			fed.CurrentTime = fed.TargetTime
			fed.IsAdvancing = false
			grantData, _ := common.TimeFloat64Payload{Time: fed.CurrentTime}.MarshalBinary()
			_ = session.SendNotification(NativeMsgTimeAdvanceGrant, grantData)

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
			log.Printf("[CRC-Native] Federate [%d] resigned gracefully", federateID)
			currentFed = nil
			federateID = -1
		}
	}
}
