package hla.rti1516e;

import java.io.Serializable;
import java.util.Objects;

public class AttributeHandle implements Serializable {
    private final int value;

    public AttributeHandle(int value) {
        this.value = value;
    }

    public int getValue() {
        return value;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        AttributeHandle that = (AttributeHandle) o;
        return value == that.value;
    }

    @Override
    public int hashCode() {
        return Objects.hash(value);
    }

    @Override
    public String toString() {
        return "AttributeHandle(" + value + ")";
    }
}
