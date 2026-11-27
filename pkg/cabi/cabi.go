package main

/*
#include "valhalla_lrc.h"
#include <stdlib.h>

static void call_discover_object(ValhallaDiscoverObjectCallback cb, int32_t theObject, int32_t theObjectClass, const char* objectName, void* userData) {
    if (cb) cb(theObject, theObjectClass, objectName, userData);
}

static void call_remove_object(ValhallaRemoveObjectCallback cb, int32_t theObject, const uint8_t* tag, int32_t tagLen, void* userData) {
    if (cb) cb(theObject, tag, tagLen, userData);
}

static void call_time_grant(ValhallaTimeAdvanceGrantCallback cb, double time, void* userData) {
    if (cb) cb(time, userData);
}

static void call_reflect_attrs(ValhallaReflectAttributesCallback cb, int32_t theObject, int32_t attrCount, const int32_t* attrHandles, const uint8_t** attrValues, const int32_t* attrLens, const uint8_t* tag, int32_t tagLen, void* userData) {
    if (cb) cb(theObject, attrCount, attrHandles, attrValues, attrLens, tag, tagLen, userData);
}

static void call_receive_interaction(ValhallaReceiveInteractionCallback cb, int32_t interactionClass, int32_t paramCount, const int32_t* paramHandles, const uint8_t** paramValues, const int32_t* paramLens, const uint8_t* tag, int32_t tagLen, void* userData) {
    if (cb) cb(interactionClass, paramCount, paramHandles, paramValues, paramLens, tag, tagLen, userData);
}

static void call_announce_sync(ValhallaAnnounceSynchronizationPointCallback cb, const char* label, const uint8_t* tag, int32_t tagLen, void* userData) {
    if (cb) cb(label, tag, tagLen, userData);
}

static void call_federation_sync(ValhallaFederationSynchronizedCallback cb, const char* label, void* userData) {
    if (cb) cb(label, userData);
}

static void call_sync_reg_succeeded(ValhallaSynchronizationPointRegistrationSucceededCallback cb, const char* label, void* userData) {
    if (cb) cb(label, userData);
}

static void call_sync_reg_failed(ValhallaSynchronizationPointRegistrationFailedCallback cb, const char* label, const char* reason, void* userData) {
    if (cb) cb(label, reason, userData);
}

// Monotonic non-freed sentinel address space in BSS (64 KiB = 65535 unique non-NULL handles).
// Handles are NEVER dereferenced and NEVER freed, eliminating both UAF during concurrent
// destroy and malloc address-reuse ABA bugs while remaining valid C pointers for Go's GC.
#define VALHALLA_HANDLE_SLOTS 65536
static uint8_t valhalla_handle_arena[VALHALLA_HANDLE_SLOTS];

static inline ValhallaAmbassador valhalla_alloc_handle(uint64_t id) {
    if (id == 0 || id >= VALHALLA_HANDLE_SLOTS) return NULL;
    return (ValhallaAmbassador)&valhalla_handle_arena[id];
}
*/
import "C"

import (
	"errors"
	"sort"
	"sync"
	"unsafe"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/lrc"
)

