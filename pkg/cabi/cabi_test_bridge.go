// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package main

/*
#include "valhalla_lrc.h"
#include <stdlib.h>
#include <string.h>

extern void goBridge_OnReceiveInteraction(int32_t interactionClass, int32_t paramCount, int32_t* paramHandles, uint8_t** paramValues, int32_t* paramLens, uint8_t* tag, int32_t tagLen);
extern void goBridge_OnAnnounceSyncPoint(char* label, uint8_t* tag, int32_t tagLen);
extern void goBridge_OnFederationSynchronized(char* label);
extern void goBridge_OnSyncRegSucceeded(char* label);
extern void goBridge_OnSyncRegFailed(char* label, char* reason);

static void c_on_receive_interaction(int32_t interactionClass, int32_t paramCount, const int32_t* paramHandles, const uint8_t** paramValues, const int32_t* paramLens, const uint8_t* tag, int32_t tagLen) {
    goBridge_OnReceiveInteraction(interactionClass, paramCount, (int32_t*)paramHandles, (uint8_t**)paramValues, (int32_t*)paramLens, (uint8_t*)tag, tagLen);
}

static void c_on_announce_sync_point(const char* label, const uint8_t* tag, int32_t tagLen) {
    goBridge_OnAnnounceSyncPoint((char*)label, (uint8_t*)tag, tagLen);
}

static void c_on_federation_synchronized(const char* label) {
    goBridge_OnFederationSynchronized((char*)label);
}

static void c_on_sync_reg_succeeded(const char* label) {
    goBridge_OnSyncRegSucceeded((char*)label);
}

static void c_on_sync_reg_failed(const char* label, const char* reason) {
    goBridge_OnSyncRegFailed((char*)label, (char*)reason);
}

static int32_t c_set_test_callbacks(ValhallaAmbassador amb) {
    ValhallaCallbacks cb;
    memset(&cb, 0, sizeof(cb));
    cb.onReceiveInteraction = c_on_receive_interaction;
    cb.onAnnounceSynchronizationPoint = c_on_announce_sync_point;
    cb.onFederationSynchronized = c_on_federation_synchronized;
    cb.onSyncPointRegistrationSucceeded = c_on_sync_reg_succeeded;
    cb.onSyncPointRegistrationFailed = c_on_sync_reg_failed;
    return valhalla_set_callbacks(amb, cb);
}
*/
import "C"

import (
	"sort"
	"sync"
	"unsafe"
)

var (
	bridgeMu           sync.Mutex
	bridgeClass        int32
	bridgeParams       map[int32][]byte
	bridgeTag          []byte
	bridgeAnnounced    string
	bridgeAnnouncedTag []byte
	bridgeSyncFed      string
	bridgeSyncRegSucc  string
	bridgeSyncRegFail  string
	bridgeSyncReason   string
)

//export goBridge_OnReceiveInteraction
func goBridge_OnReceiveInteraction(interactionClass C.int32_t, paramCount C.int32_t, paramHandles *C.int32_t, paramValues **C.uint8_t, paramLens *C.int32_t, tag *C.uint8_t, tagLen C.int32_t) {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()

	bridgeClass = int32(interactionClass)
	bridgeParams = make(map[int32][]byte)

	count := int(paramCount)
	if count > 0 && paramHandles != nil && paramValues != nil && paramLens != nil {
		hdls := unsafe.Slice(paramHandles, count)
		vals := unsafe.Slice(paramValues, count)
		lns := unsafe.Slice(paramLens, count)

		for i := 0; i < count; i++ {
			h := int32(hdls[i])
			ln := int(lns[i])
			if ln > 0 && vals[i] != nil {
				bridgeParams[h] = C.GoBytes(unsafe.Pointer(vals[i]), C.int(ln))
			} else {
				bridgeParams[h] = []byte{}
			}
		}
	}

	if tagLen > 0 && tag != nil {
		bridgeTag = C.GoBytes(unsafe.Pointer(tag), tagLen)
	} else {
		bridgeTag = nil
	}
}

