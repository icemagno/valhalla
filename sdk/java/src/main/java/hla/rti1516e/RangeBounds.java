package hla.rti1516e;

import java.io.Serializable;

public class RangeBounds implements Serializable {
    public long lower;
    public long upper;

    public RangeBounds() {
        this.lower = 0;
        this.upper = 0;
    }

    public RangeBounds(long lower, long upper) {
        this.lower = lower;
        this.upper = upper;
    }
}
