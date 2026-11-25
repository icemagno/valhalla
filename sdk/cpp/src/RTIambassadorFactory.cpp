// RTIambassadorFactory.cpp - Factory implementation creating ValhallaRTIambassador
#include <rti1516e/RTIambassadorFactory.h>
#include "ValhallaRTIambassador.h"

namespace rti1516e {

RTIambassadorFactory::RTIambassadorFactory() {}

RTIambassadorFactory::~RTIambassadorFactory() throw() {}

std::unique_ptr<RTIambassador> RTIambassadorFactory::createRTIambassador()
    throw(RTIinternalError) {
    return std::unique_ptr<RTIambassador>(new ValhallaRTIambassador());
}

} // namespace rti1516e
