package hla.rti1516e;

import hla.rti1516e.exceptions.RTIinternalError;

public interface RtiFactory {
    RTIambassador getRtiAmbassador() throws RTIinternalError;
}
