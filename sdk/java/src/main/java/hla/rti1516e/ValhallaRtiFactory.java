package hla.rti1516e;

import hla.rti1516e.exceptions.RTIinternalError;

public class ValhallaRtiFactory implements RtiFactory {
    @Override
    public RTIambassador getRtiAmbassador() throws RTIinternalError {
        try {
            return new ValhallaRtiAmbassador();
        } catch (Exception e) {
            throw new RTIinternalError("Failed to initialize Valhalla RTI Ambassador: " + e.getMessage(), e);
        }
    }
}
