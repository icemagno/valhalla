package crc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
	fomparser "github.com/icemagno/valhalla/pkg/metamodel"
)

const (
	PorticoWelcomeMsg = "Portico Router (3.0.0 (build 0)): Valhalla CRC Sovereign Gateway"
)

// extractAttributeHandles parses a Java HashSet or ArrayList of integers from a JavaObject
func extractAttributeHandles(objAny any) []common.AttributeHandle {
	var handles []common.AttributeHandle
	if obj, ok := objAny.(*metamodel.JavaObject); ok {
		var elements []any
		if els, ok := obj.Get("elements").([]any); ok {
			elements = els
		}
		for _, el := range elements {
			switch v := el.(type) {
			case int32:
				handles = append(handles, common.AttributeHandle(v))
			case *metamodel.JavaObject:
				if val, ok := v.Get("value").(int32); ok {
					handles = append(handles, common.AttributeHandle(val))
				}
			}
		}
	}
	return handles
}

// extractAttributeMap parses a Java HashMap of (Integer -> byte[]) from a JavaObject
func extractAttributeMap(objAny any) map[int32][]byte {
	attrs := make(map[int32][]byte)
	if obj, ok := objAny.(*metamodel.JavaObject); ok {
		var entries map[any]any
		if e, ok := obj.Get("entries").(map[any]any); ok {
			entries = e
		}
		for k, v := range entries {
			var handle int32
			switch key := k.(type) {
			case int32:
				handle = key
			case *metamodel.JavaObject:
				if val, ok := key.Get("value").(int32); ok {
					handle = val
				}
			}
			if b, ok := v.([]byte); ok && handle > 0 {
				attrs[handle] = b
			}
		}
	}
	return attrs
}

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

			// Ingest any FOM modules provided in the CreateFederation request
			if obj != nil {
				ingestPorticoFOMModules(currentFed.OMT(), obj.Get("modules"))
			}

			// If it's the standard Portico ExampleFederation, ensure the 4 canonical restaurant modules are loaded
			if fedName == "ExampleFederation" {
				for _, mod := range []string{"RestaurantProcesses.xml", "RestaurantFood.xml", "RestaurantDrinks.xml", "RestaurantSoup.xml"} {
					if resolved := resolveFOMPath(mod); resolved != "" {
						if err := fomparser.ParseFOMFile(currentFed.OMT(), resolved); err == nil {
							log.Printf("[CRC-Portico] Ingested canonical FOM module '%s' (from '%s') for [%s]", mod, resolved, fedName)
						} else {
							log.Printf("[CRC-Portico] Warning: failed to parse canonical module '%s': %v", resolved, err)
						}
					} else {
						log.Printf("[CRC-Portico] Warning: could not resolve canonical module '%s' on disk", mod)
					}
				}
			}

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

			// Safety check: if ExampleFederation is missing Soda, load canonical modules now before WelcomePack
			if fedName == "ExampleFederation" {
				if _, hasSoda := currentFed.OMT().GetObjectClassByName("Soda"); !hasSoda {
					for _, mod := range []string{"RestaurantProcesses.xml", "RestaurantFood.xml", "RestaurantDrinks.xml", "RestaurantSoup.xml"} {
						if resolved := resolveFOMPath(mod); resolved != "" {
							if err := fomparser.ParseFOMFile(currentFed.OMT(), resolved); err == nil {
								log.Printf("[CRC-Portico] Ingested canonical FOM module '%s' (from '%s') for [%s]", mod, resolved, fedName)
							}
						}
					}
				}
			}

			wpPayload, err := wire.BuildWelcomePackPayloadWithFOM(currentFed.Handle, federateID, currentFed.OMT(), fedName, nodeName, nodeType)
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

		case wire.MsgPublishObjectClass:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				classHandle, _ := obj.Get("classHandle").(int32)
				attrHandles := extractAttributeHandles(obj.Get("attributes"))
				if currentFed.Router() != nil && classHandle > 0 {
					_ = currentFed.Router().PublishObjectClass(common.FederateHandle(federateID), common.ObjectClassHandle(classHandle), attrHandles)
				}
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

		case wire.MsgSubscribeObjectClass:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				classHandle, _ := obj.Get("classHandle").(int32)
				attrHandles := extractAttributeHandles(obj.Get("attributes"))
				if currentFed.Router() != nil && classHandle > 0 {
					_ = currentFed.Router().SubscribeObjectClass(common.FederateHandle(federateID), common.ObjectClassHandle(classHandle), attrHandles, true)
				}
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

		case wire.MsgPublishInteractionClass:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				classHandle, _ := obj.Get("classHandle").(int32)
				if currentFed.Router() != nil && classHandle > 0 {
					_ = currentFed.Router().PublishInteractionClass(common.FederateHandle(federateID), common.InteractionClassHandle(classHandle))
				}
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

		case wire.MsgSubscribeInteractionClass:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				classHandle, _ := obj.Get("classHandle").(int32)
				if currentFed.Router() != nil && classHandle > 0 {
					_ = currentFed.Router().SubscribeInteractionClass(common.FederateHandle(federateID), common.InteractionClassHandle(classHandle), true)
				}
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

		case wire.MsgRegisterObject:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			var classHandle int32
			var objectName string
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if ch, ok := obj.Get("classHandle").(int32); ok {
					classHandle = ch
				}
				if on, ok := obj.Get("objectName").(string); ok {
					objectName = on
				}
			}
			if classHandle <= 0 {
				classHandle = 1 // fallback to HLAobjectRootHandle
			}

			objInst, err := currentFed.RegisterObjectInstance(classHandle, objectName, federateID)
			if err != nil {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error())
				continue
			}

			// Respond with the assigned ObjectInstanceHandle and keys expected by RegisterObjectHandler
			respPayload := wire.BuildRegisterObjectResponsePayload(objInst.Handle, classHandle, objInst.Name)
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload)
			log.Printf("[CRC-Portico] Object registered by federate [%d]: handle [%d], name '%s'", federateID, objInst.Handle, objInst.Name)

			// Notify other federates (both Native and Portico)
			nativeDiscPayload, _ := common.DiscoverObjectInstancePayload{
				ObjectHandle: common.ObjectInstanceHandle(objInst.Handle),
				ClassHandle:  common.ObjectClassHandle(classHandle),
				ObjectName:   objInst.Name,
			}.MarshalBinary()

			porticoDiscPayload, _ := wire.BuildDiscoverObjectPayload(objInst.Handle, classHandle, objInst.Name)

			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					if f.Protocol == ProtocolNative {
						_ = f.Session.SendNotification(common.NativeMsgDiscoverObject, nativeDiscPayload)
					} else {
						_ = f.Session.SendNotification(uint8(wire.MsgDiscoverObject), porticoDiscPayload)
					}
				}
			}

		case wire.MsgUpdateAttributes:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			var objectId int32
			var attrsMap map[int32][]byte
			var tag []byte
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if oid, ok := obj.Get("objectId").(int32); ok {
					objectId = oid
				}
				attrsMap = extractAttributeMap(obj.Get("attributes"))
				if t, ok := obj.Get("tag").([]byte); ok {
					tag = t
				}
			}

			if objectId > 0 && len(attrsMap) > 0 {
				_ = currentFed.UpdateObjectAttributes(objectId, attrsMap, [32]byte{})
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

			// Routing reflections to interested subscribers (Native or Portico)
			nativeUavAttrs := make(map[common.AttributeHandle][]byte, len(attrsMap))
			for k, v := range attrsMap {
				nativeUavAttrs[common.AttributeHandle(k)] = v
			}
			nativeUavPayload, _ := common.UpdateAttributesPayload{
				ObjectHandle: common.ObjectInstanceHandle(objectId),
				Attributes:   nativeUavAttrs,
				Tag:          tag,
			}.MarshalBinary()

			porticoReflectPayload, _ := wire.BuildReflectAttributesPayload(objectId, attrsMap, tag)

			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					if f.Protocol == ProtocolNative {
						_ = f.Session.SendNotification(common.NativeMsgReflectAttributes, nativeUavPayload)
					} else {
						_ = f.Session.SendNotification(uint8(wire.MsgReflectAttributes), porticoReflectPayload)
					}
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
			federateID = 0

		case wire.MsgDeleteObject:
			if currentFed != nil && federateID > 0 {
				var objHandle int32
				if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
					if h, ok := obj.Get("objectHandle").(int32); ok {
						objHandle = h
					}
				}
				if objHandle > 0 {
					_, _ = currentFed.DeleteObjectInstance(objHandle, federateID, nil)
					log.Printf("[CRC-Portico] Object [%d] deleted by federate [%d]", objHandle, federateID)
				}
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

		case wire.MsgDestroyFederation:
			var fedName string
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if fn, ok := obj.Get("federationName").(string); ok {
					fedName = fn
				}
			}
			if fedName == "" && currentFed != nil {
				fedName = currentFed.Name
			}
			if fedName != "" {
				_ = fedManager.DestroyFederation(fedName)
				log.Printf("[CRC-Portico] Federation [%s] destroyed", fedName)
				currentFed = nil
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

		case wire.MsgDisconnect:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			log.Printf("[CRC-Portico] Connection disconnected gracefully")
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

// resolveFOMPath searches for a FOM file on disk strictly as ./foms/<ARQUIVO>.xml
func resolveFOMPath(rawPath string) string {
	cleanPath := strings.TrimPrefix(rawPath, "file:///")
	cleanPath = strings.TrimPrefix(cleanPath, "file://")
	cleanPath = strings.TrimPrefix(cleanPath, "file:")

	normPath := strings.ReplaceAll(cleanPath, "\\", "/")
	base := filepath.Base(normPath)

	target := filepath.Join("foms", base)
	if _, err := os.Stat(target); err == nil {
		return target
	}
	return ""
}

// ingestPorticoFOMModules extracts and parses FOM XML modules from the Portico CreateFederation payload
func ingestPorticoFOMModules(omt *core.OMTRegistry, modulesAny any) {
	if omt == nil || modulesAny == nil {
		return
	}
	var elements []any
	if listObj, ok := modulesAny.(*metamodel.JavaObject); ok {
		if els, ok := listObj.Get("elements").([]any); ok {
			elements = els
		}
	} else if els, ok := modulesAny.([]any); ok {
		elements = els
	}

	for _, el := range elements {
		switch v := el.(type) {
		case []byte:
			if strings.Contains(string(v), "<objectModel") {
				if err := fomparser.ParseFOMXML(omt, v); err != nil {
					log.Printf("[CRC-Portico] Warning: failed to parse byte FOM module: %v", err)
				} else {
					log.Printf("[CRC-Portico] Ingested in-memory FOM XML module (%d bytes)", len(v))
				}
			}
		case string:
			if strings.Contains(v, "<objectModel") {
				if err := fomparser.ParseFOMXML(omt, []byte(v)); err != nil {
					log.Printf("[CRC-Portico] Warning: failed to parse string FOM module: %v", err)
				} else {
					log.Printf("[CRC-Portico] Ingested string FOM XML module (%d bytes)", len(v))
				}
			} else {
				resolved := resolveFOMPath(v)
				if resolved != "" {
					if err := fomparser.ParseFOMFile(omt, resolved); err != nil {
						log.Printf("[CRC-Portico] Warning: failed to parse FOM file '%s': %v", resolved, err)
					} else {
						log.Printf("[CRC-Portico] Ingested FOM file '%s' (from '%s')", resolved, v)
					}
				} else {
					log.Printf("[CRC-Portico] Warning: could not resolve FOM path '%s'", v)
				}
			}
		case *metamodel.JavaObject:
			// If JavaObject is java.net.URL, extract "file" or "path"
			if v.Class != nil && v.Class.Name == "java.net.URL" {
				if fileStr, ok := v.Get("file").(string); ok && fileStr != "" {
					ingestPorticoFOMModules(omt, []any{fileStr})
					continue
				}
				if pathStr, ok := v.Get("path").(string); ok && pathStr != "" {
					ingestPorticoFOMModules(omt, []any{pathStr})
					continue
				}
			}
			// Generic fallback: check known fields
			for _, f := range []string{"file", "path", "content", "data", "raw", "xml", "url"} {
				if val := v.Get(f); val != nil {
					ingestPorticoFOMModules(omt, []any{val})
					break
				}
			}
		}
	}
}