//export goBridge_OnAnnounceSyncPoint
func goBridge_OnAnnounceSyncPoint(label *C.char, tag *C.uint8_t, tagLen C.int32_t) {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()

	if label != nil {
		bridgeAnnounced = C.GoString(label)
	}
	if tagLen > 0 && tag != nil {
		bridgeAnnouncedTag = C.GoBytes(unsafe.Pointer(tag), tagLen)
	} else {
		bridgeAnnouncedTag = nil
	}
}

//export goBridge_OnFederationSynchronized
func goBridge_OnFederationSynchronized(label *C.char) {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()

	if label != nil {
		bridgeSyncFed = C.GoString(label)
	}
}

//export goBridge_OnSyncRegSucceeded
func goBridge_OnSyncRegSucceeded(label *C.char) {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()

	if label != nil {
		bridgeSyncRegSucc = C.GoString(label)
	}
}

//export goBridge_OnSyncRegFailed
func goBridge_OnSyncRegFailed(label *C.char, reason *C.char) {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()

	if label != nil {
		bridgeSyncRegFail = C.GoString(label)
	}
	if reason != nil {
		bridgeSyncReason = C.GoString(reason)
	}
}

func Bridge_ResetState() {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	bridgeClass = 0
	bridgeParams = nil
	bridgeTag = nil
	bridgeAnnounced = ""
	bridgeAnnouncedTag = nil
	bridgeSyncFed = ""
	bridgeSyncRegSucc = ""
	bridgeSyncRegFail = ""
	bridgeSyncReason = ""
}

func Bridge_GetReceivedInteraction() (int32, map[int32][]byte, []byte) {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	return bridgeClass, bridgeParams, bridgeTag
}

func Bridge_GetAnnouncedSyncPoint() (string, []byte) {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	return bridgeAnnounced, bridgeAnnouncedTag
}

func Bridge_GetFederationSynchronized() string {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	return bridgeSyncFed
}

func Bridge_GetSyncRegSucceeded() string {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	return bridgeSyncRegSucc
}

func Bridge_GetSyncRegFailed() (string, string) {
	bridgeMu.Lock()
	defer bridgeMu.Unlock()
	return bridgeSyncRegFail, bridgeSyncReason
}

// CABI Test Wrappers

func CABI_CreateAmbassador() uintptr {
	amb := valhalla_create_ambassador()
	return uintptr(amb)
}

func CABI_DestroyAmbassador(amb uintptr) {
	valhalla_destroy_ambassador(C.ValhallaAmbassador(unsafe.Pointer(amb)))
}

func CABI_SetTestCallbacks(amb uintptr) int32 {
	return int32(C.c_set_test_callbacks(C.ValhallaAmbassador(unsafe.Pointer(amb))))
}

func CABI_Connect(amb uintptr, address string) int32 {
	cAddr := C.CString(address)
	defer C.free(unsafe.Pointer(cAddr))
	return int32(valhalla_connect(C.ValhallaAmbassador(unsafe.Pointer(amb)), cAddr))
}

func CABI_Disconnect(amb uintptr) int32 {
	return int32(valhalla_disconnect(C.ValhallaAmbassador(unsafe.Pointer(amb))))
}

func CABI_CreateFederation(amb uintptr, fedName string) int32 {
	cName := C.CString(fedName)
	defer C.free(unsafe.Pointer(cName))
	return int32(valhalla_create_federation(C.ValhallaAmbassador(unsafe.Pointer(amb)), cName))
}

func CABI_DestroyFederation(amb uintptr, fedName string) int32 {
	cName := C.CString(fedName)
	defer C.free(unsafe.Pointer(cName))
	return int32(valhalla_destroy_federation(C.ValhallaAmbassador(unsafe.Pointer(amb)), cName))
}

func CABI_JoinFederation(amb uintptr, fedName, simName string) (int32, int32) {
	cFed := C.CString(fedName)
	defer C.free(unsafe.Pointer(cFed))
	cSim := C.CString(simName)
	defer C.free(unsafe.Pointer(cSim))

	var handle C.int32_t
	code := int32(valhalla_join_federation(C.ValhallaAmbassador(unsafe.Pointer(amb)), cFed, cSim, &handle))
	return int32(handle), code
}

func CABI_PublishInteractionClass(amb uintptr, interactionClass int32) int32 {
	return int32(valhalla_publish_interaction_class(C.ValhallaAmbassador(unsafe.Pointer(amb)), C.int32_t(interactionClass)))
}

