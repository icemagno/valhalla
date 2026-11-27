package core

import (
	"fmt"
	"sync"
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
)

func TestMerkleTree_DeterministicObjectRoot(t *testing.T) {
	attrs1 := map[common.AttributeHandle][]byte{
		1: []byte("PrivilegeToken"),
		2: []byte("Position_Lat_Long"),
		3: []byte("Speed_Mach_2"),
	}

	attrs2 := map[common.AttributeHandle][]byte{
		3: []byte("Speed_Mach_2"),
		1: []byte("PrivilegeToken"),
		2: []byte("Position_Lat_Long"),
	}

	root1 := ComputeObjectMerkleRoot(attrs1)
	root2 := ComputeObjectMerkleRoot(attrs2)

	if root1 != root2 {
		t.Fatalf("ComputeObjectMerkleRoot deve ser estritamente determinístico independente da ordem do mapa")
	}

	if root1 == [32]byte{} {
		t.Fatalf("raiz Merkle gerada não deve ser nula para atributos válidos")
	}
}

func TestMerkleTree_FederationStateAndProofVerification(t *testing.T) {
	stateTree := NewFederationStateTree()

	// Inserir 4 entidades na federação
	obj1Attrs := map[common.AttributeHandle][]byte{1: []byte("Ship_A_State")}
	obj2Attrs := map[common.AttributeHandle][]byte{1: []byte("Aircraft_B_State")}
	obj3Attrs := map[common.AttributeHandle][]byte{1: []byte("Tank_C_State")}
	obj4Attrs := map[common.AttributeHandle][]byte{1: []byte("Drone_D_State")}

	stateTree.UpdateObjectState(101, ComputeObjectMerkleRoot(obj1Attrs))
	stateTree.UpdateObjectState(102, ComputeObjectMerkleRoot(obj2Attrs))
	stateTree.UpdateObjectState(103, ComputeObjectMerkleRoot(obj3Attrs))
	stateTree.UpdateObjectState(104, ComputeObjectMerkleRoot(obj4Attrs))

	globalRoot := stateTree.GetGlobalRoot()
	if globalRoot == [32]byte{} {
		t.Fatalf("raiz global da federação não pode ser nula")
	}

	// 1. Gerar e validar prova criptográfica para o objeto 102
	proof, err := stateTree.GenerateProof(102)
	if err != nil {
		t.Fatalf("falha ao gerar prova de Merkle para o objeto 102: %v", err)
	}

	if proof.GlobalRoot != globalRoot {
		t.Fatalf("raiz global da prova (%x) diverge da raiz do estado (%x)", proof.GlobalRoot, globalRoot)
	}

	if !VerifyMerkleProof(proof, globalRoot) {
		t.Fatalf("verificação da prova criptográfica de Merkle falhou para objeto 102")
	}

	// 2. Testar detecção de adulteração (Tampering Detection)
	tamperedProof := *proof
	tamperedProof.LeafHash[0] ^= 0xFF // Inverter um byte do hash da folha
	if VerifyMerkleProof(&tamperedProof, globalRoot) {
		t.Fatalf("prova de Merkle adulterada foi aceita incorretamente!")
	}

	// 2.1 Testar ataque adversarial: prova forjada sem audit path e leaf index arbitrário
	fakeProof := &common.MerkleProof{
		LeafHash:   [32]byte{0xDE, 0xAD, 0xBE, 0xEF},
		AuditPath:  nil,
		LeafIndex:  999,
		GlobalRoot: [32]byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	if VerifyMerkleProof(fakeProof, globalRoot) {
		t.Fatalf("prova forjada foi aceita por VerifyMerkleProof!")
	}
	if VerifyMerkleProof(fakeProof, fakeProof.GlobalRoot) {
		t.Fatalf("prova forjada com idx != 0 foi aceita por VerifyMerkleProof!")
	}

	// 3. Testar remoção de entidade (exclusão de objeto)
	stateTree.RemoveObjectState(102)
	newRoot := stateTree.GetGlobalRoot()
	if newRoot == globalRoot {
		t.Fatalf("a raiz global deve mudar após a remoção de uma entidade")
	}

	// Prova do objeto removido deve falhar na geração
	if _, err := stateTree.GenerateProof(102); err != ErrObjectNotFoundInMerkleTree {
		t.Fatalf("esperava ErrObjectNotFoundInMerkleTree após remoção, obteve %v", err)
	}
}

func BenchmarkMerkleTree_ComputeObjectRoot(b *testing.B) {
	attrs := map[common.AttributeHandle][]byte{
		1: []byte("PrivilegeToken"),
		2: []byte("Position_Lat_Long_Altitude"),
		3: []byte("Orientation_Pitch_Roll_Yaw"),
		4: []byte("Velocity_Vector_XYZ"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ComputeObjectMerkleRoot(attrs)
	}
}

func BenchmarkMerkleTree_FederationGlobalRoot(b *testing.B) {
	stateTree := NewFederationStateTree()
	dummyRoot := ComputeObjectMerkleRoot(map[common.AttributeHandle][]byte{1: []byte("State")})
	for i := 1; i <= 100; i++ {
		stateTree.UpdateObjectState(common.ObjectInstanceHandle(i), dummyRoot)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var r [32]byte
		r[0] = byte(i)
		stateTree.UpdateObjectState(common.ObjectInstanceHandle((i%100)+1), r)
		_ = stateTree.GetGlobalRoot()
	}
}

func BenchmarkMerkleTree_UpdateAndGetRoot_1000Entities(b *testing.B) {
	stateTree := NewFederationStateTree()
	dummyRoot := ComputeObjectMerkleRoot(map[common.AttributeHandle][]byte{1: []byte("State")})
	for i := 1; i <= 1000; i++ {
		stateTree.UpdateObjectState(common.ObjectInstanceHandle(i), dummyRoot)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h := common.ObjectInstanceHandle((i % 1000) + 1)
		mutatedRoot := dummyRoot
		mutatedRoot[0] = byte(i)
		mutatedRoot[1] = byte(i >> 8)
		mutatedRoot[2] = byte(i >> 16)
		stateTree.UpdateObjectState(h, mutatedRoot)
		_ = stateTree.GetGlobalRoot()
	}
}

func TestMerkleTree_ConcurrentReadersAndIncrementalUpdates(t *testing.T) {
	stateTree := NewFederationStateTree()
	const numEntities = 500

	dummyRoot := ComputeObjectMerkleRoot(map[common.AttributeHandle][]byte{1: []byte("Init")})
	for i := 1; i <= numEntities; i++ {
		stateTree.UpdateObjectState(common.ObjectInstanceHandle(i), dummyRoot)
	}

	initialRoot := stateTree.GetGlobalRoot()
	if initialRoot == [32]byte{} {
		t.Fatalf("initial root cannot be empty")
	}

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// 10 concurrent readers running GetGlobalRoot() under RLock
	for r := 0; r < 10; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopChan:
					return
				default:
					root := stateTree.GetGlobalRoot()
					if root == [32]byte{} {
						t.Errorf("read empty root during concurrent execution")
						return
					}
				}
			}
		}()
	}

	// 4 concurrent updaters
	var updatersWg sync.WaitGroup
	for u := 0; u < 4; u++ {
		updatersWg.Add(1)
		go func(workerID int) {
			defer updatersWg.Done()
			for i := 0; i < 200; i++ {
				h := common.ObjectInstanceHandle((workerID*100 + i%100) + 1)
				updRoot := ComputeObjectMerkleRoot(map[common.AttributeHandle][]byte{
					1: []byte(fmt.Sprintf("Worker_%d_Iter_%d", workerID, i)),
				})
				stateTree.UpdateObjectState(h, updRoot)
			}
		}(u)
	}

	updatersWg.Wait()
	close(stopChan)
	wg.Wait()

	finalRoot := stateTree.GetGlobalRoot()
	if finalRoot == [32]byte{} || finalRoot == initialRoot {
		t.Fatalf("final root must be non-empty and diverged from initial root")
	}
}
