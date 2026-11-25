package hla.rti1516e;

import java.io.Serializable;

public class TransportationTypeHandle implements Serializable {
    public static final TransportationTypeHandle HLA_RELIABLE = new TransportationTypeHandle(1, "HLAreliable");
    public static final TransportationTypeHandle HLA_BEST_EFFORT = new TransportationTypeHandle(2, "HLAbestEffort");

    private final int value;
    private final String name;

    public TransportationTypeHandle(int value, String name) {
        this.value = value;
        this.name = name;
    }

    public int getValue() {
        return value;
    }

    public String getName() {
        return name;
    }

    @Override
    public String toString() {
        return name;
    }
}
