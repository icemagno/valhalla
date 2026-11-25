// NullFederateAmbassador.h - Default no-op implementation of FederateAmbassador
#ifndef RTI1516E_NULLFEDERATEAMBASSADOR_H
#define RTI1516E_NULLFEDERATEAMBASSADOR_H

#include <rti1516e/FederateAmbassador.h>

namespace rti1516e {

class NullFederateAmbassador : public FederateAmbassador {
public:
    virtual ~NullFederateAmbassador() throw() {}

    virtual void synchronizationPointRegistrationSucceeded(
        const std::wstring& label) throw(FederateInternalError) {}

    virtual void synchronizationPointRegistrationFailed(
        const std::wstring& label,
        SynchronizationPointFailureReason reason) throw(FederateInternalError) {}

    virtual void announceSynchronizationPoint(
        const std::wstring& label,
        const VariableLengthData& userSuppliedTag) throw(FederateInternalError) {}

    virtual void federationSynchronized(
        const std::wstring& label) throw(FederateInternalError) {}

    virtual void discoverObjectInstance(
        ObjectInstanceHandle theObject,
        ObjectClassHandle theObjectClass,
        const std::wstring& theObjectName) throw(FederateInternalError) {}

    virtual void reflectAttributeValues(
        ObjectInstanceHandle theObject,
        const AttributeHandleValueMap& theAttributeValues,
        const VariableLengthData& userSuppliedTag,
        OrderType sentOrder,
        TransportationTypeHandle theType) throw(FederateInternalError) {}

    virtual void receiveInteraction(
        InteractionClassHandle theInteraction,
        const ParameterHandleValueMap& theParameters,
        const VariableLengthData& userSuppliedTag,
        OrderType sentOrder,
        TransportationTypeHandle theType) throw(FederateInternalError) {}

    virtual void removeObjectInstance(
        ObjectInstanceHandle theObject,
        const VariableLengthData& userSuppliedTag,
        OrderType sentOrder) throw(FederateInternalError) {}

    virtual void timeRegulationEnabled(
        double theFederateTime) throw(FederateInternalError) {}

    virtual void timeConstrainedEnabled(
        double theFederateTime) throw(FederateInternalError) {}

    virtual void timeAdvanceGrant(
        double theTime) throw(FederateInternalError) {}
};

} // namespace rti1516e

#endif // RTI1516E_NULLFEDERATEAMBASSADOR_H
