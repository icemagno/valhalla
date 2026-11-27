package hla.rti1516e;

import com.sun.jna.Memory;
import com.sun.jna.Pointer;
import com.sun.jna.ptr.ByteByReference;
import com.sun.jna.ptr.IntByReference;
import hla.rti1516e.exceptions.*;
import valhalla.client.IValhallaLrc;
import valhalla.client.ValhallaNativeLoader;

import java.lang.ref.Cleaner;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicBoolean;

public class ValhallaRtiAmbassador implements RTIambassador, AutoCloseable {

    private static final Cleaner CLEANER = Cleaner.create();

    private final IValhallaLrc nativeLrc;
    private final Pointer amb;
    private final AtomicBoolean destroyed = new AtomicBoolean(false);
    private final Cleaner.Cleanable cleanable;
    private volatile int localFederateHandle = 0;
    private FederateAmbassador fedAmb;

    // Strong references to JNA callbacks to prevent garbage collection
    private IValhallaLrc.DiscoverObjectCallback discoverCb;
    private IValhallaLrc.RemoveObjectCallback removeCb;
    private IValhallaLrc.ReflectAttributesCallback reflectCb;
    private IValhallaLrc.TimeAdvanceGrantCallback grantCb;
    private IValhallaLrc.ReceiveInteractionCallback interactionCb;
    private IValhallaLrc.AnnounceSynchronizationPointCallback announceSyncCb;
    private IValhallaLrc.FederationSynchronizedCallback fedSyncCb;
    private IValhallaLrc.SynchronizationPointRegistrationSucceededCallback syncRegSuccCb;
    private IValhallaLrc.SynchronizationPointRegistrationFailedCallback syncRegFailCb;
    private IValhallaLrc.ValhallaCallbacks.ByValue callbacksStruct;

    public ValhallaRtiAmbassador() {
        this.nativeLrc = ValhallaNativeLoader.getInstance();
        this.amb = this.nativeLrc.valhalla_create_ambassador();
        if (this.amb == null) {
            throw new RuntimeException("Failed to create Valhalla C-ABI Ambassador handle");
        }
        final IValhallaLrc lrcRef = this.nativeLrc;
        final Pointer ambRef = this.amb;
        final AtomicBoolean destroyedRef = this.destroyed;
        this.cleanable = CLEANER.register(this, () -> {
            if (destroyedRef.compareAndSet(false, true)) {
                lrcRef.valhalla_destroy_ambassador(ambRef);
            }
        });
    }

    @Override
    public void close() {
        this.localFederateHandle = 0;
        this.cleanable.clean();
    }

    private void ensureNotDestroyed(String operation) throws RTIexception {
        if (destroyed.get()) {
            throw new RTIinternalError("ValhallaRtiAmbassador has already been closed/destroyed before " + operation);
        }
    }

