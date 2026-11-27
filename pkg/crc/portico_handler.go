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
					if _, err := currentFed.Resign(federateID); err != nil {
						logCRCPortico.Warn("failed to resign federate %d on disconnect: %v", federateID, err)
					}
				}
				return nil
			}
			return fmt.Errorf("erro lendo pacote Portico: %w", err)
		}

		if currentFed != nil && federateID > 0 {
			currentFed.TimeManager().Heartbeat(common.FederateHandle(federateID))
		}

		switch hdr.MessageType {
		case wire.MsgCreateFederation:
			var fedName string
			obj, err := wire.DeserializePorticoControlMessage(payload)
			if err != nil {
				logCRCPortico.Error("Error deserializing CreateFederation: %v", err)
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "CouldNotOpenFDD: malformed CreateFederation request"); err != nil {
					return err
				}
				continue
			}
			if n, ok := obj.Get("federationName").(string); ok && n != "" {
				fedName = n
			}
			if fedName == "" {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "CouldNotOpenFDD: empty federation name"); err != nil {
					return err
				}
				continue
			}

			fedExec, err := fedManager.CreateFederation(fedName)
			if err != nil {
				logCRCPortico.Error("CreateFederation failed for [%s]: %v", fedName, err)
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
					return err
				}
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
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload); err != nil {
				return err
			}
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
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: federation and federate names are mandatory"); err != nil {
					return err
				}
				continue
			}

			fedExec, exists := fedManager.GetFederation(fedName)
			if !exists {
				logCRCPortico.Error("Join error: federation [%s] does not exist", fedName)
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederationExecutionDoesNotExist: "+fedName); err != nil {
					return err
				}
				continue
			}

			currentFed = fedExec
			fed, err := currentFed.Join(nodeName, nodeType, ProtocolPortico, session)
			if err != nil {
				logCRCPortico.Error("Join error for %s: %v", nodeName, err)
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
					return err
				}
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
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: failed to build WelcomePack"); err != nil {
					return err
				}
				continue
			}

			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wpPayload); err != nil {
				return err
			}
			logCRCPortico.Info("Federate [%s] joined with handle [%d] to federation [%s]", nodeName, federateID, fedName)

		case wire.MsgRegisterSyncPoint:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
				continue
			}
			var label string
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if l, ok := obj.Get("label").(string); ok && l != "" {
					label = l
				}
			}
			if label == "" {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: empty sync point label"); err != nil {
					return err
				}
				continue
			}

			_, err := currentFed.RegisterSyncPoint(label, nil, federateID)
			if err != nil {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
					return err
				}
				continue
			}

			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			logCRCPortico.Info("Synchronization point '%s' registered by federate [%d]", label, federateID)

			annPayload, _ := wire.BuildAnnounceSyncPointPayload(label)
			nativeAnnounce, _ := common.RegisterSyncPointPayload{Label: label}.MarshalBinary()
			for _, f := range currentFed.GetAllFederates() {
				if f.Session != nil {
					if f.Protocol == ProtocolPortico && annPayload != nil {
						if err := f.Session.SendNotification(uint8(wire.MsgAnnounceSyncPoint), annPayload); err != nil {
							logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
						}
					} else {
						if err := f.Session.SendNotification(common.NativeMsgAnnounceSyncPoint, nativeAnnounce); err != nil {
							logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
						}
					}
				}
			}

		case wire.MsgAchieveSyncPoint:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
				continue
			}
			var label string
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if l, ok := obj.Get("label").(string); ok && l != "" {
					label = l
				}
			}
			if label == "" {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: empty sync point label"); err != nil {
					return err
				}
				continue
			}

			isSynced, err := currentFed.AchieveSyncPoint(label, federateID)
			if err != nil {
				logCRCPortico.Error("AchieveSyncPoint error: %v", err)
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
					return err
				}
				continue
			}

			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			logCRCPortico.Info("Federate [%d] achieved synchronization point '%s'", federateID, label)

			if isSynced {
				logCRCPortico.Info(">>> FEDERATION SYNCHRONIZED at point '%s'!", label)
				syncPayload, _ := wire.BuildFederationSynchronizedPayload(label)
				nativeSync, _ := common.SyncPointLabelPayload{Label: label}.MarshalBinary()
				for _, f := range currentFed.GetAllFederates() {
					if f.Session != nil {
						if f.Protocol == ProtocolPortico && syncPayload != nil {
							if err := f.Session.SendNotification(uint8(wire.MsgFederationSynchronized), syncPayload); err != nil {
								logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
							}
						} else {
							if err := f.Session.SendNotification(common.NativeMsgFederationSynchronized, nativeSync); err != nil {
								logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
							}
						}
					}
				}
			}

		case wire.MsgPublishObjectClass:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
				continue
			}
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				classHandle, _ := obj.Get("classHandle").(int32)
				attrHandles := extractAttributeHandles(obj.Get("attributes"))
				if currentFed.Router() != nil && classHandle > 0 {
					if err := currentFed.Router().PublishObjectClass(common.FederateHandle(federateID), common.ObjectClassHandle(classHandle), attrHandles); err != nil {
				logCRCPortico.Warn("failed to publish object class: %v", err)
				if sendErr := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
					return sendErr
				}
				continue
			}
				}
			}
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}

		case wire.MsgSubscribeObjectClass:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
				continue
			}
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				classHandle, _ := obj.Get("classHandle").(int32)
				attrHandles := extractAttributeHandles(obj.Get("attributes"))
				if currentFed.Router() != nil && classHandle > 0 {
					if err := currentFed.Router().SubscribeObjectClass(common.FederateHandle(federateID), common.ObjectClassHandle(classHandle), attrHandles, true); err != nil {
				logCRCPortico.Warn("failed to subscribe object class: %v", err)
				if sendErr := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
					return sendErr
				}
				continue
			}
				}
			}
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}

		case wire.MsgPublishInteractionClass:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
				continue
			}
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				classHandle, _ := obj.Get("classHandle").(int32)
				if currentFed.Router() != nil && classHandle > 0 {
					if err := currentFed.Router().PublishInteractionClass(common.FederateHandle(federateID), common.InteractionClassHandle(classHandle)); err != nil {
				logCRCPortico.Warn("failed to publish interaction class: %v", err)
				if sendErr := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
					return sendErr
				}
				continue
			}
				}
			}
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}

		case wire.MsgSubscribeInteractionClass:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
				continue
			}
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				classHandle, _ := obj.Get("classHandle").(int32)
				if currentFed.Router() != nil && classHandle > 0 {
					if err := currentFed.Router().SubscribeInteractionClass(common.FederateHandle(federateID), common.InteractionClassHandle(classHandle), true); err != nil {
				logCRCPortico.Warn("failed to subscribe interaction class: %v", err)
				if sendErr := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
					return sendErr
				}
				continue
			}
				}
			}
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}

		case wire.MsgRegisterObject:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
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
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectClassNotDefined"); err != nil {
					return err
				}
				continue
			}

			objInst, err := currentFed.RegisterObjectInstance(classHandle, objectName, federateID)
			if err != nil {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
					return err
				}
				continue
			}

			// Respond with the assigned ObjectInstanceHandle and keys expected by RegisterObjectHandler
			respPayload := wire.BuildRegisterObjectResponsePayload(objInst.Handle, classHandle, objInst.Name)
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload); err != nil {
				return err
			}
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
					if f.Protocol == ProtocolPortico && porticoDiscPayload != nil {
						if err := f.Session.SendNotification(uint8(wire.MsgDiscoverObject), porticoDiscPayload); err != nil {
							logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
						}
					} else {
						if err := f.Session.SendNotification(common.NativeMsgDiscoverObject, nativeDiscPayload); err != nil {
							logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
						}
					}
				}
			}

		case wire.MsgUpdateAttributes:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
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
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectInstanceNotKnown"); err != nil {
					return err
				}
				continue
			}

			if err := currentFed.UpdateObjectAttributes(objectId, attrsMap, [32]byte{}); err != nil {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectInstanceNotKnown"); err != nil {
					return err
				}
				continue
			}
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}

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
					if f.Protocol == ProtocolPortico && porticoReflectPayload != nil {
						if err := f.Session.SendNotification(uint8(wire.MsgReflectAttributes), porticoReflectPayload); err != nil {
							logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
						}
					} else {
						if err := f.Session.SendNotification(common.NativeMsgReflectAttributes, nativeUavPayload); err != nil {
							logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
						}
					}
				}
			}

		case wire.MsgSendInteraction:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
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
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InteractionClassNotDefined"); err != nil {
					return err
				}
				continue
			}
			if _, exists := currentFed.OMT().GetInteractionClass(common.InteractionClassHandle(interactionId)); !exists {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InteractionClassNotDefined"); err != nil {
					return err
				}
				continue
			}

			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}

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
					if f.Protocol == ProtocolPortico {
						if err := f.Session.SendNotification(uint8(wire.MsgReceiveInteraction), payload); err != nil {
							logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
						}
					} else {
						if err := f.Session.SendNotification(common.NativeMsgReceiveInteraction, nativeInterPayload); err != nil {
							logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
						}
					}
				}
			}

		case wire.MsgEnableTimeRegulation:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotFound"); err != nil {
					return err
				}
				continue
			}
			if fed.IsRegulating {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "TimeRegulationAlreadyEnabled"); err != nil {
					return err
				}
				continue
			}
			fed.IsRegulating = true

			respPayload := wire.BuildDoubleResponsePayload(fed.CurrentTime)
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload); err != nil {
				return err
			}
			logCRCPortico.Info("Federate [%d] enabled time regulation at time %f", federateID, fed.CurrentTime)

		case wire.MsgEnableTimeConstrained:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotFound"); err != nil {
					return err
				}
				continue
			}
			if fed.IsConstrained {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "TimeConstrainedAlreadyEnabled"); err != nil {
					return err
				}
				continue
			}
			fed.IsConstrained = true

			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			logCRCPortico.Info("Federate [%d] enabled time constrained", federateID)

		case wire.MsgTimeAdvanceRequest, wire.MsgNextEventRequest, wire.MsgFlushQueueRequest:
			if currentFed == nil || federateID <= 0 {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
					return err
				}
				continue
			}
			fed, exists := currentFed.GetFederate(federateID)
			if !exists {
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotFound"); err != nil {
					return err
				}
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
				if err := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InvalidLogicalTime"); err != nil {
					return err
				}
				continue
			}
			fed.CurrentTime = requestedTime

			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			if tagPayload, err := wire.BuildTimeAdvanceGrantPayload(federateID, requestedTime); err == nil {
				if err := session.SendNotification(uint8(wire.MsgTimeAdvanceGrant), tagPayload); err != nil {
				logCRCPortico.Warn("failed to send TimeAdvanceGrant to federate %d: %v", federateID, err)
			}
			}
			logCRCPortico.Info("Federate [%d] granted time advance to %f", federateID, requestedTime)

		case wire.MsgResignFederation:
			if currentFed != nil && federateID > 0 {
				if _, err := currentFed.Resign(federateID); err != nil {
					logCRCPortico.Warn("failed to resign federate %d: %v", federateID, err)
				}
			}
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
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
					if _, err := currentFed.DeleteObjectInstance(objHandle, federateID, nil); err != nil {
						logCRCPortico.Warn("failed to delete object [%d] for federate [%d]: %v", objHandle, federateID, err)
						if sendErr := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "DeletePrivilegeNotHeld"); sendErr != nil {
							return sendErr
						}
						continue
					}
					logCRCPortico.Info("Object [%d] deleted by federate [%d]", objHandle, federateID)

					removePayload, _ := common.RemoveObjectInstancePayload{
						ObjectHandle: common.ObjectInstanceHandle(objHandle),
					}.MarshalBinary()

					porticoDeletePayload, _ := wire.BuildDeleteObjectPayload(objHandle, nil)
					for _, f := range currentFed.GetAllFederates() {
						if f.ID != federateID && f.Session != nil {
							if f.Protocol == ProtocolPortico && porticoDeletePayload != nil {
								if err := f.Session.SendNotification(uint8(wire.MsgDeleteObject), porticoDeletePayload); err != nil {
									logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
								}
							} else {
								if err := f.Session.SendNotification(common.NativeMsgRemoveObjectInstance, removePayload); err != nil {
									logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
								}
							}
						}
					}
				}
			}
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}

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
				if err := fedManager.DestroyFederation(fedName); err != nil {
					logCRCPortico.Warn("failed to destroy federation [%s]: %v", fedName, err)
					if sendErr := session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederationExecutionDoesNotExist: "+fedName); sendErr != nil {
						return sendErr
					}
					continue
				}
				logCRCPortico.Info("Federation [%s] destroyed", fedName)
				currentFed = nil
			}
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}

		case wire.MsgDisconnect:
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			logCRCPortico.Info("Connection disconnected gracefully")
			return nil

		case wire.MsgSaveRequest:
			label := "Checkpoint"
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if l, ok := obj.Get("label").(string); ok && l != "" {
					label = l
				}
			}
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			logCRCPortico.Info("Federate [%d] requested federation save with label '%s'", federateID, label)

			saveInitObj := metamodel.NewJavaObject(metamodel.ClassDescObject)
			saveInitObj.Set("label", label)
			if savePayload, err := wire.SerializePorticoControlMessage(saveInitObj); err == nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.Session != nil && f.Protocol == ProtocolPortico {
						if err := f.Session.SendNotification(uint8(wire.MsgSaveInitiate), savePayload); err != nil {
							logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
						}
					}
				}
			}

		case wire.MsgSaveBegun:
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}

		case wire.MsgSaveComplete:
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			logCRCPortico.Info("Federate [%d] completed save. Broadcasting FederationSaved.", federateID)
			for _, f := range currentFed.GetAllFederates() {
				if f.Session != nil && f.Protocol == ProtocolPortico {
					if err := f.Session.SendNotification(uint8(wire.MsgFederationSaved), wire.BuildSuccessResponsePayload()); err != nil {
						logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
					}
				}
			}

		case wire.MsgRestoreRequest:
			label := "Checkpoint"
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if l, ok := obj.Get("label").(string); ok && l != "" {
					label = l
				}
			}
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			logCRCPortico.Info("Federate [%d] requested federation restore with label '%s'", federateID, label)

			restoreInitObj := metamodel.NewJavaObject(metamodel.ClassDescObject)
			restoreInitObj.Set("label", label)
			if restPayload, err := wire.SerializePorticoControlMessage(restoreInitObj); err == nil {
				for _, f := range currentFed.GetAllFederates() {
					if f.Session != nil && f.Protocol == ProtocolPortico {
						if err := f.Session.SendNotification(uint8(wire.MsgRestoreInitiate), restPayload); err != nil {
							logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
						}
					}
				}
			}

		case wire.MsgRestoreComplete:
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			logCRCPortico.Info("Federate [%d] completed restore. Broadcasting FederationRestored.", federateID)
			for _, f := range currentFed.GetAllFederates() {
				if f.Session != nil && f.Protocol == ProtocolPortico {
					if err := f.Session.SendNotification(uint8(wire.MsgFederationRestored), wire.BuildSuccessResponsePayload()); err != nil {
						logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
					}
				}
			}

		case wire.MsgAttributeDivest:
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			logCRCPortico.Info("Federate [%d] divested attribute ownership", federateID)

		case wire.MsgQueryOwnership:
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
			logCRCPortico.Info("Federate [%d] queried attribute ownership", federateID)

		default:
			if hdr.CallType == wire.CallTypeControlRequest {
				if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
					return err
				}
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
