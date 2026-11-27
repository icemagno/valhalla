package crc

import (
	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
	fomparser "github.com/icemagno/valhalla/pkg/metamodel"
)

type porticoContext struct {
	session    *PorticoSession
	fedManager *FederationManager
	currentFed *FederationExecution
	federateID int32
}

// handleFederationManagement handles Portico federation lifecycle (Create, Join, Resign, Destroy, Disconnect)
func (ctx *porticoContext) handleFederationManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgCreateFederation:
		var fedName string
		obj, err := wire.DeserializePorticoControlMessage(payload)
		if err != nil {
			logCRCPortico.Error("Error deserializing CreateFederation: %v", err)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "CouldNotOpenFDD: malformed CreateFederation request"); err != nil {
				return true, err
			}
			return true, nil
		}
		if n, ok := obj.Get("federationName").(string); ok && n != "" {
			fedName = n
		}
		if fedName == "" {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "CouldNotOpenFDD: empty federation name"); err != nil {
				return true, err
			}
			return true, nil
		}

		fedExec, err := ctx.fedManager.CreateFederation(fedName)
		if err != nil {
			logCRCPortico.Error("CreateFederation failed for [%s]: %v", fedName, err)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
				return true, err
			}
			return true, nil
		}
		ctx.currentFed = fedExec

		// Ingest any FOM modules provided in the CreateFederation request
		if obj != nil {
			for _, key := range []string{"rawFomModules", "modules", "fomModuleLocations", "rawFomModuleLocations"} {
				if val := obj.Get(key); val != nil {
					ingestPorticoFOMModules(ctx.currentFed.OMT(), val)
				}
			}
		}

		// Canonical FOM modules fallback
		if fedName == "ExampleFederation" {
			for _, mod := range []string{"RestaurantProcesses.xml", "RestaurantFood.xml", "RestaurantDrinks.xml", "RestaurantSoup.xml"} {
				if resolved := resolveFOMPath(mod); resolved != "" {
					if err := fomparser.ParseFOMFile(ctx.currentFed.OMT(), resolved); err == nil {
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
				if err := fomparser.ParseFOMFile(ctx.currentFed.OMT(), resolved); err == nil {
					logCRCPortico.Info("Ingested advanced FOM module '%s' (from '%s') for [%s]", "ValhallaAdvanced.xml", resolved, fedName)
				} else {
					logCRCPortico.Warn("Warning: failed to parse advanced module '%s': %v", resolved, err)
				}
			} else {
				logCRCPortico.Warn("Warning: could not resolve advanced module 'ValhallaAdvanced.xml' on disk")
			}
		}

		respPayload := wire.BuildCreateFederationResponsePayload(ctx.currentFed.Handle, fedName)
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federation [%s] created with handle [%d]", fedName, ctx.currentFed.Handle)
		return true, nil

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
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: federation and federate names are mandatory"); err != nil {
				return true, err
			}
			return true, nil
		}

		fedExec, exists := ctx.fedManager.GetFederation(fedName)
		if !exists {
			logCRCPortico.Error("Join error: federation [%s] does not exist", fedName)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederationExecutionDoesNotExist: "+fedName); err != nil {
				return true, err
			}
			return true, nil
		}

		// Ingest join-time FOM modules
		if obj != nil {
			for _, key := range []string{"rawFomModules", "modules", "fomModuleLocations", "rawFomModuleLocations"} {
				if val := obj.Get(key); val != nil {
					ingestPorticoFOMModules(fedExec.OMT(), val)
				}
			}
		}

		fed, err := fedExec.Join(nodeName, nodeType, ProtocolPortico, ctx.session)
		if err != nil {
			logCRCPortico.Error("Join error for %s: %v", nodeName, err)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
				return true, err
			}
			return true, nil
		}

		ctx.currentFed = fedExec
		ctx.federateID = fed.ID
		ctx.session.SetFederateID(ctx.federateID)
		ctx.session.SetFederationHandle(ctx.currentFed.Handle)

		// Safety check: ensure canonical / advanced modules are loaded before WelcomePack
		if fedName == "ExampleFederation" {
			if _, hasSoda := ctx.currentFed.OMT().GetObjectClassByName("Soda"); !hasSoda {
				for _, mod := range []string{"RestaurantProcesses.xml", "RestaurantFood.xml", "RestaurantDrinks.xml", "RestaurantSoup.xml"} {
					if resolved := resolveFOMPath(mod); resolved != "" {
						if err := fomparser.ParseFOMFile(ctx.currentFed.OMT(), resolved); err == nil {
							logCRCPortico.Info("Ingested canonical FOM module '%s' (from '%s') for [%s]", mod, resolved, fedName)
						}
					}
				}
			}
		} else if fedName == "ValhallaAdvancedFed" {
			if _, hasTactical := ctx.currentFed.OMT().GetObjectClassByName("TacticalEntity"); !hasTactical {
				if resolved := resolveFOMPath("ValhallaAdvanced.xml"); resolved != "" {
					if err := fomparser.ParseFOMFile(ctx.currentFed.OMT(), resolved); err == nil {
						logCRCPortico.Info("Ingested advanced FOM module '%s' (from '%s') for [%s]", "ValhallaAdvanced.xml", resolved, fedName)
					}
				}
			}
		}

		wpPayload, err := wire.BuildWelcomePackPayloadWithFOM(ctx.currentFed.Handle, ctx.federateID, ctx.currentFed.OMT(), fedName, nodeName, nodeType)
		if err != nil {
			logCRCPortico.Error("Error generating WelcomePack: %v", err)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: failed to build WelcomePack"); err != nil {
				return true, err
			}
			return true, nil
		}

		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wpPayload); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%s] joined with handle [%d] to federation [%s]", nodeName, ctx.federateID, fedName)
		return true, nil

	case wire.MsgResignFederation:
		if ctx.currentFed != nil && ctx.federateID > 0 {
			if _, err := ctx.currentFed.Resign(ctx.federateID); err != nil {
				logCRCPortico.Warn("failed to resign federate %d: %v", ctx.federateID, err)
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] resigned gracefully", ctx.federateID)
		ctx.federateID = 0
		return true, nil

	case wire.MsgDestroyFederation:
		var fedName string
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if fn, ok := obj.Get("federationName").(string); ok {
				fedName = fn
			}
		}
		if fedName == "" && ctx.currentFed != nil {
			fedName = ctx.currentFed.Name
		}
		if fedName != "" {
			if err := ctx.fedManager.DestroyFederation(fedName); err != nil {
				logCRCPortico.Warn("failed to destroy federation [%s]: %v", fedName, err)
				if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederationExecutionDoesNotExist: "+fedName); sendErr != nil {
					return true, sendErr
				}
				return true, nil
			}
			logCRCPortico.Info("Federation [%s] destroyed", fedName)
			ctx.currentFed = nil
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgDisconnect:
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Connection disconnected gracefully")
		return true, nil
	}
	return false, nil
}

