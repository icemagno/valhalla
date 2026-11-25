package hla.rti1516e;

import com.sun.jna.Memory;
import com.sun.jna.Pointer;
import com.sun.jna.ptr.ByteByReference;
import com.sun.jna.ptr.IntByReference;
import hla.rti1516e.exceptions.*;
import valhalla.client.IValhallaLrc;
import valhalla.client.ValhallaNativeLoader;

import java.util.Map;

public class ValhallaRtiAmbassador implements RTIambassador {

    private final IValhallaLrc nativeLrc;
    private final Pointer amb;
    private FederateAmbassador fedAmb;

    // Strong references to JNA callbacks to prevent garbage collection
    private IValhallaLrc.DiscoverObjectCallback discoverCb;
    private IValhallaLrc.RemoveObjectCallback removeCb;
    private IValhallaLrc.ReflectAttributesCallback reflectCb;
    private IValhallaLrc.TimeAdvanceGrantCallback grantCb;
    private IValhallaLrc.ValhallaCallbacks.ByValue callbacksStruct;

    public ValhallaRtiAmbassador() {
        this.nativeLrc = ValhallaNativeLoader.getInstance();
        this.amb = this.nativeLrc.valhalla_create_ambassador();
        if (this.amb == null) {
            throw new RuntimeException("Failed to create Valhalla C-ABI Ambassador handle");
        }
    }

    private void setupCallbacks() {
        this.discoverCb = (theObject, theObjectClass, objectName) -> {
            if (fedAmb != null) {
                try {
                    fedAmb.discoverObjectInstance(
                            new ObjectInstanceHandle(theObject),
                            new ObjectClassHandle(theObjectClass),
                            objectName
                    );
                } catch (Exception e) {
                    System.err.println("ValhallaRtiAmbassador: Error in discoverObjectInstance callback: " + e.getMessage());
                }
            }
        };

        this.removeCb = (theObject, tagPtr, tagLen) -> {
            if (fedAmb != null) {
                try {
                    byte[] tag = (tagPtr != null && tagLen > 0) ? tagPtr.getByteArray(0, tagLen) : new byte[0];
                    fedAmb.removeObjectInstance(
                            new ObjectInstanceHandle(theObject),
                            tag,
                            OrderType.RECEIVE
                    );
                } catch (Exception e) {
                    System.err.println("ValhallaRtiAmbassador: Error in removeObjectInstance callback: " + e.getMessage());
                }
            }
        };

        this.reflectCb = (theObject, attrCount, attrHandlesPtr, attrValuesPtr, attrLensPtr, tagPtr, tagLen) -> {
            if (fedAmb != null) {
                try {
                    AttributeHandleValueMap map = new AttributeHandleValueMap();
                    if (attrCount > 0 && attrHandlesPtr != null && attrValuesPtr != null && attrLensPtr != null) {
                        int[] handles = attrHandlesPtr.getIntArray(0, attrCount);
                        int[] lens = attrLensPtr.getIntArray(0, attrCount);
                        Pointer[] valPointers = attrValuesPtr.getPointerArray(0, attrCount);

                        for (int i = 0; i < attrCount; i++) {
                            byte[] val = (valPointers[i] != null && lens[i] > 0)
                                    ? valPointers[i].getByteArray(0, lens[i])
                                    : new byte[0];
                            map.put(new AttributeHandle(handles[i]), val);
                        }
                    }

                    byte[] tag = (tagPtr != null && tagLen > 0) ? tagPtr.getByteArray(0, tagLen) : new byte[0];
                    fedAmb.reflectAttributeValues(
                            new ObjectInstanceHandle(theObject),
                            map,
                            tag,
                            OrderType.RECEIVE,
                            TransportationTypeHandle.HLA_RELIABLE
                    );
                } catch (Exception e) {
                    System.err.println("ValhallaRtiAmbassador: Error in reflectAttributeValues callback: " + e.getMessage());
                }
            }
        };

        this.grantCb = (time) -> {
            if (fedAmb != null) {
                try {
                    fedAmb.timeAdvanceGrant(time);
                } catch (Exception e) {
                    System.err.println("ValhallaRtiAmbassador: Error in timeAdvanceGrant callback: " + e.getMessage());
                }
            }
        };

        this.callbacksStruct = new IValhallaLrc.ValhallaCallbacks.ByValue();
        this.callbacksStruct.onDiscoverObject = this.discoverCb;
        this.callbacksStruct.onRemoveObject = this.removeCb;
        this.callbacksStruct.onReflectAttributes = this.reflectCb;
        this.callbacksStruct.onTimeAdvanceGrant = this.grantCb;

        this.nativeLrc.valhalla_set_callbacks(this.amb, this.callbacksStruct);
    }

