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

func TestSSZStrictBooleans(t *testing.T) {
	// 0x00 and 0x01 are strictly allowed
	b, err := DecodeBool(0x00)
	if err != nil || b != false {
		t.Fatalf("expected false, nil; got %v, %v", b, err)
	}
	b, err = DecodeBool(0x01)
	if err != nil || b != true {
		t.Fatalf("expected true, nil; got %v, %v", b, err)
	}

	// Any other byte MUST return ErrSSZInvalidBoolean
	for _, invalid := range []byte{0x02, 0x03, 0x10, 0x80, 0xFF} {
		if _, err := DecodeBool(invalid); err == nil {
			t.Fatalf("expected error for invalid bool byte 0x%02X, got nil", invalid)
		}
	}

	// Payload test: tampering boolean in UpdateAttributesPayload
	p := &UpdateAttributesPayload{
		ObjectHandle: 100,
		HasTime:      true,
		Time:         12.34,
		Attributes: map[AttributeHandle][]byte{
			1: []byte("val1"),
		},
	}
	encoded, err := p.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Offset 4 is HasTime in UpdateAttributesPayload
	corrupted := make([]byte, len(encoded))
	copy(corrupted, encoded)
	corrupted[4] = 0x02 // illegal boolean value

	p2 := &UpdateAttributesPayload{}
	if err := p2.UnmarshalBinary(corrupted); err == nil {
		t.Fatalf("expected unmarshal to reject boolean 0x02, but succeeded")
	}
}

func TestSSZDeterminism(t *testing.T) {
	// Canonical SSZ requirement: Map serialization MUST be 100% deterministic
	// across iterations despite Go map randomization.
	p := &UpdateAttributesPayload{
		ObjectHandle: 55,
		HasTime:      true,
		Time:         123.456,
		Attributes: map[AttributeHandle][]byte{
			999: []byte("omega"),
			1:   []byte("alpha"),
			42:  []byte("middle"),
			7:   []byte("beta"),
			500: []byte("zeta"),
		},
	}

	firstBytes, err := p.MarshalBinary()
	if err != nil {
		t.Fatalf("initial marshal failed: %v", err)
	}

	for i := 0; i < 200; i++ {
		bytes, err := p.MarshalBinary()
		if err != nil {
			t.Fatalf("marshal at iteration %d failed: %v", i, err)
		}
		if string(bytes) != string(firstBytes) {
			t.Fatalf("non-deterministic SSZ serialization detected at iteration %d", i)
		}
	}

	// Same for SendInteractionPayload
	sip := &SendInteractionPayload{
		ClassHandle: 77,
		HasTime:     true,
		Time:        999.0,
		Parameters: map[ParameterHandle][]byte{
			100: []byte("param100"),
			5:   []byte("param5"),
			50:  []byte("param50"),
			1:   []byte("param1"),
		},
	}
	firstSIP, err := sip.MarshalBinary()
	if err != nil {
		t.Fatalf("sip marshal failed: %v", err)
	}
	for i := 0; i < 200; i++ {
		bytes, err := sip.MarshalBinary()
		if err != nil {
			t.Fatalf("sip marshal at %d failed: %v", i, err)
		}
		if string(bytes) != string(firstSIP) {
			t.Fatalf("non-deterministic interaction serialization detected at %d", i)
		}
	}
}

func TestSSZTrailingBytesRejection(t *testing.T) {
	// 1. Fixed container: TimeFloat64Payload (exactly 8 bytes)
	timeP := &TimeFloat64Payload{Time: 42.5}
	timeData, err := timeP.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if len(timeData) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(timeData))
	}

	// Appending trailing garbage (0xDEADBEEF) must fail with ErrSSZTrailingBytes
	withGarbage := append(timeData, 0xDE, 0xAD, 0xBE, 0xEF)
	timeDec := &TimeFloat64Payload{}
	if err := timeDec.UnmarshalBinary(withGarbage); err == nil {
		t.Fatalf("expected error on trailing bytes for TimeFloat64Payload, got nil")
	}

	// 2. Fixed container: JoinFederationResponsePayload (exactly 8 bytes)
	joinResp := &JoinFederationResponsePayload{FederateHandle: 10, FederationHandle: 20}
	joinData, err := joinResp.MarshalBinary()
	if err != nil {
		t.Fatalf("joinResp marshal failed: %v", err)
	}
	joinDec := &JoinFederationResponsePayload{}
	if err := joinDec.UnmarshalBinary(append(joinData, 0xDE, 0xAD, 0xBE, 0xEF)); err == nil {
		t.Fatalf("expected error on trailing bytes for JoinFederationResponsePayload, got nil")
	}

	// 3. Fixed container: InteractionClassPayload (exactly 4 bytes)
	icP := &InteractionClassPayload{ClassHandle: 99}
	icData, err := icP.MarshalBinary()
	if err != nil {
		t.Fatalf("ic marshal failed: %v", err)
	}
	icDec := &InteractionClassPayload{}
	if err := icDec.UnmarshalBinary(append(icData, 0xCA, 0xFE)); err == nil {
		t.Fatalf("expected error on trailing bytes for InteractionClassPayload, got nil")
	}

	// 4. Fixed container: QueryOwnershipPayload (exactly 8 bytes)
	qoP := &QueryOwnershipPayload{ObjectHandle: 1, AttributeHandle: 2}
	qoData, err := qoP.MarshalBinary()
	if err != nil {
		t.Fatalf("qo marshal failed: %v", err)
	}
	qoDec := &QueryOwnershipPayload{}
	if err := qoDec.UnmarshalBinary(append(qoData, 0x01, 0x02)); err == nil {
		t.Fatalf("expected error on trailing bytes for QueryOwnershipPayload, got nil")
	}
}

