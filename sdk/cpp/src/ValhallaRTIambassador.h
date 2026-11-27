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
        RTI1516E_THROW(ConnectionFailed,
              InvalidLocalSettingsDesignator,
              UnsupportedCallbackModel,
              AlreadyConnected,
              RTIinternalError);

    virtual void disconnect()
        RTI1516E_THROW(FederateIsExecutionMember,
              NotConnected,
              RTIinternalError);

    virtual void createFederationExecution(
        const std::wstring& federationExecutionName,
        const std::wstring& fullPathNameToFOMModule)
        RTI1516E_THROW(FederationExecutionAlreadyExists,
              CouldNotCreateLogicalTimeFactory,
              InconsistentFDD,
              ErrorReadingFDD,
              CouldNotOpenFDD,
              NotConnected,
              RTIinternalError);

    virtual void createFederationExecution(
        const std::wstring& federationExecutionName,
        const std::vector<std::wstring>& fomModules)
        RTI1516E_THROW(FederationExecutionAlreadyExists,
              CouldNotCreateLogicalTimeFactory,
              InconsistentFDD,
              ErrorReadingFDD,
              CouldNotOpenFDD,
              NotConnected,
              RTIinternalError);

    virtual void destroyFederationExecution(
        const std::wstring& federationExecutionName)
        RTI1516E_THROW(FederatesCurrentlyJoined,
              FederationExecutionDoesNotExist,
              NotConnected,
              RTIinternalError);

    virtual FederateHandle joinFederationExecution(
        const std::wstring& federateName,
        const std::wstring& federateType,
        const std::wstring& federationExecutionName,
        const std::vector<std::wstring>& additionalFomModules = {})
        RTI1516E_THROW(FederateAlreadyExecutionMember,
              FederationExecutionDoesNotExist,
              SaveInProgress,
              RestoreInProgress,
              FederateNameAlreadyInUse,
              NotConnected,
              RTIinternalError);

    virtual void resignFederationExecution(
        ResignAction resignAction)
        RTI1516E_THROW(OwnershipAcquisitionPending,
              FederateOwnsAttributes,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError);

    // OMT Introspection
    virtual ObjectClassHandle getObjectClassHandle(
        const std::wstring& theName)
        RTI1516E_THROW(NameNotFound,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError);

    virtual AttributeHandle getAttributeHandle(
        ObjectClassHandle whichClass,
        const std::wstring& theName)
        RTI1516E_THROW(NameNotFound,
              InvalidObjectClassHandle,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError);

    virtual DimensionHandle getDimensionHandle(
        const std::wstring& theName)
        RTI1516E_THROW(NameNotFound,
              FederateNotExecutionMember,
              NotConnected,
              RTIinternalError);

    // Declaration Management
    virtual void publishObjectClassAttributes(
        ObjectClassHandle theClass,
        const AttributeHandleSet& attributeList)
        RTI1516E_THROW(ObjectClassNotDefined,
              AttributeNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void subscribeObjectClassAttributes(
        ObjectClassHandle theClass,
        const AttributeHandleSet& attributeList)
        RTI1516E_THROW(ObjectClassNotDefined,
              AttributeNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void publishInteractionClass(
        InteractionClassHandle theInteraction)
        RTI1516E_THROW(InteractionClassNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void subscribeInteractionClass(
        InteractionClassHandle theInteraction)
        RTI1516E_THROW(InteractionClassNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    // Synchronization Points
    virtual void registerFederationSynchronizationPoint(
        const std::wstring& label,
        const VariableLengthData& theUserSuppliedTag)
        RTI1516E_THROW(FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void synchronizationPointAchieved(
        const std::wstring& label)
        RTI1516E_THROW(FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    // Object Management
    virtual ObjectInstanceHandle registerObjectInstance(
        ObjectClassHandle theClass,
        const std::wstring& theObjectName = L"")
        RTI1516E_THROW(ObjectClassNotDefined,
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
        RTI1516E_THROW(DeletePrivilegeNotHeld,
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
        RTI1516E_THROW(ObjectInstanceNotKnown,
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
        RTI1516E_THROW(ObjectInstanceNotKnown,
              AttributeNotDefined,
              AttributeNotOwned,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void sendInteraction(
        InteractionClassHandle theInteraction,
        const ParameterHandleValueMap& theParameterValues,
        const VariableLengthData& theUserSuppliedTag)
        RTI1516E_THROW(InteractionClassNotDefined,
              InteractionClassNotPublished,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    // Ownership Management
    virtual void unconditionalAttributeOwnershipDivestiture(
        ObjectInstanceHandle theObject,
        const AttributeHandleSet& theAttributes)
        RTI1516E_THROW(ObjectInstanceNotKnown,
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
        RTI1516E_THROW(ObjectInstanceNotKnown,
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
        RTI1516E_THROW(ObjectInstanceNotKnown,
              AttributeNotDefined,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    // Time Management
    virtual void enableTimeRegulation(
        double theLookahead)
        RTI1516E_THROW(TimeRegulationAlreadyEnabled,
              InvalidLookahead,
              InTimeAdvancingState,
              RequestForTimeRegulationPending,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void enableTimeConstrained()
        RTI1516E_THROW(TimeConstrainedAlreadyEnabled,
              InTimeAdvancingState,
              RequestForTimeConstrainedPending,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void timeAdvanceRequest(
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
              RTIinternalError);

    virtual void nextMessageRequest(
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
              RTIinternalError);

    // Data Distribution Management (DDM)
    virtual RegionHandle createRegion(
        const std::vector<DimensionHandle>& dimensions)
        RTI1516E_THROW(InvalidDimensionHandle,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void setRangeBounds(
        RegionHandle theRegion,
        DimensionHandle theDimension,
        const RangeBounds& theRangeBounds)
        RTI1516E_THROW(InvalidRegion,
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
        RTI1516E_THROW(InvalidRegion,
              FederateNotExecutionMember,
              SaveInProgress,
              RestoreInProgress,
              NotConnected,
              RTIinternalError);

    virtual void subscribeObjectClassAttributesWithRegions(
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
              RTIinternalError);

    // Evoke Callbacks
    virtual bool evokeCallback(
        double approximateMinimumTimeInSeconds)
        RTI1516E_THROW(CallNotAllowedFromWithinCallback,
              RTIinternalError);

    virtual bool evokeMultipleCallbacks(
        double minimumTime,
        double maximumTime)
        RTI1516E_THROW(CallNotAllowedFromWithinCallback,
              RTIinternalError);

    // Callback dispatching internals
    FederateAmbassador* getFederateAmbassador() const {
        std::lock_guard<std::mutex> lock(m_stateMutex);
        return m_fedAmb;
    }

private:
    mutable std::mutex m_stateMutex;
    ValhallaAmbassador m_amb;
    FederateAmbassador* m_fedAmb;
    bool m_connected;
    int32_t m_federateHandle;

    void checkStatus(int32_t status);
};

} // namespace rti1516e

#endif // VALHALLA_RTIAMBASSADOR_H
