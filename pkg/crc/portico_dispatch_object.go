package crc

import (
	"errors"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

// handleDeclarationManagement handles Portico publish/subscribe.
func (ctx *porticoContext) handleDeclarationManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgPublishObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		obj, err := wire.DeserializePorticoControlMessage(payload)
		if err != nil || obj == nil {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectClassNotDefined"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		classHandle, _ := obj.Get("classHandle").(int32)
		if classHandle <= 0 {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectClassNotDefined"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		attrHandles := extractAttributeHandles(obj.Get("attributes"))
		if ctx.currentFed.Router() != nil {
			if err := ctx.currentFed.Router().PublishObjectClass(common.FederateHandle(ctx.federateID), common.ObjectClassHandle(classHandle), attrHandles); err != nil {
				logCRCPortico.Warn("failed to publish object class: %v", err)
				if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
					return true, sendErr
				}
				return true, nil
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
		obj, err := wire.DeserializePorticoControlMessage(payload)
		if err != nil || obj == nil {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectClassNotDefined"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		classHandle, _ := obj.Get("classHandle").(int32)
		if classHandle <= 0 {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectClassNotDefined"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		attrHandles := extractAttributeHandles(obj.Get("attributes"))
		if ctx.currentFed.Router() != nil {
			if err := ctx.currentFed.Router().SubscribeObjectClass(common.FederateHandle(ctx.federateID), common.ObjectClassHandle(classHandle), attrHandles, true); err != nil {
				logCRCPortico.Warn("failed to subscribe object class: %v", err)
				if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
					return true, sendErr
				}
				return true, nil
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
		obj, err := wire.DeserializePorticoControlMessage(payload)
		if err != nil || obj == nil {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InteractionClassNotDefined"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		classHandle, _ := obj.Get("classHandle").(int32)
		if classHandle <= 0 {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InteractionClassNotDefined"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		if ctx.currentFed.Router() != nil {
			if err := ctx.currentFed.Router().PublishInteractionClass(common.FederateHandle(ctx.federateID), common.InteractionClassHandle(classHandle)); err != nil {
				logCRCPortico.Warn("failed to publish interaction class: %v", err)
				if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
					return true, sendErr
				}
				return true, nil
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
		obj, err := wire.DeserializePorticoControlMessage(payload)
		if err != nil || obj == nil {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InteractionClassNotDefined"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		classHandle, _ := obj.Get("classHandle").(int32)
		if classHandle <= 0 {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InteractionClassNotDefined"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		if ctx.currentFed.Router() != nil {
			if err := ctx.currentFed.Router().SubscribeInteractionClass(common.FederateHandle(ctx.federateID), common.InteractionClassHandle(classHandle), true); err != nil {
				logCRCPortico.Warn("failed to subscribe interaction class: %v", err)
				if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
					return true, sendErr
				}
				return true, nil
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgUnpublishObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if classHandle, ok := obj.Get("classHandle").(int32); ok && classHandle > 0 && ctx.currentFed.Router() != nil {
				_ = ctx.currentFed.Router().UnpublishObjectClass(common.FederateHandle(ctx.federateID), common.ObjectClassHandle(classHandle))
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgUnsubscribeObjectClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if classHandle, ok := obj.Get("classHandle").(int32); ok && classHandle > 0 && ctx.currentFed.Router() != nil {
				_ = ctx.currentFed.Router().UnsubscribeObjectClass(common.FederateHandle(ctx.federateID), common.ObjectClassHandle(classHandle))
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgUnpublishInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if classHandle, ok := obj.Get("classHandle").(int32); ok && classHandle > 0 && ctx.currentFed.Router() != nil {
				_ = ctx.currentFed.Router().UnpublishInteractionClass(common.FederateHandle(ctx.federateID), common.InteractionClassHandle(classHandle))
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil

	case wire.MsgUnsubscribeInteractionClass:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if classHandle, ok := obj.Get("classHandle").(int32); ok && classHandle > 0 && ctx.currentFed.Router() != nil {
				_ = ctx.currentFed.Router().UnsubscribeInteractionClass(common.FederateHandle(ctx.federateID), common.InteractionClassHandle(classHandle))
			}
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

// handleObjectManagement delegates Portico object operations directly to the unified domain service.
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
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		var objHandle int32
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if h, ok := obj.Get("objectHandle").(int32); ok {
				objHandle = h
			}
		}
		if objHandle <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectInstanceNotKnown"); err != nil {
				return true, err
			}
			return true, nil
		}
		if _, err := ctx.currentFed.DeleteAndBroadcastObject(objHandle, ctx.federateID, nil); err != nil {
			logCRCPortico.Warn("failed to delete object [%d] for federate [%d]: %v", objHandle, ctx.federateID, err)
			errMsg := "DeletePrivilegeNotHeld"
			if errors.Is(err, common.ErrObjectInstanceNotKnown) {
				errMsg = "ObjectInstanceNotKnown"
			}
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, errMsg); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		logCRCPortico.Info("Object [%d] deleted by federate [%d]", objHandle, ctx.federateID)
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
		var timestamp float64
		var hasTime bool
		if obj, err := wire.DeserializePorticoControlMessage(payload); err == nil && obj != nil {
			if oid, ok := obj.Get("objectId").(int32); ok {
				objectId = oid
			}
			attrsMap = extractAttributeMap(obj.Get("attributes"))
			if t, ok := obj.Get("tag").([]byte); ok {
				tag = t
			}
			if ts, ok := obj.Get("timestamp").(float64); ok && ts > 0 {
				timestamp = ts
				hasTime = true
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

		if _, err := ctx.currentFed.UpdateAndRouteAttributes(ctx.federateID, common.ObjectInstanceHandle(objectId), nativeUavAttrs, tag, hasTime, timestamp, nil, [32]byte{}); err != nil {
			errMsg := "RTIinternalError: " + err.Error()
			switch {
			case errors.Is(err, common.ErrObjectInstanceNotKnown):
				errMsg = "ObjectInstanceNotKnown"
			case errors.Is(err, core.ErrNotAttributeOwner), errors.Is(err, common.ErrAttributeNotOwned):
				errMsg = "AttributeNotOwned"
			case errors.Is(err, common.ErrInvalidLogicalTime):
				errMsg = "InvalidLogicalTime"
			}
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, errMsg); sendErr != nil {
				return true, sendErr
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

		nativeParams := make(map[common.ParameterHandle][]byte, len(paramsMap))
		for k, v := range paramsMap {
			nativeParams[common.ParameterHandle(k)] = v
		}

		if err := ctx.currentFed.RouteInteraction(ctx.federateID, common.InteractionClassHandle(interactionId), nativeParams, tag, false, 0); err != nil {
			logCRCPortico.Warn("failed to route interaction %d: %v", interactionId, err)
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

// handleOwnershipManagement handles Portico ownership queries and divestitures via core.OwnershipManager.
func (ctx *porticoContext) handleOwnershipManagement(hdr wire.Header, payload []byte) (bool, error) {
	switch hdr.MessageType {
	case wire.MsgAttributeDivest:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		obj, err := wire.DeserializePorticoControlMessage(payload)
		if err != nil || obj == nil {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InvalidDivestPayload"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		objHandle := extractInt32Field(obj, "objectHandle")
		attrs := extractInt32SliceField(obj, "attributes")
		if objHandle <= 0 || len(attrs) == 0 {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "ObjectInstanceNotKnown"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		attrHandles := make([]common.AttributeHandle, len(attrs))
		for i, a := range attrs {
			attrHandles[i] = common.AttributeHandle(a)
		}
		if err := ctx.currentFed.OwnershipManager().UnconditionalDivestiture(common.ObjectInstanceHandle(objHandle), attrHandles, common.FederateHandle(ctx.federateID)); err != nil {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, err.Error()); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
			return true, err
		}
		logCRCPortico.Info("Federate [%d] divested %d attributes on object %d", ctx.federateID, len(attrHandles), objHandle)
		return true, nil

	case wire.MsgQueryOwnership:
		if ctx.currentFed == nil || ctx.federateID <= 0 {
			if err := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "FederateNotExecutionMember"); err != nil {
				return true, err
			}
			return true, nil
		}
		obj, err := wire.DeserializePorticoControlMessage(payload)
		if err != nil || obj == nil {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "InvalidQueryOwnershipPayload"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		objHandle := extractInt32Field(obj, "objectHandle")
		attrHandle := extractInt32Field(obj, "attributeHandle")
		if objHandle <= 0 || attrHandle <= 0 {
			if sendErr := ctx.session.SendControlResponseErr(hdr.RequestId, hdr.MessageType, "AttributeNotDefined"); sendErr != nil {
				return true, sendErr
			}
			return true, nil
		}
		owner, owned := ctx.currentFed.OwnershipManager().QueryAttributeOwnership(common.ObjectInstanceHandle(objHandle), common.AttributeHandle(attrHandle))
		ownerID := int32(-1)
		if owned {
			ownerID = int32(owner)
		}
		if err := ctx.session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildIntegerResponsePayload(ownerID)); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

func extractInt32Field(obj *metamodel.JavaObject, name string) int32 {
	if obj == nil {
		return 0
	}
	switch v := obj.Get(name).(type) {
	case int32:
		return v
	case int:
		return int32(v)
	case *metamodel.JavaObject:
		if iv, ok := v.Get("value").(int32); ok {
			return iv
		}
	}
	return 0
}

func extractInt32SliceField(obj *metamodel.JavaObject, name string) []int32 {
	if obj == nil {
		return nil
	}
	var out []int32
	switch v := obj.Get(name).(type) {
	case []int32:
		return v
	case *metamodel.JavaObject:
		if elems, ok := v.Get("elements").([]any); ok {
			for _, el := range elems {
				switch ev := el.(type) {
				case int32:
					out = append(out, ev)
				case *metamodel.JavaObject:
					if iv, ok := ev.Get("value").(int32); ok {
						out = append(out, iv)
					}
				}
			}
		}
	}
	return out
}
