// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package common

import (
	"errors"
	"sort"
	"sync"
)

var (
	ErrObjectNotFoundInMerkleTree = errors.New("hla merkle: object instance not found in state tree")
	ErrInvalidMerkleProof         = errors.New("hla merkle: proof verification failed against root")
)

// ComputeAttributeLeaf computes the canonical SSZ leaf hash for an attribute container:
// HashTreeRoot(Container[handle uint32, value ByteList]) = HashPair(HashTreeRoot(handle), HashTreeRoot(value))
func ComputeAttributeLeaf(handle AttributeHandle, value []byte) [32]byte {
	hRoot := HashTreeRootUint32(uint32(handle))
	vRoot := HashTreeRootByteList(value, 1024*1024)
	return HashPair(hRoot, vRoot)
}

// ComputeObjectMerkleRoot calculates the deterministic Merkle root (hash_tree_root) of an entity's attributes
func ComputeObjectMerkleRoot(attributes map[AttributeHandle][]byte) [32]byte {
	if len(attributes) == 0 {
		return [32]byte{}
	}

	// 1. Extract and deterministically sort attribute handles (unsigned uint32 order, matching SSZ)
	keys := make([]AttributeHandle, 0, len(attributes))
	for k := range attributes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return uint32(keys[i]) < uint32(keys[j]) })

	// 2. Compute attribute leaf hashes
	leaves := make([][32]byte, len(keys))
	for i, h := range keys {
		leaves[i] = ComputeAttributeLeaf(h, attributes[h])
	}

	// 3. Sparse Merkle reduction
	return Merkleize(leaves, 0)
}

// FederationStateTree maintains the cryptographic state tree for all active simulation entities
type FederationStateTree struct {
	mu               sync.RWMutex
	entityRoots      map[ObjectInstanceHandle][32]byte
	entityAttributes map[ObjectInstanceHandle]map[AttributeHandle][]byte
	sortedHandles    []ObjectInstanceHandle
	leaves           [][32]byte
	treeLevels       [][][32]byte
	cachedRoot       [32]byte
}

// NewFederationStateTree initializes a federation's state tree
func NewFederationStateTree() *FederationStateTree {
	return &FederationStateTree{
		entityRoots:      make(map[ObjectInstanceHandle][32]byte),
		entityAttributes: make(map[ObjectInstanceHandle]map[AttributeHandle][]byte),
		sortedHandles:    make([]ObjectInstanceHandle, 0),
		leaves:           make([][32]byte, 0),
		treeLevels:       nil,
		cachedRoot:       [32]byte{},
	}
}

func computeFederationLeaf(handle ObjectInstanceHandle, root [32]byte) [32]byte {
	return HashPair(HashTreeRootUint32(uint32(handle)), root)
}

func (t *FederationStateTree) rebuildTreeLocked() {
	n := len(t.leaves)
	if n == 0 {
		t.cachedRoot = [32]byte{}
		t.treeLevels = nil
		return
	}

	power := NextPowerOfTwo(n)
	depth := 0
	for (1 << depth) < power {
		depth++
	}

	if depth == 0 {
		t.cachedRoot = t.leaves[0]
		t.treeLevels = [][][32]byte{{t.leaves[0]}}
		return
	}

	t.treeLevels = make([][][32]byte, depth+1)
	t.treeLevels[0] = make([][32]byte, power)
	copy(t.treeLevels[0], t.leaves)

	for h := 0; h < depth; h++ {
		curLen := len(t.treeLevels[h])
		nextLen := curLen / 2
		t.treeLevels[h+1] = make([][32]byte, nextLen)
		for i := 0; i < nextLen; i++ {
			t.treeLevels[h+1][i] = HashPair(t.treeLevels[h][2*i], t.treeLevels[h][2*i+1])
		}
	}

	t.cachedRoot = t.treeLevels[depth][0]
}

func (t *FederationStateTree) updateLeafPathLocked(idx int, leaf [32]byte) {
	t.leaves[idx] = leaf
	if len(t.treeLevels) == 0 {
		t.rebuildTreeLocked()
		return
	}
	depth := len(t.treeLevels) - 1
	if depth == 0 {
		t.treeLevels[0][0] = leaf
		t.cachedRoot = leaf
		return
	}

	t.treeLevels[0][idx] = leaf
	currIdx := idx
	for h := 0; h < depth; h++ {
		parentIdx := currIdx / 2
		left := t.treeLevels[h][parentIdx*2]
		right := t.treeLevels[h][parentIdx*2+1]
		t.treeLevels[h+1][parentIdx] = HashPair(left, right)
		currIdx = parentIdx
	}
	t.cachedRoot = t.treeLevels[depth][0]
}

// UpdateObjectState updates the Merkle root of an active entity in the simulation
func (t *FederationStateTree) UpdateObjectState(handle ObjectInstanceHandle, root [32]byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.entityRoots[handle] = root
	leaf := computeFederationLeaf(handle, root)

	idx := sort.Search(len(t.sortedHandles), func(i int) bool {
		return t.sortedHandles[i] >= handle
	})

	if idx < len(t.sortedHandles) && t.sortedHandles[idx] == handle {
		t.updateLeafPathLocked(idx, leaf)
	} else {
		t.sortedHandles = append(t.sortedHandles, 0)
		copy(t.sortedHandles[idx+1:], t.sortedHandles[idx:])
		t.sortedHandles[idx] = handle

		var zeroHash [32]byte
		t.leaves = append(t.leaves, zeroHash)
		copy(t.leaves[idx+1:], t.leaves[idx:])
		t.leaves[idx] = leaf
		t.rebuildTreeLocked()
	}
}

