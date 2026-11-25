// RTIambassador.h - Official IEEE 1516e Pure Virtual RTI Ambassador Interface
#ifndef RTI1516E_RTIAMBASSADOR_H
#define RTI1516E_RTIAMBASSADOR_H

#include <rti1516e/Enums.h>
#include <rti1516e/RTIexception.h>
#include <rti1516e/Typedefs.h>
#include <rti1516e/FederateAmbassador.h>

#include <string>
#include <vector>

namespace rti1516e {

class RTIambassador {
public:
    virtual ~RTIambassador() throw() {}

    // -------------------------------------------------------------------------
    // 4.2 Federation Management
    // -------------------------------------------------------------------------
    virtual void connect(
        FederateAmbassador& federateAmbassador,
        CallbackModel callbackModel,
        const std::wstring& localSettingsDesignator = L"")
        throw(ConnectionFailed,
              InvalidLocalSettingsDesignator,
              UnsupportedCallbackModel,
              AlreadyConnected,
              RTIinternalError) = 0;

    virtual void disconnect()
        throw(FederateIsExecutionMember,
              NotConnected,
              RTIinternalError) = 0;

    virtual void createFederationExecution(
        const std::wstring& federationExecutionName,
        const std::wstring& fullPathNameToFOMModule)
        throw(FederationExecutionAlreadyExists,
              CouldNotCreateLogicalTimeFactory,
              InconsistentFDD,
              ErrorReadingFDD,
              CouldNotOpenFDD,
              NotConnected,
              RTIinternalError) = 0;

    virtual void createFederationExecution(
        const std::wstring& federationExecutionName,
        const std::vector<std::wstring>& fomModules)
        throw(FederationExecutionAlreadyExists,
              CouldNotCreateLogicalTimeFactory,
              InconsistentFDD,
              ErrorReadingFDD,
              CouldNotOpenFDD,
              NotConnected,
              RTIinternalError) = 0;

    virtual void destroyFederationExecution(
        const std::wstring& federationExecutionName)
        throw(FederatesCurrentlyJoined,
              FederationExecutionDoesNotExist,
              NotConnected,
              RTIinternalError) = 0;

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
              RTIinternalError) = 0;

    virtual void resignFederationExecution(
        ResignAction resignAction)
        throw(OwnershipAcquisitionPending,
              FederateOwnsAttributes,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError) = 0;

    // -------------------------------------------------------------------------
    // 10. Object Model Template (OMT) Introspection
    // -------------------------------------------------------------------------
    virtual ObjectClassHandle getObjectClassHandle(
        const std::wstring& theName)
        throw(NameNotFound,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError) = 0;

    virtual AttributeHandle getAttributeHandle(
        ObjectClassHandle whichClass,
        const std::wstring& theName)
        throw(NameNotFound,
              InvalidObjectClassHandle,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError) = 0;

    virtual DimensionHandle getDimensionHandle(
        const std::wstring& theName)
        throw(NameNotFound,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError) = 0;

    // -------------------------------------------------------------------------
    // 5. Declaration Management
    // -------------------------------------------------------------------------
    virtual void publishObjectClassAttributes(
        ObjectClassHandle theClass,
        const AttributeHandleSet& attributeList)
        throw(ObjectClassNotDefined,
              AttributeNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError) = 0;

    virtual void subscribeObjectClassAttributes(
        ObjectClassHandle theClass,
        const AttributeHandleSet& attributeList)
        throw(ObjectClassNotDefined,
              AttributeNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError) = 0;

    // -------------------------------------------------------------------------
    // 6. Object Management
    // -------------------------------------------------------------------------
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
              RTIinternalError) = 0;

    virtual void deleteObjectInstance(
        ObjectInstanceHandle theObject,
        const VariableLengthData& theUserSuppliedTag)
        throw(DeletePrivilegeNotHeld,
              ObjectInstanceNotKnown,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError) = 0;

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
              RTIinternalError) = 0;

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
              RTIinternalError) = 0;

    // -------------------------------------------------------------------------
    // 7. Ownership Management
    // -------------------------------------------------------------------------
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
              RTIinternalError) = 0;

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
              RTIinternalError) = 0;

    virtual bool isAttributeOwnedByFederate(
        ObjectInstanceHandle theObject,
        AttributeHandle theAttribute)
        throw(ObjectInstanceNotKnown,
              AttributeNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError) = 0;

    // -------------------------------------------------------------------------
    // 8. Time Management
    // -------------------------------------------------------------------------
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
              RTIinternalError) = 0;

    virtual void enableTimeConstrained()
        throw(TimeConstrainedAlreadyEnabled,
              InTimeAdvancingState,
              RequestForTimeConstrainedPending,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError) = 0;

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
              RTIinternalError) = 0;

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
              RTIinternalError) = 0;

    // -------------------------------------------------------------------------
    // 9. Data Distribution Management (DDM)
    // -------------------------------------------------------------------------
    virtual RegionHandle createRegion(
        const std::vector<DimensionHandle>& dimensions)
        throw(InvalidDimensionHandle,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError) = 0;

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
              RTIinternalError) = 0;

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
              RTIinternalError) = 0;

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
              RTIinternalError) = 0;

    // -------------------------------------------------------------------------
    // Evoke Callbacks
    // -------------------------------------------------------------------------
    virtual bool evokeCallback(
        double approximateMinimumTimeInSeconds)
        throw(CallNotAllowedFromWithinCallback,
              RTIinternalError) = 0;

    virtual bool evokeMultipleCallbacks(
        double minimumTime,
        double maximumTime)
        throw(CallNotAllowedFromWithinCallback,
              RTIinternalError) = 0;
};

} // namespace rti1516e

#endif // RTI1516E_RTIAMBASSADOR_H
