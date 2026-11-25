package hla.rti1516e;

import hla.rti1516e.exceptions.FederateInternalError;
import hla.rti1516e.exceptions.RTIexception;

public interface FederateAmbassador {
    default void discoverObjectInstance(ObjectInstanceHandle theObject,
                                        ObjectClassHandle theObjectClass,
                                        String objectName) throws RTIexception {}

    default void removeObjectInstance(ObjectInstanceHandle theObject,
                                      byte[] userSuppliedTag,
                                      OrderType sentOrdering) throws RTIexception {}

    default void reflectAttributeValues(ObjectInstanceHandle theObject,
                                        AttributeHandleValueMap theAttributes,
                                        byte[] userSuppliedTag,
                                        OrderType sentOrdering,
                                        TransportationTypeHandle theTransport) throws RTIexception {}

    default void timeAdvanceGrant(double theTime) throws RTIexception {}
}
