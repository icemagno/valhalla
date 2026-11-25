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
	NativeMsgJoin                   = common.NativeMsgJoin
	NativeMsgJoinSuccess            = common.NativeMsgJoinSuccess
	NativeMsgRegisterSyncPoint      = common.NativeMsgRegisterSyncPoint
	NativeMsgAchieveSyncPoint       = common.NativeMsgAchieveSyncPoint
	NativeMsgSyncPointAchieved      = common.NativeMsgSyncPointAchieved
	NativeMsgFederationSynchronized = common.NativeMsgFederationSynchronized
	NativeMsgRegisterObject         = common.NativeMsgRegisterObject
	NativeMsgDiscoverObject         = common.NativeMsgDiscoverObject
	NativeMsgUpdateAttributes       = common.NativeMsgUpdateAttributes
	NativeMsgReflectAttributes      = common.NativeMsgReflectAttributes
	NativeMsgSendInteraction        = common.NativeMsgSendInteraction
	NativeMsgReceiveInteraction     = common.NativeMsgReceiveInteraction
	NativeMsgTimeAdvanceRequest     = common.NativeMsgTimeAdvanceRequest
	NativeMsgTimeAdvanceGrant       = common.NativeMsgTimeAdvanceGrant
	NativeMsgResign                 = common.NativeMsgResign
	NativeMsgSuccessOK              = common.NativeMsgSuccessOK
)

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
		case NativeMsgJoin:
			var joinReq common.JoinFederationPayload
			if err := joinReq.UnmarshalBinary(payload); err != nil {
				// Fallback caso venha apenas string bruta de compatibilidade
				joinReq.FederateName = string(payload)
				if joinReq.FederateName == "" {
					joinReq.FederateName = "ValhallaNativeNode"
				}
				joinReq.FederationName = "ExampleFederation"
			}
			if joinReq.FederationName == "" {
				joinReq.FederationName = "ExampleFederation"
			}

			currentFed = fedManager.GetOrCreateFederation(joinReq.FederationName)
			fed, err := currentFed.Join(joinReq.FederateName, "native-hla4", ProtocolNative, session)
			if err != nil {
				log.Printf("[CRC-Native] Join error for %s: %v", joinReq.FederateName, err)
				errPayload, _ := common.ErrorResponsePayload{ErrorCode: 1, Message: err.Error()}.MarshalBinary()
				respHdr := NativeHeader{
					Magic:      NativeHeaderMagic,
					Version:    NativeProtocolVer,
					Flags:      0x01,
					PayloadLen: uint32(len(errPayload)),
					MsgType:    common.NativeMsgError,
					ReqID:      hdr.ReqID,
				}
				_ = session.SendMessage(respHdr, errPayload)
				continue
			}

			federateID = fed.ID
			session.SetFederateID(federateID)
			session.SetFederationID(currentFed.Handle)

			// Reply with JoinSuccess contendo JoinFederationResponsePayload tipado
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
			log.Printf("[CRC-Native] Federate [%s] joined with handle [%d]", joinReq.FederateName, federateID)

		case NativeMsgRegisterSyncPoint:
			var syncReq common.RegisterSyncPointPayload
			if err := syncReq.UnmarshalBinary(payload); err != nil {
				syncReq.Label = string(payload)
			}
			if currentFed != nil {
				_, _ = currentFed.RegisterSyncPoint(syncReq.Label, syncReq.Tag, federateID)
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

		case NativeMsgAchieveSyncPoint:
			var labelReq common.SyncPointLabelPayload
			if err := labelReq.UnmarshalBinary(payload); err != nil {
				labelReq.Label = string(payload)
			}
			if labelReq.Label == "" {
				labelReq.Label = "ReadyToRun"
			}

			var isSynced bool
			if currentFed != nil {
				isSynced, _ = currentFed.AchieveSyncPoint(labelReq.Label, federateID)
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
				if currentFed != nil {
					for _, f := range currentFed.GetAllFederates() {
						if f.Session != nil {
							_ = f.Session.SendNotification(NativeMsgFederationSynchronized, syncNotif)
						}
					}
				}
			}

		case NativeMsgRegisterObject:
			var regReq common.RegisterObjectInstancePayload
			if err := regReq.UnmarshalBinary(payload); err != nil {
				if len(payload) >= 6 {
					regReq.ClassHandle = common.ObjectClassHandle(binary.BigEndian.Uint32(payload[0:4]))
					nameLen := int(binary.BigEndian.Uint16(payload[4:6]))
					if len(payload) >= 6+nameLen {
						regReq.ObjectName = string(payload[6 : 6+nameLen])
					}
				}
			}

			var objHandle int32 = 1
			if currentFed != nil {
				obj, err := currentFed.RegisterObjectInstance(int32(regReq.ClassHandle), regReq.ObjectName, federateID)
				if err == nil {
					objHandle = obj.Handle
					regReq.ObjectName = obj.Name
				}
			}

			var respPayload [4]byte
			binary.BigEndian.PutUint32(respPayload[0:4], uint32(objHandle))

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
			log.Printf("[CRC-Native] Object registered: handle=%d, class=%d, name=%s", objHandle, regReq.ClassHandle, regReq.ObjectName)

			if currentFed != nil {
				discPayload, _ := common.DiscoverObjectInstancePayload{
					ObjectHandle: common.ObjectInstanceHandle(objHandle),
					ClassHandle:  regReq.ClassHandle,
					ObjectName:   regReq.ObjectName,
				}.MarshalBinary()

				for _, f := range currentFed.GetAllFederates() {
					if f.ID != federateID && f.Session != nil {
						_ = f.Session.SendNotification(NativeMsgDiscoverObject, discPayload)
					}
				}
			}

		case NativeMsgUpdateAttributes:
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

			if currentFed != nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.ID != federateID && f.Session != nil {
						_ = f.Session.SendNotification(NativeMsgReflectAttributes, payload)
					}
				}
			}

		case NativeMsgSendInteraction:
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

			if currentFed != nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.ID != federateID && f.Session != nil {
						_ = f.Session.SendNotification(NativeMsgReceiveInteraction, payload)
					}
				}
			}

		case NativeMsgTimeAdvanceRequest:
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

			_ = session.SendNotification(NativeMsgTimeAdvanceGrant, payload)

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
