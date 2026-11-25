package hla.rti1516e;

import hla.rti1516e.exceptions.*;

public interface RTIambassador {

    void connect(FederateAmbassador federateReference,
                 CallbackModel callbackModel,
                 String localSettingsDesignator) throws RTIexception;

    void disconnect() throws RTIexception;

    void createFederationExecution(String federationExecutionName) throws RTIexception;

    void createFederationExecution(String federationExecutionName, java.net.URL fomModule) throws RTIexception;

    void createFederationExecution(String federationExecutionName, java.net.URL[] fomModules) throws RTIexception;

    void destroyFederationExecution(String federationExecutionName) throws RTIexception;

    FederateHandle joinFederationExecution(String federateType,
                                          String federationExecutionName) throws RTIexception;

    void resignFederationExecution(ResignAction resignAction) throws RTIexception;

    // FOM / OMT Introspection
    ObjectClassHandle getObjectClassHandle(String theName) throws RTIexception;

    AttributeHandle getAttributeHandle(ObjectClassHandle whichClass, String theName) throws RTIexception;

    DimensionHandle getDimensionHandle(String theName) throws RTIexception;

    void publishObjectClassAttributes(ObjectClassHandle theClass,
                                      AttributeHandleSet attributeList) throws RTIexception;

    void subscribeObjectClassAttributes(ObjectClassHandle theClass,
                                        AttributeHandleSet attributeList) throws RTIexception;

    ObjectInstanceHandle registerObjectInstance(ObjectClassHandle theClass,
                                               String theObjectName) throws RTIexception;

    void deleteObjectInstance(ObjectInstanceHandle theObject,
                              byte[] userSuppliedTag) throws RTIexception;

    void updateAttributeValues(ObjectInstanceHandle theObject,
                               AttributeHandleValueMap theAttributes,
                               byte[] userSuppliedTag) throws RTIexception;

    void enableTimeRegulation(double lookahead) throws RTIexception;

    void enableTimeConstrained() throws RTIexception;

    void timeAdvanceRequest(double theTime) throws RTIexception;

    void nextMessageRequest(double theTime) throws RTIexception;

    // Ownership Management
    void unconditionalAttributeOwnershipDivestiture(ObjectInstanceHandle theObject,
                                                   AttributeHandleSet theAttributes) throws RTIexception;

    void attributeOwnershipAcquisition(ObjectInstanceHandle theObject,
                                      AttributeHandleSet desiredAttributes,
                                      byte[] userSuppliedTag) throws RTIexception;

    boolean isAttributeOwnedByFederate(ObjectInstanceHandle theObject,
                                       AttributeHandle theAttribute) throws RTIexception;

    // Data Distribution Management (DDM)
    RegionHandle createRegion(DimensionHandle[] dimensions) throws RTIexception;

    void setRangeBounds(RegionHandle region, DimensionHandle dimension, RangeBounds bounds) throws RTIexception;

    void setMetricBounds(RegionHandle region, double minX, double maxX, double minY, double maxY, double minZ, double maxZ) throws RTIexception;

    void subscribeObjectClassAttributesWithRegions(ObjectClassHandle theClass,
                                                  AttributeHandleSet attributeList,
                                                  RegionHandle region) throws RTIexception;

    void updateAttributeValuesSpatial(ObjectInstanceHandle theObject,
                                      AttributeHandleValueMap theAttributes,
                                      double x, double y, double z,
                                      byte[] userSuppliedTag) throws RTIexception;

    // Callback Evocation
    boolean evokeCallback(double approximateMinimumTimeInSeconds) throws RTIexception;

    boolean evokeMultipleCallbacks(double minimumTime, double maximumTime) throws RTIexception;
}
