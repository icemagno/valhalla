// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package integration_test

import (
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/lrc"
)

type desyncTestFedAmb struct {
	lrc.NullFederateAmbassador
	discoveredObjects []int32
	removedObjects    []int32
	reflectedAttrs    map[int32]map[int32][]byte
	desyncEvents      int
}

func newDesyncTestFedAmb() *desyncTestFedAmb {
	return &desyncTestFedAmb{
		reflectedAttrs: make(map[int32]map[int32][]byte),
	}
}

func (f *desyncTestFedAmb) DiscoverObjectInstance(objectHandle, classHandle int32, name string) {
	f.discoveredObjects = append(f.discoveredObjects, objectHandle)
}

func (f *desyncTestFedAmb) RemoveObjectInstance(objectHandle int32, tag []byte) {
	f.removedObjects = append(f.removedObjects, objectHandle)
}

func (f *desyncTestFedAmb) ReflectAttributeValues(objectHandle int32, attributes map[int32][]byte, tag []byte) {
	if _, exists := f.reflectedAttrs[objectHandle]; !exists {
		f.reflectedAttrs[objectHandle] = make(map[int32][]byte)
	}
	for k, v := range attributes {
		f.reflectedAttrs[objectHandle][k] = v
	}
}

func (f *desyncTestFedAmb) OnStateDesyncDetected(expectedRoot, localRoot [32]byte) {
	f.desyncEvents++
}

