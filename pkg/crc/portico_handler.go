package crc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
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
			var fedName string
			obj, err := wire.DeserializePorticoControlMessage(payload)
			if err != nil {
				log.Printf("[CRC-Portico] Error deserializing CreateFederation: %v", err)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "CouldNotOpenFDD: malformed CreateFederation request")
				continue
			}
			if n, ok := obj.Get("federationName").(string); ok && n != "" {
				fedName = n
			}
			if fedName == "" {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "CouldNotOpenFDD: empty federation name")
				continue
			}

			fedExec, err := fedManager.CreateFederation(fedName)
			if err != nil {
				log.Printf("[CRC-Portico] CreateFederation failed for [%s]: %v", fedName, err)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error())
				continue
			}
			currentFed = fedExec
			respPayload := wire.BuildCreateFederationResponsePayload(currentFed.Handle, fedName)
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload)
			log.Printf("[CRC-Portico] Federation [%s] created with handle [%d]", fedName, currentFed.Handle)

		case wire.MsgJoinFederation:
			var fedName, nodeName, nodeType string
			nodeType = "ieee1516e"

			obj, err := wire.DeserializePorticoControlMessage(payload)
			if err != nil {
				log.Printf("[CRC-Portico] Error deserializing JoinFederation request: %v", err)
			}
			if obj != nil {
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

			if nodeName == "" {
				nodeName = wire.ExtractFederateNameFromJoinPayload(payload)
			}

			if fedName == "" || nodeName == "" {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: federation and federate names are mandatory")
				continue
			}

			fedExec, exists := fedManager.GetFederation(fedName)
			if !exists {
				log.Printf("[CRC-Portico] Join error: federation [%s] does not exist", fedName)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederationExecutionDoesNotExist: "+fedName)
				continue
			}

			currentFed = fedExec
			fed, err := currentFed.Join(nodeName, nodeType, ProtocolPortico, session)
			if err != nil {
				log.Printf("[CRC-Portico] Join error for %s: %v", nodeName, err)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error())
				continue
			}

			federateID = fed.ID
			session.SetFederateID(federateID)
			session.SetFederationHandle(currentFed.Handle)

			wpPayload, err := wire.BuildWelcomePackPayload(currentFed.Handle, federateID, fedName, nodeName, nodeType)
			if err != nil {
				log.Printf("[CRC-Portico] Error generating WelcomePack: %v", err)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: failed to build WelcomePack")
				continue
			}

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wpPayload)
			log.Printf("[CRC-Portico] Federate [%s] joined with handle [%d] to federation [%s]", nodeName, federateID, fedName)

		case wire.MsgRegisterSyncPoint:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			var label string
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if l, ok := obj.Get("label").(string); ok && l != "" {
					label = l
				}
			}
			if label == "" {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: empty sync point label")
				continue
			}

			_, err := currentFed.RegisterSyncPoint(label, nil, federateID)
			if err != nil {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error())
				continue
			}

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			log.Printf("[CRC-Portico] Synchronization point '%s' registered by federate [%d]", label, federateID)

			if annPayload, err := wire.BuildAnnounceSyncPointPayload(label); err == nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.Session != nil {
						_ = f.Session.SendNotification(uint8(wire.MsgAnnounceSyncPoint), annPayload)
					}
				}
			}

		case wire.MsgAchieveSyncPoint:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			var label string
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if l, ok := obj.Get("label").(string); ok && l != "" {
					label = l
				}
			}
			if label == "" {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: empty sync point label")
				continue
			}

			isSynced, err := currentFed.AchieveSyncPoint(label, federateID)
			if err != nil {
				log.Printf("[CRC-Portico] AchieveSyncPoint error: %v", err)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error())
				continue
			}

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			log.Printf("[CRC-Portico] Federate [%d] achieved synchronization point '%s'", federateID, label)

			if isSynced {
				log.Printf("[CRC-Portico] >>> FEDERATION SYNCHRONIZED at point '%s'!", label)
				if syncPayload, err := wire.BuildFederationSynchronizedPayload(label); err == nil {
					for _, f := range currentFed.GetAllFederates() {
						if f.Session != nil {
							_ = f.Session.SendNotification(uint8(wire.MsgFederationSynchronized), syncPayload)
						}
					}
				}
			}

		case wire.MsgPublishObjectClass, wire.MsgSubscribeObjectClass,
			wire.MsgPublishInteractionClass, wire.MsgSubscribeInteractionClass:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

		case wire.MsgRegisterObject:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					_ = f.Session.SendNotification(uint8(wire.MsgDiscoverObject), payload)
				}
			}

		case wire.MsgUpdateAttributes:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					_ = f.Session.SendNotification(uint8(wire.MsgReflectAttributes), payload)
				}
			}

		case wire.MsgSendInteraction:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					_ = f.Session.SendNotification(uint8(wire.MsgReceiveInteraction), payload)
				}
			}

		case wire.MsgEnableTimeRegulation:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotFound")
				continue
			}
			if fed.IsRegulating {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "TimeRegulationAlreadyEnabled")
				continue
			}
			fed.IsRegulating = true

			respPayload := wire.BuildDoubleResponsePayload(fed.CurrentTime)
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload)
			log.Printf("[CRC-Portico] Federate [%d] enabled time regulation at time %f", federateID, fed.CurrentTime)

		case wire.MsgEnableTimeConstrained:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotFound")
				continue
			}
			if fed.IsConstrained {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "TimeConstrainedAlreadyEnabled")
				continue
			}
			fed.IsConstrained = true

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			log.Printf("[CRC-Portico] Federate [%d] enabled time constrained", federateID)

		case wire.MsgTimeAdvanceRequest:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotFound")
				continue
			}
			requestedTime := fed.CurrentTime + 1.0
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if t, ok := obj.Get("time").(float64); ok {
					requestedTime = t
				}
			}
			if requestedTime < fed.CurrentTime {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InvalidLogicalTime")
				continue
			}
			fed.CurrentTime = requestedTime

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			if tagPayload, err := wire.BuildTimeAdvanceGrantPayload(federateID, requestedTime); err == nil {
				_ = session.SendNotification(uint8(wire.MsgTimeAdvanceGrant), tagPayload)
			}
			log.Printf("[CRC-Portico] Federate [%d] granted time advance to %f", federateID, requestedTime)

		case wire.MsgResignFederation:
			if currentFed != nil && federateID > 0 {
				_, _ = currentFed.Resign(federateID)
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			log.Printf("[CRC-Portico] Federate [%d] resigned gracefully", federateID)
			return nil

		default:
			if hdr.CallType == wire.CallTypeControlRequest {
				_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			}
		}
	}
}

// performPorticoHandshake executa a saudação binária oficial do Portico (0xBEEF -> 0xFEED)
func performPorticoHandshake(conn net.Conn, writer io.Writer, reader io.Reader) error {
	var prefix [8]byte
	binary.BigEndian.PutUint32(prefix[0:4], 0xBEEF)
	binary.BigEndian.PutUint32(prefix[4:8], uint32(len(PorticoWelcomeMsg)))

	if _, err := writer.Write(prefix[:]); err != nil {
		return err
	}
	if _, err := writer.Write([]byte(PorticoWelcomeMsg)); err != nil {
		return err
	}

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

	var clientReady int32
	if err := binary.Read(reader, binary.BigEndian, &clientReady); err != nil {
		return fmt.Errorf("erro lendo 0xFEED do cliente: %w", err)
	}
	if clientReady != 0xFEED {
		return fmt.Errorf("cliente enviou ready code invalido: 0x%X (esperado 0xFEED)", clientReady)
	}

	return nil
}
