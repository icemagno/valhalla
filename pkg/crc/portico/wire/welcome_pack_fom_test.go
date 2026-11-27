package wire_test

import (
	"os"
	"testing"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

func TestWelcomePack_FullFOMRoundtrip(t *testing.T) {
	data, err := os.ReadFile("../../../../test/resources/welcome_pack_base.bin")
	if err != nil {
		t.Fatalf("failed to read welcome_pack_base.bin: %v", err)
	}

	obj, err := wire.DeserializePorticoControlMessage(data)
	if err != nil {
		t.Fatalf("failed to deserialize original WelcomePack: %v", err)
	}

	results, ok := obj.Get("results").(*metamodel.JavaObject)
	if !ok {
		t.Fatalf("expected results to be *metamodel.JavaObject, got %T", obj.Get("results"))
	}
	entries, ok := results.Get("entries").(map[any]any)
	if !ok {
		t.Fatalf("expected entries to be map[any]any, got %T", results.Get("entries"))
	}
	wp, ok := entries["result"].(*metamodel.JavaObject)
	if !ok {
		t.Fatalf("expected result to be *metamodel.JavaObject, got %T", entries["result"])
	}

	fom, ok := wp.Get("fom").(*metamodel.JavaObject)
	if !ok || fom == nil {
		t.Fatal("expected non-nil fom in WelcomePack")
	}

	// Verify ObjectModel classes
	oclasses, ok := fom.Get("oclasses").(*metamodel.JavaObject)
	if !ok {
		t.Fatal("expected oclasses in FOM")
	}
	ocEntries, ok := oclasses.Get("entries").(map[any]any)
	if !ok || len(ocEntries) == 0 {
		t.Fatalf("expected non-empty oclasses, got %d", len(ocEntries))
	}

	// Re-serialize
	serialized, err := wire.SerializePorticoControlMessage(obj)
	if err != nil {
		t.Fatalf("failed to re-serialize WelcomePack with FOM: %v", err)
	}

	// Re-deserialize to verify roundtrip
	obj2, err := wire.DeserializePorticoControlMessage(serialized)
	if err != nil {
		t.Fatalf("failed to deserialize re-serialized WelcomePack: %v", err)
	}

	if obj2.Class.Name != "org.portico2.common.messaging.ExtendedSuccessResponse" {
		t.Errorf("unexpected root class name: %s", obj2.Class.Name)
	}
	res2, ok := obj2.Get("results").(*metamodel.JavaObject)
	if !ok || res2 == nil {
		t.Fatalf("roundtrip lost results HashMap")
	}
	entries2, ok := res2.Get("entries").(map[any]any)
	if !ok {
		t.Fatalf("roundtrip results entries missing")
	}
	wp2, ok := entries2["result"].(*metamodel.JavaObject)
	if !ok || wp2 == nil {
		t.Fatalf("roundtrip lost WelcomePack result object")
	}
	fom2, ok := wp2.Get("fom").(*metamodel.JavaObject)
	if !ok || fom2 == nil {
		t.Fatalf("roundtrip lost inner FOM ObjectModel")
	}
	oc2, ok := fom2.Get("oclasses").(*metamodel.JavaObject)
	if !ok || oc2 == nil {
		t.Fatalf("roundtrip lost oclasses inside FOM")
	}
	ocEntries2, ok := oc2.Get("entries").(map[any]any)
	if !ok || len(ocEntries2) != len(ocEntries) {
		t.Fatalf("roundtrip oclasses count mismatch: got %d, want %d", len(ocEntries2), len(ocEntries))
	}
}
