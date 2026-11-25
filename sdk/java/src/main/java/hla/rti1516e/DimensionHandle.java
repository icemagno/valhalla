package hla.rti1516e;

import java.io.Serializable;
import java.util.Objects;

public class DimensionHandle implements Serializable {
    private final int value;

    public DimensionHandle(int value) {
        this.value = value;
    }

    public int getValue() {
        return value;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        DimensionHandle that = (DimensionHandle) o;
        return value == that.value;
    }

    @Override
    public int hashCode() {
        return Objects.hash(value);
    }

    @Override
    public String toString() {
        return "DimensionHandle(" + value + ")";
    }
}
