package org.valhalla.example;

import hla.rti1516e.*;
import hla.rti1516e.exceptions.RTIexception;

import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;

public class ExampleFederateAmbassador extends NullFederateAmbassador {

    private final AtomicInteger discoverCount = new AtomicInteger(0);
    private final AtomicInteger reflectCount = new AtomicInteger(0);
    private final AtomicInteger removeCount = new AtomicInteger(0);

    @Override
    public void discoverObjectInstance(ObjectInstanceHandle theObject,
                                        ObjectClassHandle theObjectClass,
                                        String objectName) throws RTIexception {
        discoverCount.incrementAndGet();
        System.out.println("  [Ambassador Callback] >>> OBJECT DISCOVERED: name='" + objectName +
                "', handle=" + theObject + ", class=" + theObjectClass);
    }

    @Override
    public void reflectAttributeValues(ObjectInstanceHandle theObject,
                                        AttributeHandleValueMap theAttributes,
                                        byte[] userSuppliedTag,
                                        OrderType sentOrdering,
                                        TransportationTypeHandle theTransport) throws RTIexception {
        reflectCount.incrementAndGet();
        System.out.println("  [Ambassador Callback] >>> ATTRIBUTES REFLECTED for object " + theObject +
                " (" + theAttributes.size() + " attributes):");
        for (Map.Entry<AttributeHandle, byte[]> entry : theAttributes.entrySet()) {
            String valStr = new String(entry.getValue(), StandardCharsets.UTF_8);
            System.out.println("      * Attr " + entry.getKey() + " => \"" + valStr + "\"");
        }
    }

    @Override
    public void removeObjectInstance(ObjectInstanceHandle theObject,
                                      byte[] userSuppliedTag,
                                      OrderType sentOrdering) throws RTIexception {
        removeCount.incrementAndGet();
        System.out.println("  [Ambassador Callback] >>> OBJECT REMOVED: handle=" + theObject);
    }

    @Override
    public void timeAdvanceGrant(double theTime) throws RTIexception {
        System.out.println("  [Ambassador Callback] >>> TIME ADVANCE GRANTED: time=" + theTime);
    }

    public int getDiscoverCount() {
        return discoverCount.get();
    }

    public int getReflectCount() {
        return reflectCount.get();
    }

    public int getRemoveCount() {
        return removeCount.get();
    }
}
