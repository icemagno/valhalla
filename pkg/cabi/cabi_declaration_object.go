package main

/*
#include "valhalla_lrc.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"
)


//export valhalla_get_object_class_handle
func valhalla_get_object_class_handle(amb C.ValhallaAmbassador, className *C.char, outHandle *C.int32_t) C.int32_t {
	if amb == nil || className == nil || outHandle == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	name := C.GoString(className)
	handle, err := ctx.amb.GetObjectClassHandle(name)
	if err != nil {
		return mapErrorToStatusCode(err)
	}
	if outHandle != nil {
		*outHandle = C.int32_t(handle)
	}
	return C.VALHALLA_OK
}

//export valhalla_get_attribute_handle
func valhalla_get_attribute_handle(amb C.ValhallaAmbassador, classHandle C.int32_t, attrName *C.char, outHandle *C.int32_t) C.int32_t {
	if amb == nil || attrName == nil || outHandle == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	name := C.GoString(attrName)
	handle, err := ctx.amb.GetAttributeHandle(int32(classHandle), name)
	if err != nil {
		return mapErrorToStatusCode(err)
	}
	if outHandle != nil {
		*outHandle = C.int32_t(handle)
	}
	return C.VALHALLA_OK
}

//export valhalla_get_dimension_handle
func valhalla_get_dimension_handle(amb C.ValhallaAmbassador, dimName *C.char, outHandle *C.int32_t) C.int32_t {
	if amb == nil || dimName == nil || outHandle == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	name := C.GoString(dimName)
	handle, err := ctx.amb.GetDimensionHandle(name)
	if err != nil {
		return mapErrorToStatusCode(err)
	}
	if outHandle != nil {
		*outHandle = C.int32_t(handle)
	}
	return C.VALHALLA_OK
}

//export valhalla_publish_object_class
func valhalla_publish_object_class(amb C.ValhallaAmbassador, classHandle C.int32_t, attrHandles *C.int32_t, attrCount C.int32_t) C.int32_t {
	if amb == nil || !validateCABICount(int32(attrCount)) || (attrCount > 0 && attrHandles == nil) {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	count := int(attrCount)
	attrs := make([]int32, count)
	if count > 0 && attrHandles != nil {
		cSlice := unsafe.Slice(attrHandles, count)
		for i := 0; i < count; i++ {
			attrs[i] = int32(cSlice[i])
		}
	}
	if err := ctx.amb.PublishObjectClassAttributes(int32(classHandle), attrs); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_unpublish_object_class
func valhalla_unpublish_object_class(amb C.ValhallaAmbassador, classHandle C.int32_t) C.int32_t {
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.UnpublishObjectClass(int32(classHandle)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_subscribe_object_class
func valhalla_subscribe_object_class(amb C.ValhallaAmbassador, classHandle C.int32_t, attrHandles *C.int32_t, attrCount C.int32_t) C.int32_t {
	if amb == nil || !validateCABICount(int32(attrCount)) || (attrCount > 0 && attrHandles == nil) {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	count := int(attrCount)
	attrs := make([]int32, count)
	if count > 0 && attrHandles != nil {
		cSlice := unsafe.Slice(attrHandles, count)
		for i := 0; i < count; i++ {
			attrs[i] = int32(cSlice[i])
		}
	}
	if err := ctx.amb.SubscribeObjectClassAttributes(int32(classHandle), attrs); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_unsubscribe_object_class
func valhalla_unsubscribe_object_class(amb C.ValhallaAmbassador, classHandle C.int32_t) C.int32_t {
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.UnsubscribeObjectClass(int32(classHandle)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_publish_interaction_class
func valhalla_publish_interaction_class(amb C.ValhallaAmbassador, interactionClass C.int32_t) C.int32_t {
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.PublishInteractionClass(int32(interactionClass)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_unpublish_interaction_class
func valhalla_unpublish_interaction_class(amb C.ValhallaAmbassador, interactionClass C.int32_t) C.int32_t {
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.UnpublishInteractionClass(int32(interactionClass)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_subscribe_interaction_class
func valhalla_subscribe_interaction_class(amb C.ValhallaAmbassador, interactionClass C.int32_t) C.int32_t {
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.SubscribeInteractionClass(int32(interactionClass)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_unsubscribe_interaction_class
func valhalla_unsubscribe_interaction_class(amb C.ValhallaAmbassador, interactionClass C.int32_t) C.int32_t {
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.UnsubscribeInteractionClass(int32(interactionClass)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_register_object
func valhalla_register_object(amb C.ValhallaAmbassador, classHandle C.int32_t, objectName *C.char, outObjectHandle *C.int32_t) C.int32_t {
	if amb == nil || outObjectHandle == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	name := ""
	if objectName != nil {
		name = C.GoString(objectName)
	}
	handle, err := ctx.amb.RegisterObjectInstance(int32(classHandle), name)
	if err != nil {
		return mapErrorToStatusCode(err)
	}
	if outObjectHandle != nil {
		*outObjectHandle = C.int32_t(handle)
	}
	return C.VALHALLA_OK
}

//export valhalla_delete_object
func valhalla_delete_object(amb C.ValhallaAmbassador, objectHandle C.int32_t, tag *C.uint8_t, tagLen C.int32_t) C.int32_t {
	if amb == nil || !validateCABITagLen(int32(tagLen)) || (tagLen > 0 && tag == nil) {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	var goTag []byte
	if tagLen > 0 && tag != nil {
		goTag = C.GoBytes(unsafe.Pointer(tag), tagLen)
	}
	if err := ctx.amb.DeleteObjectInstance(int32(objectHandle), goTag); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_update_attributes
func valhalla_update_attributes(amb C.ValhallaAmbassador, objectHandle C.int32_t, attrHandles *C.int32_t, attrValues **C.uint8_t, attrLens *C.int32_t, attrCount C.int32_t, tag *C.uint8_t, tagLen C.int32_t) C.int32_t {
	if amb == nil || !validateCABICount(int32(attrCount)) || (attrCount > 0 && (attrHandles == nil || attrValues == nil || attrLens == nil)) || !validateCABITagLen(int32(tagLen)) || (tagLen > 0 && tag == nil) {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	count := int(attrCount)
	attrs := make(map[int32][]byte, count)
	if count > 0 && attrHandles != nil && attrValues != nil && attrLens != nil {
		cHdls := unsafe.Slice(attrHandles, count)
		cVals := unsafe.Slice(attrValues, count)
		cLns := unsafe.Slice(attrLens, count)
		for i := 0; i < count; i++ {
			h := int32(cHdls[i])
			ln := int32(cLns[i])
			if !validateCABIValueLen(ln) {
				return C.VALHALLA_ERR_INVALID_ARG
			}
			if ln > 0 && cVals[i] != nil {
				attrs[h] = C.GoBytes(unsafe.Pointer(cVals[i]), C.int(ln))
			} else {
				attrs[h] = []byte{}
			}
		}
	}
	var goTag []byte
	if tagLen > 0 && tag != nil {
		goTag = C.GoBytes(unsafe.Pointer(tag), tagLen)
	}
	if err := ctx.amb.UpdateAttributeValues(int32(objectHandle), attrs, goTag); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_send_interaction
func valhalla_send_interaction(amb C.ValhallaAmbassador, interactionClass C.int32_t, paramHandles *C.int32_t, paramValues **C.uint8_t, paramLens *C.int32_t, paramCount C.int32_t, tag *C.uint8_t, tagLen C.int32_t) C.int32_t {
	if amb == nil || !validateCABICount(int32(paramCount)) || (paramCount > 0 && (paramHandles == nil || paramValues == nil || paramLens == nil)) || !validateCABITagLen(int32(tagLen)) || (tagLen > 0 && tag == nil) {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	count := int(paramCount)
	params := make(map[int32][]byte, count)
	if count > 0 && paramHandles != nil && paramValues != nil && paramLens != nil {
		cHdls := unsafe.Slice(paramHandles, count)
		cVals := unsafe.Slice(paramValues, count)
		cLns := unsafe.Slice(paramLens, count)
		for i := 0; i < count; i++ {
			h := int32(cHdls[i])
			ln := int32(cLns[i])
			if !validateCABIValueLen(ln) {
				return C.VALHALLA_ERR_INVALID_ARG
			}
			if ln > 0 && cVals[i] != nil {
				params[h] = C.GoBytes(unsafe.Pointer(cVals[i]), C.int(ln))
			} else {
				params[h] = []byte{}
			}
		}
	}
	var goTag []byte
	if tagLen > 0 && tag != nil {
		goTag = C.GoBytes(unsafe.Pointer(tag), tagLen)
	}
	if err := ctx.amb.SendInteraction(int32(interactionClass), params, goTag); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}
