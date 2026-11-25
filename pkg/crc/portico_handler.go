package crc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
				logCRCPortico.Error("Error deserializing CreateFederation: %v", err)
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
				logCRCPortico.Error("CreateFederation failed for [%s]: %v", fedName, err)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error())
				continue
			}
			currentFed = fedExec

			// Ingest any FOM modules provided in the CreateFederation request
			if obj != nil {
				for _, key := range []string{"rawFomModules", "modules", "fomModuleLocations", "rawFomModuleLocations"} {
					if val := obj.Get(key); val != nil {
						ingestPorticoFOMModules(currentFed.OMT(), val)
					}
				}
			}

			// If it's the standard Portico ExampleFederation, ensure the 4 canonical restaurant modules are loaded
			if fedName == "ExampleFederation" {
				for _, mod := range []string{"RestaurantProcesses.xml", "RestaurantFood.xml", "RestaurantDrinks.xml", "RestaurantSoup.xml"} {
					if resolved := resolveFOMPath(mod); resolved != "" {
						if err := fomparser.ParseFOMFile(currentFed.OMT(), resolved); err == nil {
							logCRCPortico.Info("Ingested canonical FOM module '%s' (from '%s') for [%s]", mod, resolved, fedName)
						} else {
							logCRCPortico.Warn("Warning: failed to parse canonical module '%s': %v", resolved, err)
						}
					} else {
						logCRCPortico.Warn("Warning: could not resolve canonical module '%s' on disk", mod)
					}
				}
			} else if fedName == "ValhallaAdvancedFed" {
				if resolved := resolveFOMPath("ValhallaAdvanced.xml"); resolved != "" {
					if err := fomparser.ParseFOMFile(currentFed.OMT(), resolved); err == nil {
						logCRCPortico.Info("Ingested advanced FOM module '%s' (from '%s') for [%s]", "ValhallaAdvanced.xml", resolved, fedName)
					} else {
						logCRCPortico.Warn("Warning: failed to parse advanced module '%s': %v", resolved, err)
					}
				} else {
					logCRCPortico.Warn("Warning: could not resolve advanced module 'ValhallaAdvanced.xml' on disk")
				}
			}

			respPayload := wire.BuildCreateFederationResponsePayload(currentFed.Handle, fedName)
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload)
			logCRCPortico.Info("Federation [%s] created with handle [%d]", fedName, currentFed.Handle)

		case wire.MsgJoinFederation:
			var fedName, nodeName, nodeType string
			nodeType = "ieee1516e"

			obj, err := wire.DeserializePorticoControlMessage(payload)
			if err != nil {
				logCRCPortico.Error("Error deserializing JoinFederation request: %v", err)
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
				logCRCPortico.Error("Join error: federation [%s] does not exist", fedName)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederationExecutionDoesNotExist: "+fedName)
				continue
			}

			currentFed = fedExec
			fed, err := currentFed.Join(nodeName, nodeType, ProtocolPortico, session)
			if err != nil {
				logCRCPortico.Error("Join error for %s: %v", nodeName, err)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error())
				continue
			}

			federateID = fed.ID
			session.SetFederateID(federateID)
			session.SetFederationHandle(currentFed.Handle)

			// Safety check: ensure canonical / advanced modules are loaded before WelcomePack
			if fedName == "ExampleFederation" {
				if _, hasSoda := currentFed.OMT().GetObjectClassByName("Soda"); !hasSoda {
					for _, mod := range []string{"RestaurantProcesses.xml", "RestaurantFood.xml", "RestaurantDrinks.xml", "RestaurantSoup.xml"} {
						if resolved := resolveFOMPath(mod); resolved != "" {
							if err := fomparser.ParseFOMFile(currentFed.OMT(), resolved); err == nil {
								logCRCPortico.Info("Ingested canonical FOM module '%s' (from '%s') for [%s]", mod, resolved, fedName)
							}
						}
					}
				}
			} else if fedName == "ValhallaAdvancedFed" {
				if _, hasTactical := currentFed.OMT().GetObjectClassByName("TacticalEntity"); !hasTactical {
					if resolved := resolveFOMPath("ValhallaAdvanced.xml"); resolved != "" {
						if err := fomparser.ParseFOMFile(currentFed.OMT(), resolved); err == nil {
							logCRCPortico.Info("Ingested advanced FOM module '%s' (from '%s') for [%s]", "ValhallaAdvanced.xml", resolved, fedName)
						}
					}
				}
			}

			wpPayload, err := wire.BuildWelcomePackPayloadWithFOM(currentFed.Handle, federateID, currentFed.OMT(), fedName, nodeName, nodeType)
			if err != nil {
				logCRCPortico.Error("Error generating WelcomePack: %v", err)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: failed to build WelcomePack")
				continue
			}

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wpPayload)
			logCRCPortico.Info("Federate [%s] joined with handle [%d] to federation [%s]", nodeName, federateID, fedName)

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
			logCRCPortico.Info("Synchronization point '%s' registered by federate [%d]", label, federateID)

			annPayload, _ := wire.BuildAnnounceSyncPointPayload(label)
			nativeAnnounce, _ := common.RegisterSyncPointPayload{Label: label}.MarshalBinary()
			for _, f := range currentFed.GetAllFederates() {
				if f.Session != nil {
					if f.Protocol == ProtocolNative {
						_ = f.Session.SendNotification(common.NativeMsgAnnounceSyncPoint, nativeAnnounce)
					} else if annPayload != nil {
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
				logCRCPortico.Error("AchieveSyncPoint error: %v", err)
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error())
				continue
			}

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			logCRCPortico.Info("Federate [%d] achieved synchronization point '%s'", federateID, label)

			if isSynced {
				logCRCPortico.Info(">>> FEDERATION SYNCHRONIZED at point '%s'!", label)
				syncPayload, _ := wire.BuildFederationSynchronizedPayload(label)
				nativeSync, _ := common.SyncPointLabelPayload{Label: label}.MarshalBinary()
				for _, f := range currentFed.GetAllFederates() {
					if f.Session != nil {
						if f.Protocol == ProtocolNative {
							_ = f.Session.SendNotification(common.NativeMsgFederationSynchronized, nativeSync)
						} else if syncPayload != nil {
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
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectClassNotDefined")
				continue
			}

			objInst, err := currentFed.RegisterObjectInstance(classHandle, objectName, federateID)
			if err != nil {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error())
				continue
			}

			// Respond with the assigned ObjectInstanceHandle and keys expected by RegisterObjectHandler
			respPayload := wire.BuildRegisterObjectResponsePayload(objInst.Handle, classHandle, objInst.Name)
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload)
			logCRCPortico.Info("Object registered by federate [%d]: handle [%d], name '%s'", federateID, objInst.Handle, objInst.Name)

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

			if objectId <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectInstanceNotKnown")
				continue
			}

			if err := currentFed.UpdateObjectAttributes(objectId, attrsMap, [32]byte{}); err != nil {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectInstanceNotKnown")
				continue
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

			var interactionId int32
			var paramsMap map[int32][]byte
			var tag []byte
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if iid, ok := obj.Get("interactionId").(int32); ok {
					interactionId = iid
				}
				paramsMap = extractAttributeMap(obj.Get("parameters"))
				if t, ok := obj.Get("tag").([]byte); ok {
					tag = t
				}
			}

			if interactionId <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InteractionClassNotDefined")
				continue
			}
			if _, exists := currentFed.OMT().GetInteractionClass(common.InteractionClassHandle(interactionId)); !exists {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InteractionClassNotDefined")
				continue
			}

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

			nativeParams := make(map[common.ParameterHandle][]byte, len(paramsMap))
			for k, v := range paramsMap {
				nativeParams[common.ParameterHandle(k)] = v
			}
			nativeInterPayload, _ := common.SendInteractionPayload{
				ClassHandle: common.InteractionClassHandle(interactionId),
				Parameters:  nativeParams,
				Tag:         tag,
			}.MarshalBinary()

			for _, f := range currentFed.GetAllFederates() {
				if f.ID != federateID && f.Session != nil {
					if f.Protocol == ProtocolNative {
						_ = f.Session.SendNotification(common.NativeMsgReceiveInteraction, nativeInterPayload)
					} else {
						_ = f.Session.SendNotification(uint8(wire.MsgReceiveInteraction), payload)
					}
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
			logCRCPortico.Info("Federate [%d] enabled time regulation at time %f", federateID, fed.CurrentTime)

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
			logCRCPortico.Info("Federate [%d] enabled time constrained", federateID)

		case wire.MsgTimeAdvanceRequest, wire.MsgNextEventRequest, wire.MsgFlushQueueRequest:
			if currentFed == nil || federateID <= 0 {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember")
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotFound")
				continue
			}
			var requestedTime float64
			hasTime := false
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if t, ok := obj.Get("time").(float64); ok {
					requestedTime = t
					hasTime = true
				}
			}
			if !hasTime || requestedTime < fed.CurrentTime {
				_ = session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InvalidLogicalTime")
				continue
			}
			fed.CurrentTime = requestedTime

			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			if tagPayload, err := wire.BuildTimeAdvanceGrantPayload(federateID, requestedTime); err == nil {
				_ = session.SendNotification(uint8(wire.MsgTimeAdvanceGrant), tagPayload)
			}
			logCRCPortico.Info("Federate [%d] granted time advance to %f", federateID, requestedTime)

		case wire.MsgResignFederation:
			if currentFed != nil && federateID > 0 {
				_, _ = currentFed.Resign(federateID)
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			logCRCPortico.Info("Federate [%d] resigned gracefully", federateID)
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
					logCRCPortico.Info("Object [%d] deleted by federate [%d]", objHandle, federateID)

					removePayload, _ := common.RemoveObjectInstancePayload{
						ObjectHandle: common.ObjectInstanceHandle(objHandle),
					}.MarshalBinary()

					for _, f := range currentFed.GetAllFederates() {
						if f.ID != federateID && f.Session != nil {
							if f.Protocol == ProtocolNative {
								_ = f.Session.SendNotification(common.NativeMsgRemoveObjectInstance, removePayload)
							}
						}
					}
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
				logCRCPortico.Info("Federation [%s] destroyed", fedName)
				currentFed = nil
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

		case wire.MsgDisconnect:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			logCRCPortico.Info("Connection disconnected gracefully")
			return nil

		case wire.MsgSaveRequest:
			label := "Checkpoint"
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if l, ok := obj.Get("label").(string); ok && l != "" {
					label = l
				}
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			logCRCPortico.Info("Federate [%d] requested federation save with label '%s'", federateID, label)

			saveInitObj := metamodel.NewJavaObject(metamodel.ClassDescObject)
			saveInitObj.Set("label", label)
			if savePayload, err := wire.SerializePorticoControlMessage(saveInitObj); err == nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.Session != nil && f.Protocol == ProtocolPortico {
						_ = f.Session.SendNotification(uint8(wire.MsgSaveInitiate), savePayload)
					}
				}
			}

		case wire.MsgSaveBegun:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())

		case wire.MsgSaveComplete:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			logCRCPortico.Info("Federate [%d] completed save. Broadcasting FederationSaved.", federateID)
			for _, f := range currentFed.GetAllFederates() {
				if f.Session != nil && f.Protocol == ProtocolPortico {
					_ = f.Session.SendNotification(uint8(wire.MsgFederationSaved), wire.BuildSuccessResponsePayload())
				}
			}

		case wire.MsgRestoreRequest:
			label := "Checkpoint"
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if l, ok := obj.Get("label").(string); ok && l != "" {
					label = l
				}
			}
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			logCRCPortico.Info("Federate [%d] requested federation restore with label '%s'", federateID, label)

			restoreInitObj := metamodel.NewJavaObject(metamodel.ClassDescObject)
			restoreInitObj.Set("label", label)
			if restPayload, err := wire.SerializePorticoControlMessage(restoreInitObj); err == nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.Session != nil && f.Protocol == ProtocolPortico {
						_ = f.Session.SendNotification(uint8(wire.MsgRestoreInitiate), restPayload)
					}
				}
			}

		case wire.MsgRestoreComplete:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			logCRCPortico.Info("Federate [%d] completed restore. Broadcasting FederationRestored.", federateID)
			for _, f := range currentFed.GetAllFederates() {
				if f.Session != nil && f.Protocol == ProtocolPortico {
					_ = f.Session.SendNotification(uint8(wire.MsgFederationRestored), wire.BuildSuccessResponsePayload())
				}
			}

		case wire.MsgAttributeDivest:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			logCRCPortico.Info("Federate [%d] divested attribute ownership", federateID)

		case wire.MsgQueryOwnership:
			_ = session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload())
			logCRCPortico.Info("Federate [%d] queried attribute ownership", federateID)

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

