package core

import (
	"math"
	"testing"
)

func TestTimeManager_RejectsNaNAndInf(t *testing.T) {
	tm := NewTimeManager()
	tm.RegisterFederate(1)
	tm.RegisterFederate(2)

	badFloats := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}

	for _, bad := range badFloats {
		if err := tm.EnableTimeRegulation(1, bad); err == nil {
			t.Fatalf("EnableTimeRegulation accepted %v", bad)
		}
		if _, err := tm.TimeAdvanceRequest(1, bad); err == nil {
			t.Fatalf("TimeAdvanceRequest accepted %v", bad)
		}
		if _, err := tm.EnqueueTSOMessage(TSOMessage{
			Sender:    1,
			Receiver:  2,
			Timestamp: bad,
		}); err == nil {
			t.Fatalf("EnqueueTSOMessage accepted %v", bad)
		}
	}

	if err := tm.EnableTimeRegulation(1, 1.0); err != nil {
		t.Fatalf("valid EnableTimeRegulation failed: %v", err)
	}
	for _, bad := range badFloats {
		if err := tm.ModifyLookahead(1, bad); err == nil {
			t.Fatalf("ModifyLookahead accepted %v", bad)
		}
	}
}
