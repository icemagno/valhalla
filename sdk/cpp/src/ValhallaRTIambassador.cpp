// ValhallaRTIambassador.cpp - Implementation of IEEE 1516e RTIambassador for Valhalla
#include "ValhallaRTIambassador.h"
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

static thread_local ValhallaRTIambassador* t_activeAmbassador = nullptr;

static void c_onDiscoverObject(int32_t theObject, int32_t theObjectClass, const char* objectName) {
    try {
        if (t_activeAmbassador && t_activeAmbassador->getFederateAmbassador()) {
            std::string s(objectName ? objectName : "");
            t_activeAmbassador->getFederateAmbassador()->discoverObjectInstance(
                ObjectInstanceHandle(theObject),
                ObjectClassHandle(theObjectClass),
                toWide(s));
        }
    } catch (...) {
        // Never allow C++ exceptions to unwind across CGO/C-ABI boundary
    }
}

static void c_onRemoveObject(int32_t theObject, const uint8_t* tag, int32_t tagLen) {
    try {
        if (t_activeAmbassador && t_activeAmbassador->getFederateAmbassador()) {
            VariableLengthData userTag(tag, tagLen > 0 ? tagLen : 0);
            t_activeAmbassador->getFederateAmbassador()->removeObjectInstance(
                ObjectInstanceHandle(theObject),
                userTag,
                OrderType::RECEIVE);
        }
    } catch (...) {
    }
}

static void c_onReflectAttributes(int32_t theObject, int32_t attrCount, const int32_t* attrHandles, const uint8_t** attrValues, const int32_t* attrLens, const uint8_t* tag, int32_t tagLen) {
    try {
        if (t_activeAmbassador && t_activeAmbassador->getFederateAmbassador()) {
            AttributeHandleValueMap valMap;
            for (int32_t i = 0; i < attrCount; ++i) {
                valMap[AttributeHandle(attrHandles[i])] = VariableLengthData(attrValues[i], attrLens[i]);
            }
            VariableLengthData userTag(tag, tagLen > 0 ? tagLen : 0);
            t_activeAmbassador->getFederateAmbassador()->reflectAttributeValues(
                ObjectInstanceHandle(theObject),
                valMap,
                userTag,
                OrderType::RECEIVE,
                TransportationTypeHandle::HLA_RELIABLE);
        }
    } catch (...) {
    }
}

static void c_onTimeAdvanceGrant(double time) {
    try {
        if (t_activeAmbassador && t_activeAmbassador->getFederateAmbassador()) {
            t_activeAmbassador->getFederateAmbassador()->timeAdvanceGrant(time);
        }
    } catch (...) {
    }
}

ValhallaRTIambassador::ValhallaRTIambassador()
    : m_amb(nullptr), m_fedAmb(nullptr), m_connected(false) {
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
        default:
            throw RTIinternalError(L"Valhalla RTI operation failed with status " + std::to_wstring(status));
    }
}

void ValhallaRTIambassador::connect(
    FederateAmbassador& federateAmbassador,
    CallbackModel callbackModel,
    const std::wstring& localSettingsDesignator)
    throw(ConnectionFailed,
          InvalidLocalSettingsDesignator,
          UnsupportedCallbackModel,
          AlreadyConnected,
          RTIinternalError) {
    if (m_connected) {
        throw AlreadyConnected(L"Already connected to RTI");
    }

    m_fedAmb = &federateAmbassador;
    std::string addr = toUtf8(localSettingsDesignator);
    if (addr.empty()) {
        addr = "127.0.0.1:52295";
    }

    int32_t st = valhalla_connect(m_amb, const_cast<char*>(addr.c_str()));
    if (st != VALHALLA_OK) {
        throw ConnectionFailed(L"Could not connect to Valhalla CRC at " + localSettingsDesignator);
    }
    m_connected = true;
}

