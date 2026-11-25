package wire

import (
	"testing"
)

func TestRegisterObjectResponsePayload(t *testing.T) {
	payload := BuildRegisterObjectResponsePayload(1001, 1034, "ThorObject")
	if len(payload) == 0 {
		t.Fatalf("payload vazio")
	}
	t.Logf("Payload length: %d", len(payload))
}
