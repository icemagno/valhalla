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
			return fmt.Errorf("erro lendo cabecalho nativo: %w", err)
		}

		var payload []byte
		if hdr.PayloadLen > 0 {
			payload = make([]byte, hdr.PayloadLen)
			if _, err := io.ReadFull(reader, payload); err != nil {
				return fmt.Errorf("erro lendo payload nativo (%d bytes): %w", hdr.PayloadLen, err)
			}
		}

		switch hdr.MsgType {
		case NativeMsgJoin:
			// Formato simples de Join nativo: Nome do Federado em bytes
			fedName := "ExampleFederation"
			nodeName := string(payload)
			if nodeName == "" {
				nodeName = "ValhallaNativeNode"
			}

			currentFed = fedManager.GetOrCreateFederation(fedName)
			fed, err := currentFed.Join(nodeName, "native-hla4", ProtocolNative, session)
			if err != nil {
				log.Printf("[CRC-Native] Erro no join de %s: %v", nodeName, err)
				continue
			}

			federateID = fed.ID
			session.SetFederateID(federateID)
			session.SetFederationID(currentFed.Handle)

			// Responder com JoinSuccess contendo o ID atribuído
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
			log.Printf("[CRC-Native] Federado nativo [%s] ingressou com handle [%d]", nodeName, federateID)

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
			log.Printf("[CRC-Native] Federado nativo [%d] conquistou ponto '%s'", federateID, label)

			if isSynced {
				log.Printf("[CRC-Native] >>> FEDERACAO SINCRONIZADA no ponto '%s'!", label)
				// Broadcast para TODOS os federados (Nativos e Portico!)
				if currentFed != nil {
					for _, f := range currentFed.GetAllFederates() {
						if f.Session != nil {
							_ = f.Session.SendNotification(NativeMsgFederationSynchronized, []byte(label))
						}
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

			// Replicar atributos para todos os federados na federação
			if currentFed != nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.ID != federateID && f.Session != nil {
						_ = f.Session.SendNotification(NativeMsgReflectAttributes, payload)
					}
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
			log.Printf("[CRC-Native] Federado nativo [%d] desingressou graciosamente (Resign)", federateID)
			return nil
		}
	}
}