// resolveFOMPath searches for a FOM file on disk strictly as ./foms/<ARQUIVO>.xml or bin/foms/<ARQUIVO>.xml
func resolveFOMPath(rawPath string) string {
	cleanPath := strings.TrimPrefix(rawPath, "file:///")
	cleanPath = strings.TrimPrefix(cleanPath, "file://")
	cleanPath = strings.TrimPrefix(cleanPath, "file:")

	normPath := strings.ReplaceAll(cleanPath, "\\", "/")
	base := filepath.Base(normPath)

	candidates := []string{
		filepath.Join("foms", base),
		filepath.Join("bin", "foms", base),
	}
	for _, target := range candidates {
		if _, err := os.Stat(target); err == nil {
			return target
		}
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
					logCRCPortico.Warn("Warning: failed to parse byte FOM module: %v", err)
				} else {
					logCRCPortico.Info("Ingested in-memory FOM XML module (%d bytes)", len(v))
				}
			}
		case string:
			if strings.Contains(v, "<objectModel") {
				if err := fomparser.ParseFOMXML(omt, []byte(v)); err != nil {
					logCRCPortico.Warn("Warning: failed to parse string FOM module: %v", err)
				} else {
					logCRCPortico.Info("Ingested string FOM XML module (%d bytes)", len(v))
				}
			} else {
				resolved := resolveFOMPath(v)
				if resolved != "" {
					if err := fomparser.ParseFOMFile(omt, resolved); err != nil {
						logCRCPortico.Warn("Warning: failed to parse FOM file '%s': %v", resolved, err)
					} else {
						logCRCPortico.Info("Ingested FOM file '%s' (from '%s')", resolved, v)
					}
				} else {
					logCRCPortico.Warn("Warning: could not resolve FOM path '%s'", v)
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