    private void checkStatus(int status, String operation) throws RTIexception {
        if (status == IValhallaLrc.VALHALLA_OK) return;
        switch (status) {
            case IValhallaLrc.VALHALLA_ERR_NOT_CONNECTED:
                throw new NotConnected("Not connected to Valhalla CRC during " + operation);
            case IValhallaLrc.VALHALLA_ERR_INVALID_ARG:
                throw new RTIinternalError("Invalid argument provided to " + operation);
            case IValhallaLrc.VALHALLA_ERR_NOT_OWNER:
                throw new AttributeNotOwned("Attribute is not owned by federate in " + operation);
            case IValhallaLrc.VALHALLA_ERR_ALREADY_OWNED:
                throw new RTIexception("Attribute is already owned in " + operation);
            default:
                throw new RTIinternalError("Valhalla RTI Error (" + status + ") during " + operation);
        }
    }

    @Override
    public void connect(FederateAmbassador federateReference, CallbackModel callbackModel, String localSettingsDesignator) throws RTIexception {
        this.fedAmb = federateReference;
        setupCallbacks();
        String address = (localSettingsDesignator != null && !localSettingsDesignator.trim().isEmpty())
                ? localSettingsDesignator.trim()
                : "127.0.0.1:4000";
        int rc = nativeLrc.valhalla_connect(amb, address);
        checkStatus(rc, "connect");
    }

    @Override
    public void disconnect() throws RTIexception {
        int rc = nativeLrc.valhalla_disconnect(amb);
        checkStatus(rc, "disconnect");
    }

    @Override
    public void createFederationExecution(String federationExecutionName) throws RTIexception {
        int rc = nativeLrc.valhalla_create_federation(amb, federationExecutionName);
        checkStatus(rc, "createFederationExecution");
    }

    @Override
    public void destroyFederationExecution(String federationExecutionName) throws RTIexception {
        int rc = nativeLrc.valhalla_destroy_federation(amb, federationExecutionName);
        checkStatus(rc, "destroyFederationExecution");
    }

    @Override
    public FederateHandle joinFederationExecution(String federateType, String federationExecutionName) throws RTIexception {
        IntByReference outHandle = new IntByReference();
        int rc = nativeLrc.valhalla_join_federation(amb, federateType, federationExecutionName, outHandle);
        checkStatus(rc, "joinFederationExecution");
        return new FederateHandle(outHandle.getValue());
    }

    @Override
    public void resignFederationExecution(ResignAction resignAction) throws RTIexception {
        int rc = nativeLrc.valhalla_resign_federation(amb);
        checkStatus(rc, "resignFederationExecution");
    }

    @Override
    public void publishObjectClassAttributes(ObjectClassHandle theClass, AttributeHandleSet attributeList) throws RTIexception {
        int[] handles = attributeList.stream().mapToInt(AttributeHandle::getValue).toArray();
        int rc = nativeLrc.valhalla_publish_object_class(amb, theClass.getValue(), handles, handles.length);
        checkStatus(rc, "publishObjectClassAttributes");
    }

    @Override
    public void subscribeObjectClassAttributes(ObjectClassHandle theClass, AttributeHandleSet attributeList) throws RTIexception {
        int[] handles = attributeList.stream().mapToInt(AttributeHandle::getValue).toArray();
        int rc = nativeLrc.valhalla_subscribe_object_class(amb, theClass.getValue(), handles, handles.length);
        checkStatus(rc, "subscribeObjectClassAttributes");
    }

