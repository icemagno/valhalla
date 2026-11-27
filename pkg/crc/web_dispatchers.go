package crc

import (
	"errors"
	"strconv"
	"strings"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

type webContext struct {
	gateway    *WebGateway
	session    *WebSession
	currentFed **FederationExecution
	federateID *int32
	reqID      int64
}

func (ctx *webContext) sendSuccess(data interface{}) {
	if err := ctx.session.SendJSON(map[string]interface{}{
		"id":     ctx.reqID,
		"status": "OK",
		"result": data,
	}); err != nil {
		logCRCWeb.Warn("failed to send JSON success response (reqID=%d): %v", ctx.reqID, err)
	}
}

func (ctx *webContext) sendError(errCode string, msg string) {
	if err := ctx.session.SendJSON(map[string]interface{}{
		"id":      ctx.reqID,
		"status":  "ERROR",
		"code":    errCode,
		"message": msg,
	}); err != nil {
		logCRCWeb.Warn("failed to send JSON error response (reqID=%d, code=%s): %v", ctx.reqID, errCode, err)
	}
}

// handleFederationManagement handles Web federation lifecycle
func (ctx *webContext) handleFederationManagement(action string, params map[string]interface{}) bool {
	switch action {
	case "connect":
		ctx.sendSuccess(map[string]interface{}{
			"server":  "Valhalla CRC",
			"version": "IEEE 1516-2025 (HLA-4 Web Profile)",
		})
		return true

	case "createFederationExecution":
		fedName := getString(params, "federationName")
		if fedName == "" {
			ctx.sendError("InvalidLocalSettingsDesignator", "Missing federationName")
			return true
		}
		fomModules := getStringSlice(params, "fomModules")
		if len(fomModules) == 0 {
			fomSingle := getString(params, "fomModule")
			if fomSingle != "" {
				fomModules = []string{fomSingle}
			}
		}

		var err error
		if len(fomModules) > 0 {
			_, err = ctx.gateway.fedManager.CreateFederationWithFOMs(fedName, fomModules)
		} else {
			_, err = ctx.gateway.fedManager.CreateFederation(fedName)
		}

		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				ctx.sendError("FederationExecutionAlreadyExists", err.Error())
			} else {
				ctx.sendError("RTIinternalError", err.Error())
			}
			return true
		}
		ctx.sendSuccess(map[string]interface{}{"federationName": fedName})
		return true

	case "destroyFederationExecution":
		fedName := getString(params, "federationName")
		err := ctx.gateway.fedManager.DestroyFederation(fedName)
		if err != nil {
			if errors.Is(err, ErrFederationExecutionDoesNotExist) || errors.Is(err, common.ErrFederationExecutionDoesNotExist) {
				ctx.sendError("FederationExecutionDoesNotExist", err.Error())
			} else if errors.Is(err, ErrFederatesCurrentlyJoined) || errors.Is(err, common.ErrFederatesCurrentlyJoined) {
				ctx.sendError("FederatesCurrentlyJoined", err.Error())
			} else {
				ctx.sendError("RTIinternalError", err.Error())
			}
			return true
		}
		ctx.sendSuccess(map[string]interface{}{"federationName": fedName})
		return true

	case "joinFederationExecution":
		fedName := getString(params, "federationName")
		fedNameParam := getString(params, "federateName")
		fedType := getString(params, "federateType")
		if fedType == "" {
			fedType = "WebFederate"
		}

		fedExec, exists := ctx.gateway.fedManager.GetFederation(fedName)
		if !exists {
			ctx.sendError("FederationExecutionDoesNotExist", "Federation does not exist: "+fedName)
			return true
		}

		fed, err := fedExec.Join(fedNameParam, fedType, ProtocolWeb, ctx.session)
		if err != nil {
			ctx.sendError("FederateNameAlreadyInUse", err.Error())
			return true
		}

		*ctx.currentFed = fedExec
		*ctx.federateID = fed.ID
		ctx.session.SetFederateID(fed.ID)

		ctx.sendSuccess(map[string]interface{}{
			"federateHandle": fed.ID,
			"federationName": fedName,
		})
		return true

	case "resignFederationExecution":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		if _, err := (*ctx.currentFed).Resign(*ctx.federateID); err != nil {
			logCRCWeb.Warn("failed to resign federate %d: %v", *ctx.federateID, err)
		}
		*ctx.currentFed = nil
		*ctx.federateID = 0
		ctx.session.SetFederateID(0)
		ctx.sendSuccess(nil)
		return true
	}
	return false
}

