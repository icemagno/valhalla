// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package core

import "github.com/icemagno/valhalla/pkg/common"

var (
	ErrObjectNotFoundInMerkleTree = common.ErrObjectNotFoundInMerkleTree
	ErrInvalidMerkleProof         = common.ErrInvalidMerkleProof
)

// Re-export canonical Merkle types from pkg/common for server engine compatibility
type MerkleProof = common.MerkleProof
type FederationStateTree = common.FederationStateTree

var (
	NewFederationStateTree  = common.NewFederationStateTree
	ComputeAttributeLeaf    = common.ComputeAttributeLeaf
	ComputeObjectMerkleRoot = common.ComputeObjectMerkleRoot
	HashPair                = common.HashPair
	VerifyMerkleProof       = common.VerifyMerkleProof
)
