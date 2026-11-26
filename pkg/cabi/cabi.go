package main

/*
#include "valhalla_lrc.h"
#include <stdlib.h>

static void call_discover_object(ValhallaDiscoverObjectCallback cb, int32_t theObject, int32_t theObjectClass, const char* objectName) {
    if (cb) cb(theObject, theObjectClass, objectName);
}

static void call_remove_object(ValhallaRemoveObjectCallback cb, int32_t theObject, const uint8_t* tag, int32_t tagLen) {
    if (cb) cb(theObject, tag, tagLen);
}

static void call_time_grant(ValhallaTimeAdvanceGrantCallback cb, double time) {
    if (cb) cb(time);
}

static void call_reflect_attrs(ValhallaReflectAttributesCallback cb, int32_t theObject, int32_t attrCount, const int32_t* attrHandles, const uint8_t** attrValues, const int32_t* attrLens, const uint8_t* tag, int32_t tagLen) {
    if (cb) cb(theObject, attrCount, attrHandles, attrValues, attrLens, tag, tagLen);
}

static void call_receive_interaction(ValhallaReceiveInteractionCallback cb, int32_t interactionClass, int32_t paramCount, const int32_t* paramHandles, const uint8_t** paramValues, const int32_t* paramLens, const uint8_t* tag, int32_t tagLen) {
    if (cb) cb(interactionClass, paramCount, paramHandles, paramValues, paramLens, tag, tagLen);
}

static void call_announce_sync(ValhallaAnnounceSynchronizationPointCallback cb, const char* label, const uint8_t* tag, int32_t tagLen) {
    if (cb) cb(label, tag, tagLen);
}

static void call_federation_sync(ValhallaFederationSynchronizedCallback cb, const char* label) {
    if (cb) cb(label);
}

static void call_sync_reg_succeeded(ValhallaSynchronizationPointRegistrationSucceededCallback cb, const char* label) {
    if (cb) cb(label);
}

static void call_sync_reg_failed(ValhallaSynchronizationPointRegistrationFailedCallback cb, const char* label, const char* reason) {
    if (cb) cb(label, reason);
}
*/
import "C"

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/lrc"
)