func mapErrorToStatusCode(err error) C.int32_t {
	if err == nil {
		return C.VALHALLA_OK
	}

	checkTyped := func(e error) (C.int32_t, bool) {
		switch {
		case errors.Is(e, lrc.ErrNotConnected),
			errors.Is(e, lrc.ErrConnectionClosed):
			return C.VALHALLA_ERR_NOT_CONNECTED, true

		case errors.Is(e, common.ErrFederationExecutionAlreadyExists):
			return C.VALHALLA_ERR_FEDERATION_EXISTS, true

		case errors.Is(e, common.ErrFederationExecutionDoesNotExist):
			return C.VALHALLA_ERR_FEDERATION_NOT_FOUND, true

		case errors.Is(e, common.ErrFederateNameAlreadyInUse):
			return C.VALHALLA_ERR_FEDERATE_NAME_IN_USE, true

		case errors.Is(e, common.ErrObjectInstanceNameInUse):
			return C.VALHALLA_ERR_OBJECT_NAME_IN_USE, true

		case errors.Is(e, common.ErrFederatesCurrentlyJoined):
			return C.VALHALLA_ERR_FEDERATES_JOINED, true

		case errors.Is(e, lrc.ErrFederateAlreadyExecutionMember),
			errors.Is(e, common.ErrFederateAlreadyExecutionMember):
			return C.VALHALLA_ERR_ALREADY_MEMBER, true

		case errors.Is(e, lrc.ErrFederateNotExecutionMember),
			errors.Is(e, common.ErrFederateNotExecutionMember),
			errors.Is(e, common.ErrFederateNotFound):
			return C.VALHALLA_ERR_NOT_MEMBER, true

		case errors.Is(e, common.ErrAttributeNotOwned):
			return C.VALHALLA_ERR_NOT_OWNER, true

		case errors.Is(e, common.ErrDeletePrivilegeNotHeld):
			return C.VALHALLA_ERR_DELETE_PRIVILEGE_NOT_HELD, true

		case errors.Is(e, common.ErrObjectClassNotDefined),
			errors.Is(e, common.ErrClassNotFound):
			return C.VALHALLA_ERR_CLASS_NOT_FOUND, true

		case errors.Is(e, common.ErrAttributeNotDefined),
			errors.Is(e, common.ErrAttributeNotFound):
			return C.VALHALLA_ERR_ATTR_NOT_FOUND, true

		case errors.Is(e, common.ErrInteractionClassNotDefined),
			errors.Is(e, common.ErrInteractionNotFound),
			errors.Is(e, common.ErrParameterNotFound):
			return C.VALHALLA_ERR_INTERACTION_NOT_FOUND, true

		case errors.Is(e, common.ErrObjectInstanceNotKnown):
			return C.VALHALLA_ERR_OBJECT_NOT_FOUND, true

		case errors.Is(e, common.ErrInvalidParentClass),
			errors.Is(e, common.ErrAmbiguousClass),
			errors.Is(e, common.ErrAmbiguousInteraction),
			errors.Is(e, common.ErrInvalidLogicalTime),
			errors.Is(e, common.ErrInvalidLookahead),
			errors.Is(e, common.ErrSyncPointLabelNotUnique):
			return C.VALHALLA_ERR_INVALID_ARG, true
		}
		return 0, false
	}

	if code, ok := checkTyped(err); ok {
		return code
	}

	// Canonical resolution for remote error messages
	if canonical := common.AsCanonicalError(err.Error()); canonical != nil {
		if code, ok := checkTyped(canonical); ok {
			return code
		}
	}

	return C.VALHALLA_ERR_RTI_EXCEPTION
}

type ambContext struct {
	cbMu    sync.RWMutex
	evokeWg sync.WaitGroup
	amb     lrc.RTIambassador
	cb      C.ValhallaCallbacks
}

func (ctx *ambContext) getCallbacks() C.ValhallaCallbacks {
	ctx.cbMu.RLock()
	defer ctx.cbMu.RUnlock()
	return ctx.cb
}

type cFedAmbassador struct {
	ctx *ambContext
}

func (f *cFedAmbassador) SynchronizationPointRegistrationSucceeded(label string) {
	cb := f.ctx.getCallbacks()
	if cb.onSyncPointRegistrationSucceeded != nil {
		cLabel := C.CString(label)
		defer C.free(unsafe.Pointer(cLabel))
		C.call_sync_reg_succeeded(cb.onSyncPointRegistrationSucceeded, cLabel, cb.userData)
	}
}

func (f *cFedAmbassador) SynchronizationPointRegistrationFailed(label string, reason string) {
	cb := f.ctx.getCallbacks()
	if cb.onSyncPointRegistrationFailed != nil {
		cLabel := C.CString(label)
		defer C.free(unsafe.Pointer(cLabel))
		cReason := C.CString(reason)
		defer C.free(unsafe.Pointer(cReason))
		C.call_sync_reg_failed(cb.onSyncPointRegistrationFailed, cLabel, cReason, cb.userData)
	}
}

func (f *cFedAmbassador) AnnounceSynchronizationPoint(label string, tag []byte) {
	cb := f.ctx.getCallbacks()
	if cb.onAnnounceSynchronizationPoint != nil {
		cLabel := C.CString(label)
		defer C.free(unsafe.Pointer(cLabel))
		var cTag *C.uint8_t
		if len(tag) > 0 {
			cTagPtr := C.CBytes(tag)
			defer C.free(cTagPtr)
			cTag = (*C.uint8_t)(cTagPtr)
		}
		C.call_announce_sync(cb.onAnnounceSynchronizationPoint, cLabel, cTag, C.int32_t(len(tag)), cb.userData)
	}
}

