package main

/*
#include "valhalla_lrc.h"
#include <stdlib.h>
*/
import "C"

import (
	"math"
	"time"
	"unsafe"
)


//export valhalla_enable_time_regulation
func valhalla_enable_time_regulation(amb C.ValhallaAmbassador, lookahead C.double) C.int32_t {
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
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
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
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
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
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
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.NextMessageRequest(float64(time)); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_heartbeat
func valhalla_heartbeat(amb C.ValhallaAmbassador) C.int32_t {
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
	ctx, exists := getContext(amb)
	if !exists {
		return C.VALHALLA_ERR_NOT_CONNECTED
	}
	if err := ctx.amb.Heartbeat(); err != nil {
		return mapErrorToStatusCode(err)
	}
	return C.VALHALLA_OK
}

//export valhalla_divest_ownership
func valhalla_divest_ownership(amb C.ValhallaAmbassador, objectHandle C.int32_t, attrHandles *C.int32_t, attrCount C.int32_t) C.int32_t {
	if amb == nil || attrCount < 0 || (attrCount > 0 && attrHandles == nil) {
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
	if amb == nil || attrCount < 0 || (attrCount > 0 && attrHandles == nil) {
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
	if amb == nil || outOwner == nil || outIsOwned == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
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
	if amb == nil || outRegionHandle == nil || dimCount < 0 || (dimCount > 0 && dimensions == nil) {
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
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
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
	if amb == nil {
		return C.VALHALLA_ERR_INVALID_ARG
	}
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
	if amb == nil || attrCount < 0 || (attrCount > 0 && attrHandles == nil) {
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
	if amb == nil || attrCount < 0 || (attrCount > 0 && (attrHandles == nil || attrValues == nil || attrLens == nil)) || tagLen < 0 || (tagLen > 0 && tag == nil) {
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
	tSec := float64(timeoutSeconds)
	if amb == nil || outEvoked == nil || math.IsNaN(tSec) || math.IsInf(tSec, 0) || tSec < 0 {
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
