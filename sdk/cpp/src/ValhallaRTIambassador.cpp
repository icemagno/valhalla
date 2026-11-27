// ValhallaRTIambassador.cpp - Implementation of IEEE 1516e RTIambassador for Valhalla
#include "ValhallaRTIambassador.h"
#include <chrono>
#include <cstdio>
#include <cstring>
#include <iostream>
#include <vector>

namespace rti1516e {

static inline std::string toUtf8(const std::wstring& wstr) {
    std::string out;
    for (wchar_t wc : wstr) {
        if (wc < 0x80) {
            out.push_back(static_cast<char>(wc));
        } else if (wc < 0x800) {
            out.push_back(static_cast<char>(0xC0 | ((wc >> 6) & 0x1F)));
            out.push_back(static_cast<char>(0x80 | (wc & 0x3F)));
        } else {
            out.push_back(static_cast<char>(0xE0 | ((wc >> 12) & 0x0F)));
            out.push_back(static_cast<char>(0x80 | ((wc >> 6) & 0x3F)));
            out.push_back(static_cast<char>(0x80 | (wc & 0x3F)));
        }
    }
    return out;
}

static inline std::wstring toWide(const std::string& str) {
    std::wstring out;
    for (char c : str) {
        out.push_back(static_cast<wchar_t>(static_cast<unsigned char>(c)));
    }
    return out;
}

static void c_onDiscoverObject(int32_t theObject, int32_t theObjectClass, const char* objectName, void* userData) {
    try {
        auto* self = static_cast<ValhallaRTIambassador*>(userData);
        if (self && self->getFederateAmbassador()) {
            std::string s(objectName ? objectName : "");
            self->getFederateAmbassador()->discoverObjectInstance(
                ObjectInstanceHandle(theObject),
                ObjectClassHandle(theObjectClass),
                toWide(s));
        }
    } catch (const std::exception& e) {
        std::fprintf(stderr, "ValhallaRTIambassador: exception in discoverObjectInstance: %s\n", e.what());
    } catch (...) {
        std::fprintf(stderr, "ValhallaRTIambassador: unknown exception in discoverObjectInstance\n");
    }
}

static void c_onRemoveObject(int32_t theObject, const uint8_t* tag, int32_t tagLen, void* userData) {
    try {
        auto* self = static_cast<ValhallaRTIambassador*>(userData);
        if (self && self->getFederateAmbassador()) {
            VariableLengthData userTag(tag, tagLen > 0 ? tagLen : 0);
            self->getFederateAmbassador()->removeObjectInstance(
                ObjectInstanceHandle(theObject),
                userTag,
                OrderType::RECEIVE);
        }
    } catch (const std::exception& e) {
        std::fprintf(stderr, "ValhallaRTIambassador: exception in removeObjectInstance: %s\n", e.what());
    } catch (...) {
        std::fprintf(stderr, "ValhallaRTIambassador: unknown exception in removeObjectInstance\n");
    }
}

static void c_onReflectAttributes(int32_t theObject, int32_t attrCount, const int32_t* attrHandles, const uint8_t** attrValues, const int32_t* attrLens, const uint8_t* tag, int32_t tagLen, void* userData) {
    try {
        auto* self = static_cast<ValhallaRTIambassador*>(userData);
        if (self && self->getFederateAmbassador()) {
            AttributeHandleValueMap valMap;
            for (int32_t i = 0; i < attrCount; ++i) {
                valMap[AttributeHandle(attrHandles[i])] = VariableLengthData(attrValues[i], attrLens[i]);
            }
            VariableLengthData userTag(tag, tagLen > 0 ? tagLen : 0);
            self->getFederateAmbassador()->reflectAttributeValues(
                ObjectInstanceHandle(theObject),
                valMap,
                userTag,
                OrderType::RECEIVE,
                TransportationTypeHandle::HLA_RELIABLE);
        }
    } catch (const std::exception& e) {
        std::fprintf(stderr, "ValhallaRTIambassador: exception in reflectAttributeValues: %s\n", e.what());
    } catch (...) {
        std::fprintf(stderr, "ValhallaRTIambassador: unknown exception in reflectAttributeValues\n");
    }
}

static void c_onTimeAdvanceGrant(double time, void* userData) {
    try {
        auto* self = static_cast<ValhallaRTIambassador*>(userData);
        if (self && self->getFederateAmbassador()) {
            self->getFederateAmbassador()->timeAdvanceGrant(time);
        }
    } catch (const std::exception& e) {
        std::fprintf(stderr, "ValhallaRTIambassador: exception in timeAdvanceGrant: %s\n", e.what());
    } catch (...) {
        std::fprintf(stderr, "ValhallaRTIambassador: unknown exception in timeAdvanceGrant\n");
    }
}

static void c_onReceiveInteraction(int32_t interactionClass, int32_t paramCount, const int32_t* paramHandles, const uint8_t** paramValues, const int32_t* paramLens, const uint8_t* tag, int32_t tagLen, void* userData) {
    try {
        auto* self = static_cast<ValhallaRTIambassador*>(userData);
        if (self && self->getFederateAmbassador()) {
            ParameterHandleValueMap valMap;
            for (int32_t i = 0; i < paramCount; ++i) {
                valMap[ParameterHandle(paramHandles[i])] = VariableLengthData(paramValues[i], paramLens[i]);
            }
            VariableLengthData userTag(tag, tagLen > 0 ? tagLen : 0);
            self->getFederateAmbassador()->receiveInteraction(
                InteractionClassHandle(interactionClass),
                valMap,
                userTag,
                OrderType::RECEIVE,
                TransportationTypeHandle::HLA_RELIABLE);
        }
    } catch (const std::exception& e) {
        std::fprintf(stderr, "ValhallaRTIambassador: exception in receiveInteraction: %s\n", e.what());
    } catch (...) {
        std::fprintf(stderr, "ValhallaRTIambassador: unknown exception in receiveInteraction\n");
    }
}

static void c_onAnnounceSyncPoint(const char* label, const uint8_t* tag, int32_t tagLen, void* userData) {
    try {
        auto* self = static_cast<ValhallaRTIambassador*>(userData);
        if (self && self->getFederateAmbassador()) {
            std::string s(label ? label : "");
            VariableLengthData userTag(tag, tagLen > 0 ? tagLen : 0);
            self->getFederateAmbassador()->announceSynchronizationPoint(toWide(s), userTag);
        }
    } catch (const std::exception& e) {
        std::fprintf(stderr, "ValhallaRTIambassador: exception in announceSynchronizationPoint: %s\n", e.what());
    } catch (...) {
        std::fprintf(stderr, "ValhallaRTIambassador: unknown exception in announceSynchronizationPoint\n");
    }
}

static void c_onFederationSynchronized(const char* label, void* userData) {
    try {
        auto* self = static_cast<ValhallaRTIambassador*>(userData);
        if (self && self->getFederateAmbassador()) {
            std::string s(label ? label : "");
            self->getFederateAmbassador()->federationSynchronized(toWide(s));
        }
    } catch (const std::exception& e) {
        std::fprintf(stderr, "ValhallaRTIambassador: exception in federationSynchronized: %s\n", e.what());
    } catch (...) {
        std::fprintf(stderr, "ValhallaRTIambassador: unknown exception in federationSynchronized\n");
    }
}

static void c_onSyncPointRegSucceeded(const char* label, void* userData) {
    try {
        auto* self = static_cast<ValhallaRTIambassador*>(userData);
        if (self && self->getFederateAmbassador()) {
            std::string s(label ? label : "");
            self->getFederateAmbassador()->synchronizationPointRegistrationSucceeded(toWide(s));
        }
    } catch (const std::exception& e) {
        std::fprintf(stderr, "ValhallaRTIambassador: exception in synchronizationPointRegistrationSucceeded: %s\n", e.what());
    } catch (...) {
        std::fprintf(stderr, "ValhallaRTIambassador: unknown exception in synchronizationPointRegistrationSucceeded\n");
    }
}

static void c_onSyncPointRegFailed(const char* label, const char* /*reason*/, void* userData) {
    try {
        auto* self = static_cast<ValhallaRTIambassador*>(userData);
        if (self && self->getFederateAmbassador()) {
            std::string s(label ? label : "");
            self->getFederateAmbassador()->synchronizationPointRegistrationFailed(
                toWide(s),
                SynchronizationPointFailureReason::SYNCHRONIZATION_POINT_LABEL_NOT_UNIQUE);
        }
    } catch (const std::exception& e) {
        std::fprintf(stderr, "ValhallaRTIambassador: exception in synchronizationPointRegistrationFailed: %s\n", e.what());
    } catch (...) {
        std::fprintf(stderr, "ValhallaRTIambassador: unknown exception in synchronizationPointRegistrationFailed\n");
    }
}

ValhallaRTIambassador::ValhallaRTIambassador()
    : m_amb(nullptr), m_fedAmb(nullptr), m_connected(false), m_federateHandle(0) {
    m_amb = valhalla_create_ambassador();
    if (!m_amb) {
        throw RTIinternalError(L"Failed to instantiate Valhalla native ambassador");
    }

    ValhallaCallbacks cbs;
    std::memset(&cbs, 0, sizeof(cbs));
    cbs.onDiscoverObject = c_onDiscoverObject;
    cbs.onRemoveObject = c_onRemoveObject;
    cbs.onReflectAttributes = c_onReflectAttributes;
    cbs.onTimeAdvanceGrant = c_onTimeAdvanceGrant;
    cbs.onReceiveInteraction = c_onReceiveInteraction;
    cbs.onAnnounceSynchronizationPoint = c_onAnnounceSyncPoint;
    cbs.onFederationSynchronized = c_onFederationSynchronized;
    cbs.onSyncPointRegistrationSucceeded = c_onSyncPointRegSucceeded;
    cbs.onSyncPointRegistrationFailed = c_onSyncPointRegFailed;
    cbs.userData = this;
    int32_t st = valhalla_set_callbacks(m_amb, cbs);
    if (st != VALHALLA_OK) {
        valhalla_destroy_ambassador(m_amb);
        m_amb = nullptr;
        throw RTIinternalError(L"Failed to register Valhalla C-ABI callbacks");
    }
}

ValhallaRTIambassador::~ValhallaRTIambassador() throw() {
    if (m_amb) {
        valhalla_destroy_ambassador(m_amb);
        m_amb = nullptr;
    }
}

void ValhallaRTIambassador::checkStatus(int32_t status) {
    switch (status) {
        case VALHALLA_OK:
            return;
        case VALHALLA_ERR_NOT_CONNECTED:
            throw NotConnected(L"Federate is not connected to Valhalla RTI");
        case VALHALLA_ERR_INVALID_ARG:
            throw RTIinternalError(L"Invalid argument passed to Valhalla RTI");
        case VALHALLA_ERR_NOT_OWNER:
            throw AttributeNotOwned(L"Attribute is not owned by federate");
        case VALHALLA_ERR_ALREADY_OWNED:
            throw FederateOwnsAttributes(L"Attribute is already owned by federate");
        case VALHALLA_ERR_FEDERATION_EXISTS:
            throw FederationExecutionAlreadyExists(L"Federation execution already exists");
        case VALHALLA_ERR_FEDERATION_NOT_FOUND:
            throw FederationExecutionDoesNotExist(L"Federation execution does not exist");
        case VALHALLA_ERR_CLASS_NOT_FOUND:
            throw ObjectClassNotDefined(L"Object class not defined in FOM");
        case VALHALLA_ERR_ATTR_NOT_FOUND:
            throw AttributeNotDefined(L"Attribute not defined in FOM");
        case VALHALLA_ERR_OBJECT_NOT_FOUND:
            throw ObjectInstanceNotKnown(L"Object instance is not known");
        case VALHALLA_ERR_NOT_MEMBER:
            throw FederateNotExecutionMember(L"Federate is not a member of the federation");
        case VALHALLA_ERR_FEDERATES_JOINED:
            throw FederatesCurrentlyJoined(L"Federates are currently joined to the federation");
        case VALHALLA_ERR_FEDERATE_NAME_IN_USE:
            throw FederateNameAlreadyInUse(L"Federate name is already in use");
        case VALHALLA_ERR_INTERACTION_NOT_FOUND:
            throw InteractionClassNotDefined(L"Interaction class not defined in FOM");
        case VALHALLA_ERR_ALREADY_MEMBER:
            throw FederateAlreadyExecutionMember(L"Federate is already a member of the federation");
        case VALHALLA_ERR_OBJECT_NAME_IN_USE:
            throw ObjectInstanceNameInUse(L"Object instance name is already in use");
        case VALHALLA_ERR_DELETE_PRIVILEGE_NOT_HELD:
            throw DeletePrivilegeNotHeld(L"Federate does not hold delete privilege for object instance");
        default:
            throw RTIinternalError(L"Valhalla RTI operation failed with status " + std::to_wstring(status));
    }
}

void ValhallaRTIambassador::connect(
    FederateAmbassador& federateAmbassador,
    CallbackModel callbackModel,
    const std::wstring& localSettingsDesignator)
    RTI1516E_THROW(ConnectionFailed,
          InvalidLocalSettingsDesignator,
          UnsupportedCallbackModel,
          AlreadyConnected,
          RTIinternalError) {
    {
        std::lock_guard<std::mutex> lock(m_stateMutex);
        if (m_connected) {
            throw AlreadyConnected(L"Already connected to RTI");
        }
        m_fedAmb = &federateAmbassador;
    }

    std::string addr = toUtf8(localSettingsDesignator);
    if (addr.empty()) {
        addr = "127.0.0.1:52295";
    }

    int32_t st = valhalla_connect(m_amb, const_cast<char*>(addr.c_str()));
    if (st != VALHALLA_OK) {
        std::lock_guard<std::mutex> lock(m_stateMutex);
        m_fedAmb = nullptr;
        throw ConnectionFailed(L"Could not connect to Valhalla CRC at " + localSettingsDesignator);
    }
    std::lock_guard<std::mutex> lock(m_stateMutex);
    m_connected = true;
}

void ValhallaRTIambassador::disconnect()
    RTI1516E_THROW(FederateIsExecutionMember,
          NotConnected,
          RTIinternalError) {
    {
        std::lock_guard<std::mutex> lock(m_stateMutex);
        if (!m_connected) {
            throw NotConnected(L"Not connected");
        }
    }
    int32_t st = valhalla_disconnect(m_amb);
    checkStatus(st);
    std::lock_guard<std::mutex> lock(m_stateMutex);
    m_connected = false;
    m_fedAmb = nullptr;
    m_federateHandle = 0;
}

void ValhallaRTIambassador::createFederationExecution(
    const std::wstring& federationExecutionName,
    const std::wstring& fullPathNameToFOMModule)
    RTI1516E_THROW(FederationExecutionAlreadyExists,
          CouldNotCreateLogicalTimeFactory,
          InconsistentFDD,
          ErrorReadingFDD,
          CouldNotOpenFDD,
          NotConnected,
          RTIinternalError) {
    std::vector<std::wstring> foms;
    if (!fullPathNameToFOMModule.empty()) {
        foms.push_back(fullPathNameToFOMModule);
    }
    createFederationExecution(federationExecutionName, foms);
}

void ValhallaRTIambassador::createFederationExecution(
    const std::wstring& federationExecutionName,
    const std::vector<std::wstring>& fomModules)
    RTI1516E_THROW(FederationExecutionAlreadyExists,
          CouldNotCreateLogicalTimeFactory,
          InconsistentFDD,
          ErrorReadingFDD,
          CouldNotOpenFDD,
          NotConnected,
          RTIinternalError) {
    std::string fedName = toUtf8(federationExecutionName);
    if (fomModules.empty()) {
        int32_t st = valhalla_create_federation(m_amb, const_cast<char*>(fedName.c_str()));
        checkStatus(st);
        return;
    }

    std::vector<std::string> utfFoms;
    std::vector<char*> fomPtrs;
    utfFoms.reserve(fomModules.size());
    fomPtrs.reserve(fomModules.size());
    for (const auto& f : fomModules) {
        utfFoms.push_back(toUtf8(f));
        fomPtrs.push_back(const_cast<char*>(utfFoms.back().c_str()));
    }

    int32_t st = valhalla_create_federation_with_fom(
        m_amb,
        const_cast<char*>(fedName.c_str()),
        fomPtrs.data(),
        static_cast<int32_t>(fomPtrs.size()));
    checkStatus(st);
}

void ValhallaRTIambassador::destroyFederationExecution(
    const std::wstring& federationExecutionName)
    RTI1516E_THROW(FederatesCurrentlyJoined,
          FederationExecutionDoesNotExist,
          NotConnected,
          RTIinternalError) {
    std::string fedName = toUtf8(federationExecutionName);
    int32_t st = valhalla_destroy_federation(m_amb, const_cast<char*>(fedName.c_str()));
    checkStatus(st);
}

FederateHandle ValhallaRTIambassador::joinFederationExecution(
    const std::wstring& federateName,
    const std::wstring& federateType,
    const std::wstring& federationExecutionName,
    const std::vector<std::wstring>& additionalFomModules)
    RTI1516E_THROW(FederateAlreadyExecutionMember,
          FederationExecutionDoesNotExist,
          SaveInProgress,
          RestoreInProgress,
          FederateNameAlreadyInUse,
          NotConnected,
          RTIinternalError) {
    std::string fedName = toUtf8(federateName);
    std::string execName = toUtf8(federationExecutionName);
    int32_t h = 0;
    int32_t st = valhalla_join_federation(m_amb, const_cast<char*>(fedName.c_str()), const_cast<char*>(execName.c_str()), &h);
    checkStatus(st);
    {
        std::lock_guard<std::mutex> lock(m_stateMutex);
        m_federateHandle = h;
    }
    return FederateHandle(h);
}

void ValhallaRTIambassador::resignFederationExecution(
    ResignAction resignAction)
    RTI1516E_THROW(OwnershipAcquisitionPending,
          FederateOwnsAttributes,
          FederateNotExecutionMember,
          NotConnected,
          RTIinternalError) {
    int32_t st = valhalla_resign_federation(m_amb);
    checkStatus(st);
    std::lock_guard<std::mutex> lock(m_stateMutex);
    m_federateHandle = 0;
}

ObjectClassHandle ValhallaRTIambassador::getObjectClassHandle(
    const std::wstring& theName)
    RTI1516E_THROW(NameNotFound,
          FederateNotExecutionMember,
          NotConnected,
          RTIinternalError) {
    std::string name = toUtf8(theName);
    int32_t h = 0;
    int32_t st = valhalla_get_object_class_handle(m_amb, const_cast<char*>(name.c_str()), &h);
    if (st == VALHALLA_ERR_CLASS_NOT_FOUND) {
        throw NameNotFound(L"Object class not found: " + theName);
    }
    checkStatus(st);
    return ObjectClassHandle(h);
}

AttributeHandle ValhallaRTIambassador::getAttributeHandle(
    ObjectClassHandle whichClass,
    const std::wstring& theName)
    RTI1516E_THROW(NameNotFound,
          InvalidObjectClassHandle,
          FederateNotExecutionMember,
          NotConnected,
          RTIinternalError) {
    std::string name = toUtf8(theName);
    int32_t h = 0;
    int32_t st = valhalla_get_attribute_handle(m_amb, whichClass.getValue(), const_cast<char*>(name.c_str()), &h);
    if (st == VALHALLA_ERR_ATTR_NOT_FOUND) {
        throw NameNotFound(L"Attribute not found: " + theName);
    }
    if (st == VALHALLA_ERR_CLASS_NOT_FOUND) {
        throw InvalidObjectClassHandle(L"Invalid object class handle");
    }
    checkStatus(st);
    return AttributeHandle(h);
}

DimensionHandle ValhallaRTIambassador::getDimensionHandle(
    const std::wstring& theName)
    RTI1516E_THROW(NameNotFound,
          FederateNotExecutionMember,
          NotConnected,
          RTIinternalError) {
    std::string name = toUtf8(theName);
    int32_t h = 0;
    int32_t st = valhalla_get_dimension_handle(m_amb, const_cast<char*>(name.c_str()), &h);
    if (st == VALHALLA_ERR_CLASS_NOT_FOUND || st == VALHALLA_ERR_ATTR_NOT_FOUND) {
        throw NameNotFound(L"Dimension not found: " + theName);
    }
    checkStatus(st);
    return DimensionHandle(h);
}

void ValhallaRTIambassador::publishObjectClassAttributes(
    ObjectClassHandle theClass,
    const AttributeHandleSet& attributeList)
    RTI1516E_THROW(ObjectClassNotDefined,
          AttributeNotDefined,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::vector<int32_t> attrs;
    for (const auto& a : attributeList) {
        attrs.push_back(a.getValue());
    }
    int32_t st = valhalla_publish_object_class(m_amb, theClass.getValue(), attrs.data(), static_cast<int32_t>(attrs.size()));
    checkStatus(st);
}

void ValhallaRTIambassador::subscribeObjectClassAttributes(
    ObjectClassHandle theClass,
    const AttributeHandleSet& attributeList)
    RTI1516E_THROW(ObjectClassNotDefined,
          AttributeNotDefined,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::vector<int32_t> attrs;
    for (const auto& a : attributeList) {
        attrs.push_back(a.getValue());
    }
    int32_t st = valhalla_subscribe_object_class(m_amb, theClass.getValue(), attrs.data(), static_cast<int32_t>(attrs.size()));
    checkStatus(st);
}

void ValhallaRTIambassador::publishInteractionClass(
    InteractionClassHandle theInteraction)
    RTI1516E_THROW(InteractionClassNotDefined,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    int32_t st = valhalla_publish_interaction_class(m_amb, theInteraction.getValue());
    checkStatus(st);
}

void ValhallaRTIambassador::subscribeInteractionClass(
    InteractionClassHandle theInteraction)
    RTI1516E_THROW(InteractionClassNotDefined,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    int32_t st = valhalla_subscribe_interaction_class(m_amb, theInteraction.getValue());
    checkStatus(st);
}

void ValhallaRTIambassador::registerFederationSynchronizationPoint(
    const std::wstring& label,
    const VariableLengthData& theUserSuppliedTag)
    RTI1516E_THROW(FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::string labelUtf8 = toUtf8(label);
    int32_t st = valhalla_register_federation_synchronization_point(
        m_amb,
        const_cast<char*>(labelUtf8.c_str()),
        const_cast<uint8_t*>(static_cast<const uint8_t*>(theUserSuppliedTag.data())),
        static_cast<int32_t>(theUserSuppliedTag.size()));
    checkStatus(st);
}

void ValhallaRTIambassador::synchronizationPointAchieved(
    const std::wstring& label)
    RTI1516E_THROW(FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::string labelUtf8 = toUtf8(label);
    int32_t st = valhalla_synchronization_point_achieved(m_amb, const_cast<char*>(labelUtf8.c_str()));
    checkStatus(st);
}

void ValhallaRTIambassador::sendInteraction(
    InteractionClassHandle theInteraction,
    const ParameterHandleValueMap& theParameterValues,
    const VariableLengthData& theUserSuppliedTag)
    RTI1516E_THROW(InteractionClassNotDefined,
          InteractionClassNotPublished,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::vector<int32_t> handles;
    std::vector<uint8_t*> valPtrs;
    std::vector<int32_t> lens;
    handles.reserve(theParameterValues.size());
    valPtrs.reserve(theParameterValues.size());
    lens.reserve(theParameterValues.size());

    for (const auto& pair : theParameterValues) {
        handles.push_back(pair.first.getValue());
        valPtrs.push_back(const_cast<uint8_t*>(static_cast<const uint8_t*>(pair.second.data())));
        lens.push_back(static_cast<int32_t>(pair.second.size()));
    }

    int32_t st = valhalla_send_interaction(
        m_amb,
        theInteraction.getValue(),
        handles.data(),
        valPtrs.data(),
        lens.data(),
        static_cast<int32_t>(handles.size()),
        const_cast<uint8_t*>(static_cast<const uint8_t*>(theUserSuppliedTag.data())),
        static_cast<int32_t>(theUserSuppliedTag.size()));
    checkStatus(st);
}

ObjectInstanceHandle ValhallaRTIambassador::registerObjectInstance(
    ObjectClassHandle theClass,
    const std::wstring& theObjectName)
    RTI1516E_THROW(ObjectClassNotDefined,
          ObjectClassNotPublished,
          ObjectInstanceNameInUse,
          ObjectInstanceNameNotReserved,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::string name = toUtf8(theObjectName);
    int32_t h = 0;
    int32_t st = valhalla_register_object(m_amb, theClass.getValue(), const_cast<char*>(name.c_str()), &h);
    checkStatus(st);
    return ObjectInstanceHandle(h);
}

void ValhallaRTIambassador::deleteObjectInstance(
    ObjectInstanceHandle theObject,
    const VariableLengthData& theUserSuppliedTag)
    RTI1516E_THROW(DeletePrivilegeNotHeld,
          ObjectInstanceNotKnown,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    int32_t st = valhalla_delete_object(
        m_amb,
        theObject.getValue(),
        const_cast<uint8_t*>(static_cast<const uint8_t*>(theUserSuppliedTag.data())),
        static_cast<int32_t>(theUserSuppliedTag.size()));
    checkStatus(st);
}

void ValhallaRTIambassador::updateAttributeValues(
    ObjectInstanceHandle theObject,
    const AttributeHandleValueMap& theAttributeValues,
    const VariableLengthData& theUserSuppliedTag)
    RTI1516E_THROW(ObjectInstanceNotKnown,
          AttributeNotDefined,
          AttributeNotOwned,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::vector<int32_t> handles;
    std::vector<uint8_t*> valPtrs;
    std::vector<int32_t> lens;

    for (const auto& pair : theAttributeValues) {
        handles.push_back(pair.first.getValue());
        valPtrs.push_back(const_cast<uint8_t*>(static_cast<const uint8_t*>(pair.second.data())));
        lens.push_back(static_cast<int32_t>(pair.second.size()));
    }

    int32_t st = valhalla_update_attributes(
        m_amb,
        theObject.getValue(),
        handles.data(),
        valPtrs.data(),
        lens.data(),
        static_cast<int32_t>(handles.size()),
        const_cast<uint8_t*>(static_cast<const uint8_t*>(theUserSuppliedTag.data())),
        static_cast<int32_t>(theUserSuppliedTag.size()));
    checkStatus(st);
}

void ValhallaRTIambassador::updateAttributeValuesSpatial(
    ObjectInstanceHandle theObject,
    const AttributeHandleValueMap& theAttributeValues,
    double x, double y, double z,
    const VariableLengthData& theUserSuppliedTag)
    RTI1516E_THROW(ObjectInstanceNotKnown,
          AttributeNotDefined,
          AttributeNotOwned,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::vector<int32_t> handles;
    std::vector<uint8_t*> valPtrs;
    std::vector<int32_t> lens;

    for (const auto& pair : theAttributeValues) {
        handles.push_back(pair.first.getValue());
        valPtrs.push_back(const_cast<uint8_t*>(static_cast<const uint8_t*>(pair.second.data())));
        lens.push_back(static_cast<int32_t>(pair.second.size()));
    }

    int32_t st = valhalla_update_attributes_spatial(
        m_amb,
        theObject.getValue(),
        handles.data(),
        valPtrs.data(),
        lens.data(),
        static_cast<int32_t>(handles.size()),
        x, y, z,
        const_cast<uint8_t*>(static_cast<const uint8_t*>(theUserSuppliedTag.data())),
        static_cast<int32_t>(theUserSuppliedTag.size()));
    checkStatus(st);
}

void ValhallaRTIambassador::unconditionalAttributeOwnershipDivestiture(
    ObjectInstanceHandle theObject,
    const AttributeHandleSet& theAttributes)
    RTI1516E_THROW(ObjectInstanceNotKnown,
          AttributeNotDefined,
          AttributeNotOwned,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::vector<int32_t> attrs;
    for (const auto& a : theAttributes) {
        attrs.push_back(a.getValue());
    }
    int32_t st = valhalla_divest_ownership(
        m_amb,
        theObject.getValue(),
        attrs.data(),
        static_cast<int32_t>(attrs.size()));
    checkStatus(st);
}

void ValhallaRTIambassador::attributeOwnershipAcquisition(
    ObjectInstanceHandle theObject,
    const AttributeHandleSet& desiredAttributes,
    const VariableLengthData& theUserSuppliedTag)
    RTI1516E_THROW(ObjectInstanceNotKnown,
          ObjectClassNotPublished,
          AttributeNotDefined,
          FederateOwnsAttributes,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::vector<int32_t> attrs;
    for (const auto& a : desiredAttributes) {
        attrs.push_back(a.getValue());
    }
    int32_t st = valhalla_acquire_ownership(
        m_amb,
        theObject.getValue(),
        attrs.data(),
        static_cast<int32_t>(attrs.size()));
    checkStatus(st);
}

bool ValhallaRTIambassador::isAttributeOwnedByFederate(
    ObjectInstanceHandle theObject,
    AttributeHandle theAttribute)
    RTI1516E_THROW(ObjectInstanceNotKnown,
          AttributeNotDefined,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    int32_t owner = 0;
    bool owned = false;
    int32_t st = valhalla_query_ownership(
        m_amb,
        theObject.getValue(),
        theAttribute.getValue(),
        &owner,
        &owned);
    checkStatus(st);
    int32_t localHandle = 0;
    {
        std::lock_guard<std::mutex> lock(m_stateMutex);
        localHandle = m_federateHandle;
    }
    return owned && (localHandle > 0) && (owner == localHandle);
}

void ValhallaRTIambassador::enableTimeRegulation(
    double theLookahead)
    RTI1516E_THROW(TimeRegulationAlreadyEnabled,
          InvalidLookahead,
          InTimeAdvancingState,
          RequestForTimeRegulationPending,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    int32_t st = valhalla_enable_time_regulation(m_amb, theLookahead);
    checkStatus(st);
}

void ValhallaRTIambassador::enableTimeConstrained()
    RTI1516E_THROW(TimeConstrainedAlreadyEnabled,
          InTimeAdvancingState,
          RequestForTimeConstrainedPending,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    int32_t st = valhalla_enable_time_constrained(m_amb);
    checkStatus(st);
}

void ValhallaRTIambassador::timeAdvanceRequest(
    double theTime)
    RTI1516E_THROW(InvalidLogicalTime,
              LogicalTimeAlreadyPassed,
              InTimeAdvancingState,
              RequestForTimeRegulationPending,
              RequestForTimeConstrainedPending,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError) {
    int32_t st = valhalla_time_advance_request(m_amb, theTime);
    checkStatus(st);
}

void ValhallaRTIambassador::nextMessageRequest(
    double theTime)
    RTI1516E_THROW(InvalidLogicalTime,
              LogicalTimeAlreadyPassed,
              InTimeAdvancingState,
              RequestForTimeRegulationPending,
              RequestForTimeConstrainedPending,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError) {
    int32_t st = valhalla_next_message_request(m_amb, theTime);
    checkStatus(st);
}

RegionHandle ValhallaRTIambassador::createRegion(
    const std::vector<DimensionHandle>& dimensions)
    RTI1516E_THROW(InvalidDimensionHandle,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::vector<int32_t> dims;
    for (const auto& d : dimensions) {
        dims.push_back(d.getValue());
    }
    uint32_t regHandle = 0;
    int32_t st = valhalla_create_region(m_amb, dims.data(), static_cast<int32_t>(dims.size()), &regHandle);
    checkStatus(st);
    return RegionHandle(regHandle);
}

void ValhallaRTIambassador::setRangeBounds(
    RegionHandle theRegion,
    DimensionHandle theDimension,
    const RangeBounds& theRangeBounds)
    RTI1516E_THROW(InvalidRegion,
          InvalidDimensionHandle,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    int32_t st = valhalla_set_range_bounds(
        m_amb,
        theRegion.getValue(),
        theDimension.getValue(),
        theRangeBounds.lowerBound,
        theRangeBounds.upperBound);
    checkStatus(st);
}

void ValhallaRTIambassador::setMetricBounds(
    RegionHandle theRegion,
    double minX, double maxX,
    double minY, double maxY,
    double minZ, double maxZ)
    RTI1516E_THROW(InvalidRegion,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    int32_t st = valhalla_set_metric_bounds(
        m_amb,
        theRegion.getValue(),
        minX, maxX, minY, maxY, minZ, maxZ);
    checkStatus(st);
}

void ValhallaRTIambassador::subscribeObjectClassAttributesWithRegions(
    ObjectClassHandle theClass,
    const AttributeHandleSet& attributeList,
    RegionHandle theRegion)
    RTI1516E_THROW(ObjectClassNotDefined,
          AttributeNotDefined,
          InvalidRegion,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    std::vector<int32_t> attrs;
    for (const auto& a : attributeList) {
        attrs.push_back(a.getValue());
    }
    int32_t st = valhalla_subscribe_with_region(
        m_amb,
        theClass.getValue(),
        attrs.data(),
        static_cast<int32_t>(attrs.size()),
        theRegion.getValue());
    checkStatus(st);
}

bool ValhallaRTIambassador::evokeCallback(
    double approximateMinimumTimeInSeconds)
    RTI1516E_THROW(CallNotAllowedFromWithinCallback,
          RTIinternalError) {
    bool evoked = false;
    int32_t st = valhalla_evoke_callback(m_amb, approximateMinimumTimeInSeconds, &evoked);
    if (st != VALHALLA_OK) {
        throw RTIinternalError(L"evokeCallback failed with status " + std::to_wstring(st));
    }
    return evoked;
}

bool ValhallaRTIambassador::evokeMultipleCallbacks(
    double minimumTime,
    double maximumTime)
    RTI1516E_THROW(CallNotAllowedFromWithinCallback,
                   RTIinternalError) {
    if (minimumTime < 0.0) {
        minimumTime = 0.0;
    }
    if (maximumTime < minimumTime) {
        maximumTime = minimumTime;
    }
    const auto start = std::chrono::steady_clock::now();
    bool anyEvoked = false;
    const double step = (minimumTime > 0.0 && minimumTime < 0.01) ? minimumTime : 0.01;

    while (true) {
        const double elapsed = std::chrono::duration<double>(std::chrono::steady_clock::now() - start).count();
        const double remaining = maximumTime - elapsed;
        if (remaining <= 0.0 && elapsed >= minimumTime) {
            break;
        }
        double slice = step;
        if (remaining > 0.0 && slice > remaining) {
            slice = remaining;
        } else if (remaining <= 0.0) {
            slice = 0.0;
        }
        if (evokeCallback(slice)) {
            anyEvoked = true;
        } else if (std::chrono::duration<double>(std::chrono::steady_clock::now() - start).count() >= minimumTime) {
            break;
        }
        if (remaining <= 0.0) {
            break;
        }
    }
    return anyEvoked;
}

} // namespace rti1516e