func (f *cFedAmbassador) FederationSynchronized(label string) {
	cb := f.ctx.getCallbacks()
	if cb.onFederationSynchronized != nil {
		cLabel := C.CString(label)
		defer C.free(unsafe.Pointer(cLabel))
		C.call_federation_sync(cb.onFederationSynchronized, cLabel, cb.userData)
	}
}

func (f *cFedAmbassador) DiscoverObjectInstance(theObject int32, theObjectClass int32, objectName string) {
	cb := f.ctx.getCallbacks()
	if cb.onDiscoverObject != nil {
		cName := C.CString(objectName)
		defer C.free(unsafe.Pointer(cName))
		C.call_discover_object(cb.onDiscoverObject, C.int32_t(theObject), C.int32_t(theObjectClass), cName, cb.userData)
	}
}

func (f *cFedAmbassador) RemoveObjectInstance(theObject int32, tag []byte) {
	cb := f.ctx.getCallbacks()
	if cb.onRemoveObject != nil {
		var cTag *C.uint8_t
		if len(tag) > 0 {
			cTagPtr := C.CBytes(tag)
			defer C.free(cTagPtr)
			cTag = (*C.uint8_t)(cTagPtr)
		}
		C.call_remove_object(cb.onRemoveObject, C.int32_t(theObject), cTag, C.int32_t(len(tag)), cb.userData)
	}
}

func (f *cFedAmbassador) ReflectAttributeValues(theObject int32, theAttributes map[int32][]byte, tag []byte) {
	cb := f.ctx.getCallbacks()
	if cb.onReflectAttributes != nil {
		count := len(theAttributes)
		if count == 0 {
			return
		}

		cHandles := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(C.int32_t(0))))
		if cHandles == nil {
			return
		}
		defer C.free(cHandles)
		cValues := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(uintptr(0))))
		if cValues == nil {
			return
		}
		defer C.free(cValues)
		cLens := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(C.int32_t(0))))
		if cLens == nil {
			return
		}
		defer C.free(cLens)

		handlesPtr := unsafe.Slice((*C.int32_t)(cHandles), count)
		valuesPtr := unsafe.Slice((**C.uint8_t)(cValues), count)
		lensPtr := unsafe.Slice((*C.int32_t)(cLens), count)

		var toFree []unsafe.Pointer
		defer func() {
			for _, p := range toFree {
				C.free(p)
			}
		}()

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
				cVal := C.CBytes(v)
				toFree = append(toFree, cVal)
				valuesPtr[i] = (*C.uint8_t)(cVal)
			} else {
				valuesPtr[i] = nil
			}
		}

		var cTag *C.uint8_t
		if len(tag) > 0 {
			cTagPtr := C.CBytes(tag)
			toFree = append(toFree, cTagPtr)
			cTag = (*C.uint8_t)(cTagPtr)
		}

		C.call_reflect_attrs(
			cb.onReflectAttributes,
			C.int32_t(theObject),
			C.int32_t(count),
			(*C.int32_t)(cHandles),
			(**C.uint8_t)(cValues),
			(*C.int32_t)(cLens),
			cTag,
			C.int32_t(len(tag)),
			cb.userData,
		)
	}
}

func (f *cFedAmbassador) ReceiveInteraction(interactionClass int32, theParameters map[int32][]byte, tag []byte) {
	cb := f.ctx.getCallbacks()
	if cb.onReceiveInteraction != nil {
		count := len(theParameters)
		var cHandles unsafe.Pointer
		var cValues unsafe.Pointer
		var cLens unsafe.Pointer

		var toFree []unsafe.Pointer
		defer func() {
			for _, p := range toFree {
				C.free(p)
			}
		}()

		if count > 0 {
			cHandles = C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(C.int32_t(0))))
			if cHandles == nil {
				return
			}
			defer C.free(cHandles)
			cValues = C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(uintptr(0))))
			if cValues == nil {
				return
			}
			defer C.free(cValues)
			cLens = C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(C.int32_t(0))))
			if cLens == nil {
				return
			}
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
					cVal := C.CBytes(v)
					toFree = append(toFree, cVal)
					valuesPtr[i] = (*C.uint8_t)(cVal)
				} else {
					valuesPtr[i] = nil
				}
			}
		}

		var cTag *C.uint8_t
		if len(tag) > 0 {
			cTagPtr := C.CBytes(tag)
			toFree = append(toFree, cTagPtr)
			cTag = (*C.uint8_t)(cTagPtr)
		}

		C.call_receive_interaction(
			cb.onReceiveInteraction,
			C.int32_t(interactionClass),
			C.int32_t(count),
			(*C.int32_t)(cHandles),
			(**C.uint8_t)(cValues),
			(*C.int32_t)(cLens),
			cTag,
			C.int32_t(len(tag)),
			cb.userData,
		)
	}
}

