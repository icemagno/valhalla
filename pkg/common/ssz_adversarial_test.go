package common

import (
	"math"
	"testing"
)

func TestNextPowerOfTwo_ExactValuesAndNoInfiniteLoop(t *testing.T) {
	const maxPow2 = 1 << 62
	cases := []struct {
		in   int
		want int
	}{
		{-100, 1},
		{-1, 1},
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{1 << 61, 1 << 61},
		{maxPow2, maxPow2},
		{maxPow2 + 1, maxPow2},
		{math.MaxInt, maxPow2},
	}
	for _, tc := range cases {
		got := NextPowerOfTwo(tc.in)
		if got != tc.want {
			t.Fatalf("NextPowerOfTwo(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestMerkleize_ExtremeLimitMatchesExactReferenceFolding(t *testing.T) {
	chunk := [32]byte{1, 2, 3}

	// Independent reference: 1 chunk folded to depth 63 (limit = 1<<63)
	expected63 := chunk
	for level := 0; level < 63; level++ {
		expected63 = HashPair(expected63, zeroHashes[level])
	}
	got63 := Merkleize([][32]byte{chunk}, uint64(1)<<63)
	if got63 != expected63 {
		t.Fatalf("Merkleize(limit=1<<63) mismatch: got %x, want %x", got63, expected63)
	}

	// Independent reference: 1 chunk folded to depth 64 (limit = MaxUint64)
	expected64 := HashPair(expected63, zeroHashes[63])
	gotMax := Merkleize([][32]byte{chunk}, math.MaxUint64)
	if gotMax != expected64 {
		t.Fatalf("Merkleize(limit=MaxUint64) mismatch: got %x, want %x", gotMax, expected64)
	}
}
