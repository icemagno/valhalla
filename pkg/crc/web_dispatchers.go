package crc

import (
	"errors"
	"strings"

	"github.com/icemagno/valhalla/pkg/common"
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

		if *ctx.currentFed != nil && *ctx.federateID > 0 {
			ctx.sendError("FederateAlreadyExecutionMember", "Session is already joined to a federation execution")
			return true
		}

		fed, err := fedExec.Join(fedNameParam, fedType, ProtocolWeb, ctx.session)
		if err != nil {
			errCode := "RTIinternalError"
			if errors.Is(err, ErrFederateNameAlreadyInUse) || errors.Is(err, common.ErrFederateNameAlreadyInUse) {
				errCode = "FederateNameAlreadyInUse"
			} else if errors.Is(err, ErrFederationExecutionDoesNotExist) || errors.Is(err, common.ErrFederationExecutionDoesNotExist) {
				errCode = "FederationExecutionDoesNotExist"
			}
			ctx.sendError(errCode, err.Error())
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
