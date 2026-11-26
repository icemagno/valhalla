package core

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
	"sync"

	"github.com/icemagno/valhalla/pkg/common"
)

var (
	ErrObjectNotFoundInMerkleTree = errors.New("hla merkle: object instance not found in state tree")
	ErrInvalidMerkleProof         = errors.New("hla merkle: proof verification failed against root")
)

// HashPair computes the SHA-256 hash of two contiguous 32-byte nodes without heap allocation
func HashPair(left, right [32]byte) [32]byte {
	var buf [64]byte
	copy(buf[:32], left[:])
	copy(buf[32:], right[:])
	return sha256.Sum256(buf[:])
}

// ComputeAttributeLeaf computes the canonical SSZ leaf hash for an attribute container:
// HashTreeRoot(Container[handle uint32, value ByteList]) = HashPair(HashTreeRoot(handle), HashTreeRoot(value))
func ComputeAttributeLeaf(handle common.AttributeHandle, value []byte) [32]byte {
	hRoot := common.HashTreeRootUint32(uint32(handle))
	vRoot := common.HashTreeRootByteList(value, 1024*1024)
	return common.HashPair(hRoot, vRoot)
}

// ComputeObjectMerkleRoot calculates the deterministic Merkle root (hash_tree_root) of an entity's attributes
func ComputeObjectMerkleRoot(attributes map[common.AttributeHandle][]byte) [32]byte {
	if len(attributes) == 0 {
		return [32]byte{}
	}

	// 1. Extract and deterministically sort attribute handles
	keys := make([]int, 0, len(attributes))
	for k := range attributes {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)

	// 2. Compute attribute leaf hashes
	leaves := make([][32]byte, len(keys))
	for i, k := range keys {
		h := common.AttributeHandle(k)
		leaves[i] = ComputeAttributeLeaf(h, attributes[h])
	}

	// 3. Binary Merkle tree reduction
	return reduceMerkleTree(leaves)
}

// nextPowerOfTwo calculates the next power of 2 greater than or equal to n
func nextPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

// reduceMerkleTree performs bottom-up pairwise reduction of leaves to a single root
func reduceMerkleTree(leaves [][32]byte) [32]byte {
	n := len(leaves)
	if n == 0 {
		return [32]byte{}
	}
	if n == 1 {
		return leaves[0]
	}

	power := nextPowerOfTwo(n)
	tree := make([][32]byte, power)
	copy(tree, leaves)

	for size := power; size > 1; size /= 2 {
		for i := 0; i < size/2; i++ {
			tree[i] = HashPair(tree[2*i], tree[2*i+1])
		}
	}

	return tree[0]
}

// FederationStateTree maintains the cryptographic state tree for all active simulation entities
type FederationStateTree struct {
	mu          sync.RWMutex
	entityRoots map[common.ObjectInstanceHandle][32]byte
	cachedRoot  [32]byte
	isDirty     bool
}

// NewFederationStateTree initializes a federation's state tree
func NewFederationStateTree() *FederationStateTree {
	return &FederationStateTree{
		entityRoots: make(map[common.ObjectInstanceHandle][32]byte),
		cachedRoot:  [32]byte{},
		isDirty:     false,
	}
}

// UpdateObjectState updates the Merkle root of an active entity in the simulation
func (t *FederationStateTree) UpdateObjectState(handle common.ObjectInstanceHandle, root [32]byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entityRoots[handle] = root
	t.isDirty = true
}

// RemoveObjectState removes an entity from the state tree upon deletion
func (t *FederationStateTree) RemoveObjectState(handle common.ObjectInstanceHandle) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entityRoots, handle)
	t.isDirty = true
}

// GetGlobalRoot returns the deterministic 32-byte Merkle root of the entire federation state
func (t *FederationStateTree) GetGlobalRoot() [32]byte {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.isDirty {
		return t.cachedRoot
	}

	if len(t.entityRoots) == 0 {
		t.cachedRoot = [32]byte{}
		t.isDirty = false
		return t.cachedRoot
	}

	// Sort entities deterministically by handle
	handles := make([]int, 0, len(t.entityRoots))
	for h := range t.entityRoots {
		handles = append(handles, int(h))
	}
	sort.Ints(handles)

	leaves := make([][32]byte, len(handles))
	for i, h := range handles {
		leafHash := t.entityRoots[common.ObjectInstanceHandle(h)]
		// Combine handle and entity root as a federation leaf
		var handleBuf [4]byte
		binary.LittleEndian.PutUint32(handleBuf[:], uint32(h))
		var combined [64]byte
		copy(combined[:4], handleBuf[:])
		copy(combined[4:36], leafHash[:])
		leaves[i] = sha256.Sum256(combined[:])
	}

	t.cachedRoot = reduceMerkleTree(leaves)
	t.isDirty = false
	return t.cachedRoot
}

// MerkleProof represents the cryptographic audit path for an entity
type MerkleProof struct {
	LeafHash   [32]byte
	AuditPath  [][32]byte
	LeafIndex  uint64
	GlobalRoot [32]byte
}

// GenerateProof builds the cryptographic audit path (multiproof) of an object against the global root
func (t *FederationStateTree) GenerateProof(handle common.ObjectInstanceHandle) (*MerkleProof, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.entityRoots[handle]; !exists {
		return nil, ErrObjectNotFoundInMerkleTree
	}

	handles := make([]int, 0, len(t.entityRoots))
	for h := range t.entityRoots {
		handles = append(handles, int(h))
	}
	sort.Ints(handles)

	leafIdx := -1
	leaves := make([][32]byte, len(handles))
	for i, h := range handles {
		objH := common.ObjectInstanceHandle(h)
		leafHash := t.entityRoots[objH]
		var handleBuf [4]byte
		binary.LittleEndian.PutUint32(handleBuf[:], uint32(h))
		var combined [64]byte
		copy(combined[:4], handleBuf[:])
		copy(combined[4:36], leafHash[:])
		leaves[i] = sha256.Sum256(combined[:])
		if objH == handle {
			leafIdx = i
		}
	}

	power := nextPowerOfTwo(len(leaves))
	currentLevel := make([][32]byte, power)
	copy(currentLevel, leaves)

	targetIdx := leafIdx
	auditPath := make([][32]byte, 0)

	for size := power; size > 1; size /= 2 {
		siblingIdx := targetIdx ^ 1 // Sibling index (0->1, 1->0)
		auditPath = append(auditPath, currentLevel[siblingIdx])

		nextLevel := make([][32]byte, size/2)
		for i := 0; i < size/2; i++ {
			nextLevel[i] = HashPair(currentLevel[2*i], currentLevel[2*i+1])
		}
		currentLevel = nextLevel
		targetIdx /= 2
	}

	globalRoot := currentLevel[0]
	t.cachedRoot = globalRoot
	t.isDirty = false

	return &MerkleProof{
		LeafHash:   leaves[leafIdx],
		AuditPath:  auditPath,
		LeafIndex:  uint64(leafIdx),
		GlobalRoot: globalRoot,
	}, nil
}

// VerifyMerkleProof mathematically validates a Merkle proof against the global root
func VerifyMerkleProof(proof *MerkleProof) bool {
	if proof == nil {
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

	return currentHash == proof.GlobalRoot
}
