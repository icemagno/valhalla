#ifndef VALHALLA_LRC_H
#define VALHALLA_LRC_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stdbool.h>

// Valhalla C-ABI Status Codes
#define VALHALLA_OK 0
#define VALHALLA_ERR_NOT_CONNECTED 1
#define VALHALLA_ERR_INVALID_ARG 2
#define VALHALLA_ERR_RTI_EXCEPTION 3
#define VALHALLA_ERR_NOT_OWNER 4
#define VALHALLA_ERR_ALREADY_OWNED 5
#define VALHALLA_ERR_FEDERATION_EXISTS 6
#define VALHALLA_ERR_FEDERATION_NOT_FOUND 7
#define VALHALLA_ERR_CLASS_NOT_FOUND 8
#define VALHALLA_ERR_ATTR_NOT_FOUND 9
#define VALHALLA_ERR_OBJECT_NOT_FOUND 10
#define VALHALLA_ERR_NOT_MEMBER 11
#define VALHALLA_ERR_FEDERATES_JOINED 12
#define VALHALLA_ERR_FEDERATE_NAME_IN_USE 13
#define VALHALLA_ERR_INTERACTION_NOT_FOUND 14

typedef void* ValhallaAmbassador;

// Callback function pointer types for C FederateAmbassador (each receives userData)
typedef void (*ValhallaDiscoverObjectCallback)(int32_t theObject, int32_t theObjectClass, const char* objectName, void* userData);
typedef void (*ValhallaRemoveObjectCallback)(int32_t theObject, const uint8_t* tag, int32_t tagLen, void* userData);
typedef void (*ValhallaReflectAttributesCallback)(int32_t theObject, int32_t attrCount, const int32_t* attrHandles, const uint8_t** attrValues, const int32_t* attrLens, const uint8_t* tag, int32_t tagLen, void* userData);
typedef void (*ValhallaTimeAdvanceGrantCallback)(double time, void* userData);
typedef void (*ValhallaReceiveInteractionCallback)(int32_t interactionClass, int32_t paramCount, const int32_t* paramHandles, const uint8_t** paramValues, const int32_t* paramLens, const uint8_t* tag, int32_t tagLen, void* userData);
typedef void (*ValhallaAnnounceSynchronizationPointCallback)(const char* label, const uint8_t* tag, int32_t tagLen, void* userData);
typedef void (*ValhallaFederationSynchronizedCallback)(const char* label, void* userData);
typedef void (*ValhallaSynchronizationPointRegistrationSucceededCallback)(const char* label, void* userData);
typedef void (*ValhallaSynchronizationPointRegistrationFailedCallback)(const char* label, const char* reason, void* userData);

typedef struct {
    ValhallaDiscoverObjectCallback onDiscoverObject;
    ValhallaRemoveObjectCallback onRemoveObject;
    ValhallaReflectAttributesCallback onReflectAttributes;
    ValhallaTimeAdvanceGrantCallback onTimeAdvanceGrant;
    ValhallaReceiveInteractionCallback onReceiveInteraction;
    ValhallaAnnounceSynchronizationPointCallback onAnnounceSynchronizationPoint;
    ValhallaFederationSynchronizedCallback onFederationSynchronized;
    ValhallaSynchronizationPointRegistrationSucceededCallback onSyncPointRegistrationSucceeded;
    ValhallaSynchronizationPointRegistrationFailedCallback onSyncPointRegistrationFailed;
    void* userData;
} ValhallaCallbacks;

// Lifecycle
ValhallaAmbassador valhalla_create_ambassador(void);
void valhalla_destroy_ambassador(ValhallaAmbassador amb);
int32_t valhalla_set_callbacks(ValhallaAmbassador amb, ValhallaCallbacks callbacks);

// Connection & Federation Execution
int32_t valhalla_connect(ValhallaAmbassador amb, char* address);
int32_t valhalla_disconnect(ValhallaAmbassador amb);
int32_t valhalla_create_federation(ValhallaAmbassador amb, char* fedName);
int32_t valhalla_create_federation_with_fom(ValhallaAmbassador amb, char* fedName, char** fomModules, int32_t fomCount);
int32_t valhalla_destroy_federation(ValhallaAmbassador amb, char* fedName);
int32_t valhalla_join_federation(ValhallaAmbassador amb, char* federateName, char* fedName, int32_t* outFederateHandle);
int32_t valhalla_resign_federation(ValhallaAmbassador amb);

// Synchronization Points
int32_t valhalla_register_federation_synchronization_point(ValhallaAmbassador amb, char* label, uint8_t* tag, int32_t tagLen);
int32_t valhalla_synchronization_point_achieved(ValhallaAmbassador amb, char* label);

