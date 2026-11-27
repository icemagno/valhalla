package crc

import (
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
)

func TestFederationExecution_RejectsForgedClientMerkleRoot(t *testing.T) {
	fed := NewFederationExecution(1, "MerkleAuditFed")
	fed.SetMerkleAuditEnabled(true)

	// Register an object instance
	obj, err := fed.RegisterObjectInstance(10, "Tank-01", 1)
	if err != nil {
		t.Fatalf("RegisterObjectInstance failed: %v", err)
	}

	// Valid update with zero client Merkle root (CRC computes authoritative root)
	initialAttrs := map[int32][]byte{
		1: []byte("pos-alpha"),
	}
	if err := fed.UpdateObjectAttributes(obj.Handle, initialAttrs, [32]byte{}); err != nil {
		t.Fatalf("valid UpdateObjectAttributes failed: %v", err)
	}
	expectedInitialObjRoot := core.ComputeObjectMerkleRoot(map[common.AttributeHandle][]byte{
		1: []byte("pos-alpha"),
	})
	snap, ok := fed.GetObjectInstance(obj.Handle)
	if !ok {
		t.Fatalf("GetObjectInstance failed")
	}
	if snap.MerkleRoot != expectedInitialObjRoot {
		t.Fatalf("expected authoritative object root %x, got %x", expectedInitialObjRoot, snap.MerkleRoot)
	}
	initialGlobalRoot := fed.StateTree().GetGlobalRoot()

	// Adversarial attack: client supplies a forged non-zero MerkleRoot
	forgedRoot := [32]byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04}
	attackAttrs := map[int32][]byte{
		1: []byte("tampered-value"),
	}
	err = fed.UpdateObjectAttributes(obj.Handle, attackAttrs, forgedRoot)
	if err == nil {
		t.Fatalf("SECURITY VULNERABILITY: CRC accepted forged client MerkleRoot %x!", forgedRoot)
	}

	// Verify state was NOT mutated or poisoned by the rejected attack
	snap, _ = fed.GetObjectInstance(obj.Handle)
	if snap.MerkleRoot == forgedRoot {
		t.Fatalf("object MerkleRoot was poisoned with forged root")
	}
	if string(snap.Attributes[1]) != "pos-alpha" {
		t.Fatalf("object attribute was mutated despite rejected forged MerkleRoot: got %q", string(snap.Attributes[1]))
	}
	if fed.StateTree().GetGlobalRoot() != initialGlobalRoot {
		t.Fatalf("federation global MerkleRoot was poisoned by rejected forged update")
	}

	// Verify legitimate client supplying the EXACT matching authoritative root is accepted
	validAttrs := map[int32][]byte{
		1: []byte("pos-beta"),
	}
	validRoot := core.ComputeObjectMerkleRoot(map[common.AttributeHandle][]byte{
		1: []byte("pos-beta"),
	})
	if err := fed.UpdateObjectAttributes(obj.Handle, validAttrs, validRoot); err != nil {
		t.Fatalf("expected exact matching client MerkleRoot to succeed, got: %v", err)
	}
	snap, _ = fed.GetObjectInstance(obj.Handle)
	if snap.MerkleRoot != validRoot {
		t.Fatalf("expected updated MerkleRoot %x, got %x", validRoot, snap.MerkleRoot)
	}
}
