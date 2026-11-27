package wire

import (
	"testing"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

func TestRegisterObjectResponsePayload(t *testing.T) {
	payload := BuildRegisterObjectResponsePayload(1001, 1034, "ThorObject")
	if len(payload) == 0 {
		t.Fatalf("payload vazio")
	}

	respObj, err := DeserializePorticoControlMessage(payload)
	if err != nil || respObj == nil {
		t.Fatalf("failed to deserialize RegisterObjectResponsePayload: %v", err)
	}
	hm, ok := respObj.Get("results").(*metamodel.JavaObject)
	if !ok || hm == nil {
		t.Fatalf("expected results HashMap in response, got %T", respObj.Get("results"))
	}
	entries, ok := hm.Get("entries").(map[any]any)
	if !ok {
		t.Fatalf("expected map[any]any entries in results HashMap, got %T", hm.Get("entries"))
	}
	if gotName, _ := entries["object-name"].(string); gotName != "ThorObject" {
		t.Fatalf("expected object-name 'ThorObject', got %q", gotName)
	}
	if handleObj, ok := entries["object-handle"].(*metamodel.JavaObject); !ok || handleObj.Get("value") != int32(1001) {
		t.Fatalf("expected object-handle 1001, got %v", entries["object-handle"])
	}
	if classObj, ok := entries["object-class"].(*metamodel.JavaObject); !ok || classObj.Get("value") != int32(1034) {
		t.Fatalf("expected object-class 1034, got %v", entries["object-class"])
	}
}
