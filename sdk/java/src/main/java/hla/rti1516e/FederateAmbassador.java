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

    default void receiveInteraction(int interactionClass,
                                    java.util.Map<Integer, byte[]> theParameters,
                                    byte[] userSuppliedTag,
                                    OrderType sentOrdering,
                                    TransportationTypeHandle theTransport) throws RTIexception {}

    default void announceSynchronizationPoint(String label, byte[] userSuppliedTag) throws RTIexception {}

    default void federationSynchronized(String label) throws RTIexception {}

    default void synchronizationPointRegistrationSucceeded(String label) throws RTIexception {}

    default void synchronizationPointRegistrationFailed(String label, String reason) throws RTIexception {}
}