func (f *cFedAmbassador) TimeAdvanceGrant(grantTime float64) {
	cb := f.ctx.getCallbacks()
	if cb.onTimeAdvanceGrant != nil {
		C.call_time_grant(cb.onTimeAdvanceGrant, C.double(grantTime), cb.userData)
	}
}

var (
	registryMu  sync.RWMutex
	ambRegistry = make(map[uintptr]*ambContext)
	nextAmbID   uint64
)

func getContext(amb C.ValhallaAmbassador) (*ambContext, bool) {
	if amb == nil {
		return nil, false
	}
	registryMu.RLock()
	ctx, exists := ambRegistry[uintptr(amb)]
	registryMu.RUnlock()
	return ctx, exists
}

// acquireContextForEvoke atomically looks up the ambassador and increments its evokeWg
// under registryMu.RLock so valhalla_destroy_ambassador waits for in-flight callbacks.
func acquireContextForEvoke(amb C.ValhallaAmbassador) (*ambContext, bool) {
	if amb == nil {
		return nil, false
	}
	registryMu.RLock()
	ctx, exists := ambRegistry[uintptr(amb)]
	if exists {
		ctx.evokeWg.Add(1)
	}
	registryMu.RUnlock()
	return ctx, exists
}

//export valhalla_create_ambassador
func valhalla_create_ambassador() C.ValhallaAmbassador {
	registryMu.Lock()
	defer registryMu.Unlock()

	nextAmbID++
	handle := C.valhalla_alloc_handle(C.uint64_t(nextAmbID))
	if handle == nil {
		return nil
	}

	ctx := &ambContext{
		amb: lrc.NewRTIambassador(),
	}
	ambRegistry[uintptr(handle)] = ctx
	return handle
}

//export valhalla_destroy_ambassador
func valhalla_destroy_ambassador(amb C.ValhallaAmbassador) {
	if amb == nil {
		return
	}

	registryMu.Lock()
	key := uintptr(amb)
	ctx, exists := ambRegistry[key]
	if exists {
		delete(ambRegistry, key)
	}
	registryMu.Unlock()

	if exists {
		_ = ctx.amb.Disconnect()
		ctx.evokeWg.Wait()
		ctx.cbMu.Lock()
		ctx.cb = C.ValhallaCallbacks{}
		ctx.cbMu.Unlock()
	}
}

//export valhalla_set_callbacks
func valhalla_set_callbacks(amb C.ValhallaAmbassador, callbacks C.ValhallaCallbacks) C.int32_t {
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	ctx.cbMu.Lock()
	ctx.cb = callbacks
	ctx.cbMu.Unlock()
	return C.VALHALLA_OK
}

//export valhalla_connect
func valhalla_connect(amb C.ValhallaAmbassador, address *C.char) C.int32_t {
	if amb == nil || address == nil {
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
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
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
	if amb == nil || fedName == nil {
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

const (
	maxCABICount    int32 = 65536
	maxCABITagLen   int32 = 1024 * 1024      // 1 MiB
	maxCABIValueLen int32 = 16 * 1024 * 1024 // 16 MiB
)

func validateCABICount(n int32) bool {
	return n >= 0 && n <= maxCABICount
}

func validateCABITagLen(n int32) bool {
	return n >= 0 && n <= maxCABITagLen
}

func validateCABIValueLen(n int32) bool {
	return n >= 0 && n <= maxCABIValueLen
}

//export valhalla_create_federation_with_fom
func valhalla_create_federation_with_fom(amb C.ValhallaAmbassador, fedName *C.char, fomModules **C.char, fomCount C.int32_t) C.int32_t {
	if amb == nil || fedName == nil || !validateCABICount(int32(fomCount)) || (fomCount > 0 && fomModules == nil) {
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
	if amb == nil || fedName == nil {
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
	if amb == nil || federateName == nil || fedName == nil || outFederateHandle == nil {
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
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
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
	if amb == nil || label == nil || !validateCABITagLen(int32(tagLen)) || (tagLen > 0 && tag == nil) {
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
	if amb == nil || label == nil {
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
