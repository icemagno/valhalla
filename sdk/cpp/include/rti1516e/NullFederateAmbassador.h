// NullFederateAmbassador.h - Default no-op implementation of FederateAmbassador
#ifndef RTI1516E_NULLFEDERATEAMBASSADOR_H
#define RTI1516E_NULLFEDERATEAMBASSADOR_H

#include <rti1516e/FederateAmbassador.h>
#include <rti1516e/RTIexception.h>

namespace rti1516e {

class NullFederateAmbassador : public FederateAmbassador {
public:
    virtual ~NullFederateAmbassador() throw() {}

    void synchronizationPointRegistrationSucceeded(
        const std::wstring& label) override {}

    void synchronizationPointRegistrationFailed(
        const std::wstring& label,
        SynchronizationPointFailureReason reason) override {}

    void announceSynchronizationPoint(
        const std::wstring& label,
        const VariableLengthData& userSuppliedTag) override {}

    void federationSynchronized(
        const std::wstring& label) override {}

    void discoverObjectInstance(
        ObjectInstanceHandle theObject,
        ObjectClassHandle theObjectClass,
        const std::wstring& theObjectName) override {}

    void reflectAttributeValues(
        ObjectInstanceHandle theObject,
        const AttributeHandleValueMap& theAttributeValues,
        const VariableLengthData& userSuppliedTag,
        OrderType sentOrder,
        TransportationTypeHandle theType) override {}

    void receiveInteraction(
        InteractionClassHandle theInteraction,
        const ParameterHandleValueMap& theParameters,
        const VariableLengthData& userSuppliedTag,
        OrderType sentOrder,
        TransportationTypeHandle theType) override {}

    void removeObjectInstance(
        ObjectInstanceHandle theObject,
        const VariableLengthData& userSuppliedTag,
        OrderType sentOrder) override {}

    void timeRegulationEnabled(double theFederateTime) override {}

    void timeConstrainedEnabled(double theFederateTime) override {}

    void timeAdvanceGrant(double theTime) override {}
};

} // namespace rti1516e

#endif // RTI1516E_NULLFEDERATEAMBASSADOR_H
