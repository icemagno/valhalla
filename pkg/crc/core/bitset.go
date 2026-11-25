package core

import (
	"math/bits"

	"github.com/icemagno/valhalla/pkg/common"
)

// Bitset implements a dynamic, high-performance sparse bitset
// optimized for federate interest matching and routing in massive simulations.
// Backed by contiguous uint64 blocks with on-demand expansion and vectorized lookups via bits.TrailingZeros64.
type Bitset struct {
	words []uint64
}

// NewBitset initializes an empty bitset with an optional initial word capacity
func NewBitset(initialWords ...int) *Bitset {
	capWords := 2 // 128 bits default (covers up to 128 federates in 16 bytes)
	if len(initialWords) > 0 && initialWords[0] > 0 {
		capWords = initialWords[0]
	}
	return &Bitset{
		words: make([]uint64, 0, capWords),
	}
}

// ensureCapacity ensures the words slice can hold the requested word index
func (b *Bitset) ensureCapacity(wordIdx int) {
	if wordIdx >= len(b.words) {
		newLen := wordIdx + 1
		if newLen <= cap(b.words) {
			b.words = b.words[:newLen]
		} else {
			expanded := make([]uint64, newLen, (newLen*3)/2+2)
			copy(expanded, b.words)
			b.words = expanded
		}
	}
}

// Set sets the bit corresponding to the specified ID (0-based or 1-based)
func (b *Bitset) Set(id uint32) {
	wordIdx := int(id >> 6) // id / 64
	bitIdx := id & 63       // id % 64
	b.ensureCapacity(wordIdx)
	b.words[wordIdx] |= (1 << bitIdx)
}

// Clear clears the bit corresponding to the specified ID
func (b *Bitset) Clear(id uint32) {
	wordIdx := int(id >> 6)
	if wordIdx < len(b.words) {
		bitIdx := id & 63
		b.words[wordIdx] &^= (1 << bitIdx)
	}
}

// Contains checks if the bit corresponding to the specified ID is set
func (b *Bitset) Contains(id uint32) bool {
	wordIdx := int(id >> 6)
	if wordIdx >= len(b.words) {
		return false
	}
	bitIdx := id & 63
	return (b.words[wordIdx] & (1 << bitIdx)) != 0
}

// IsEmpty returns true if no bits are set
func (b *Bitset) IsEmpty() bool {
	for _, w := range b.words {
		if w != 0 {
			return false
		}
	}
	return true
}

// Count returns the total number of active bits (popcount)
func (b *Bitset) Count() int {
	total := 0
	for _, w := range b.words {
		total += bits.OnesCount64(w)
	}
	return total
}

// Union performs an in-place union: b = b OR other
func (b *Bitset) Union(other *Bitset) {
	if other == nil || len(other.words) == 0 {
		return
	}
	if len(other.words) > len(b.words) {
		b.ensureCapacity(len(other.words) - 1)
	}
	for i := 0; i < len(other.words); i++ {
		b.words[i] |= other.words[i]
	}
}

// Intersect performs an in-place intersection: b = b AND other
func (b *Bitset) Intersect(other *Bitset) {
	if other == nil {
		b.Reset()
		return
	}
	minLen := len(b.words)
	if len(other.words) < minLen {
		minLen = len(other.words)
	}
	for i := 0; i < minLen; i++ {
		b.words[i] &= other.words[i]
	}
	// Zero out any remaining words beyond other's length
	for i := minLen; i < len(b.words); i++ {
		b.words[i] = 0
	}
}

// Clone creates a deep copy of the Bitset
func (b *Bitset) Clone() *Bitset {
	clone := &Bitset{
		words: make([]uint64, len(b.words)),
	}
	copy(clone.words, b.words)
	return clone
}

// Reset clears all bits without reallocating the underlying slice
func (b *Bitset) Reset() {
	for i := range b.words {
		b.words[i] = 0
	}
}

// ToFederateHandles extracts all active IDs as a slice of FederateHandle.
// Uses bits.TrailingZeros64 to skip directly to set bits in O(k) time,
// where k is the number of active bits, without iterating over zero words.
func (b *Bitset) ToFederateHandles() []common.FederateHandle {
	count := b.Count()
	if count == 0 {
		return nil
	}

	handles := make([]common.FederateHandle, 0, count)
	for wordIdx, w := range b.words {
		for w != 0 {
			tz := bits.TrailingZeros64(w)
			id := uint32(wordIdx<<6) + uint32(tz)
			handles = append(handles, common.FederateHandle(id))
			w &^= (1 << tz) // Clear the processed lowest bit
		}
	}
	return handles
}