// handleSyncPoints handles Portico synchronization points
func (ctx *porticoContext) handleSyncPoints(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgRegisterSyncPoint:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		var label string
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if l, ok := obj.Get("label").(string); ok && l != "" {
				label = l
			}
		}
		if label == "" {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: empty sync point label"); err != nil {
				return true, err
			}
			return true, nil
		}

		_, err := ctx.currentFed.RegisterSyncPoint(label, nil, ctx.federateID)
		if err != nil {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
				return true, err
			}
			return true, nil
		}

		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Synchronization point '%s' registered by federate [%d]", label, ctx.federateID)

		if err := ctx.currentFed.BroadcastSyncPointAnnounce(label, nil); err != nil {
			logCRCPortico.Warn("failed to broadcast sync point announce: %v", err)
		}
		return true, nil

	case wire.MsgAchieveSyncPoint:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		var label string
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if l, ok := obj.Get("label").(string); ok && l != "" {
				label = l
			}
		}
		if label == "" {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "RTIinternalError: empty sync point label"); err != nil {
				return true, err
			}
			return true, nil
		}

		isSynced, err := ctx.currentFed.AchieveSyncPoint(label, ctx.federateID)
		if err != nil {
			logCRCPortico.Error("AchieveSyncPoint error: %v", err)
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
				return true, err
			}
			return true, nil
		}

		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] achieved synchronization point '%s'", ctx.federateID, label)

		if isSynced {
			logCRCPortico.Info(">>> FEDERATION SYNCHRONIZED at point '%s'!", label)
			if err := ctx.currentFed.BroadcastFederationSynchronized(label); err != nil {
				logCRCPortico.Warn("failed to broadcast federation synchronized: %v", err)
			}
		}
		return true, nil
	}
	return false, nil
}

