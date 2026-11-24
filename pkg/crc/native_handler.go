package crc

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
)

// Tipos de Mensagem do Protocolo Nativo Valhalla (HLA-4)
const (
	NativeMsgJoin                   uint8 = 1
	NativeMsgJoinSuccess            uint8 = 2
	NativeMsgRegisterSyncPoint      uint8 = 3
	NativeMsgAchieveSyncPoint       uint8 = 4
	NativeMsgSyncPointAchieved      uint8 = 5
	NativeMsgFederationSynchronized uint8 = 6
	NativeMsgRegisterObject         uint8 = 7
	NativeMsgDiscoverObject         uint8 = 8
	NativeMsgUpdateAttributes       uint8 = 9
	NativeMsgReflectAttributes      uint8 = 10
	NativeMsgSendInteraction        uint8 = 11
	NativeMsgReceiveInteraction     uint8 = 12
	NativeMsgTimeAdvanceRequest     uint8 = 13
	NativeMsgTimeAdvanceGrant       uint8 = 14
	NativeMsgResign                 uint8 = 15
	NativeMsgSuccessOK              uint8 = 16
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
			fedName := "ExampleFederation"
			nodeName := string(payload)
			if nodeName == "" {
				nodeName = "ValhallaNativeNode"
			}

			currentFed = fedManager.GetOrCreateFederation(fedName)
			fed, err := currentFed.Join(nodeName, "native-hla4", ProtocolNative, session)
			if err != nil {
				log.Printf("[CRC-Native] Join error for %s: %v", nodeName, err)
				continue
			}

			federateID = fed.ID
			session.SetFederateID(federateID)
			session.SetFederationID(currentFed.Handle)

			// Reply with JoinSuccess containing assigned handle
			respHdr := NativeHeader{
				Magic:      NativeHeaderMagic,
				Version:    NativeProtocolVer,
				Flags:      0x01,
				PayloadLen: 4,
				MsgType:    NativeMsgJoinSuccess,
				ReqID:      hdr.ReqID,
				FedID:      uint32(currentFed.Handle),
				FederateID: uint32(federateID),
			}
			var idBuf [4]byte
			idBuf[0] = byte(federateID >> 24)
			idBuf[1] = byte(federateID >> 16)
			idBuf[2] = byte(federateID >> 8)
			idBuf[3] = byte(federateID)

			_ = session.SendMessage(respHdr, idBuf[:])
			log.Printf("[CRC-Native] Federate [%s] joined with handle [%d]", nodeName, federateID)

		case NativeMsgRegisterSyncPoint:
			label := string(payload)
			if currentFed != nil {
				_, _ = currentFed.RegisterSyncPoint(label, nil, federateID)
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
			log.Printf("[CRC-Native] Federate [%d] registered synchronization point '%s'", federateID, label)

		case NativeMsgAchieveSyncPoint:
			label := string(payload)
			if label == "" {
				label = "ReadyToRun"
			}

			var isSynced bool
			if currentFed != nil {
				isSynced, _ = currentFed.AchieveSyncPoint(label, federateID)
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
			log.Printf("[CRC-Native] Federate [%d] achieved synchronization point '%s'", federateID, label)

			if isSynced {
				log.Printf("[CRC-Native] >>> FEDERATION SYNCHRONIZED at point '%s'!", label)
				if currentFed != nil {
					for _, f := range currentFed.GetAllFederates() {
						if f.Session != nil {
							_ = f.Session.SendNotification(NativeMsgFederationSynchronized, []byte(label))
						}
					}
				}
			}

		case NativeMsgRegisterObject:
			var classHandle int32
			var objName string
			if len(payload) >= 6 {
				classHandle = int32(uint32(payload[0])<<24 | uint32(payload[1])<<16 | uint32(payload[2])<<8 | uint32(payload[3]))
				nameLen := int(uint16(payload[4])<<8 | uint16(payload[5]))
				if len(payload) >= 6+nameLen {
					objName = string(payload[6 : 6+nameLen])
				}
			}

			var objHandle int32 = 1
			if currentFed != nil {
				obj, err := currentFed.RegisterObjectInstance(classHandle, objName, federateID)
				if err == nil {
					objHandle = obj.Handle
					objName = obj.Name
				}
			}

			var respPayload [4]byte
			respPayload[0] = byte(objHandle >> 24)
			respPayload[1] = byte(objHandle >> 16)
			respPayload[2] = byte(objHandle >> 8)
			respPayload[3] = byte(objHandle)

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
			log.Printf("[CRC-Native] Object registered: handle=%d, class=%d, name=%s", objHandle, classHandle, objName)

			if currentFed != nil {
				discPayload := make([]byte, 10+len(objName))
				discPayload[0] = byte(objHandle >> 24)
				discPayload[1] = byte(objHandle >> 16)
				discPayload[2] = byte(objHandle >> 8)
				discPayload[3] = byte(objHandle)
				discPayload[4] = byte(classHandle >> 24)
				discPayload[5] = byte(classHandle >> 16)
				discPayload[6] = byte(classHandle >> 8)
				discPayload[7] = byte(classHandle)
				discPayload[8] = byte(len(objName) >> 8)
				discPayload[9] = byte(len(objName))
				copy(discPayload[10:], []byte(objName))

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