func mapErrorToStatusCode(err error) C.int32_t {
	if err == nil {
		return C.VALHALLA_OK
	}

	// 1. Strict typed error checking
	switch {
	case errors.Is(err, lrc.ErrNotConnected):
		return C.VALHALLA_ERR_NOT_CONNECTED
	case errors.Is(err, common.ErrFederationExecutionAlreadyExists):
		return C.VALHALLA_ERR_FEDERATION_EXISTS
	case errors.Is(err, common.ErrFederationExecutionDoesNotExist):
		return C.VALHALLA_ERR_FEDERATION_NOT_FOUND
	case errors.Is(err, common.ErrFederateNameAlreadyInUse):
		return C.VALHALLA_ERR_FEDERATE_NAME_IN_USE
	case errors.Is(err, common.ErrFederatesCurrentlyJoined):
		return C.VALHALLA_ERR_FEDERATES_JOINED
	case errors.Is(err, lrc.ErrFederateAlreadyExecutionMember), errors.Is(err, lrc.ErrFederateNotExecutionMember), errors.Is(err, common.ErrFederateNotExecutionMember):
		return C.VALHALLA_ERR_NOT_MEMBER
	case errors.Is(err, common.ErrAttributeNotOwned):
		return C.VALHALLA_ERR_NOT_OWNER
	case errors.Is(err, common.ErrObjectClassNotDefined):
		return C.VALHALLA_ERR_CLASS_NOT_FOUND
	case errors.Is(err, common.ErrAttributeNotDefined):
		return C.VALHALLA_ERR_ATTR_NOT_FOUND
	case errors.Is(err, common.ErrInteractionClassNotDefined):
		return C.VALHALLA_ERR_INTERACTION_NOT_FOUND
	case errors.Is(err, common.ErrObjectInstanceNotKnown):
		return C.VALHALLA_ERR_OBJECT_NOT_FOUND
	}

	// 2. Fallback to message inspection for remote CRC string responses
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "federates currently joined"):
		return C.VALHALLA_ERR_FEDERATES_JOINED
	case strings.Contains(msg, "federate name already in use"):
		return C.VALHALLA_ERR_FEDERATE_NAME_IN_USE
	case strings.Contains(msg, "already exists"):
		return C.VALHALLA_ERR_FEDERATION_EXISTS
	case strings.Contains(msg, "does not exist") || (strings.Contains(msg, "not found") && strings.Contains(msg, "federation")):
		return C.VALHALLA_ERR_FEDERATION_NOT_FOUND
	case strings.Contains(msg, "interaction") && (strings.Contains(msg, "not defined") || strings.Contains(msg, "not found")):
		return C.VALHALLA_ERR_INTERACTION_NOT_FOUND
	case strings.Contains(msg, "class not defined") || strings.Contains(msg, "class not found") || strings.Contains(msg, "classnotdefined"):
		return C.VALHALLA_ERR_CLASS_NOT_FOUND
	case strings.Contains(msg, "attribute not defined") || strings.Contains(msg, "attribute not found") || strings.Contains(msg, "attributenotdefined"):
		return C.VALHALLA_ERR_ATTR_NOT_FOUND
	case strings.Contains(msg, "object instance not known") || strings.Contains(msg, "object not found") || strings.Contains(msg, "objectinstancenotknown"):
		return C.VALHALLA_ERR_OBJECT_NOT_FOUND
	case strings.Contains(msg, "not member") || strings.Contains(msg, "not execution member"):
		return C.VALHALLA_ERR_NOT_MEMBER
	case strings.Contains(msg, "not connected"):
		return C.VALHALLA_ERR_NOT_CONNECTED
	case strings.Contains(msg, "not owner") || strings.Contains(msg, "attribute not owned"):
		return C.VALHALLA_ERR_NOT_OWNER
	case strings.Contains(msg, "already owned"):
		return C.VALHALLA_ERR_ALREADY_OWNED
	default:
		return C.VALHALLA_ERR_RTI_EXCEPTION
	}
}

type ambContext struct {
	amb lrc.RTIambassador
	cb  C.ValhallaCallbacks
}

type cFedAmbassador struct {
	ctx *ambContext
}

func (f *cFedAmbassador) SynchronizationPointRegistrationSucceeded(label string) {
	if f.ctx.cb.onSyncPointRegistrationSucceeded != nil {
		cLabel := C.CString(label)
		defer C.free(unsafe.Pointer(cLabel))
		C.call_sync_reg_succeeded(f.ctx.cb.onSyncPointRegistrationSucceeded, cLabel)
	}
}

func (f *cFedAmbassador) SynchronizationPointRegistrationFailed(label string, reason string) {
	if f.ctx.cb.onSyncPointRegistrationFailed != nil {
		cLabel := C.CString(label)
		defer C.free(unsafe.Pointer(cLabel))
		cReason := C.CString(reason)
		defer C.free(unsafe.Pointer(cReason))
		C.call_sync_reg_failed(f.ctx.cb.onSyncPointRegistrationFailed, cLabel, cReason)
	}
}

func (f *cFedAmbassador) AnnounceSynchronizationPoint(label string, tag []byte) {
	if f.ctx.cb.onAnnounceSynchronizationPoint != nil {
		cLabel := C.CString(label)
		defer C.free(unsafe.Pointer(cLabel))
		var cTag *C.uint8_t
		if len(tag) > 0 {
			cTag = (*C.uint8_t)(unsafe.Pointer(&tag[0]))
		}
		C.call_announce_sync(f.ctx.cb.onAnnounceSynchronizationPoint, cLabel, cTag, C.int32_t(len(tag)))
	}
}

func (f *cFedAmbassador) FederationSynchronized(label string) {
	if f.ctx.cb.onFederationSynchronized != nil {
		cLabel := C.CString(label)
		defer C.free(unsafe.Pointer(cLabel))
		C.call_federation_sync(f.ctx.cb.onFederationSynchronized, cLabel)
	}
}

