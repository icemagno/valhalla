package crc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/icemagno/valhalla/pkg/portico/wire"
)

const (
	PorticoWelcomeMsg = "Portico Router (3.0.0 (build 0)): Valhalla CRC Sovereign Gateway"
)

// HandlePorticoConnection atende uma conexão roteada pelo multiplexador como dialeto Portico
func HandlePorticoConnection(conn net.Conn, reader io.Reader, fedManager *FederationManager) error {
	session := NewPorticoSession(conn)
	defer session.Close()

	// 1. Executar o Handshake Server-First exigido pelo Portico (0xBEEF / 0xFEED)
	if err := performPorticoHandshake(conn, session.writer, reader); err != nil {
		return fmt.Errorf("falha no handshake inicial do Portico: %w", err)
	}

	var currentFed *FederationExecution
	var federateID int32 = -1

	// 2. Loop de despacho de pacotes de controle e dados
	for {
		hdr, payload, err := session.ReadPacket(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				// Conexão encerrada normalmente pelo cliente
				if currentFed != nil && federateID > 0 {
					_, _ = currentFed.Resign(federateID)
				}
				return nil
			}
			return fmt.Errorf("erro lendo pacote Portico: %w", err)
		}

		switch hdr.MessageType {
		case wire.MsgCreateFederation:
			fedName := "ExampleFederation"
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil {
				if n, ok := obj.Get("federationName").(string); ok && n != "" {
					fedName = n
				}
			}
			currentFed = fedManager.GetOrCreateFederation(fedName)
			respPayload := wire.BuildCreateFederationResponsePayload(currentFed.Handle, fedName)
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload)
			log.Printf("[CRC-Portico] Federation [%s] created with handle [%d]", fedName, currentFed.Handle)

		case wire.MsgJoinFederation:
			fedName := "ExampleFederation"
			nodeName := fmt.Sprintf("PorticoNode_%d", time.Now().UnixNano()%100000)
			nodeType := "ieee1516e"

			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil {
				if n, ok := obj.Get("federationName").(string); ok && n != "" {
					fedName = n
				}
				if n, ok := obj.Get("federateName").(string); ok && n != "" {
					nodeName = n
				}
				if n, ok := obj.Get("federateType").(string); ok && n != "" {
					nodeType = n
				}
			}

			currentFed = fedManager.GetOrCreateFederation(fedName)
			fed, err := currentFed.Join(nodeName, nodeType, ProtocolPortico, session)
			if err != nil {
				// Resposta de erro se nome em uso
				log.Printf("[CRC-Portico] Join error for %s: %v", nodeName, err)
				_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
				continue
			}

			federateID = fed.ID
			session.SetFederateID(federateID)
			session.SetFederationHandle(currentFed.Handle)

			wpPayload, err := wire.BuildWelcomePackPayload(currentFed.Handle, federateID)
			if err != nil {
				log.Printf("[CRC-Portico] Error generating WelcomePack: %v", err)
				wpPayload = wire.BuildSuccessResponsePayload()
			}

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wpPayload)
			log.Printf("[CRC-Portico] Federate [%s] joined with handle [%d] to federation [%s]", nodeName, federateID, fedName)

		case wire.MsgRegisterSyncPoint:
			label := "ReadyToRun"
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil {
				if l, ok := obj.Get("label").(string); ok && l != "" {
					label = l
				}
			}

			if currentFed != nil {
				_, _ = currentFed.RegisterSyncPoint(label, nil, federateID)
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			log.Printf("[CRC-Portico] Synchronization point '%s' registered by federate [%d]", label, federateID)

		case wire.MsgAchieveSyncPoint:
			label := "ReadyToRun"
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil {
				if l, ok := obj.Get("label").(string); ok && l != "" {
					label = l
				}
			}

			isSynced := false
			if currentFed != nil {
				var err error
				isSynced, err = currentFed.AchieveSyncPoint(label, federateID)
				if err != nil {
					log.Printf("[CRC-Portico] Warning achieving sync point: %v", err)
				}
			}

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			log.Printf("[CRC-Portico] Federate [%d] achieved synchronization point '%s'", federateID, label)

			if isSynced {
				log.Printf("[CRC-Portico] >>> FEDERATION SYNCHRONIZED at point '%s'!", label)
				// Broadcast FederationSynchronized para todos os federados da federação
				if currentFed != nil {
					for _, f := range currentFed.GetAllFederates() {
						if f.Session != nil {
							_ = f.Session.SendNotification(uint8(wire.MsgSyncPointFederation), wire.BuildSuccessResponsePayload())
						}
					}
				}
			}

		case wire.MsgPublishObjectClass, wire.MsgSubscribeObjectClass,
			wire.MsgPublishInteractionClass, wire.MsgSubscribeInteractionClass:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

		case wire.MsgRegisterObject:
			// Registrar nova instancia de objeto e notificar federados
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			if currentFed != nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.ID != federateID && f.Session != nil {
						_ = f.Session.SendNotification(uint8(wire.MsgDiscoverObject), payload)
					}
				}
			}

		case wire.MsgUpdateAttributes:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			if currentFed != nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.ID != federateID && f.Session != nil {
						_ = f.Session.SendNotification(uint8(wire.MsgReflectAttributes), payload)
					}
				}
			}

		case wire.MsgSendInteraction:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			if currentFed != nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.ID != federateID && f.Session != nil {
						_ = f.Session.SendNotification(uint8(wire.MsgReceiveInteraction), payload)
					}
				}
			}

		case wire.MsgTimeAdvanceRequest:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			// Conceder avanço temporal (TimeAdvanceGrant)
			_ = session.SendNotification(uint8(wire.MsgTimeAdvanceGrant), wire.BuildSuccessResponsePayload())

		case wire.MsgResignFederation:
			if currentFed != nil && federateID > 0 {
				_, _ = currentFed.Resign(federateID)
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			log.Printf("[CRC-Portico] Federate [%d] resigned gracefully", federateID)
			return nil

		default:
			// Responder OK para chamadas genéricas
			if hdr.CallType == wire.CallTypeControlRequest {
				_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			}
		}
	}
}

// performPorticoHandshake executa a saudação binária oficial do Portico (0xBEEF -> 0xFEED)
func performPorticoHandshake(conn net.Conn, writer io.Writer, reader io.Reader) error {
	// 1. Escrever o Magic de boas-vindas do Servidor (0xBEEF = 48879)
	var prefix [8]byte
	binary.BigEndian.PutUint32(prefix[0:4], 0xBEEF)
	binary.BigEndian.PutUint32(prefix[4:8], uint32(len(PorticoWelcomeMsg)))

	if _, err := writer.Write(prefix[:]); err != nil {
		return err
	}
	if _, err := writer.Write([]byte(PorticoWelcomeMsg)); err != nil {
		return err
	}

	// Servidor envia sinal de prontidão 0xFEED
	var srvReady [4]byte
	binary.BigEndian.PutUint32(srvReady[:], 0xFEED)
	if _, err := writer.Write(srvReady[:]); err != nil {
		return err
	}

	if flusher, ok := writer.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			return err
		}
	}

	// 2. Ler o Magic de resposta do Cliente (0xFEED = 65261)
	var clientReady int32
	if err := binary.Read(reader, binary.BigEndian, &clientReady); err != nil {
		return fmt.Errorf("erro lendo 0xFEED do cliente: %w", err)
	}
	if clientReady != 0xFEED {
		return fmt.Errorf("cliente enviou ready code invalido: 0x%X (esperado 0xFEED)", clientReady)
	}

	return nil
}
