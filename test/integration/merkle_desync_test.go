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

	// 4. Register object and update attributes
	objHandle, err := ambA.RegisterObjectInstance(classHandle, "EntityAlpha")
	if err != nil {
		t.Fatalf("register object failed: %v", err)
	}

	attrVal := []byte("InitialConsensusStatePayload")
	if err := ambA.UpdateAttributeValues(objHandle, map[int32][]byte{attrHandle: attrVal}, []byte("tag1")); err != nil {
		t.Fatalf("update attributes failed: %v", err)
	}

	// 5. Subscriber evokes callbacks
	time.Sleep(50 * time.Millisecond)
	_ = ambB.EvokeMultipleCallbacks(10*time.Millisecond, 200*time.Millisecond)

	if len(fedAmbB.reflectedAttrs[objHandle]) == 0 {
		t.Fatalf("subscriber did not receive reflected attributes")
	}

	// 6. Check cryptographic state consensus:
	// Subscriber local state root must match CRC state root
	rootB := ambB.GetLocalStateRoot()
	if rootB == [32]byte{} {
		t.Fatalf("ambB returned empty local state root")
	}

	if err := ambB.LastDesyncDetected(); err != nil {
		t.Fatalf("unexpected desync detected in valid consensus: %v", err)
	}

	// 7. Adversarial Test: Force a desync on Subscriber by corrupting its local state with a ghost entity
	ambB.CorruptLocalStateForTesting(999, 1, []byte("GhostEntityInSubscriber"))

	// Publisher sends another update
	if err := ambA.UpdateAttributeValues(objHandle, map[int32][]byte{attrHandle: []byte("SecondValidUpdate")}, []byte("tag2")); err != nil {
		t.Fatalf("second update failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	_ = ambB.EvokeMultipleCallbacks(10*time.Millisecond, 200*time.Millisecond)

	// Subscriber MUST detect state divergence
	desyncErr := ambB.LastDesyncDetected()
	if desyncErr == nil {
		t.Fatalf("ambB failed to detect cryptographic desync between local state and CRC root")
	}
	if fedAmbB.desyncEvents == 0 {
		t.Fatalf("OnStateDesyncDetected was not evoked on desync-aware federate ambassador")
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

	objHandle, err := amb.RegisterObjectInstance(classHandle, "AuditedObject")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	val := []byte("CryptographicAuditPayload_42")
	if err := amb.UpdateAttributeValues(objHandle, map[int32][]byte{attrHandle: val}, []byte("audit")); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// 1. Request Merkle inclusion proof over network from CRC
	proof, err := amb.GetObjectMerkleProof(objHandle)
	if err != nil {
		t.Fatalf("GetObjectMerkleProof failed over network: %v", err)
	}
	if proof == nil {
		t.Fatalf("expected non-nil proof")
	}

	// 2. Mathematically verify proof against global root on client
	valid := amb.VerifyObjectInclusion(proof)
	if !valid {
		t.Fatalf("valid Merkle proof failed client-side mathematical verification")
	}

	// 3. Adversarial Attack: Tamper with audit path
	tamperedProof := *proof
	if len(tamperedProof.AuditPath) > 0 {
		tamperedProof.AuditPath[0][0] ^= 0xFF
		if amb.VerifyObjectInclusion(&tamperedProof) {
			t.Fatalf("tampered audit path was accepted by VerifyObjectInclusion")
		}
	}

	// 4. Adversarial Attack: Tamper with leaf hash
	tamperedLeaf := *proof
	tamperedLeaf.LeafHash[0] ^= 0xFF
	if amb.VerifyObjectInclusion(&tamperedLeaf) {
		t.Fatalf("tampered leaf hash was accepted by VerifyObjectInclusion")
	}
}
