// RTIambassadorFactory.h - Official IEEE 1516e RTIambassador Factory
#ifndef RTI1516E_RTIAMBASSADORFACTORY_H
#define RTI1516E_RTIAMBASSADORFACTORY_H

#include <rti1516e/RTIambassador.h>
#include <memory>

namespace rti1516e {

class RTIambassadorFactory {
public:
    RTIambassadorFactory();
    virtual ~RTIambassadorFactory() throw();

    virtual std::unique_ptr<RTIambassador> createRTIambassador()
        throw(RTIinternalError);
};

} // namespace rti1516e

#endif // RTI1516E_RTIAMBASSADORFACTORY_H