func TestMerkleDesync_ConsensusAndDivergence(t *testing.T) {
	// 1. Start ephemeral CRC
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC server: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()

	// 2. Connect Federate A (Publisher) and Federate B (Subscriber)
	ambA := lrc.NewRTIambassador()
	fedAmbA := newDesyncTestFedAmb()
	if err := ambA.Connect(fedAmbA, crcAddr); err != nil {
		t.Fatalf("ambA connect failed: %v", err)
	}
	defer ambA.Disconnect()

	ambB := lrc.NewRTIambassador()
	fedAmbB := newDesyncTestFedAmb()
	if err := ambB.Connect(fedAmbB, crcAddr); err != nil {
		t.Fatalf("ambB connect failed: %v", err)
	}
	defer ambB.Disconnect()

	fedName := "DesyncTestFederation"
	if err := ambA.CreateFederationExecution(fedName); err != nil {
		t.Fatalf("create federation failed: %v", err)
	}
	defer func() { _ = ambA.DestroyFederationExecution(fedName) }()

	if _, err := ambA.JoinFederationExecution("Publisher", fedName); err != nil {
		t.Fatalf("ambA join failed: %v", err)
	}
	if _, err := ambB.JoinFederationExecution("Subscriber", fedName); err != nil {
		t.Fatalf("ambB join failed: %v", err)
	}

	// 3. Setup Publish / Subscribe
	classHandle := int32(common.HLAobjectRootHandle)
	attrHandle := int32(common.HLAprivilegeToDeleteObjectHandle)
	if err := ambA.PublishObjectClassAttributes(classHandle, []int32{attrHandle}); err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if err := ambB.SubscribeObjectClassAttributes(classHandle, []int32{attrHandle}); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	// 4. Test Multi-Object Registration (No False Positive Desync):
	// Register 2 objects (obj1, obj2). Both CRC and LRC insert them with [32]byte{}.
	obj1Handle, err := ambA.RegisterObjectInstance(classHandle, "EntityAlpha")
	if err != nil {
		t.Fatalf("register obj1 failed: %v", err)
	}
	obj2Handle, err := ambA.RegisterObjectInstance(classHandle, "EntityBeta")
	if err != nil {
		t.Fatalf("register obj2 failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	_ = ambB.EvokeMultipleCallbacks(10*time.Millisecond, 200*time.Millisecond)

	if len(fedAmbB.discoveredObjects) < 2 {
		t.Fatalf("subscriber did not discover both objects: got %d, expected at least 2", len(fedAmbB.discoveredObjects))
	}

	// Update attributes ONLY on obj1. obj2 remains with its initial state.
	attrVal1 := []byte("Obj1_State_Initial")
	if err := ambA.UpdateAttributeValues(obj1Handle, map[int32][]byte{attrHandle: attrVal1}, []byte("tag1")); err != nil {
		t.Fatalf("update obj1 failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	_ = ambB.EvokeMultipleCallbacks(10*time.Millisecond, 200*time.Millisecond)

	// Validate consensus: neither Publisher nor Subscriber should report desync!
	if err := ambA.LastDesyncDetected(); err != nil {
		t.Fatalf("false positive desync detected on Publisher after multi-object register: %v", err)
	}
	if err := ambB.LastDesyncDetected(); err != nil {
		t.Fatalf("false positive desync detected on Subscriber after multi-object register: %v", err)
	}
	if ambA.GetLocalStateRoot() != ambB.GetLocalStateRoot() {
		t.Fatalf("state root mismatch between Publisher (%x) and Subscriber (%x)", ambA.GetLocalStateRoot(), ambB.GetLocalStateRoot())
	}

	// 5. Test Object Deletion (No False Positive Desync):
	// Publisher deletes obj2. Subscriber should discover removal and remove from local state tree.
	if err := ambA.DeleteObjectInstance(obj2Handle, []byte("delete_tag")); err != nil {
		t.Fatalf("delete obj2 failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	_ = ambB.EvokeMultipleCallbacks(10*time.Millisecond, 200*time.Millisecond)

	if len(fedAmbB.removedObjects) < 1 {
		t.Fatalf("subscriber did not receive RemoveObjectInstance for obj2")
	}

	// Update obj1 again to force state verification
	attrVal2 := []byte("Obj1_State_PostDeletion")
	if err := ambA.UpdateAttributeValues(obj1Handle, map[int32][]byte{attrHandle: attrVal2}, []byte("tag2")); err != nil {
		t.Fatalf("update obj1 post-deletion failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	_ = ambB.EvokeMultipleCallbacks(10*time.Millisecond, 200*time.Millisecond)

	if err := ambA.LastDesyncDetected(); err != nil {
		t.Fatalf("false positive desync on Publisher post-deletion: %v", err)
	}
	if err := ambB.LastDesyncDetected(); err != nil {
		t.Fatalf("false positive desync on Subscriber post-deletion: %v", err)
	}
	if ambA.GetLocalStateRoot() != ambB.GetLocalStateRoot() {
		t.Fatalf("state root mismatch post-deletion: Publisher (%x) vs Subscriber (%x)", ambA.GetLocalStateRoot(), ambB.GetLocalStateRoot())
	}

	// 6. Test Spatial Updates (No False Positive Desync):
	attrValSpatial := []byte("Obj1_Spatial_Coordinate_Update")
	if err := ambA.UpdateAttributeValuesWithSpatial(obj1Handle, map[int32][]byte{attrHandle: attrValSpatial}, 1000.0, 2000.0, 3000.0, []byte("spatial_tag")); err != nil {
		t.Fatalf("spatial update failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	_ = ambB.EvokeMultipleCallbacks(10*time.Millisecond, 200*time.Millisecond)

	if err := ambA.LastDesyncDetected(); err != nil {
		t.Fatalf("false positive desync on Publisher after spatial update: %v", err)
	}
	if err := ambB.LastDesyncDetected(); err != nil {
		t.Fatalf("false positive desync on Subscriber after spatial update: %v", err)
	}
	if ambA.GetLocalStateRoot() != ambB.GetLocalStateRoot() {
		t.Fatalf("state root mismatch after spatial update: Publisher (%x) vs Subscriber (%x)", ambA.GetLocalStateRoot(), ambB.GetLocalStateRoot())
	}

	// 7. Adversarial Test: Deliberately corrupt Subscriber's local state with a ghost entity
	implB, ok := ambB.(*lrc.RTIambassadorImpl)
	if !ok {
		t.Fatalf("ambB does not implement *lrc.RTIambassadorImpl")
	}
	implB.CorruptLocalStateForTesting(common.ObjectInstanceHandle(999), 1, []byte("GhostEntityInSubscriber"))

	// Publisher sends another valid update on obj1
	if err := ambA.UpdateAttributeValues(obj1Handle, map[int32][]byte{attrHandle: []byte("TriggerDivergence")}, []byte("tag3")); err != nil {
		t.Fatalf("third update failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	_ = ambB.EvokeMultipleCallbacks(10*time.Millisecond, 200*time.Millisecond)

	// Subscriber MUST detect cryptographic desync
	desyncErr := ambB.LastDesyncDetected()
	if desyncErr == nil {
		t.Fatalf("ambB failed to detect cryptographic desync after local state corruption")
	}
	if fedAmbB.desyncEvents == 0 {
		t.Fatalf("OnStateDesyncDetected callback was not evoked on desync-aware federate ambassador")
	}
}

func TestMerkleProof_NetworkRequestAndVerification(t *testing.T) {
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()

	amb := lrc.NewRTIambassador()
	fedAmb := newDesyncTestFedAmb()
	if err := amb.Connect(fedAmb, crcAddr); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer amb.Disconnect()

	fedName := "MerkleProofTestFed"
	_ = amb.CreateFederationExecution(fedName)
	defer func() { _ = amb.DestroyFederationExecution(fedName) }()

	_, err := amb.JoinFederationExecution("AuditorFed", fedName)
	if err != nil {
		t.Fatalf("join failed: %v", err)
	}

	classHandle := int32(common.HLAobjectRootHandle)
	attrHandle := int32(common.HLAprivilegeToDeleteObjectHandle)
	_ = amb.PublishObjectClassAttributes(classHandle, []int32{attrHandle})

	// Register 4 distinct objects to ensure the Merkle tree has at least 4 leaves
	// and produce an audit path with at least 2 sibling hashes (depth >= 2)
	objHandles := make([]int32, 4)
	for i := 0; i < 4; i++ {
		h, err := amb.RegisterObjectInstance(classHandle, "AuditedObject_"+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("register obj %d failed: %v", i, err)
		}
		objHandles[i] = h
		val := []byte("CryptographicAuditPayload_" + string(rune('A'+i)))
		if err := amb.UpdateAttributeValues(h, map[int32][]byte{attrHandle: val}, []byte("audit")); err != nil {
			t.Fatalf("update obj %d failed: %v", i, err)
		}
	}

	localTrustedRoot := amb.GetLocalStateRoot()
	if localTrustedRoot == [32]byte{} {
		t.Fatalf("amb returned empty local state root")
	}

	// 1. Request Merkle inclusion proof over network from CRC for obj0
	proof, err := amb.GetObjectMerkleProof(objHandles[0])
	if err != nil {
		t.Fatalf("GetObjectMerkleProof failed over network: %v", err)
	}
	if proof == nil {
		t.Fatalf("expected non-nil proof")
	}

	// Verify that the tree depth produced an audit path with >= 2 sibling chunks
	if len(proof.AuditPath) < 2 {
		t.Fatalf("audit path too shallow: expected len(AuditPath) >= 2 for 4 entities, got %d", len(proof.AuditPath))
	}

	// 2. Mathematically verify proof against client's trusted local root
	valid := amb.VerifyObjectInclusion(proof, localTrustedRoot)
	if !valid {
		t.Fatalf("valid Merkle proof failed client-side mathematical verification against trusted root")
	}

	// 3. Adversarial Attack: Tamper with audit path sibling 0
	tamperedProof0 := *proof
	tamperedProof0.AuditPath = make([][32]byte, len(proof.AuditPath))
	copy(tamperedProof0.AuditPath, proof.AuditPath)
	tamperedProof0.AuditPath[0][0] ^= 0xFF
	if amb.VerifyObjectInclusion(&tamperedProof0, localTrustedRoot) {
		t.Fatalf("tampered audit path [0] was accepted by VerifyObjectInclusion")
	}

	// 4. Adversarial Attack: Tamper with audit path sibling 1
	tamperedProof1 := *proof
	tamperedProof1.AuditPath = make([][32]byte, len(proof.AuditPath))
	copy(tamperedProof1.AuditPath, proof.AuditPath)
	tamperedProof1.AuditPath[1][0] ^= 0xFF
	if amb.VerifyObjectInclusion(&tamperedProof1, localTrustedRoot) {
		t.Fatalf("tampered audit path [1] was accepted by VerifyObjectInclusion")
	}

	// 5. Adversarial Attack: Tamper with leaf hash
	tamperedLeaf := *proof
	tamperedLeaf.LeafHash[0] ^= 0xFF
	if amb.VerifyObjectInclusion(&tamperedLeaf, localTrustedRoot) {
		t.Fatalf("tampered leaf hash was accepted by VerifyObjectInclusion")
	}

	// 6. Adversarial Attack: Tamper with leaf index
	tamperedIndex := *proof
	tamperedIndex.LeafIndex ^= 0x01
	if amb.VerifyObjectInclusion(&tamperedIndex, localTrustedRoot) {
		t.Fatalf("tampered leaf index was accepted by VerifyObjectInclusion")
	}

	// 7. Adversarial Attack: Auditor's forged proof (self-referential attack)
	// Attacker crafts a fake proof claiming LeafIndex 999 with empty AuditPath and matching self-root
	forgedProof := &common.MerkleProof{
		LeafIndex:  999,
		AuditPath:  nil,
		LeafHash:   [32]byte{0xDE, 0xAD, 0xBE, 0xEF},
		GlobalRoot: [32]byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	if amb.VerifyObjectInclusion(forgedProof, localTrustedRoot) {
		t.Fatalf("forged proof with self-referential root was accepted against trusted root")
	}

	// 8. Adversarial Attack: Malicious proof with oversized AuditPath (> 64 chunks)
	inflatedPath := make([][32]byte, 65)
	inflatedProof := &common.MerkleProof{
		LeafIndex: proof.LeafIndex,
		AuditPath: inflatedPath,
		LeafHash:  proof.LeafHash,
	}
	if amb.VerifyObjectInclusion(inflatedProof, localTrustedRoot) {
		t.Fatalf("inflated audit path (>64 chunks) was accepted by VerifyObjectInclusion")
	}

	// 9. Adversarial Attack: Verification against an untrusted/arbitrary expected root
	untrustedRoot := [32]byte{0xBA, 0xAD, 0xF0, 0x0D}
	if amb.VerifyObjectInclusion(proof, untrustedRoot) {
		t.Fatalf("valid proof was accepted against an invalid expected root")
	}
}
