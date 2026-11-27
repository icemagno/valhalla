package common

import (
	"math"
	"testing"
)

func TestNextPowerOfTwo_NoInfiniteLoopOnExtremeValues(t *testing.T) {
	cases := []int{0, -1, 1, 2, 3, 1 << 61, 1 << 62, (1 << 62) + 1, math.MaxInt}
	for _, n := range cases {
		p := NextPowerOfTwo(n)
		if p < 1 {
			t.Fatalf("NextPowerOfTwo(%d) returned invalid value %d", n, p)
		}
	}
}

func TestMerkleize_ExtremeLimitDoesNotHangOrPanic(t *testing.T) {
	chunk := [32]byte{1, 2, 3}
	root := Merkleize([][32]byte{chunk}, uint64(1)<<63)
	if root == ([32]byte{}) {
		t.Fatalf("expected non-zero merkle root for single non-zero chunk")
	}
	rootMax := Merkleize([][32]byte{chunk}, math.MaxUint64)
	if rootMax == ([32]byte{}) {
		t.Fatalf("expected non-zero merkle root for MaxUint64 limit")
	}
}