func (f *cFedAmbassador) DiscoverObjectInstance(theObject int32, theObjectClass int32, objectName string) {
	if f.ctx.cb.onDiscoverObject != nil {
		cName := C.CString(objectName)
		defer C.free(unsafe.Pointer(cName))
		C.call_discover_object(f.ctx.cb.onDiscoverObject, C.int32_t(theObject), C.int32_t(theObjectClass), cName)
	}
}

func (f *cFedAmbassador) RemoveObjectInstance(theObject int32, tag []byte) {
	if f.ctx.cb.onRemoveObject != nil {
		var cTag *C.uint8_t
		if len(tag) > 0 {
			cTag = (*C.uint8_t)(unsafe.Pointer(&tag[0]))
		}
		C.call_remove_object(f.ctx.cb.onRemoveObject, C.int32_t(theObject), cTag, C.int32_t(len(tag)))
	}
}

func (f *cFedAmbassador) ReflectAttributeValues(theObject int32, theAttributes map[int32][]byte, tag []byte) {
	if f.ctx.cb.onReflectAttributes != nil {
		count := len(theAttributes)
		if count == 0 {
			return
		}

		cHandles := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(C.int32_t(0))))
		defer C.free(cHandles)
		cValues := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(uintptr(0))))
		defer C.free(cValues)
		cLens := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(C.int32_t(0))))
		defer C.free(cLens)

		handlesPtr := unsafe.Slice((*C.int32_t)(cHandles), count)
		valuesPtr := unsafe.Slice((**C.uint8_t)(cValues), count)
		lensPtr := unsafe.Slice((*C.int32_t)(cLens), count)

		keys := make([]int32, 0, count)
		for k := range theAttributes {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

		for i, k := range keys {
			v := theAttributes[k]
			handlesPtr[i] = C.int32_t(k)
			lensPtr[i] = C.int32_t(len(v))
			if len(v) > 0 {
				valuesPtr[i] = (*C.uint8_t)(unsafe.Pointer(&v[0]))
			} else {
				valuesPtr[i] = nil
			}
		}

		var cTag *C.uint8_t
		if len(tag) > 0 {
			cTag = (*C.uint8_t)(unsafe.Pointer(&tag[0]))
		}

		C.call_reflect_attrs(
			f.ctx.cb.onReflectAttributes,
			C.int32_t(theObject),
			C.int32_t(count),
			(*C.int32_t)(cHandles),
			(**C.uint8_t)(cValues),
			(*C.int32_t)(cLens),
			cTag,
			C.int32_t(len(tag)),
		)
	}
}

func (f *cFedAmbassador) ReceiveInteraction(interactionClass int32, theParameters map[int32][]byte, tag []byte) {
	if f.ctx.cb.onReceiveInteraction != nil {
		count := len(theParameters)
		var cHandles unsafe.Pointer
		var cValues unsafe.Pointer
		var cLens unsafe.Pointer

		if count > 0 {
			cHandles = C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(C.int32_t(0))))
			defer C.free(cHandles)
			cValues = C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(uintptr(0))))
			defer C.free(cValues)
			cLens = C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(C.int32_t(0))))
			defer C.free(cLens)

			handlesPtr := unsafe.Slice((*C.int32_t)(cHandles), count)
			valuesPtr := unsafe.Slice((**C.uint8_t)(cValues), count)
			lensPtr := unsafe.Slice((*C.int32_t)(cLens), count)

			keys := make([]int32, 0, count)
			for k := range theParameters {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

			for i, k := range keys {
				v := theParameters[k]
				handlesPtr[i] = C.int32_t(k)
				lensPtr[i] = C.int32_t(len(v))
				if len(v) > 0 {
					valuesPtr[i] = (*C.uint8_t)(unsafe.Pointer(&v[0]))
				} else {
					valuesPtr[i] = nil
				}
			}
		}

		var cTag *C.uint8_t
		if len(tag) > 0 {
			cTag = (*C.uint8_t)(unsafe.Pointer(&tag[0]))
		}

		C.call_receive_interaction(
			f.ctx.cb.onReceiveInteraction,
			C.int32_t(interactionClass),
			C.int32_t(count),
			(*C.int32_t)(cHandles),
			(**C.uint8_t)(cValues),
			(*C.int32_t)(cLens),
			cTag,
			C.int32_t(len(tag)),
		)
	}
}

