package common

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// TestSSZGoldenVectors_HandConstructed verifies exact wire layout against independent,
// hand-constructed canonical SSZ byte vectors (no circular self-serialization).
func TestSSZGoldenVectors_HandConstructed(t *testing.T) {
	t.Run("JoinFederationPayload", func(t *testing.T) {
		// Fixed header (12 bytes):
		//   offset0 = 12 (0x0C,0,0,0) -> "FedA" (len 4)
		//   offset1 = 16 (0x10,0,0,0) -> "Type1" (len 5)
		//   offset2 = 21 (0x15,0,0,0) -> "Sim" (len 3)
		golden := []byte{
			0x0C, 0x00, 0x00, 0x00,
			0x10, 0x00, 0x00, 0x00,
			0x15, 0x00, 0x00, 0x00,
			'F', 'e', 'd', 'A',
			'T', 'y', 'p', 'e', '1',
			'S', 'i', 'm',
		}

		p := JoinFederationPayload{
			FederateName:   "FedA",
			FederateType:   "Type1",
			FederationName: "Sim",
		}
		got, err := p.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		if !bytes.Equal(got, golden) {
			t.Fatalf("MarshalBinary mismatch:\n got: %x\nwant: %x", got, golden)
		}

		var decoded JoinFederationPayload
		if err := decoded.UnmarshalBinary(golden); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if decoded != p {
			t.Fatalf("UnmarshalBinary mismatch: got %+v, want %+v", decoded, p)
		}
	})

	t.Run("DiscoverObjectInstancePayload", func(t *testing.T) {
		// Fixed header (12 bytes):
		//   ObjectHandle = 42 (0x2A,0,0,0)
		//   ClassHandle  = 1001 (0xE9,0x03,0,0)
		//   offset0      = 12 (0x0C,0,0,0) -> "Tank1"
		golden := []byte{
			0x2A, 0x00, 0x00, 0x00,
			0xE9, 0x03, 0x00, 0x00,
			0x0C, 0x00, 0x00, 0x00,
			'T', 'a', 'n', 'k', '1',
		}

		p := DiscoverObjectInstancePayload{
			ObjectHandle: 42,
			ClassHandle:  1001,
			ObjectName:   "Tank1",
		}
		got, err := p.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		if !bytes.Equal(got, golden) {
			t.Fatalf("MarshalBinary mismatch:\n got: %x\nwant: %x", got, golden)
		}

		var decoded DiscoverObjectInstancePayload
		if err := decoded.UnmarshalBinary(golden); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if decoded != p {
			t.Fatalf("UnmarshalBinary mismatch: got %+v, want %+v", decoded, p)
		}
	})

	t.Run("UpdateAttributesPayload_CanonicalOrder", func(t *testing.T) {
		// Fixed header (21 bytes):
		//   ObjectHandle = 7 (LE)
		//   HasTime      = 0x01
		//   Time         = 2.5 (IEEE-754 float64 LE)
		//   Offset0(Tag) = 21
		//   Offset1(Attrs)= 23 (after 2-byte tag "OK")
		// Dynamic Attrs (2 entries sorted ascending: handle 10 -> [0xAA], handle 20 -> [0xBB, 0xCC]):
		//   List offset table (8 bytes): elem0=8, elem1=17
		//   Elem0 (9 bytes): handle=10 (4), valOff=8 (4), val=[0xAA] (1)
		//   Elem1 (10 bytes): handle=20 (4), valOff=8 (4), val=[0xBB, 0xCC] (2)
		var golden []byte
		golden = binary.LittleEndian.AppendUint32(golden, 7)
		golden = append(golden, 0x01)
		golden = binary.LittleEndian.AppendUint64(golden, math.Float64bits(2.5))
		golden = binary.LittleEndian.AppendUint32(golden, 21)
		golden = binary.LittleEndian.AppendUint32(golden, 23)
		golden = append(golden, 'O', 'K')
		// Attrs list offset table
		golden = binary.LittleEndian.AppendUint32(golden, 8)
		golden = binary.LittleEndian.AppendUint32(golden, 17)
		// Elem 0 (handle 10)
		golden = binary.LittleEndian.AppendUint32(golden, 10)
		golden = binary.LittleEndian.AppendUint32(golden, 8)
		golden = append(golden, 0xAA)
		// Elem 1 (handle 20)
		golden = binary.LittleEndian.AppendUint32(golden, 20)
		golden = binary.LittleEndian.AppendUint32(golden, 8)
		golden = append(golden, 0xBB, 0xCC)

		p := UpdateAttributesPayload{
			ObjectHandle: 7,
			HasTime:      true,
			Time:         2.5,
			Tag:          []byte("OK"),
			Attributes: map[AttributeHandle][]byte{
				20: {0xBB, 0xCC},
				10: {0xAA},
			},
		}
		got, err := p.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		if !bytes.Equal(got, golden) {
			t.Fatalf("MarshalBinary mismatch:\n got: %x\nwant: %x", got, golden)
		}

		var decoded UpdateAttributesPayload
		if err := decoded.UnmarshalBinary(golden); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if decoded.ObjectHandle != 7 || !decoded.HasTime || decoded.Time != 2.5 || string(decoded.Tag) != "OK" {
			t.Fatalf("decoded header mismatch: %+v", decoded)
		}
		if !bytes.Equal(decoded.Attributes[10], []byte{0xAA}) || !bytes.Equal(decoded.Attributes[20], []byte{0xBB, 0xCC}) {
			t.Fatalf("decoded attributes mismatch: %+v", decoded.Attributes)
		}
	})

	t.Run("SendInteractionPayload_CanonicalOrder", func(t *testing.T) {
		// Same 21-byte fixed header as UpdateAttributesPayload:
		//   InteractionClass = 1005 (LE)
		//   HasTime          = 0x00
		//   Time             = 0.0
		//   Offset0(Tag)     = 21
		//   Offset1(Params)  = 24 (after 3-byte tag "INT")
		// Params list: handle 1 -> [0x01, 0x02]
		var golden []byte
		golden = binary.LittleEndian.AppendUint32(golden, 1005)
		golden = append(golden, 0x00)
		golden = binary.LittleEndian.AppendUint64(golden, 0)
		golden = binary.LittleEndian.AppendUint32(golden, 21)
		golden = binary.LittleEndian.AppendUint32(golden, 24)
		golden = append(golden, 'I', 'N', 'T')
		// Params list offset table (1 elem -> offset 4)
		golden = binary.LittleEndian.AppendUint32(golden, 4)
		// Elem 0 (handle 1)
		golden = binary.LittleEndian.AppendUint32(golden, 1)
		golden = binary.LittleEndian.AppendUint32(golden, 8)
		golden = append(golden, 0x01, 0x02)

		p := SendInteractionPayload{
			ClassHandle: 1005,
			HasTime:     false,
			Time:        0,
			Tag:         []byte("INT"),
			Parameters: map[ParameterHandle][]byte{
				1: {0x01, 0x02},
			},
		}
		got, err := p.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		if !bytes.Equal(got, golden) {
			t.Fatalf("MarshalBinary mismatch:\n got: %x\nwant: %x", got, golden)
		}

		var decoded SendInteractionPayload
		if err := decoded.UnmarshalBinary(golden); err != nil {
			t.Fatalf("UnmarshalBinary failed: %v", err)
		}
		if decoded.ClassHandle != 1005 || decoded.HasTime || string(decoded.Tag) != "INT" || !bytes.Equal(decoded.Parameters[1], []byte{0x01, 0x02}) {
			t.Fatalf("decoded interaction mismatch: %+v", decoded)
		}
	})
}