// handleDeclarationManagement handles Portico publish/subscribe
func (ctx *porticoContext) handleDeclarationManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgPublishObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			classHandle, _ := obj.Get("classHandle").(int32)
			attrHandles := extractAttributeHandles(obj.Get("attributes"))
			if ctx.currentFed.Router() != nil && classHandle > 0 {
				if err := ctx.currentFed.Router().PublishObjectClass(common.FederateHandle(ctx.federateID), common.ObjectClassHandle(classHandle), attrHandles); err != nil {
					logCRCPortico.Warn("failed to publish object class: %v", err)
					if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
						return true, sendErr
					}
					return true, nil
				}
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgSubscribeObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			classHandle, _ := obj.Get("classHandle").(int32)
			attrHandles := extractAttributeHandles(obj.Get("attributes"))
			if ctx.currentFed.Router() != nil && classHandle > 0 {
				if err := ctx.currentFed.Router().SubscribeObjectClass(common.FederateHandle(ctx.federateID), common.ObjectClassHandle(classHandle), attrHandles, true); err != nil {
					logCRCPortico.Warn("failed to subscribe object class: %v", err)
					if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
						return true, sendErr
					}
					return true, nil
				}
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgPublishInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			classHandle, _ := obj.Get("classHandle").(int32)
			if ctx.currentFed.Router() != nil && classHandle > 0 {
				if err := ctx.currentFed.Router().PublishInteractionClass(common.FederateHandle(ctx.federateID), common.InteractionClassHandle(classHandle)); err != nil {
					logCRCPortico.Warn("failed to publish interaction class: %v", err)
					if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
						return true, sendErr
					}
					return true, nil
				}
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgSubscribeInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			classHandle, _ := obj.Get("classHandle").(int32)
			if ctx.currentFed.Router() != nil && classHandle > 0 {
				if err := ctx.currentFed.Router().SubscribeInteractionClass(common.FederateHandle(ctx.federateID), common.InteractionClassHandle(classHandle), true); err != nil {
					logCRCPortico.Warn("failed to subscribe interaction class: %v", err)
					if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
						return true, sendErr
					}
					return true, nil
				}
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

// handleObjectManagement delegates Portico object operations directly to the unified domain service
func (ctx *porticoContext) handleObjectManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgRegisterObject:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
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
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectClassNotDefined"); err != nil {
				return true, err
			}
			return true, nil
		}

		objInst, err := ctx.currentFed.RegisterAndBroadcastObject(classHandle, objectName, ctx.federateID)
		if err != nil {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); err != nil {
				return true, err
			}
			return true, nil
		}

		respPayload := wire.BuildRegisterObjectResponsePayload(objInst.Handle, classHandle, objInst.Name)
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload); err != nil {
			return true, err
		}
		logCRCPortico.Info("Object registered by federate [%d]: handle [%d], name '%s'", ctx.federateID, objInst.Handle, objInst.Name)
		return true, nil

	case wire.MsgDeleteObject:
		if ctx.currentFed != nil && ctx.federateID > 0 {
			var objHandle int32
			if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
				if h, ok := obj.Get("objectHandle").(int32); ok {
					objHandle = h
				}
			}
			if objHandle > 0 {
				if _, err := ctx.currentFed.DeleteAndBroadcastObject(objHandle, ctx.federateID, nil); err != nil {
					logCRCPortico.Warn("failed to delete object [%d] for federate [%d]: %v", objHandle, ctx.federateID, err)
					if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "DeletePrivilegeNotHeld"); sendErr != nil {
						return true, sendErr
					}
					return true, nil
				}
				logCRCPortico.Info("Object [%d] deleted by federate [%d]", objHandle, ctx.federateID)
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgUpdateAttributes:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
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
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectInstanceNotKnown"); err != nil {
				return true, err
			}
			return true, nil
		}

		nativeUavAttrs := make(map[common.AttributeHandle][]byte, len(attrsMap))
		for k, v := range attrsMap {
			nativeUavAttrs[common.AttributeHandle(k)] = v
		}

		if _, err := ctx.currentFed.UpdateAndRouteAttributes(ctx.federateID, common.ObjectInstanceHandle(objectId), nativeUavAttrs, tag, false, 0, nil, [32]byte{}); err != nil {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectInstanceNotKnown"); err != nil {
				return true, err
			}
			return true, nil
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgSendInteraction:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
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
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InteractionClassNotDefined"); err != nil {
				return true, err
			}
			return true, nil
		}
		if _, exists := ctx.currentFed.OMT().GetInteractionClass(common.InteractionClassHandle(interactionId)); !exists {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InteractionClassNotDefined"); err != nil {
				return true, err
			}
			return true, nil
		}

		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}

		nativeParams := make(map[common.ParameterHandle][]byte, len(paramsMap))
		for k, v := range paramsMap {
			nativeParams[common.ParameterHandle(k)] = v
		}

		if err := ctx.currentFed.RouteInteraction(ctx.federateID, common.InteractionClassHandle(interactionId), nativeParams, tag, false, 0); err != nil {
			logCRCPortico.Warn("failed to route interaction %d: %v", interactionId, err)
		}
		return true, nil
	}
	return false, nil
}