    @Override
    public ObjectInstanceHandle registerObjectInstance(ObjectClassHandle theClass, String theObjectName) throws RTIexception {
        IntByReference outHandle = new IntByReference();
        int rc = nativeLrc.valhalla_register_object(amb, theClass.getValue(), theObjectName, outHandle);
        checkStatus(rc, "registerObjectInstance");
        return new ObjectInstanceHandle(outHandle.getValue());
    }

    @Override
    public void deleteObjectInstance(ObjectInstanceHandle theObject, byte[] userSuppliedTag) throws RTIexception {
        byte[] tag = userSuppliedTag != null ? userSuppliedTag : new byte[0];
        int rc = nativeLrc.valhalla_delete_object(amb, theObject.getValue(), tag, tag.length);
        checkStatus(rc, "deleteObjectInstance");
    }

    @Override
    public void updateAttributeValues(ObjectInstanceHandle theObject, AttributeHandleValueMap theAttributes, byte[] userSuppliedTag) throws RTIexception {
        int count = theAttributes.size();
        int[] handles = new int[count];
        int[] lens = new int[count];
        Pointer[] ptrs = new Pointer[count];

        int idx = 0;
        for (Map.Entry<AttributeHandle, byte[]> entry : theAttributes.entrySet()) {
            handles[idx] = entry.getKey().getValue();
            byte[] val = entry.getValue() != null ? entry.getValue() : new byte[0];
            lens[idx] = val.length;
            if (val.length > 0) {
                Memory mem = new Memory(val.length);
                mem.write(0, val, 0, val.length);
                ptrs[idx] = mem;
            } else {
                ptrs[idx] = Pointer.NULL;
            }
            idx++;
        }

        byte[] tag = userSuppliedTag != null ? userSuppliedTag : new byte[0];
        int rc = nativeLrc.valhalla_update_attributes(amb, theObject.getValue(), handles, ptrs, lens, count, tag, tag.length);
        checkStatus(rc, "updateAttributeValues");
    }

    @Override
    public void enableTimeRegulation(double lookahead) throws RTIexception {
        int rc = nativeLrc.valhalla_enable_time_regulation(amb, lookahead);
        checkStatus(rc, "enableTimeRegulation");
    }

    @Override
    public void enableTimeConstrained() throws RTIexception {
        int rc = nativeLrc.valhalla_enable_time_constrained(amb);
        checkStatus(rc, "enableTimeConstrained");
    }

    @Override
    public void timeAdvanceRequest(double theTime) throws RTIexception {
        int rc = nativeLrc.valhalla_time_advance_request(amb, theTime);
        checkStatus(rc, "timeAdvanceRequest");
    }

    @Override
    public void nextMessageRequest(double theTime) throws RTIexception {
        int rc = nativeLrc.valhalla_next_message_request(amb, theTime);
        checkStatus(rc, "nextMessageRequest");
    }

    @Override
    public void unconditionalAttributeOwnershipDivestiture(ObjectInstanceHandle theObject, AttributeHandleSet theAttributes) throws RTIexception {
        int[] handles = theAttributes.stream().mapToInt(AttributeHandle::getValue).toArray();
        int rc = nativeLrc.valhalla_divest_ownership(amb, theObject.getValue(), handles, handles.length);
        checkStatus(rc, "unconditionalAttributeOwnershipDivestiture");
    }

    @Override
    public void attributeOwnershipAcquisition(ObjectInstanceHandle theObject, AttributeHandleSet desiredAttributes, byte[] userSuppliedTag) throws RTIexception {
        int[] handles = desiredAttributes.stream().mapToInt(AttributeHandle::getValue).toArray();
        int rc = nativeLrc.valhalla_acquire_ownership(amb, theObject.getValue(), handles, handles.length);
        checkStatus(rc, "attributeOwnershipAcquisition");
    }

    @Override
    public boolean isAttributeOwnedByFederate(ObjectInstanceHandle theObject, AttributeHandle theAttribute) throws RTIexception {
        IntByReference outOwner = new IntByReference();
        ByteByReference outIsOwned = new ByteByReference();
        int rc = nativeLrc.valhalla_query_ownership(amb, theObject.getValue(), theAttribute.getValue(), outOwner, outIsOwned);
        checkStatus(rc, "isAttributeOwnedByFederate");
        return outIsOwned.getValue() != 0;
    }