// handleMetadataManagement handles Web metadata queries
func (ctx *webContext) handleMetadataManagement(action string, params map[string]interface{}) bool {
	switch action {
	case "getObjectClassHandle":
		if *ctx.currentFed == nil {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		className := getString(params, "className")
		cls, ok := (*ctx.currentFed).OMT().GetObjectClassByName(className)
		if !ok {
			ctx.sendError("NameNotFound", "class not found: "+className)
			return true
		}
		ctx.sendSuccess(map[string]interface{}{"handle": int32(cls.Handle)})
		return true

	case "getAttributeHandle":
		if *ctx.currentFed == nil {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		classH := getInt32(params, "classHandle")
		attrName := getString(params, "attributeName")
		attr, err := (*ctx.currentFed).OMT().ResolveAttributeByName(common.ObjectClassHandle(classH), attrName)
		if err != nil {
			ctx.sendError("NameNotFound", err.Error())
			return true
		}
		ctx.sendSuccess(map[string]interface{}{"handle": int32(attr.Handle)})
		return true

	case "getDimensionHandle":
		if *ctx.currentFed == nil {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		dimName := getString(params, "dimensionName")
		dim, ok := (*ctx.currentFed).OMT().GetDimensionByName(dimName)
		if !ok {
			ctx.sendError("NameNotFound", "dimension not found: "+dimName)
			return true
		}
		ctx.sendSuccess(map[string]interface{}{"handle": int32(dim.Handle)})
		return true
	}
	return false
}

// handleDeclarationManagement handles Web publish and subscribe services
func (ctx *webContext) handleDeclarationManagement(action string, params map[string]interface{}) bool {
	switch action {
	case "publishObjectClassAttributes":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		classH := getInt32(params, "classHandle")
		attrs := getInt32Slice(params, "attributeHandles")
		if len(attrs) == 0 {
			attrs = getInt32Slice(params, "attributes")
		}
		var attrList []common.AttributeHandle
		for _, a := range attrs {
			attrList = append(attrList, common.AttributeHandle(a))
		}
		err := (*ctx.currentFed).Router().PublishObjectClass(common.FederateHandle(*ctx.federateID), common.ObjectClassHandle(classH), attrList)
		if err != nil {
			ctx.sendError("ObjectClassNotDefined", err.Error())
			return true
		}
		ctx.sendSuccess(nil)
		return true

	case "subscribeObjectClassAttributes":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		classH := getInt32(params, "classHandle")
		attrs := getInt32Slice(params, "attributeHandles")
		if len(attrs) == 0 {
			attrs = getInt32Slice(params, "attributes")
		}
		var attrList []common.AttributeHandle
		for _, a := range attrs {
			attrList = append(attrList, common.AttributeHandle(a))
		}
		err := (*ctx.currentFed).Router().SubscribeObjectClass(common.FederateHandle(*ctx.federateID), common.ObjectClassHandle(classH), attrList, true)
		if err != nil {
			ctx.sendError("ObjectClassNotDefined", err.Error())
			return true
		}
		ctx.sendSuccess(nil)
		if err := (*ctx.currentFed).DispatchCatchUpDiscovery(ctx.session, common.ObjectClassHandle(classH)); err != nil {
			logCRCWeb.Warn("failed catch-up discovery for web federate %d: %v", *ctx.federateID, err)
		}
		return true

	case "subscribeObjectClassAttributesWithRegions":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		classH := getInt32(params, "classHandle")
		attrs := getInt32Slice(params, "attributeHandles")
		if len(attrs) == 0 {
			attrs = getInt32Slice(params, "attributes")
		}
		regH := uint32(getInt32(params, "regionHandle"))

		region, exists := (*ctx.currentFed).GetRegion(regH)
		if !exists {
			ctx.sendError("InvalidRegion", "region handle not found")
			return true
		}

		var attrList []common.AttributeHandle
		for _, a := range attrs {
			attrList = append(attrList, common.AttributeHandle(a))
		}
		err := (*ctx.currentFed).Router().SubscribeObjectClassWithRegion(common.FederateHandle(*ctx.federateID), common.ObjectClassHandle(classH), attrList, region)
		if err != nil {
			ctx.sendError("InvalidRegion", err.Error())
			return true
		}
		ctx.sendSuccess(nil)
		return true
	}
	return false
}

// handleObjectManagement delegates Web object operations to the unified domain service
func (ctx *webContext) handleObjectManagement(action string, params map[string]interface{}) bool {
	switch action {
	case "registerObjectInstance":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		classH := getInt32(params, "classHandle")
		objName := getString(params, "objectName")

		obj, err := (*ctx.currentFed).RegisterAndBroadcastObject(classH, objName, *ctx.federateID)
		if err != nil {
			ctx.sendError("ObjectClassNotDefined", err.Error())
			return true
		}
		ctx.sendSuccess(map[string]interface{}{"objectHandle": obj.Handle})
		return true

	case "deleteObjectInstance":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		objH := getInt32(params, "objectHandle")
		tag := getString(params, "tag")

		_, err := (*ctx.currentFed).DeleteAndBroadcastObject(objH, *ctx.federateID, []byte(tag))
		if err != nil {
			ctx.sendError("ObjectInstanceNotKnown", err.Error())
			return true
		}
		ctx.sendSuccess(nil)
		return true

	case "updateAttributeValues":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		objH := getInt32(params, "objectHandle")
		tag := getString(params, "tag")
		rawAttrs := getStringMap(params, "attributes")

		attrsPayloadMap := make(map[common.AttributeHandle][]byte)
		for k, v := range rawAttrs {
			if id, err := strconv.Atoi(k); err == nil {
				attrsPayloadMap[common.AttributeHandle(id)] = []byte(v)
			}
		}

		if _, err := (*ctx.currentFed).UpdateAndRouteAttributes(*ctx.federateID, common.ObjectInstanceHandle(objH), attrsPayloadMap, []byte(tag), false, 0, nil, [32]byte{}); err != nil {
			logCRCWeb.Warn("failed to update object attributes: %v", err)
			ctx.sendError("UpdateAttributesFailed", err.Error())
			return true
		}
		ctx.sendSuccess(nil)
		return true

	case "updateAttributeValuesSpatial":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		objH := getInt32(params, "objectHandle")
		x := getFloat64(params, "x")
		y := getFloat64(params, "y")
		z := getFloat64(params, "z")
		tag := getString(params, "tag")
		rawAttrs := getStringMap(params, "attributes")

		attrsPayloadMap := make(map[common.AttributeHandle][]byte)
		for k, v := range rawAttrs {
			if id, err := strconv.Atoi(k); err == nil {
				attrsPayloadMap[common.AttributeHandle(id)] = []byte(v)
			}
		}

		morton := core.EncodeECEF(x, y, z)
		if _, err := (*ctx.currentFed).UpdateAndRouteAttributes(*ctx.federateID, common.ObjectInstanceHandle(objH), attrsPayloadMap, []byte(tag), false, 0, &morton, [32]byte{}); err != nil {
			logCRCWeb.Warn("failed to update object attributes: %v", err)
			ctx.sendError("UpdateAttributesFailed", err.Error())
			return true
		}
		ctx.sendSuccess(nil)
		return true
	}
	return false
}

// handleDDMManagement handles Web DDM operations
func (ctx *webContext) handleDDMManagement(action string, params map[string]interface{}) bool {
	switch action {
	case "createRegion":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		regH := (*ctx.currentFed).CreateRegion()
		ctx.sendSuccess(map[string]interface{}{"regionHandle": regH})
		return true

	case "setMetricBounds":
		if *ctx.currentFed == nil {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		regH := uint32(getInt32(params, "regionHandle"))
		minX := getFloat64(params, "minX")
		maxX := getFloat64(params, "maxX")
		minY := getFloat64(params, "minY")
		maxY := getFloat64(params, "maxY")
		minZ := getFloat64(params, "minZ")
		maxZ := getFloat64(params, "maxZ")

		err := (*ctx.currentFed).SetRegionMetricBounds(regH, minX, maxX, minY, maxY, minZ, maxZ)
		if err != nil {
			ctx.sendError("InvalidRegion", err.Error())
			return true
		}
		ctx.sendSuccess(nil)
		return true
	}
	return false
}

// handleOwnershipManagement handles Web attribute ownership services
func (ctx *webContext) handleOwnershipManagement(action string, params map[string]interface{}) bool {
	switch action {
	case "isAttributeOwnedByFederate":
		if *ctx.currentFed == nil {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		objH := getInt32(params, "objectHandle")
		attrH := getInt32(params, "attributeHandle")

		owner, owned := (*ctx.currentFed).OwnershipManager().QueryAttributeOwnership(common.ObjectInstanceHandle(objH), common.AttributeHandle(attrH))
		ctx.sendSuccess(map[string]interface{}{
			"isOwned": owned,
			"owner":   int32(owner),
		})
		return true

	case "unconditionalAttributeOwnershipDivestiture":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		objH := getInt32(params, "objectHandle")
		attrs := getInt32Slice(params, "attributeHandles")
		if len(attrs) == 0 {
			attrs = getInt32Slice(params, "attributes")
		}

		var attrList []common.AttributeHandle
		for _, a := range attrs {
			attrList = append(attrList, common.AttributeHandle(a))
		}

		err := (*ctx.currentFed).OwnershipManager().UnconditionalDivestiture(common.ObjectInstanceHandle(objH), attrList, common.FederateHandle(*ctx.federateID))
		if err != nil {
			ctx.sendError("AttributeNotOwned", err.Error())
			return true
		}
		ctx.sendSuccess(nil)
		return true

	case "attributeOwnershipAcquisition":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		objH := getInt32(params, "objectHandle")
		attrs := getInt32Slice(params, "attributeHandles")
		if len(attrs) == 0 {
			attrs = getInt32Slice(params, "attributes")
		}

		var attrList []common.AttributeHandle
		for _, a := range attrs {
			attrList = append(attrList, common.AttributeHandle(a))
		}

		err := (*ctx.currentFed).OwnershipManager().AcquireAttributeOwnership(common.ObjectInstanceHandle(objH), attrList, common.FederateHandle(*ctx.federateID))
		if err != nil {
			ctx.sendError("FederateOwnsAttributes", err.Error())
			return true
		}
		ctx.sendSuccess(nil)
		return true
	}
	return false
}

// handleTimeManagement handles Web time advancement services
func (ctx *webContext) handleTimeManagement(action string, params map[string]interface{}) bool {
	switch action {
	case "timeAdvanceRequest":
		if *ctx.currentFed == nil || *ctx.federateID <= 0 {
			ctx.sendError("FederateNotExecutionMember", "Not joined to federation")
			return true
		}
		reqTime := getFloat64(params, "time")
		grants, err := (*ctx.currentFed).TimeManager().TimeAdvanceRequest(common.FederateHandle(*ctx.federateID), reqTime)
		if err != nil {
			ctx.sendError("InvalidLogicalTime", err.Error())
			return true
		}
		if len(grants) > 0 {
			dispatchTimeGrants(*ctx.currentFed, grants)
		}
		ctx.sendSuccess(nil)
		return true
	}
	return false
}
