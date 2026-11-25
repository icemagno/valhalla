package hla.rti1516e;

import java.util.Collection;
import java.util.HashSet;
import java.util.Set;

public class AttributeHandleSet extends HashSet<AttributeHandle> implements Set<AttributeHandle> {
    public AttributeHandleSet() {
        super();
    }

    public AttributeHandleSet(Collection<? extends AttributeHandle> c) {
        super(c);
    }
}