    @Override
    public RegionHandle createRegion(DimensionHandle[] dimensions) throws RTIexception {
        int[] dims = new int[dimensions.length];
        for (int i = 0; i < dimensions.length; i++) {
            dims[i] = dimensions[i].getValue();
        }
        IntByReference outRegion = new IntByReference();
        int rc = nativeLrc.valhalla_create_region(amb, dims, dims.length, outRegion);
        checkStatus(rc, "createRegion");
        return new RegionHandle(outRegion.getValue());
    }

    @Override
    public void setRangeBounds(RegionHandle region, DimensionHandle dimension, RangeBounds bounds) throws RTIexception {
        int rc = nativeLrc.valhalla_set_range_bounds(amb, region.getValue(), dimension.getValue(), (int) bounds.lower, (int) bounds.upper);
        checkStatus(rc, "setRangeBounds");
    }

    @Override
    public void setMetricBounds(RegionHandle region, double minX, double maxX, double minY, double maxY, double minZ, double maxZ) throws RTIexception {
        int rc = nativeLrc.valhalla_set_metric_bounds(amb, region.getValue(), minX, maxX, minY, maxY, minZ, maxZ);
        checkStatus(rc, "setMetricBounds");
    }

    @Override
    public void subscribeObjectClassAttributesWithRegions(ObjectClassHandle theClass, AttributeHandleSet attributeList, RegionHandle region) throws RTIexception {
        int[] handles = attributeList.stream().mapToInt(AttributeHandle::getValue).toArray();
        int rc = nativeLrc.valhalla_subscribe_with_region(amb, theClass.getValue(), handles, handles.length, region.getValue());
        checkStatus(rc, "subscribeObjectClassAttributesWithRegions");
    }

    @Override
    public void updateAttributeValuesSpatial(ObjectInstanceHandle theObject, AttributeHandleValueMap theAttributes, double x, double y, double z, byte[] userSuppliedTag) throws RTIexception {
        int count = theAttributes.size();
        int[] handles = new int[count];
        int[] lens = new int[count];
        Pointer[] ptrs = new Pointer[count];

        int idx = 0;
        for (Map.Entry<AttributeHandle, byte[]> entry : theAttributes.entrySet()) {
            handles[idx] = entry.getKey().getValue();
            byte[] val = entry.getValue() != null ? entry.getValue() : new byte[0];
            lens[idx] = val.length;
            if (val.length > 0) {
                Memory mem = new Memory(val.length);
                mem.write(0, val, 0, val.length);
                ptrs[idx] = mem;
            } else {
                ptrs[idx] = Pointer.NULL;
            }
            idx++;
        }

        byte[] tag = userSuppliedTag != null ? userSuppliedTag : new byte[0];
        int rc = nativeLrc.valhalla_update_attributes_spatial(amb, theObject.getValue(), handles, ptrs, lens, count, x, y, z, tag, tag.length);
        checkStatus(rc, "updateAttributeValuesSpatial");
    }

    @Override
    public boolean evokeCallback(double approximateMinimumTimeInSeconds) throws RTIexception {
        ByteByReference outEvoked = new ByteByReference();
        int rc = nativeLrc.valhalla_evoke_callback(amb, approximateMinimumTimeInSeconds, outEvoked);
        checkStatus(rc, "evokeCallback");
        return outEvoked.getValue() != 0;
    }

    @Override
    public boolean evokeMultipleCallbacks(double minimumTime, double maximumTime) throws RTIexception {
        long start = System.currentTimeMillis();
        long maxDurationMs = (long) (maximumTime * 1000.0);
        boolean any = false;

        while (true) {
            boolean evoked = evokeCallback(0.01);
            if (evoked) any = true;
            if (System.currentTimeMillis() - start >= maxDurationMs) {
                break;
            }
            if (!evoked) {
                try {
                    Thread.sleep(5);
                } catch (InterruptedException ignored) {}
            }
        }
        return any;
    }
}