// handleTimeManagement handles Portico time advancement services
func (ctx *porticoContext) handleTimeManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgEnableTimeRegulation:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		fed, exists := ctx.currentFed.GetFederate(ctx.federateID)
		if !exists {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotFound"); err != nil {
				return true, err
			}
			return true, nil
		}
		if fed.IsRegulating {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "TimeRegulationAlreadyEnabled"); err != nil {
				return true, err
			}
			return true, nil
		}
		fed.IsRegulating = true

		respPayload := wire.BuildDoubleResponsePayload(fed.CurrentTime)
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, respPayload); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] enabled time regulation at time %f", ctx.federateID, fed.CurrentTime)
		return true, nil

	case wire.MsgEnableTimeConstrained:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		fed, exists := ctx.currentFed.GetFederate(ctx.federateID)
		if !exists {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotFound"); err != nil {
				return true, err
			}
			return true, nil
		}
		if fed.IsConstrained {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "TimeConstrainedAlreadyEnabled"); err != nil {
				return true, err
			}
			return true, nil
		}
		fed.IsConstrained = true

		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] enabled time constrained", ctx.federateID)
		return true, nil

	case wire.MsgTimeAdvanceRequest, wire.MsgNextEventRequest, wire.MsgFlushQueueRequest:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		fed, exists := ctx.currentFed.GetFederate(ctx.federateID)
		if !exists {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotFound"); err != nil {
				return true, err
			}
			return true, nil
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
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InvalidLogicalTime"); err != nil {
				return true, err
			}
			return true, nil
		}
		fed.CurrentTime = requestedTime

		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		if tagPayload, err := wire.BuildTimeAdvanceGrantPayload(ctx.federateID, requestedTime); err == nil {
			if err := ctx.session.SendNotification(uint8(wire.MsgTimeAdvanceGrant), tagPayload); err != nil {
				logCRCPortico.Warn("failed to send TimeAdvanceGrant to federate %d: %v", ctx.federateID, err)
			}
		}
		logCRCPortico.Info("Federate [%d] granted time advance to %f", ctx.federateID, requestedTime)
		return true, nil
	}
	return false, nil
}

// handleSaveRestoreManagement handles Portico federation save and restore
func (ctx *porticoContext) handleSaveRestoreManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgSaveRequest:
		label := "Checkpoint"
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if l, ok := obj.Get("label").(string); ok && l != "" {
				label = l
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] requested federation save with label '%s'", ctx.federateID, label)

		saveInitObj := metamodel.NewJavaObject(metamodel.ClassDescObject)
		saveInitObj.Set("label", label)
		if savePayload, err := wire.SerializePorticoControlMessage(saveInitObj); err == nil {
			for _, f := range ctx.currentFed.GetAllFederates() {
				if f.Session != nil && f.Protocol == ProtocolPortico {
					if err := f.Session.SendNotification(uint8(wire.MsgSaveInitiate), savePayload); err != nil {
						logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
					}
				}
			}
		}
		return true, nil

	case wire.MsgSaveBegun:
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgSaveComplete:
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] completed save. Broadcasting FederationSaved.", ctx.federateID)
		for _, f := range ctx.currentFed.GetAllFederates() {
			if f.Session != nil && f.Protocol == ProtocolPortico {
				if err := f.Session.SendNotification(uint8(wire.MsgFederationSaved), wire.BuildSuccessResponsePayload()); err != nil {
					logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
				}
			}
		}
		return true, nil

	case wire.MsgRestoreRequest:
		label := "Checkpoint"
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if l, ok := obj.Get("label").(string); ok && l != "" {
				label = l
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] requested federation restore with label '%s'", ctx.federateID, label)

		restoreInitObj := metamodel.NewJavaObject(metamodel.ClassDescObject)
		restoreInitObj.Set("label", label)
		if restPayload, err := wire.SerializePorticoControlMessage(restoreInitObj); err == nil {
			for _, f := range ctx.currentFed.GetAllFederates() {
				if f.Session != nil && f.Protocol == ProtocolPortico {
					if err := f.Session.SendNotification(uint8(wire.MsgRestoreInitiate), restPayload); err != nil {
						logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
					}
				}
			}
		}
		return true, nil

	case wire.MsgRestoreComplete:
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] completed restore. Broadcasting FederationRestored.", ctx.federateID)
		for _, f := range ctx.currentFed.GetAllFederates() {
			if f.Session != nil && f.Protocol == ProtocolPortico {
				if err := f.Session.SendNotification(uint8(wire.MsgFederationRestored), wire.BuildSuccessResponsePayload()); err != nil {
					logCRCPortico.Warn("failed to dispatch notification to federate %d: %v", f.ID, err)
				}
			}
		}
		return true, nil
	}
	return false, nil
}

// handleOwnershipManagement handles Portico ownership queries and divestitures
func (ctx *porticoContext) handleOwnershipManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgAttributeDivest:
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] divested attribute ownership", ctx.federateID)
		return true, nil

	case wire.MsgQueryOwnership:
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] queried attribute ownership", ctx.federateID)
		return true, nil
	}
	return false, nil
}
