// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package common

import (
	"math"
	"testing"
)

func TestSSZPrimitives(t *testing.T) {
	buf := make([]byte, 64)

	// Uint8
	PutUint8(buf[0:1], 0x42)
	if Uint8FromBytes(buf[0:1]) != 0x42 {
		t.Fatalf("Uint8 mismatch")
	}

	// Uint16 Little-Endian
	PutUint16(buf[1:3], 0x1234)
	if buf[1] != 0x34 || buf[2] != 0x12 {
		t.Fatalf("Uint16 not little-endian: %02x %02x", buf[1], buf[2])
	}
	if Uint16FromBytes(buf[1:3]) != 0x1234 {
		t.Fatalf("Uint16 mismatch")
	}

	// Uint32 Little-Endian
	PutUint32(buf[3:7], 0x12345678)
	if buf[3] != 0x78 || buf[4] != 0x56 || buf[5] != 0x34 || buf[6] != 0x12 {
		t.Fatalf("Uint32 not little-endian")
	}
	if Uint32FromBytes(buf[3:7]) != 0x12345678 {
		t.Fatalf("Uint32 mismatch")
	}

	// Int32 Little-Endian
	PutInt32(buf[7:11], -42)
	if Int32FromBytes(buf[7:11]) != -42 {
		t.Fatalf("Int32 mismatch")
	}

	// Uint64 Little-Endian
	PutUint64(buf[11:19], 0x0102030405060708)
	if buf[11] != 0x08 || buf[18] != 0x01 {
		t.Fatalf("Uint64 not little-endian")
	}
	if Uint64FromBytes(buf[11:19]) != 0x0102030405060708 {
		t.Fatalf("Uint64 mismatch")
	}

	// Float64 Little-Endian
	pi := 3.141592653589793
	PutFloat64(buf[19:27], pi)
	if math.Abs(Float64FromBytes(buf[19:27])-pi) > 1e-15 {
		t.Fatalf("Float64 mismatch")
	}
}

func TestSSZOffsetValidation(t *testing.T) {
	// Valid offsets
	valid := []uint32{8, 12, 20}
	if err := ValidateOffsets(valid, 8, 20); err != nil {
		t.Fatalf("expected valid offsets, got %v", err)
	}

	// First offset mismatch
	if err := ValidateOffsets([]uint32{10, 15}, 8, 20); err == nil {
		t.Fatalf("expected error for first offset mismatch")
	}

	// Non-monotonic offsets (overlap attack)
	if err := ValidateOffsets([]uint32{8, 6, 20}, 8, 20); err == nil {
		t.Fatalf("expected error for non-monotonic offsets")
	}

	// Last offset exceeds totalLen
	if err := ValidateOffsets([]uint32{8, 15, 25}, 8, 20); err == nil {
		t.Fatalf("expected error when offset exceeds totalLen")
	}
}

func TestSSZStringList(t *testing.T) {
	items := []string{"FederationAlpha", "FederationBravo", "TacticalSectorZ"}
	neededSize := SizeSSZStringList(items)

	buf := make([]byte, neededSize)
	endOff := EncodeSSZStringList(buf, 0, items)
	if endOff != neededSize {
		t.Fatalf("size mismatch: wrote %d, expected %d", endOff, neededSize)
	}

	decoded, err := DecodeSSZStringList(buf, 0, uint32(len(buf)))
	if err != nil {
		t.Fatalf("failed to decode string list: %v", err)
	}

	if len(decoded) != len(items) {
		t.Fatalf("count mismatch: got %d, expected %d", len(decoded), len(items))
	}
	for i := range items {
		if decoded[i] != items[i] {
			t.Fatalf("item[%d] mismatch: got %s, expected %s", i, decoded[i], items[i])
		}
	}
}

func TestSSZInt32List(t *testing.T) {
	list := []int32{10, -20, 30, -40, 50}
	buf := make([]byte, len(list)*4)
	endOff := EncodeSSZInt32List(buf, 0, list)
	if endOff != uint32(len(buf)) {
		t.Fatalf("size mismatch")
	}

	decoded, err := DecodeSSZInt32List(buf, 0, uint32(len(buf)))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for i := range list {
		if decoded[i] != list[i] {
			t.Fatalf("mismatch at %d: got %d, expected %d", i, decoded[i], list[i])
		}
	}
}