func (f *cFedAmbassador) TimeAdvanceGrant(grantTime float64) {
	if f.ctx.cb.onTimeAdvanceGrant != nil {
		C.call_time_grant(f.ctx.cb.onTimeAdvanceGrant, C.double(grantTime))
	}
}

var (
	registryMu  sync.RWMutex
	ambCounter  uintptr = 1000
	ambRegistry         = make(map[uintptr]*ambContext)
)

func getContext(amb C.ValhallaAmbassador) (*ambContext, bool) {
	if amb == nil {
		return nil, false
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	id := uintptr(amb)
	ctx, exists := ambRegistry[id]
	return ctx, exists
}

//export valhalla_create_ambassador
func valhalla_create_ambassador() C.ValhallaAmbassador {
	cPtr := C.malloc(1)
	if cPtr == nil {
		return nil
	}
	id := uintptr(cPtr)

	registryMu.Lock()
	defer registryMu.Unlock()

	ctx := &ambContext{
		amb: lrc.NewRTIambassador(),
	}
	ambRegistry[id] = ctx
	return C.ValhallaAmbassador(cPtr)
}

//export valhalla_destroy_ambassador
func valhalla_destroy_ambassador(amb C.ValhallaAmbassador) {
	if amb == nil {
		return
	}
	registryMu.Lock()
	defer registryMu.Unlock()

	id := uintptr(amb)
	if ctx, exists := ambRegistry[id]; exists {
		_ = ctx.amb.Disconnect()
		delete(ambRegistry, id)
		C.free(unsafe.Pointer(amb))
	}
}

//export valhalla_set_callbacks
func valhalla_set_callbacks(amb C.ValhallaAmbassador, callbacks C.ValhallaCallbacks) C.int32_t {
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	ctx.cb = callbacks
	return C.VALHALLA_OK
}

//export valhalla_connect
func valhalla_connect(amb C.ValhallaAmbassador, address *C.char) C.int32_t {
	if address == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	addr := C.GoString(address)
	fedAmb := &cFedAmbassador{ctx: ctx}
	if err := ctx.amb.Connect(fedAmb, addr); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_disconnect
func valhalla_disconnect(amb C.ValhallaAmbassador) C.int32_t {
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.Disconnect(); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_create_federation
func valhalla_create_federation(amb C.ValhallaAmbassador, fedName *C.char) C.int32_t {
	if fedName == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	name := C.GoString(fedName)
	if err := ctx.amb.CreateFederationExecution(name); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_create_federation_with_fom
func valhalla_create_federation_with_fom(amb C.ValhallaAmbassador, fedName *C.char, fomModules **C.char, fomCount C.int32_t) C.int32_t {
	if fedName == nil || fomCount < 0 || (fomCount > 0 && fomModules == nil) {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	name := C.GoString(fedName)
	count := int(fomCount)
	foms := make([]string, 0, count)
	if count > 0 && fomModules != nil {
		cSlice := unsafe.Slice(fomModules, count)
		for i := 0; i < count; i++ {
			if cSlice[i] != nil {
				foms = append(foms, C.GoString(cSlice[i]))
			}
		}
	}
	if err := ctx.amb.CreateFederationExecutionWithFOM(name, foms); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_destroy_federation
func valhalla_destroy_federation(amb C.ValhallaAmbassador, fedName *C.char) C.int32_t {
	if fedName == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	name := C.GoString(fedName)
	if err := ctx.amb.DestroyFederationExecution(name); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_join_federation
func valhalla_join_federation(amb C.ValhallaAmbassador, federateName, fedName *C.char, outFederateHandle *C.int32_t) C.int32_t {
	if federateName == nil || fedName == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	fed := C.GoString(federateName)
	sim := C.GoString(fedName)
	handle, err := ctx.amb.JoinFederationExecution(fed, sim)
	if err != nil {
		return mapErrorToStatusCode(err)
	}
	if outFederateHandle != nil {
		*outFederateHandle = C.int32_t(handle)
	}
	return C.VALHALLA_OK
}

//export valhalla_resign_federation
func valhalla_resign_federation(amb C.ValhallaAmbassador) C.int32_t {
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.ResignFederationExecution(); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_register_federation_synchronization_point
func valhalla_register_federation_synchronization_point(amb C.ValhallaAmbassador, label *C.char, tag *C.uint8_t, tagLen C.int32_t) C.int32_t {
	if label == nil || tagLen < 0 || (tagLen > 0 && tag == nil) {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	goLabel := C.GoString(label)
	var goTag []byte
	if tagLen > 0 && tag != nil {
		goTag = C.GoBytes(unsafe.Pointer(tag), tagLen)
	}
	if err := ctx.amb.RegisterFederationSynchronizationPoint(goLabel, goTag); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_synchronization_point_achieved
func valhalla_synchronization_point_achieved(amb C.ValhallaAmbassador, label *C.char) C.int32_t {
	if label == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	goLabel := C.GoString(label)
	if err := ctx.amb.SynchronizationPointAchieved(goLabel); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_get_object_class_handle
func valhalla_get_object_class_handle(amb C.ValhallaAmbassador, className *C.char, outHandle *C.int32_t) C.int32_t {
	if className == nil {
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
	if attrName == nil {
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
	if dimName == nil {
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
	if attrCount < 0 || (attrCount > 0 && attrHandles == nil) {
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
	if attrCount < 0 || (attrCount > 0 && attrHandles == nil) {
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
	if tagLen < 0 || (tagLen > 0 && tag == nil) {
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
	if attrCount < 0 || (attrCount > 0 && (attrHandles == nil || attrValues == nil || attrLens == nil)) || tagLen < 0 || (tagLen > 0 && tag == nil) {
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
			ln := int(cLns[i])
			if ln < 0 {
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
	if paramCount < 0 || (paramCount > 0 && (paramHandles == nil || paramValues == nil || paramLens == nil)) || tagLen < 0 || (tagLen > 0 && tag == nil) {
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
			ln := int(cLns[i])
			if ln < 0 {
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

//export valhalla_enable_time_regulation
func valhalla_enable_time_regulation(amb C.ValhallaAmbassador, lookahead C.double) C.int32_t {
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.EnableTimeRegulation(float64(lookahead)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_enable_time_constrained
func valhalla_enable_time_constrained(amb C.ValhallaAmbassador) C.int32_t {
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.EnableTimeConstrained(); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_time_advance_request
func valhalla_time_advance_request(amb C.ValhallaAmbassador, time C.double) C.int32_t {
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.TimeAdvanceRequest(float64(time)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_next_message_request
func valhalla_next_message_request(amb C.ValhallaAmbassador, time C.double) C.int32_t {
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.NextMessageRequest(float64(time)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_divest_ownership
func valhalla_divest_ownership(amb C.ValhallaAmbassador, objectHandle C.int32_t, attrHandles *C.int32_t, attrCount C.int32_t) C.int32_t {
	if attrCount < 0 || (attrCount > 0 && attrHandles == nil) {
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
	if err := ctx.amb.UnconditionalAttributeOwnershipDivestiture(int32(objectHandle), attrs); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_acquire_ownership
func valhalla_acquire_ownership(amb C.ValhallaAmbassador, objectHandle C.int32_t, attrHandles *C.int32_t, attrCount C.int32_t) C.int32_t {
	if attrCount < 0 || (attrCount > 0 && attrHandles == nil) {
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
	if err := ctx.amb.AcquireAttributeOwnership(int32(objectHandle), attrs); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_query_ownership
func valhalla_query_ownership(amb C.ValhallaAmbassador, objectHandle, attrHandle C.int32_t, outOwner *C.int32_t, outIsOwned *C.bool) C.int32_t {
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	owner, isOwned, err := ctx.amb.QueryAttributeOwnership(int32(objectHandle), int32(attrHandle))
	if err != nil {
		return mapErrorToStatusCode(err)
	}
	if outOwner != nil {
		*outOwner = C.int32_t(owner)
	}
	if outIsOwned != nil {
		*outIsOwned = C.bool(isOwned)
	}
	return C.VALHALLA_OK
}

//export valhalla_create_region
func valhalla_create_region(amb C.ValhallaAmbassador, dimensions *C.int32_t, dimCount C.int32_t, outRegionHandle *C.uint32_t) C.int32_t {
	if dimCount < 0 || (dimCount > 0 && dimensions == nil) {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	count := int(dimCount)
	dims := make([]int32, count)
	if count > 0 && dimensions != nil {
		cSlice := unsafe.Slice(dimensions, count)
		for i := 0; i < count; i++ {
			dims[i] = int32(cSlice[i])
		}
	}
	handle, err := ctx.amb.CreateRegion(dims)
	if err != nil {
		return mapErrorToStatusCode(err)
	}
	if outRegionHandle != nil {
		*outRegionHandle = C.uint32_t(handle)
	}
	return C.VALHALLA_OK
}

//export valhalla_set_range_bounds
func valhalla_set_range_bounds(amb C.ValhallaAmbassador, regionHandle C.uint32_t, dimension C.int32_t, lowerBound, upperBound C.uint32_t) C.int32_t {
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.SetRangeBounds(uint32(regionHandle), int32(dimension), uint32(lowerBound), uint32(upperBound)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_set_metric_bounds
func valhalla_set_metric_bounds(amb C.ValhallaAmbassador, regionHandle C.uint32_t, minX, maxX, minY, maxY, minZ, maxZ C.double) C.int32_t {
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.SetMetricBounds(uint32(regionHandle), float64(minX), float64(maxX), float64(minY), float64(maxY), float64(minZ), float64(maxZ)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_subscribe_with_region
func valhalla_subscribe_with_region(amb C.ValhallaAmbassador, classHandle C.int32_t, attrHandles *C.int32_t, attrCount C.int32_t, regionHandle C.uint32_t) C.int32_t {
	if attrCount < 0 || (attrCount > 0 && attrHandles == nil) {
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
	if err := ctx.amb.SubscribeObjectClassAttributesWithRegion(int32(classHandle), attrs, uint32(regionHandle)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_update_attributes_spatial
func valhalla_update_attributes_spatial(amb C.ValhallaAmbassador, objectHandle C.int32_t, attrHandles *C.int32_t, attrValues **C.uint8_t, attrLens *C.int32_t, attrCount C.int32_t, x, y, z C.double, tag *C.uint8_t, tagLen C.int32_t) C.int32_t {
	if attrCount < 0 || (attrCount > 0 && (attrHandles == nil || attrValues == nil || attrLens == nil)) || tagLen < 0 || (tagLen > 0 && tag == nil) {
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
			ln := int(cLns[i])
			if ln < 0 {
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
	if err := ctx.amb.UpdateAttributeValuesWithSpatial(int32(objectHandle), attrs, float64(x), float64(y), float64(z), goTag); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_evoke_callback
func valhalla_evoke_callback(amb C.ValhallaAmbassador, timeoutSeconds C.double, outEvoked *C.bool) C.int32_t {
	if timeoutSeconds < 0 {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	evoked, err := ctx.amb.EvokeCallback(time.Duration(float64(timeoutSeconds) * float64(time.Second)))
	if err != nil {
		return mapErrorToStatusCode(err)
	}
	if outEvoked != nil {
		*outEvoked = C.bool(evoked)
	}
	return C.VALHALLA_OK
}

func main() {}
