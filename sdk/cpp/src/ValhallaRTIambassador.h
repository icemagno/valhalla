// ValhallaRTIambassador.h - Valhalla Implementation of rti1516e::RTIambassador
#ifndef VALHALLA_RTIAMBASSADOR_H
#define VALHALLA_RTIAMBASSADOR_H

#include <rti1516e/RTIambassador.h>
#include <valhalla_lrc.h>
#include <string>
#include <vector>
#include <mutex>

namespace rti1516e {

class ValhallaRTIambassador : public RTIambassador {
public:
    ValhallaRTIambassador();
    virtual ~ValhallaRTIambassador() throw();

    // Non-copyable (owns opaque native C-ABI handle)
    ValhallaRTIambassador(const ValhallaRTIambassador&) = delete;
    ValhallaRTIambassador& operator=(const ValhallaRTIambassador&) = delete;

    // Federation Management
    virtual void connect(
        FederateAmbassador& federateAmbassador,
        CallbackModel callbackModel,
        const std::wstring& localSettingsDesignator = L"")
        throw(ConnectionFailed,
              InvalidLocalSettingsDesignator,
              UnsupportedCallbackModel,
              AlreadyConnected,
              RTIinternalError);

    virtual void disconnect()
        throw(FederateIsExecutionMember,
              NotConnected,
              RTIinternalError);

    virtual void createFederationExecution(
        const std::wstring& federationExecutionName,
        const std::wstring& fullPathNameToFOMModule)
        throw(FederationExecutionAlreadyExists,
              CouldNotCreateLogicalTimeFactory,
              InconsistentFDD,
              ErrorReadingFDD,
              CouldNotOpenFDD,
              NotConnected,
              RTIinternalError);

    virtual void createFederationExecution(
        const std::wstring& federationExecutionName,
        const std::vector<std::wstring>& fomModules)
        throw(FederationExecutionAlreadyExists,
              CouldNotCreateLogicalTimeFactory,
              InconsistentFDD,
              ErrorReadingFDD,
              CouldNotOpenFDD,
              NotConnected,
              RTIinternalError);

    virtual void destroyFederationExecution(
        const std::wstring& federationExecutionName)
        throw(FederatesCurrentlyJoined,
              FederationExecutionDoesNotExist,
              NotConnected,
              RTIinternalError);

    virtual FederateHandle joinFederationExecution(
        const std::wstring& federateName,
        const std::wstring& federateType,
        const std::wstring& federationExecutionName,
        const std::vector<std::wstring>& additionalFomModules = {})
        throw(FederateAlreadyExecutionMember,
              FederationExecutionDoesNotExist,
              SaveInProgress,
              RestoreInProgress,
              FederateNameAlreadyInUse,
              NotConnected,
              RTIinternalError);

    virtual void resignFederationExecution(
        ResignAction resignAction)
        throw(OwnershipAcquisitionPending,
              FederateOwnsAttributes,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError);

    // OMT Introspection
    virtual ObjectClassHandle getObjectClassHandle(
        const std::wstring& theName)
        throw(NameNotFound,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError);

    virtual AttributeHandle getAttributeHandle(
        ObjectClassHandle whichClass,
        const std::wstring& theName)
        throw(NameNotFound,
              InvalidObjectClassHandle,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError);

    virtual DimensionHandle getDimensionHandle(
        const std::wstring& theName)
        throw(NameNotFound,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError);

    // Declaration Management
    virtual void publishObjectClassAttributes(
        ObjectClassHandle theClass,
        const AttributeHandleSet& attributeList)
        throw(ObjectClassNotDefined,
              AttributeNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void subscribeObjectClassAttributes(
        ObjectClassHandle theClass,
        const AttributeHandleSet& attributeList)
        throw(ObjectClassNotDefined,
              AttributeNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    // Object Management
    virtual ObjectInstanceHandle registerObjectInstance(
        ObjectClassHandle theClass,
        const std::wstring& theObjectName = L"")
        throw(ObjectClassNotDefined,
              ObjectClassNotPublished,
              ObjectInstanceNameInUse,
              ObjectInstanceNameNotReserved,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void deleteObjectInstance(
        ObjectInstanceHandle theObject,
        const VariableLengthData& theUserSuppliedTag)
        throw(DeletePrivilegeNotHeld,
              ObjectInstanceNotKnown,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void updateAttributeValues(
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
              RTIinternalError);

    virtual void updateAttributeValuesSpatial(
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
              RTIinternalError);

    // Ownership Management
    virtual void unconditionalAttributeOwnershipDivestiture(
        ObjectInstanceHandle theObject,
        const AttributeHandleSet& theAttributes)
        throw(ObjectInstanceNotKnown,
              AttributeNotDefined,
              AttributeNotOwned,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void attributeOwnershipAcquisition(
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
              RTIinternalError);

    virtual bool isAttributeOwnedByFederate(
        ObjectInstanceHandle theObject,
        AttributeHandle theAttribute)
        throw(ObjectInstanceNotKnown,
              AttributeNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    // Time Management
    virtual void enableTimeRegulation(
        double theLookahead)
        throw(TimeRegulationAlreadyEnabled,
              InvalidLookahead,
              InTimeAdvancingState,
              RequestForTimeRegulationPending,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void enableTimeConstrained()
        throw(TimeConstrainedAlreadyEnabled,
              InTimeAdvancingState,
              RequestForTimeConstrainedPending,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void timeAdvanceRequest(
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
              RTIinternalError);

    virtual void nextMessageRequest(
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
              RTIinternalError);

    // Data Distribution Management (DDM)
    virtual RegionHandle createRegion(
        const std::vector<DimensionHandle>& dimensions)
        throw(InvalidDimensionHandle,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void setRangeBounds(
        RegionHandle theRegion,
        DimensionHandle theDimension,
        const RangeBounds& theRangeBounds)
        throw(InvalidRegion,
              InvalidDimensionHandle,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void setMetricBounds(
        RegionHandle theRegion,
        double minX, double maxX,
        double minY, double maxY,
        double minZ, double maxZ)
        throw(InvalidRegion,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void subscribeObjectClassAttributesWithRegions(
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
              RTIinternalError);

    // Evoke Callbacks
    virtual bool evokeCallback(
        double approximateMinimumTimeInSeconds)
        throw(CallNotAllowedFromWithinCallback,
              RTIinternalError);

    virtual bool evokeMultipleCallbacks(
        double minimumTime,
        double maximumTime)
        throw(CallNotAllowedFromWithinCallback,
              RTIinternalError);

    // Callback dispatching internals
    FederateAmbassador* getFederateAmbassador() const { return m_fedAmb; }

private:
    ValhallaAmbassador m_amb;
    FederateAmbassador* m_fedAmb;
    bool m_connected;

    void checkStatus(int32_t status);
};

} // namespace rti1516e

#endif // VALHALLA_RTIAMBASSADOR_H
