package hla.rti1516e;

import hla.rti1516e.exceptions.RTIinternalError;

public class RtiFactoryFactory {

    public static RtiFactory getRtiFactory() throws RTIinternalError {
        return new ValhallaRtiFactory();
    }

    public static RtiFactory getRtiFactory(String name) throws RTIinternalError {
        return new ValhallaRtiFactory();
    }
}
