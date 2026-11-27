package common

import (
	"errors"
	"math"
	"testing"
)

func TestHashTreeRootByteList_NoOverflowAtMaxUint64(t *testing.T) {
	data := []byte("valhalla-hla4")
	rootNoLimit := HashTreeRootByteList(data, 0)
	rootMaxLimit := HashTreeRootByteList(data, math.MaxUint64)

	// If (limit + 31) / 32 overflows uint64, chunkLimit wraps to 0 and produces
	// the exact same root as limit=0 instead of depth 59!
	if rootMaxLimit == rootNoLimit {
		t.Fatalf("uint64 overflow in HashTreeRootByteList: limit=MaxUint64 wrapped to 0 and matched limit=0")
	}
}

func TestSSZPrimitiveDecoders_StrictErrorOnShortSlice(t *testing.T) {
	if _, err := DecodeUint8(nil); !errors.Is(err, ErrSSZBufferTooShort) {
		t.Fatalf("DecodeUint8(nil): want ErrSSZBufferTooShort, got %v", err)
	}
	if _, err := DecodeUint16([]byte{0x01}); !errors.Is(err, ErrSSZBufferTooShort) {
		t.Fatalf("DecodeUint16(1 byte): want ErrSSZBufferTooShort, got %v", err)
	}
	if _, err := DecodeUint32([]byte{1, 2, 3}); !errors.Is(err, ErrSSZBufferTooShort) {
		t.Fatalf("DecodeUint32(3 bytes): want ErrSSZBufferTooShort, got %v", err)
	}
	if _, err := DecodeUint64([]byte{1, 2, 3, 4, 5, 6, 7}); !errors.Is(err, ErrSSZBufferTooShort) {
		t.Fatalf("DecodeUint64(7 bytes): want ErrSSZBufferTooShort, got %v", err)
	}
	if _, err := DecodeInt32([]byte{1, 2, 3}); !errors.Is(err, ErrSSZBufferTooShort) {
		t.Fatalf("DecodeInt32(3 bytes): want ErrSSZBufferTooShort, got %v", err)
	}
	if _, err := DecodeFloat64([]byte{1, 2, 3, 4, 5, 6, 7}); !errors.Is(err, ErrSSZBufferTooShort) {
		t.Fatalf("DecodeFloat64(7 bytes): want ErrSSZBufferTooShort, got %v", err)
	}
	if _, err := DecodeOffset([]byte{1, 2, 3}); !errors.Is(err, ErrSSZBufferTooShort) {
		t.Fatalf("DecodeOffset(3 bytes): want ErrSSZBufferTooShort, got %v", err)
	}
}

func TestSSZSafeUint32Checks_RejectOverflow(t *testing.T) {
	if _, err := CheckedSSZUint32(uint64(math.MaxUint32) + 1); !errors.Is(err, ErrSSZMemoryOverflow) {
		t.Fatalf("CheckedSSZUint32(MaxUint32+1): want ErrSSZMemoryOverflow, got %v", err)
	}
	if _, err := CheckedSSZSize(uint64(MaxNativePayloadLen) + 1); !errors.Is(err, ErrSSZMemoryOverflow) {
		t.Fatalf("CheckedSSZSize(MaxNativePayloadLen+1): want ErrSSZMemoryOverflow, got %v", err)
	}
	if v, err := CheckedSSZSize(uint64(MaxNativePayloadLen)); err != nil || v != MaxNativePayloadLen {
		t.Fatalf("CheckedSSZSize(MaxNativePayloadLen): unexpected (%d, %v)", v, err)
	}
}
