#ifndef RTI1516E_FEDERATEAMBASSADOR_H
#define RTI1516E_FEDERATEAMBASSADOR_H

#include "Typedefs.h"
#include "Enums.h"
#include <string>

namespace rti1516e {

class FederateAmbassador {
public:
    virtual ~FederateAmbassador() = default;

    virtual void announceSynchronizationPoint(
        const std::wstring& label,
        const VariableLengthData& tag) {}

    virtual void federationSynchronized(
        const std::wstring& label) {}

    virtual void discoverObjectInstance(
        ObjectInstanceHandle theObject,
        ObjectClassHandle theObjectClass,
        const std::wstring& objectName) {}

    virtual void reflectAttributeValues(
        ObjectInstanceHandle theObject,
        const AttributeHandleValueMap& theAttributes,
        const VariableLengthData& userSuppliedTag,
        OrderType sentOrder,
        TransportationTypeHandle theTransport) {}

    virtual void removeObjectInstance(
        ObjectInstanceHandle theObject,
        const VariableLengthData& userSuppliedTag,
        OrderType sentOrder) {}

    virtual void timeAdvanceGrant(double theTime) {}
};

} // namespace rti1516e

#endif // RTI1516E_FEDERATEAMBASSADOR_H
