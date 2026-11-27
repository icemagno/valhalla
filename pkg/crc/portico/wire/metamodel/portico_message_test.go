package metamodel_test

import (
	"bytes"
	"testing"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

func TestPorticoMessage_TimestampPreservedInRoundtrip(t *testing.T) {
	msg := metamodel.NewJavaObject(metamodel.ClassDescPorticoMessage)
	msg.Set("isFromRti", true)
	msg.Set("sourceFederate", int32(7))
	msg.Set("targetFederate", int32(11))
	msg.Set("targetFederation", int32(3))
	msg.Set("timestamp", 42.75)
	msg.Set("isAsync", true)

	enc := wire.NewJOSEncoder()
	if err := enc.WriteObject(msg); err != nil {
		t.Fatalf("WriteObject failed: %v", err)
	}

	dec := wire.NewJOSDecoder(enc.Bytes())
	gotAny, err := dec.ReadObject()
	if err != nil {
		t.Fatalf("ReadObject failed: %v", err)
	}
	got, ok := gotAny.(*metamodel.JavaObject)
	if !ok {
		t.Fatalf("expected *JavaObject, got %T", gotAny)
	}
	if ts, ok := got.Get("timestamp").(float64); !ok || ts != 42.75 {
		t.Fatalf("timestamp not preserved: got %v (%T), want 42.75", got.Get("timestamp"), got.Get("timestamp"))
	}
}

func TestJavaHashMap_StrictDeterminism100Iterations(t *testing.T) {
	m := map[string]any{
		"z_key": "last",
		"a_key": "first",
		"m_key": "middle",
		"b_key": "second",
	}
	var baseline []byte
	for i := 0; i < 100; i++ {
		enc := wire.NewJOSEncoder()
		if err := enc.WriteObject(metamodel.NewJavaHashMap(m)); err != nil {
			t.Fatalf("iteration %d WriteObject failed: %v", i, err)
		}
		cur := enc.Bytes()
		if i == 0 {
			baseline = append([]byte(nil), cur...)
		} else if !bytes.Equal(baseline, cur) {
			t.Fatalf("non-deterministic HashMap serialization detected at iteration %d", i)
		}
	}
}
