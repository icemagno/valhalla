package valhalla.client;

import com.sun.jna.Callback;
import com.sun.jna.Library;
import com.sun.jna.Pointer;
import com.sun.jna.Structure;
import com.sun.jna.ptr.ByteByReference;
import com.sun.jna.ptr.IntByReference;

import java.util.Arrays;
import java.util.List;

public interface IValhallaLrc extends Library {

    int VALHALLA_OK = 0;
    int VALHALLA_ERR_NOT_CONNECTED = 1;
    int VALHALLA_ERR_INVALID_ARG = 2;
    int VALHALLA_ERR_RTI_EXCEPTION = 3;
    int VALHALLA_ERR_NOT_OWNER = 4;
    int VALHALLA_ERR_ALREADY_OWNED = 5;
    int VALHALLA_ERR_FEDERATION_EXISTS = 6;
    int VALHALLA_ERR_FEDERATION_NOT_FOUND = 7;
    int VALHALLA_ERR_CLASS_NOT_FOUND = 8;
    int VALHALLA_ERR_ATTR_NOT_FOUND = 9;
    int VALHALLA_ERR_OBJECT_NOT_FOUND = 10;
    int VALHALLA_ERR_NOT_MEMBER = 11;
    int VALHALLA_ERR_FEDERATES_JOINED = 12;
    int VALHALLA_ERR_FEDERATE_NAME_IN_USE = 13;
    int VALHALLA_ERR_INTERACTION_NOT_FOUND = 14;

    interface DiscoverObjectCallback extends Callback {
        void invoke(int theObject, int theObjectClass, String objectName);
    }

    interface RemoveObjectCallback extends Callback {
        void invoke(int theObject, Pointer tag, int tagLen);
    }

    interface ReflectAttributesCallback extends Callback {
        void invoke(int theObject, int attrCount, Pointer attrHandles, Pointer attrValues, Pointer attrLens, Pointer tag, int tagLen);
    }

    interface TimeAdvanceGrantCallback extends Callback {
        void invoke(double time);
    }

    @Structure.FieldOrder({"onDiscoverObject", "onRemoveObject", "onReflectAttributes", "onTimeAdvanceGrant"})
    class ValhallaCallbacks extends Structure {
        public static class ByValue extends ValhallaCallbacks implements Structure.ByValue {}

        public DiscoverObjectCallback onDiscoverObject;
        public RemoveObjectCallback onRemoveObject;
        public ReflectAttributesCallback onReflectAttributes;
        public TimeAdvanceGrantCallback onTimeAdvanceGrant;

        @Override
        protected List<String> getFieldOrder() {
            return Arrays.asList("onDiscoverObject", "onRemoveObject", "onReflectAttributes", "onTimeAdvanceGrant");
        }
    }

    Pointer valhalla_create_ambassador();
    void valhalla_destroy_ambassador(Pointer amb);
    int valhalla_set_callbacks(Pointer amb, ValhallaCallbacks.ByValue callbacks);

    int valhalla_connect(Pointer amb, String address);
    int valhalla_disconnect(Pointer amb);
    int valhalla_create_federation(Pointer amb, String fedName);
    int valhalla_create_federation_with_fom(Pointer amb, String fedName, String[] fomModules, int fomCount);
    int valhalla_destroy_federation(Pointer amb, String fedName);
    int valhalla_join_federation(Pointer amb, String federateName, String fedName, IntByReference outFederateHandle);
    int valhalla_resign_federation(Pointer amb);

    int valhalla_get_object_class_handle(Pointer amb, String className, IntByReference outHandle);
    int valhalla_get_attribute_handle(Pointer amb, int classHandle, String attrName, IntByReference outHandle);
    int valhalla_get_dimension_handle(Pointer amb, String dimName, IntByReference outHandle);

    int valhalla_publish_object_class(Pointer amb, int classHandle, int[] attrHandles, int attrCount);
    int valhalla_subscribe_object_class(Pointer amb, int classHandle, int[] attrHandles, int attrCount);

    int valhalla_register_object(Pointer amb, int classHandle, String objectName, IntByReference outObjectHandle);
    int valhalla_delete_object(Pointer amb, int objectHandle, byte[] tag, int tagLen);
    int valhalla_update_attributes(Pointer amb, int objectHandle, int[] attrHandles, Pointer[] attrValues, int[] attrLens, int attrCount, byte[] tag, int tagLen);

    int valhalla_enable_time_regulation(Pointer amb, double lookahead);
    int valhalla_enable_time_constrained(Pointer amb);
    int valhalla_time_advance_request(Pointer amb, double time);
    int valhalla_next_message_request(Pointer amb, double time);

    int valhalla_divest_ownership(Pointer amb, int objectHandle, int[] attrHandles, int attrCount);
    int valhalla_acquire_ownership(Pointer amb, int objectHandle, int[] attrHandles, int attrCount);
    int valhalla_query_ownership(Pointer amb, int objectHandle, int attrHandle, IntByReference outOwner, ByteByReference outIsOwned);

    int valhalla_create_region(Pointer amb, int[] dimensions, int dimCount, IntByReference outRegionHandle);
    int valhalla_set_range_bounds(Pointer amb, int regionHandle, int dimension, int lowerBound, int upperBound);
    int valhalla_set_metric_bounds(Pointer amb, int regionHandle, double minX, double maxX, double minY, double maxY, double minZ, double maxZ);
    int valhalla_subscribe_with_region(Pointer amb, int classHandle, int[] attrHandles, int attrCount, int regionHandle);
    int valhalla_update_attributes_spatial(Pointer amb, int objectHandle, int[] attrHandles, Pointer[] attrValues, int[] attrLens, int attrCount, double x, double y, double z, byte[] tag, int tagLen);

    int valhalla_evoke_callback(Pointer amb, double timeoutSeconds, ByteByReference outEvoked);
}