func CABI_UnpublishInteractionClass(amb uintptr, interactionClass int32) int32 {
	return int32(valhalla_unpublish_interaction_class(C.ValhallaAmbassador(unsafe.Pointer(amb)), C.int32_t(interactionClass)))
}

func CABI_SubscribeInteractionClass(amb uintptr, interactionClass int32) int32 {
	return int32(valhalla_subscribe_interaction_class(C.ValhallaAmbassador(unsafe.Pointer(amb)), C.int32_t(interactionClass)))
}

func CABI_UnsubscribeInteractionClass(amb uintptr, interactionClass int32) int32 {
	return int32(valhalla_unsubscribe_interaction_class(C.ValhallaAmbassador(unsafe.Pointer(amb)), C.int32_t(interactionClass)))
}

func CABI_SendInteraction(amb uintptr, interactionClass int32, params map[int32][]byte, tag []byte) int32 {
	count := len(params)
	var cHandles *C.int32_t
	var cValues **C.uint8_t
	var cLens *C.int32_t

	if count > 0 {
		handles := make([]C.int32_t, count)
		values := make([]*C.uint8_t, count)
		lens := make([]C.int32_t, count)

		keys := make([]int32, 0, count)
		for k := range params {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

		for i, k := range keys {
			v := params[k]
			handles[i] = C.int32_t(k)
			lens[i] = C.int32_t(len(v))
			if len(v) > 0 {
				values[i] = (*C.uint8_t)(unsafe.Pointer(&v[0]))
			}
		}

		cHandles = &handles[0]
		cValues = &values[0]
		cLens = &lens[0]
	}

	var cTag *C.uint8_t
	if len(tag) > 0 {
		cTag = (*C.uint8_t)(unsafe.Pointer(&tag[0]))
	}

	return int32(valhalla_send_interaction(
		C.ValhallaAmbassador(unsafe.Pointer(amb)),
		C.int32_t(interactionClass),
		cHandles,
		cValues,
		cLens,
		C.int32_t(count),
		cTag,
		C.int32_t(len(tag)),
	))
}

func CABI_RegisterFederationSyncPoint(amb uintptr, label string, tag []byte) int32 {
	cLabel := C.CString(label)
	defer C.free(unsafe.Pointer(cLabel))

	var cTag *C.uint8_t
	if len(tag) > 0 {
		cTag = (*C.uint8_t)(unsafe.Pointer(&tag[0]))
	}

	return int32(valhalla_register_federation_synchronization_point(
		C.ValhallaAmbassador(unsafe.Pointer(amb)),
		cLabel,
		cTag,
		C.int32_t(len(tag)),
	))
}

func CABI_SyncPointAchieved(amb uintptr, label string) int32 {
	cLabel := C.CString(label)
	defer C.free(unsafe.Pointer(cLabel))

	return int32(valhalla_synchronization_point_achieved(
		C.ValhallaAmbassador(unsafe.Pointer(amb)),
		cLabel,
	))
}

func CABI_EvokeCallback(amb uintptr, timeoutSeconds float64) (bool, int32) {
	var evoked C.bool
	code := valhalla_evoke_callback(
		C.ValhallaAmbassador(unsafe.Pointer(amb)),
		C.double(timeoutSeconds),
		&evoked,
	)
	return bool(evoked), int32(code)
}

func CABI_NullPointerChecks(amb uintptr) []int32 {
	cAmb := C.ValhallaAmbassador(unsafe.Pointer(amb))
	results := []int32{
		int32(valhalla_connect(cAmb, nil)),
		int32(valhalla_create_federation(cAmb, nil)),
		int32(valhalla_destroy_federation(cAmb, nil)),
		int32(valhalla_join_federation(cAmb, nil, nil, nil)),
		int32(valhalla_get_object_class_handle(cAmb, nil, nil)),
		int32(valhalla_get_attribute_handle(cAmb, 1, nil, nil)),
		int32(valhalla_get_dimension_handle(cAmb, nil, nil)),
		int32(valhalla_send_interaction(cAmb, 1, nil, nil, nil, 2, nil, 0)),
		int32(valhalla_register_federation_synchronization_point(cAmb, nil, nil, 0)),
		int32(valhalla_synchronization_point_achieved(cAmb, nil)),
	}
	return results
}