// UpdateEntityAttribute updates an attribute value for an entity and recalculates its object root
func (t *FederationStateTree) UpdateEntityAttribute(handle ObjectInstanceHandle, attrHandle AttributeHandle, val []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	attrs, exists := t.entityAttributes[handle]
	if !exists {
		attrs = make(map[AttributeHandle][]byte)
		t.entityAttributes[handle] = attrs
	}
	valCopy := make([]byte, len(val))
	copy(valCopy, val)
	attrs[attrHandle] = valCopy

	objRoot := ComputeObjectMerkleRoot(attrs)
	t.entityRoots[handle] = objRoot
	leaf := computeFederationLeaf(handle, objRoot)

	idx := sort.Search(len(t.sortedHandles), func(i int) bool {
		return t.sortedHandles[i] >= handle
	})

	if idx < len(t.sortedHandles) && t.sortedHandles[idx] == handle {
		t.updateLeafPathLocked(idx, leaf)
	} else {
		t.sortedHandles = append(t.sortedHandles, 0)
		copy(t.sortedHandles[idx+1:], t.sortedHandles[idx:])
		t.sortedHandles[idx] = handle

		var zeroHash [32]byte
		t.leaves = append(t.leaves, zeroHash)
		copy(t.leaves[idx+1:], t.leaves[idx:])
		t.leaves[idx] = leaf
		t.rebuildTreeLocked()
	}
}

// RemoveObjectState removes an entity from the state tree upon deletion
func (t *FederationStateTree) RemoveObjectState(handle ObjectInstanceHandle) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.entityRoots, handle)
	delete(t.entityAttributes, handle)

	idx := sort.Search(len(t.sortedHandles), func(i int) bool {
		return t.sortedHandles[i] >= handle
	})

	if idx < len(t.sortedHandles) && t.sortedHandles[idx] == handle {
		copy(t.sortedHandles[idx:], t.sortedHandles[idx+1:])
		t.sortedHandles = t.sortedHandles[:len(t.sortedHandles)-1]

		copy(t.leaves[idx:], t.leaves[idx+1:])
		t.leaves = t.leaves[:len(t.leaves)-1]
		t.rebuildTreeLocked()
	}
}

// GetGlobalRoot returns the deterministic 32-byte Merkle root of the entire federation state
func (t *FederationStateTree) GetGlobalRoot() [32]byte {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.cachedRoot
}

// MerkleProof represents the cryptographic audit path for an entity
type MerkleProof struct {
	LeafHash   [32]byte
	AuditPath  [][32]byte
	LeafIndex  uint64
	GlobalRoot [32]byte
}

// GenerateProof builds the cryptographic audit path of an object against the global root
func (t *FederationStateTree) GenerateProof(handle ObjectInstanceHandle) (*MerkleProof, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	idx := sort.Search(len(t.sortedHandles), func(i int) bool {
		return t.sortedHandles[i] >= handle
	})
	if idx >= len(t.sortedHandles) || t.sortedHandles[idx] != handle {
		return nil, ErrObjectNotFoundInMerkleTree
	}

	if len(t.treeLevels) == 0 {
		return nil, ErrObjectNotFoundInMerkleTree
	}

	depth := len(t.treeLevels) - 1
	auditPath := make([][32]byte, depth)
	currIdx := idx
	for h := 0; h < depth; h++ {
		siblingIdx := currIdx ^ 1
		auditPath[h] = t.treeLevels[h][siblingIdx]
		currIdx /= 2
	}

	return &MerkleProof{
		LeafHash:   t.treeLevels[0][idx],
		AuditPath:  auditPath,
		LeafIndex:  uint64(idx),
		GlobalRoot: t.cachedRoot,
	}, nil
}

// VerifyMerkleProof mathematically validates a cryptographic Merkle proof against a trusted expectedRoot.
// Returns false if:
// - proof is nil or expectedRoot is zero
// - len(AuditPath) > 64 (maximum tree depth)
// - leaf index does not reduce to 0 (all path bits consumed)
// - final computed hash does not strictly equal expectedRoot
func VerifyMerkleProof(proof *MerkleProof, expectedRoot [32]byte) bool {
	if proof == nil || expectedRoot == [32]byte{} {
		return false
	}
	if len(proof.AuditPath) > 64 {
		return false
	}

	currentHash := proof.LeafHash
	idx := proof.LeafIndex

	for _, sibling := range proof.AuditPath {
		if idx%2 == 0 {
			currentHash = HashPair(currentHash, sibling)
		} else {
			currentHash = HashPair(sibling, currentHash)
		}
		idx /= 2
	}

	// After traversing the entire audit path, idx MUST have reduced to 0 (root level)
	if idx != 0 {
		return false
	}

	return currentHash == expectedRoot
}
