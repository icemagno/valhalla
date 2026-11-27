package crc

import (
	"errors"
	"strconv"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

// handleObjectManagement delegates Web object operations to the unified domain service.
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
			errCode := "UpdateAttributesFailed"
			switch {
			case errors.Is(err, common.ErrObjectInstanceNotKnown):
				errCode = "ObjectInstanceNotKnown"
			case errors.Is(err, core.ErrNotAttributeOwner), errors.Is(err, common.ErrAttributeNotOwned):
				errCode = "AttributeNotOwned"
			case errors.Is(err, common.ErrInvalidLogicalTime):
				errCode = "InvalidLogicalTime"
			}
			ctx.sendError(errCode, err.Error())
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

// handleDDMManagement handles Web DDM operations.
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

// handleOwnershipManagement handles Web attribute ownership services.
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

// handleTimeManagement handles Web time advancement services.
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
