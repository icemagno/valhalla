package hla.rti1516e;

import java.util.HashMap;
import java.util.Map;

public class AttributeHandleValueMap extends HashMap<AttributeHandle, byte[]> implements Map<AttributeHandle, byte[]> {
    public AttributeHandleValueMap() {
        super();
    }

    public AttributeHandleValueMap(Map<? extends AttributeHandle, ? extends byte[]> m) {
        super(m);
    }
}