    private void setupCallbacks() {
        this.discoverCb = (theObject, theObjectClass, objectName, userData) -> {
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

        this.removeCb = (theObject, tagPtr, tagLen, userData) -> {
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

        this.reflectCb = (theObject, attrCount, attrHandlesPtr, attrValuesPtr, attrLensPtr, tagPtr, tagLen, userData) -> {
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

        this.grantCb = (time, userData) -> {
            if (fedAmb != null) {
                try {
                    fedAmb.timeAdvanceGrant(time);
                } catch (Exception e) {
                    System.err.println("ValhallaRtiAmbassador: Error in timeAdvanceGrant callback: " + e.getMessage());
                }
            }
        };

        this.interactionCb = (interactionClass, paramCount, paramHandles, paramValues, paramLens, tag, tagLen, userData) -> {
            if (fedAmb != null) {
                try {
                    java.util.Map<Integer, byte[]> paramMap = new java.util.HashMap<>();
                    if (paramCount > 0 && paramHandles != null && paramValues != null && paramLens != null) {
                        int[] handles = paramHandles.getIntArray(0, paramCount);
                        Pointer[] valPtrs = paramValues.getPointerArray(0, paramCount);
                        int[] lens = paramLens.getIntArray(0, paramCount);
                        for (int i = 0; i < paramCount; i++) {
                            byte[] val = (lens[i] > 0 && valPtrs[i] != null) ? valPtrs[i].getByteArray(0, lens[i]) : new byte[0];
                            paramMap.put(handles[i], val);
                        }
                    }
                    byte[] tagBytes = (tag != null && tagLen > 0) ? tag.getByteArray(0, tagLen) : new byte[0];
                    fedAmb.receiveInteraction(interactionClass, paramMap, tagBytes, OrderType.RECEIVE, TransportationTypeHandle.HLA_RELIABLE);
                } catch (Exception e) {
                    System.err.println("ValhallaRtiAmbassador: Error in receiveInteraction callback: " + e.getMessage());
                }
            }
        };

        this.announceSyncCb = (label, tag, tagLen, userData) -> {
            if (fedAmb != null) {
                try {
                    byte[] tagBytes = (tag != null && tagLen > 0) ? tag.getByteArray(0, tagLen) : new byte[0];
                    fedAmb.announceSynchronizationPoint(label, tagBytes);
                } catch (Exception e) {
                    System.err.println("ValhallaRtiAmbassador: Error in announceSynchronizationPoint callback: " + e.getMessage());
                }
            }
        };

        this.fedSyncCb = (label, userData) -> {
            if (fedAmb != null) {
                try {
                    fedAmb.federationSynchronized(label);
                } catch (Exception e) {
                    System.err.println("ValhallaRtiAmbassador: Error in federationSynchronized callback: " + e.getMessage());
                }
            }
        };

        this.syncRegSuccCb = (label, userData) -> {
            if (fedAmb != null) {
                try {
                    fedAmb.synchronizationPointRegistrationSucceeded(label);
                } catch (Exception e) {
                    System.err.println("ValhallaRtiAmbassador: Error in synchronizationPointRegistrationSucceeded callback: " + e.getMessage());
                }
            }
        };

        this.syncRegFailCb = (label, reason, userData) -> {
            if (fedAmb != null) {
                try {
                    fedAmb.synchronizationPointRegistrationFailed(label, reason);
                } catch (Exception e) {
                    System.err.println("ValhallaRtiAmbassador: Error in synchronizationPointRegistrationFailed callback: " + e.getMessage());
                }
            }
        };

        this.callbacksStruct = new IValhallaLrc.ValhallaCallbacks.ByValue();
        this.callbacksStruct.onDiscoverObject = this.discoverCb;
        this.callbacksStruct.onRemoveObject = this.removeCb;
        this.callbacksStruct.onReflectAttributes = this.reflectCb;
        this.callbacksStruct.onTimeAdvanceGrant = this.grantCb;
        this.callbacksStruct.onReceiveInteraction = this.interactionCb;
        this.callbacksStruct.onAnnounceSynchronizationPoint = this.announceSyncCb;
        this.callbacksStruct.onFederationSynchronized = this.fedSyncCb;
        this.callbacksStruct.onSyncPointRegistrationSucceeded = this.syncRegSuccCb;
        this.callbacksStruct.onSyncPointRegistrationFailed = this.syncRegFailCb;

        int rc = this.nativeLrc.valhalla_set_callbacks(this.amb, this.callbacksStruct);
        if (rc != IValhallaLrc.VALHALLA_OK) {
            throw new RuntimeException("Failed to register Valhalla C-ABI callbacks (status=" + rc + ")");
        }
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
            case IValhallaLrc.VALHALLA_ERR_FEDERATION_EXISTS:
                throw new FederationExecutionAlreadyExists("Federation execution already exists in " + operation);
            case IValhallaLrc.VALHALLA_ERR_FEDERATION_NOT_FOUND:
                throw new FederationExecutionDoesNotExist("Federation execution does not exist in " + operation);
            case IValhallaLrc.VALHALLA_ERR_CLASS_NOT_FOUND:
                throw new ObjectClassNotDefined("Object class is not defined in " + operation);
            case IValhallaLrc.VALHALLA_ERR_ATTR_NOT_FOUND:
                throw new AttributeNotDefined("Attribute is not defined in " + operation);
            case IValhallaLrc.VALHALLA_ERR_OBJECT_NOT_FOUND:
                throw new ObjectInstanceNotKnown("Object instance is not known in " + operation);
            case IValhallaLrc.VALHALLA_ERR_NOT_MEMBER:
                throw new FederateNotExecutionMember("Federate is not execution member in " + operation);
            case IValhallaLrc.VALHALLA_ERR_FEDERATES_JOINED:
                throw new FederatesCurrentlyJoined("Federates are currently joined in " + operation);
            case IValhallaLrc.VALHALLA_ERR_FEDERATE_NAME_IN_USE:
                throw new FederateNameAlreadyInUse("Federate name already in use in " + operation);
            case IValhallaLrc.VALHALLA_ERR_INTERACTION_NOT_FOUND:
                throw new InteractionClassNotDefined("Interaction class is not defined in " + operation);
            case IValhallaLrc.VALHALLA_ERR_ALREADY_MEMBER:
                throw new RTIexception("Federate is already execution member in " + operation);
            case IValhallaLrc.VALHALLA_ERR_OBJECT_NAME_IN_USE:
                throw new RTIexception("Object instance name is already in use in " + operation);
            case IValhallaLrc.VALHALLA_ERR_DELETE_PRIVILEGE_NOT_HELD:
                throw new RTIexception("Delete privilege not held in " + operation);
            default:
                throw new RTIinternalError("Valhalla RTI Error (" + status + ") during " + operation);
        }
    }

    @Override
    public void connect(FederateAmbassador federateReference, CallbackModel callbackModel, String localSettingsDesignator) throws RTIexception {
        ensureNotDestroyed("connect");
        this.fedAmb = federateReference;
        setupCallbacks();
        String address = (localSettingsDesignator != null && !localSettingsDesignator.trim().isEmpty())
                ? localSettingsDesignator.trim()
                : "127.0.0.1:52295";
        int rc = nativeLrc.valhalla_connect(amb, address);
        checkStatus(rc, "connect");
    }

    @Override
    public void disconnect() throws RTIexception {
        ensureNotDestroyed("disconnect");
        int rc = nativeLrc.valhalla_disconnect(amb);
        checkStatus(rc, "disconnect");
        this.localFederateHandle = 0;
    }

    @Override
    public void createFederationExecution(String federationExecutionName) throws RTIexception {
        ensureNotDestroyed("createFederationExecution");
        int rc = nativeLrc.valhalla_create_federation(amb, federationExecutionName);
        checkStatus(rc, "createFederationExecution");
    }

    @Override
    public void createFederationExecution(String federationExecutionName, java.net.URL fomModule) throws RTIexception {
        ensureNotDestroyed("createFederationExecution");
        if (fomModule == null) {
            createFederationExecution(federationExecutionName);
            return;
        }
        createFederationExecution(federationExecutionName, new java.net.URL[]{fomModule});
    }

    @Override
    public void createFederationExecution(String federationExecutionName, java.net.URL[] fomModules) throws RTIexception {
        ensureNotDestroyed("createFederationExecutionWithFOM");
        if (fomModules == null || fomModules.length == 0) {
            createFederationExecution(federationExecutionName);
            return;
        }
        String[] foms = new String[fomModules.length];
        for (int i = 0; i < fomModules.length; i++) {
            foms[i] = fomModules[i].toString();
        }
        int rc = nativeLrc.valhalla_create_federation_with_fom(amb, federationExecutionName, foms, foms.length);
        checkStatus(rc, "createFederationExecutionWithFOM");
    }

    @Override
    public ObjectClassHandle getObjectClassHandle(String theName) throws RTIexception {
        ensureNotDestroyed("getObjectClassHandle");
        IntByReference outHandle = new IntByReference();
        int rc = nativeLrc.valhalla_get_object_class_handle(amb, theName, outHandle);
        checkStatus(rc, "getObjectClassHandle");
        return new ObjectClassHandle(outHandle.getValue());
    }

    @Override
    public AttributeHandle getAttributeHandle(ObjectClassHandle whichClass, String theName) throws RTIexception {
        ensureNotDestroyed("getAttributeHandle");
        IntByReference outHandle = new IntByReference();
        int rc = nativeLrc.valhalla_get_attribute_handle(amb, whichClass.getValue(), theName, outHandle);
        checkStatus(rc, "getAttributeHandle");
        return new AttributeHandle(outHandle.getValue());
    }

    @Override
    public DimensionHandle getDimensionHandle(String theName) throws RTIexception {
        ensureNotDestroyed("getDimensionHandle");
        IntByReference outHandle = new IntByReference();
        int rc = nativeLrc.valhalla_get_dimension_handle(amb, theName, outHandle);
        checkStatus(rc, "getDimensionHandle");
        return new DimensionHandle(outHandle.getValue());
    }

    @Override
    public void destroyFederationExecution(String federationExecutionName) throws RTIexception {
        ensureNotDestroyed("destroyFederationExecution");
        int rc = nativeLrc.valhalla_destroy_federation(amb, federationExecutionName);
        checkStatus(rc, "destroyFederationExecution");
    }

    @Override
    public FederateHandle joinFederationExecution(String federateType, String federationExecutionName) throws RTIexception {
        ensureNotDestroyed("joinFederationExecution");
        IntByReference outHandle = new IntByReference();
        int rc = nativeLrc.valhalla_join_federation(amb, federateType, federationExecutionName, outHandle);
        checkStatus(rc, "joinFederationExecution");
        this.localFederateHandle = outHandle.getValue();
        return new FederateHandle(this.localFederateHandle);
    }

    @Override
    public void resignFederationExecution(ResignAction resignAction) throws RTIexception {
        ensureNotDestroyed("resignFederationExecution");
        int rc = nativeLrc.valhalla_resign_federation(amb);
        checkStatus(rc, "resignFederationExecution");
        this.localFederateHandle = 0;
    }

    @Override
    public void publishObjectClassAttributes(ObjectClassHandle theClass, AttributeHandleSet attributeList) throws RTIexception {
        ensureNotDestroyed("publishObjectClassAttributes");
        int[] handles = attributeList.stream().mapToInt(AttributeHandle::getValue).toArray();
        int rc = nativeLrc.valhalla_publish_object_class(amb, theClass.getValue(), handles, handles.length);
        checkStatus(rc, "publishObjectClassAttributes");
    }

    @Override
    public void subscribeObjectClassAttributes(ObjectClassHandle theClass, AttributeHandleSet attributeList) throws RTIexception {
        ensureNotDestroyed("subscribeObjectClassAttributes");
        int[] handles = attributeList.stream().mapToInt(AttributeHandle::getValue).toArray();
        int rc = nativeLrc.valhalla_subscribe_object_class(amb, theClass.getValue(), handles, handles.length);
        checkStatus(rc, "subscribeObjectClassAttributes");
    }

    @Override
    public void publishInteractionClass(int interactionClass) throws RTIexception {
        ensureNotDestroyed("publishInteractionClass");
        int rc = nativeLrc.valhalla_publish_interaction_class(amb, interactionClass);
        checkStatus(rc, "publishInteractionClass");
    }

    @Override
    public void subscribeInteractionClass(int interactionClass) throws RTIexception {
        ensureNotDestroyed("subscribeInteractionClass");
        int rc = nativeLrc.valhalla_subscribe_interaction_class(amb, interactionClass);
        checkStatus(rc, "subscribeInteractionClass");
    }

    @Override
    public void sendInteraction(int interactionClass, Map<Integer, byte[]> theParameters, byte[] userSuppliedTag) throws RTIexception {
        ensureNotDestroyed("sendInteraction");
        int count = theParameters != null ? theParameters.size() : 0;
        int[] handles = new int[count];
        int[] lens = new int[count];
        Pointer[] ptrs = new Pointer[count];
        List<Memory> allocated = new ArrayList<>(count);

        try {
            if (theParameters != null) {
                int idx = 0;
                for (Map.Entry<Integer, byte[]> entry : theParameters.entrySet()) {
                    handles[idx] = entry.getKey();
                    byte[] val = entry.getValue() != null ? entry.getValue() : new byte[0];
                    lens[idx] = val.length;
                    if (val.length > 0) {
                        Memory mem = new Memory(val.length);
                        mem.write(0, val, 0, val.length);
                        allocated.add(mem);
                        ptrs[idx] = mem;
                    } else {
                        ptrs[idx] = Pointer.NULL;
                    }
                    idx++;
                }
            }

            byte[] tag = userSuppliedTag != null ? userSuppliedTag : new byte[0];
            int rc = nativeLrc.valhalla_send_interaction(amb, interactionClass, handles, ptrs, lens, count, tag, tag.length);
            checkStatus(rc, "sendInteraction");
        } finally {
            for (Memory mem : allocated) {
                mem.close();
            }
        }
    }

    @Override
    public void registerFederationSynchronizationPoint(String label, byte[] userSuppliedTag) throws RTIexception {
        ensureNotDestroyed("registerFederationSynchronizationPoint");
        byte[] tag = userSuppliedTag != null ? userSuppliedTag : new byte[0];
        int rc = nativeLrc.valhalla_register_federation_synchronization_point(amb, label, tag, tag.length);
        checkStatus(rc, "registerFederationSynchronizationPoint");
    }

    @Override
    public void synchronizationPointAchieved(String label) throws RTIexception {
        ensureNotDestroyed("synchronizationPointAchieved");
        int rc = nativeLrc.valhalla_synchronization_point_achieved(amb, label);
        checkStatus(rc, "synchronizationPointAchieved");
    }

    @Override
    public ObjectInstanceHandle registerObjectInstance(ObjectClassHandle theClass, String theObjectName) throws RTIexception {
        ensureNotDestroyed("registerObjectInstance");
        IntByReference outHandle = new IntByReference();
        int rc = nativeLrc.valhalla_register_object(amb, theClass.getValue(), theObjectName, outHandle);
        checkStatus(rc, "registerObjectInstance");
        return new ObjectInstanceHandle(outHandle.getValue());
    }

    @Override
    public void deleteObjectInstance(ObjectInstanceHandle theObject, byte[] userSuppliedTag) throws RTIexception {
        ensureNotDestroyed("deleteObjectInstance");
        byte[] tag = userSuppliedTag != null ? userSuppliedTag : new byte[0];
        int rc = nativeLrc.valhalla_delete_object(amb, theObject.getValue(), tag, tag.length);
        checkStatus(rc, "deleteObjectInstance");
    }

    @Override
    public void updateAttributeValues(ObjectInstanceHandle theObject, AttributeHandleValueMap theAttributes, byte[] userSuppliedTag) throws RTIexception {
        ensureNotDestroyed("updateAttributeValues");
        int count = theAttributes.size();
        int[] handles = new int[count];
        int[] lens = new int[count];
        Pointer[] ptrs = new Pointer[count];
        List<Memory> allocated = new ArrayList<>(count);

        try {
            int idx = 0;
            for (Map.Entry<AttributeHandle, byte[]> entry : theAttributes.entrySet()) {
                handles[idx] = entry.getKey().getValue();
                byte[] val = entry.getValue() != null ? entry.getValue() : new byte[0];
                lens[idx] = val.length;
                if (val.length > 0) {
                    Memory mem = new Memory(val.length);
                    mem.write(0, val, 0, val.length);
                    allocated.add(mem);
                    ptrs[idx] = mem;
                } else {
                    ptrs[idx] = Pointer.NULL;
                }
                idx++;
            }

            byte[] tag = userSuppliedTag != null ? userSuppliedTag : new byte[0];
            int rc = nativeLrc.valhalla_update_attributes(amb, theObject.getValue(), handles, ptrs, lens, count, tag, tag.length);
            checkStatus(rc, "updateAttributeValues");
        } finally {
            for (Memory mem : allocated) {
                mem.close();
            }
        }
    }

    @Override
    public void enableTimeRegulation(double lookahead) throws RTIexception {
        ensureNotDestroyed("enableTimeRegulation");
        int rc = nativeLrc.valhalla_enable_time_regulation(amb, lookahead);
        checkStatus(rc, "enableTimeRegulation");
    }

    @Override
    public void disableTimeRegulation() throws RTIexception {
        ensureNotDestroyed("disableTimeRegulation");
        int rc = nativeLrc.valhalla_disable_time_regulation(amb);
        checkStatus(rc, "disableTimeRegulation");
    }

    @Override
    public void enableTimeConstrained() throws RTIexception {
        ensureNotDestroyed("enableTimeConstrained");
        int rc = nativeLrc.valhalla_enable_time_constrained(amb);
        checkStatus(rc, "enableTimeConstrained");
    }

    @Override
    public void disableTimeConstrained() throws RTIexception {
        ensureNotDestroyed("disableTimeConstrained");
        int rc = nativeLrc.valhalla_disable_time_constrained(amb);
        checkStatus(rc, "disableTimeConstrained");
    }

    @Override
    public void timeAdvanceRequest(double theTime) throws RTIexception {
        ensureNotDestroyed("timeAdvanceRequest");
        int rc = nativeLrc.valhalla_time_advance_request(amb, theTime);
        checkStatus(rc, "timeAdvanceRequest");
    }

    @Override
    public void nextMessageRequest(double theTime) throws RTIexception {
        ensureNotDestroyed("nextMessageRequest");
        int rc = nativeLrc.valhalla_next_message_request(amb, theTime);
        checkStatus(rc, "nextMessageRequest");
    }

    @Override
    public void flushQueueRequest(double theTime) throws RTIexception {
        ensureNotDestroyed("flushQueueRequest");
        int rc = nativeLrc.valhalla_flush_queue_request(amb, theTime);
        checkStatus(rc, "flushQueueRequest");
    }

    @Override
    public void unconditionalAttributeOwnershipDivestiture(ObjectInstanceHandle theObject, AttributeHandleSet theAttributes) throws RTIexception {
        ensureNotDestroyed("unconditionalAttributeOwnershipDivestiture");
        int[] handles = theAttributes.stream().mapToInt(AttributeHandle::getValue).toArray();
        int rc = nativeLrc.valhalla_divest_ownership(amb, theObject.getValue(), handles, handles.length);
        checkStatus(rc, "unconditionalAttributeOwnershipDivestiture");
    }

    @Override
    public void attributeOwnershipAcquisition(ObjectInstanceHandle theObject, AttributeHandleSet desiredAttributes, byte[] userSuppliedTag) throws RTIexception {
        ensureNotDestroyed("attributeOwnershipAcquisition");
        int[] handles = desiredAttributes.stream().mapToInt(AttributeHandle::getValue).toArray();
        int rc = nativeLrc.valhalla_acquire_ownership(amb, theObject.getValue(), handles, handles.length);
        checkStatus(rc, "attributeOwnershipAcquisition");
    }

    @Override
    public boolean isAttributeOwnedByFederate(ObjectInstanceHandle theObject, AttributeHandle theAttribute) throws RTIexception {
        ensureNotDestroyed("isAttributeOwnedByFederate");
        IntByReference outOwner = new IntByReference();
        ByteByReference outIsOwned = new ByteByReference();
        int rc = nativeLrc.valhalla_query_ownership(amb, theObject.getValue(), theAttribute.getValue(), outOwner, outIsOwned);
        checkStatus(rc, "isAttributeOwnedByFederate");
        int localHandle = this.localFederateHandle;
        return outIsOwned.getValue() != 0 && localHandle > 0 && outOwner.getValue() == localHandle;
    }

    @Override
    public RegionHandle createRegion(DimensionHandle[] dimensions) throws RTIexception {
        ensureNotDestroyed("createRegion");
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
        ensureNotDestroyed("setRangeBounds");
        if (bounds == null || bounds.lower < 0 || bounds.upper < 0 ||
                bounds.lower > 0xFFFFFFFFL || bounds.upper > 0xFFFFFFFFL || bounds.lower > bounds.upper) {
            throw new RTIinternalError("RangeBounds out of uint32 range or lower > upper");
        }
        int rc = nativeLrc.valhalla_set_range_bounds(amb, region.getValue(), dimension.getValue(), (int) bounds.lower, (int) bounds.upper);
        checkStatus(rc, "setRangeBounds");
    }

    @Override
    public void setMetricBounds(RegionHandle region, double minX, double maxX, double minY, double maxY, double minZ, double maxZ) throws RTIexception {
        ensureNotDestroyed("setMetricBounds");
        int rc = nativeLrc.valhalla_set_metric_bounds(amb, region.getValue(), minX, maxX, minY, maxY, minZ, maxZ);
        checkStatus(rc, "setMetricBounds");
    }

    @Override
    public void subscribeObjectClassAttributesWithRegions(ObjectClassHandle theClass, AttributeHandleSet attributeList, RegionHandle region) throws RTIexception {
        ensureNotDestroyed("subscribeObjectClassAttributesWithRegions");
        int[] handles = attributeList.stream().mapToInt(AttributeHandle::getValue).toArray();
        int rc = nativeLrc.valhalla_subscribe_with_region(amb, theClass.getValue(), handles, handles.length, region.getValue());
        checkStatus(rc, "subscribeObjectClassAttributesWithRegions");
    }

    @Override
    public void updateAttributeValuesSpatial(ObjectInstanceHandle theObject, AttributeHandleValueMap theAttributes, double x, double y, double z, byte[] userSuppliedTag) throws RTIexception {
        ensureNotDestroyed("updateAttributeValuesSpatial");
        int count = theAttributes.size();
        int[] handles = new int[count];
        int[] lens = new int[count];
        Pointer[] ptrs = new Pointer[count];
        List<Memory> allocated = new ArrayList<>(count);

        try {
            int idx = 0;
            for (Map.Entry<AttributeHandle, byte[]> entry : theAttributes.entrySet()) {
                handles[idx] = entry.getKey().getValue();
                byte[] val = entry.getValue() != null ? entry.getValue() : new byte[0];
                lens[idx] = val.length;
                if (val.length > 0) {
                    Memory mem = new Memory(val.length);
                    mem.write(0, val, 0, val.length);
                    allocated.add(mem);
                    ptrs[idx] = mem;
                } else {
                    ptrs[idx] = Pointer.NULL;
                }
                idx++;
            }

            byte[] tag = userSuppliedTag != null ? userSuppliedTag : new byte[0];
            int rc = nativeLrc.valhalla_update_attributes_spatial(amb, theObject.getValue(), handles, ptrs, lens, count, x, y, z, tag, tag.length);
            checkStatus(rc, "updateAttributeValuesSpatial");
        } finally {
            for (Memory mem : allocated) {
                mem.close();
            }
        }
    }

    @Override
    public boolean evokeCallback(double approximateMinimumTimeInSeconds) throws RTIexception {
        ensureNotDestroyed("evokeCallback");
        ByteByReference outEvoked = new ByteByReference();
        int rc = nativeLrc.valhalla_evoke_callback(amb, approximateMinimumTimeInSeconds, outEvoked);
        checkStatus(rc, "evokeCallback");
        return outEvoked.getValue() != 0;
    }

    @Override
    public boolean evokeMultipleCallbacks(double minimumTime, double maximumTime) throws RTIexception {
        ensureNotDestroyed("evokeMultipleCallbacks");
        double minSec = Math.max(0.0, minimumTime);
        double maxSec = Math.max(minSec, maximumTime);
        long startNs = System.nanoTime();
        long minNs = (long) (minSec * 1_000_000_000.0);
        long maxNs = (long) (maxSec * 1_000_000_000.0);
        boolean any = false;

        while (true) {
            long elapsedNs = System.nanoTime() - startNs;
            if (elapsedNs >= maxNs && any) {
                break;
            }
            long targetNs = (!any && elapsedNs < minNs) ? (minNs - elapsedNs) : Math.max(0L, maxNs - elapsedNs);
            double sliceSec = Math.min(0.05, targetNs / 1_000_000_000.0);
            boolean evoked = evokeCallback(sliceSec);
            if (evoked) {
                any = true;
            }
            elapsedNs = System.nanoTime() - startNs;
            if (elapsedNs >= maxNs) {
                break;
            }
            if (!evoked && elapsedNs >= minNs) {
                break;
            }
        }
        return any;
    }
}
