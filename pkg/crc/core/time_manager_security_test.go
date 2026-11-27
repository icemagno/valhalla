package core

import (
	"errors"
	"testing"
)

func TestTimeManager_EnqueueTSOMessage_EnforcesQueueCap(t *testing.T) {
	tm := NewTimeManager()
	tm.SetMaxTSOQueueSize(4)

	tm.RegisterFederate(1)
	tm.RegisterFederate(2)
	_ = tm.EnableTimeConstrained(2)

	for i := 0; i < 4; i++ {
		_, err := tm.EnqueueTSOMessage(TSOMessage{
			Sender:    1,
			Receiver:  2,
			Timestamp: float64(i + 1),
			Payload:   []byte("ev"),
		})
		if err != nil {
			t.Fatalf("enqueue %d unexpectedly failed: %v", i, err)
		}
	}

	_, err := tm.EnqueueTSOMessage(TSOMessage{
		Sender:    1,
		Receiver:  2,
		Timestamp: 10.0,
		Payload:   []byte("overflow"),
	})
	if !errors.Is(err, ErrTSOQueueFull) {
		t.Fatalf("expected ErrTSOQueueFull on 5th message, got %v", err)
	}
}