void ValhallaRTIambassador::disconnect()
    throw(FederateIsExecutionMember,
          NotConnected,
          RTIinternalError) {
    if (!m_connected) {
        throw NotConnected(L"Not connected");
    }
    int32_t st = valhalla_disconnect(m_amb);
    checkStatus(st);
    m_connected = false;
    m_fedAmb = nullptr;
}

void ValhallaRTIambassador::createFederationExecution(
    const std::wstring& federationExecutionName,
    const std::wstring& fullPathNameToFOMModule)
    throw(FederationExecutionAlreadyExists,
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
    throw(FederationExecutionAlreadyExists,
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
    throw(FederatesCurrentlyJoined,
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
    throw(FederateAlreadyExecutionMember,
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
    return FederateHandle(h);
}

void ValhallaRTIambassador::resignFederationExecution(
    ResignAction resignAction)
    throw(OwnershipAcquisitionPending,
          FederateOwnsAttributes,
          FederateNotExecutionMember,
          NotConnected,
          RTIinternalError) {
    int32_t st = valhalla_resign_federation(m_amb);
    checkStatus(st);
}

ObjectClassHandle ValhallaRTIambassador::getObjectClassHandle(
    const std::wstring& theName)
    throw(NameNotFound,
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
    throw(NameNotFound,
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
    throw(NameNotFound,
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
    throw(ObjectClassNotDefined,
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
    throw(ObjectClassNotDefined,
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

ObjectInstanceHandle ValhallaRTIambassador::registerObjectInstance(
    ObjectClassHandle theClass,
    const std::wstring& theObjectName)
    throw(ObjectClassNotDefined,
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
    throw(DeletePrivilegeNotHeld,
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
    throw(ObjectInstanceNotKnown,
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
    throw(ObjectInstanceNotKnown,
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
    throw(ObjectInstanceNotKnown,
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
    throw(ObjectInstanceNotKnown,
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
    throw(ObjectInstanceNotKnown,
          AttributeNotDefined,
          FederateNotExecutionMember,
          SaveInProgress,
          RestoreInProgress,
          NotConnected,
          RTIinternalError) {
    int32_t owner = 0;
    _Bool owned = false;
    int32_t st = valhalla_query_ownership(
        m_amb,
        theObject.getValue(),
        theAttribute.getValue(),
        &owner,
        &owned);
    checkStatus(st);
    return owned;
}

void ValhallaRTIambassador::enableTimeRegulation(
    double theLookahead)
    throw(TimeRegulationAlreadyEnabled,
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
    throw(TimeConstrainedAlreadyEnabled,
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
    throw(InvalidLogicalTime,
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
    throw(InvalidLogicalTime,
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
    throw(InvalidDimensionHandle,
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
    throw(InvalidRegion,
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
    throw(InvalidRegion,
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
    throw(ObjectClassNotDefined,
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
    throw(CallNotAllowedFromWithinCallback,
          RTIinternalError) {
    t_activeAmbassador = this;
    _Bool evoked = false;
    int32_t st = valhalla_evoke_callback(m_amb, approximateMinimumTimeInSeconds, &evoked);
    t_activeAmbassador = nullptr;
    if (st != VALHALLA_OK) {
        throw RTIinternalError(L"evokeCallback failed with status " + std::to_wstring(st));
    }
    return evoked;
}

bool ValhallaRTIambassador::evokeMultipleCallbacks(
    double minimumTime,
    double maximumTime)
    throw(CallNotAllowedFromWithinCallback,
          RTIinternalError) {
    if (maximumTime < minimumTime) {
        maximumTime = minimumTime;
    }
    bool anyEvoked = false;
    double elapsed = 0.0;
    const double step = (minimumTime > 0.0 && minimumTime < 0.01) ? minimumTime : 0.01;
    do {
        double slice = step;
        if (elapsed + slice > maximumTime) {
            slice = maximumTime - elapsed;
        }
        if (slice < 0.0) {
            slice = 0.0;
        }
        if (evokeCallback(slice)) {
            anyEvoked = true;
        }
        elapsed += slice;
    } while (elapsed < maximumTime && slice > 0.0);
    return anyEvoked;
}

} // namespace rti1516e