func TestSSZAntiOOMDoS(t *testing.T) {
	// A malicious packet claiming an immense offset / count in a tiny buffer
	// e.g. 25-byte packet claiming first offset of 20,000,000 * 4 bytes
	malicious := make([]byte, 25)
	Endian.PutUint32(malicious[0:4], 1) // ObjectHandle
	malicious[4] = 0x00                 // HasTime = false
	Endian.PutUint64(malicious[5:13], 0) // Time
	Endian.PutUint32(malicious[13:17], 21) // TagOffset = 21 (valid start)
	Endian.PutUint32(malicious[17:21], 21) // AttrOffset = 21
	// Now at byte 21, try to forge an offset table with huge fake count:
	Endian.PutUint32(malicious[21:25], 80_000_000)

	p := &UpdateAttributesPayload{}
	err := p.UnmarshalBinary(malicious)
	if err == nil {
		t.Fatalf("expected error on malicious OOM packet, got nil")
	}
}

func TestSSZIntegerOverflowAttack(t *testing.T) {
	// Attempt to trigger uint32 wraparound with 0xFFFFFFF0
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PANIC on uint32 overflow: %v", r)
		}
	}()

	malicious := make([]byte, 32)
	// Fill with overflow offsets
	for i := 0; i < len(malicious); i += 4 {
		Endian.PutUint32(malicious[i:i+4], 0xFFFFFFF0)
	}

	p := &UpdateAttributesPayload{}
	_ = p.UnmarshalBinary(malicious)

	p2 := &SendInteractionPayload{}
	_ = p2.UnmarshalBinary(malicious)

	p3 := &UpdateAttributesSpatialPayload{}
	_ = p3.UnmarshalBinary(malicious)
}

func TestSSZMerkleization(t *testing.T) {
	// Test basic Merkleization primitives
	h1 := HashTreeRootUint32(42)
	h2 := HashTreeRootUint32(42)
	if h1 != h2 {
		t.Fatalf("expected identical root for same scalar")
	}

	h3 := HashTreeRootUint32(43)
	if h1 == h3 {
		t.Fatalf("different values must produce different root")
	}

	// Byte list merkleization
	data := []byte("Project Valhalla Sovereign HLA-4")
	root1 := HashTreeRootByteList(data, 1024)
	root2 := HashTreeRootByteList(data, 1024)
	if root1 != root2 {
		t.Fatalf("byte list merkleization not deterministic")
	}

	// Pair hashing
	pair := HashPair(h1, h3)
	if pair == [32]byte{} {
		t.Fatalf("HashPair returned zero hash")
	}
}

func TestSSZListFirstOffsetZeroRejection(t *testing.T) {
	// A non-empty buffer (byteLen >= 4) with firstOffset == 0 followed by trailing garbage
	// MUST be rejected with ErrSSZInvalidOffset and NOT accepted as an empty list.
	malicious := make([]byte, 16)
	Endian.PutUint32(malicious[0:4], 0) // firstOffset = 0
	for i := 4; i < 16; i++ {
		malicious[i] = 0xAA // trailing garbage
	}

	// 1. DecodeSSZStringList
	if _, err := DecodeSSZStringList(malicious, 0, uint32(len(malicious))); err == nil {
		t.Fatalf("DecodeSSZStringList accepted non-empty buffer with firstOffset == 0")
	}

	// 2. DecodeSSZAttributeList
	if _, err := DecodeSSZAttributeList(malicious, 0, uint32(len(malicious))); err == nil {
		t.Fatalf("DecodeSSZAttributeList accepted non-empty buffer with firstOffset == 0")
	}

	// 3. DecodeSSZParameterList
	if _, err := DecodeSSZParameterList(malicious, 0, uint32(len(malicious))); err == nil {
		t.Fatalf("DecodeSSZParameterList accepted non-empty buffer with firstOffset == 0")
	}
}

func BenchmarkHashTreeRootByteList_1MB(b *testing.B) {
	data := []byte("AttributeValueBenchmarkDataPayload1234567890")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = HashTreeRootByteList(data, 1024*1024)
	}
}