// FOM / OMT Introspection
int32_t valhalla_get_object_class_handle(ValhallaAmbassador amb, char* className, int32_t* outHandle);
int32_t valhalla_get_attribute_handle(ValhallaAmbassador amb, int32_t classHandle, char* attrName, int32_t* outHandle);
int32_t valhalla_get_dimension_handle(ValhallaAmbassador amb, char* dimName, int32_t* outHandle);

// Declaration Management (Objects & Interactions)
int32_t valhalla_publish_object_class(ValhallaAmbassador amb, int32_t classHandle, int32_t* attrHandles, int32_t attrCount);
int32_t valhalla_unpublish_object_class(ValhallaAmbassador amb, int32_t classHandle);
int32_t valhalla_subscribe_object_class(ValhallaAmbassador amb, int32_t classHandle, int32_t* attrHandles, int32_t attrCount);
int32_t valhalla_unsubscribe_object_class(ValhallaAmbassador amb, int32_t classHandle);
int32_t valhalla_publish_interaction_class(ValhallaAmbassador amb, int32_t interactionClass);
int32_t valhalla_unpublish_interaction_class(ValhallaAmbassador amb, int32_t interactionClass);
int32_t valhalla_subscribe_interaction_class(ValhallaAmbassador amb, int32_t interactionClass);
int32_t valhalla_unsubscribe_interaction_class(ValhallaAmbassador amb, int32_t interactionClass);

// Object Management
int32_t valhalla_register_object(ValhallaAmbassador amb, int32_t classHandle, char* objectName, int32_t* outObjectHandle);
int32_t valhalla_delete_object(ValhallaAmbassador amb, int32_t objectHandle, uint8_t* tag, int32_t tagLen);
int32_t valhalla_update_attributes(ValhallaAmbassador amb, int32_t objectHandle, int32_t* attrHandles, uint8_t** attrValues, int32_t* attrLens, int32_t attrCount, uint8_t* tag, int32_t tagLen);

// Interaction Management
int32_t valhalla_send_interaction(ValhallaAmbassador amb, int32_t interactionClass, int32_t* paramHandles, uint8_t** paramValues, int32_t* paramLens, int32_t paramCount, uint8_t* tag, int32_t tagLen);

// Time Management
int32_t valhalla_enable_time_regulation(ValhallaAmbassador amb, double lookahead);
int32_t valhalla_enable_time_constrained(ValhallaAmbassador amb);
int32_t valhalla_time_advance_request(ValhallaAmbassador amb, double time);
int32_t valhalla_next_message_request(ValhallaAmbassador amb, double time);
int32_t valhalla_heartbeat(ValhallaAmbassador amb);

// Ownership Management (Sprint 6 & IEEE 1516-2025)
int32_t valhalla_divest_ownership(ValhallaAmbassador amb, int32_t objectHandle, int32_t* attrHandles, int32_t attrCount);
int32_t valhalla_acquire_ownership(ValhallaAmbassador amb, int32_t objectHandle, int32_t* attrHandles, int32_t attrCount);
int32_t valhalla_query_ownership(ValhallaAmbassador amb, int32_t objectHandle, int32_t attrHandle, int32_t* outOwner, _Bool* outIsOwned);

// Data Distribution Management (DDM) (Sprint 6 & IEEE 1516-2025)
int32_t valhalla_create_region(ValhallaAmbassador amb, int32_t* dimensions, int32_t dimCount, uint32_t* outRegionHandle);
int32_t valhalla_set_range_bounds(ValhallaAmbassador amb, uint32_t regionHandle, int32_t dimension, uint32_t lowerBound, uint32_t upperBound);
int32_t valhalla_set_metric_bounds(ValhallaAmbassador amb, uint32_t regionHandle, double minX, double maxX, double minY, double maxY, double minZ, double maxZ);
int32_t valhalla_subscribe_with_region(ValhallaAmbassador amb, int32_t classHandle, int32_t* attrHandles, int32_t attrCount, uint32_t regionHandle);
int32_t valhalla_update_attributes_spatial(ValhallaAmbassador amb, int32_t objectHandle, int32_t* attrHandles, uint8_t** attrValues, int32_t* attrLens, int32_t attrCount, double x, double y, double z, uint8_t* tag, int32_t tagLen);

// Callback Dispatching
int32_t valhalla_evoke_callback(ValhallaAmbassador amb, double timeoutSeconds, _Bool* outEvoked);

#ifdef __cplusplus
}
#endif

#endif // VALHALLA_LRC_H
