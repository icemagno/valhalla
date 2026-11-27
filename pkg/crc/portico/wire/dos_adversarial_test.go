package wire

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

func TestJOSDecoder_RejectsNegativeAndOversizedBlockDataLong(t *testing.T) {
	cases := []int32{-1, -100, 0x7FFFFFFF}
	for _, badLen := range cases {
		var buf bytes.Buffer
		buf.WriteByte(0x7A) // TC_BLOCKDATALONG
		_ = binary.Write(&buf, binary.BigEndian, badLen)
		dec := NewJOSDecoder(buf.Bytes())
		dec.readHdr = true

		if _, err := dec.ReadBlockData(); err == nil {
			t.Fatalf("expected error for TC_BLOCKDATALONG length %d, got nil", badLen)
		}
	}
}

func TestJOSDecoder_SkipCustomData_RejectsNegativeBlockDataLong(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(0x7A) // TC_BLOCKDATALONG
	_ = binary.Write(&buf, binary.BigEndian, int32(-5))
	dec := NewJOSDecoder(buf.Bytes())
	dec.readHdr = true

	if err := dec.skipCustomData(); err == nil {
		t.Fatalf("expected error for negative TC_BLOCKDATALONG in skipCustomData, got nil")
	}
}

func TestJOSDecoder_RejectsOversizedAndNegativeArray(t *testing.T) {
	cases := []int32{-1, 0x7FFFFFFF}
	for _, badSize := range cases {
		var buf bytes.Buffer
		// Stream header + TC_ARRAY + TC_CLASSDESC("[B") + size
		buf.Write([]byte{0xAC, 0xED, 0x00, 0x05, TC_ARRAY, TC_CLASSDESC})
		_ = binary.Write(&buf, binary.BigEndian, uint16(2))
		buf.WriteString("[B")
		_ = binary.Write(&buf, binary.BigEndian, uint64(0)) // SUID
		buf.WriteByte(SC_SERIALIZABLE)
		_ = binary.Write(&buf, binary.BigEndian, uint16(0)) // fields
		buf.WriteByte(TC_ENDBLOCKDATA)
		buf.WriteByte(TC_NULL) // super
		_ = binary.Write(&buf, binary.BigEndian, badSize)

		dec := NewJOSDecoder(buf.Bytes())
		if _, err := dec.ReadObject(); err == nil {
			t.Fatalf("expected error for TC_ARRAY size %d, got nil", badSize)
		}
	}
}

func TestJOSDecoder_RejectsDeepRecursion(t *testing.T) {
	// Nested arrays of length 1, 200 levels deep
	var buf bytes.Buffer
	buf.Write([]byte{0xAC, 0xED, 0x00, 0x05})
	// First TC_ARRAY defines class "[Ljava.lang.Object;"
	buf.Write([]byte{TC_ARRAY, TC_CLASSDESC})
	name := "[Ljava.lang.Object;"
	_ = binary.Write(&buf, binary.BigEndian, uint16(len(name)))
	buf.WriteString(name)
	_ = binary.Write(&buf, binary.BigEndian, uint64(0))
	buf.WriteByte(SC_SERIALIZABLE)
	_ = binary.Write(&buf, binary.BigEndian, uint16(0))
	buf.WriteByte(TC_ENDBLOCKDATA)
	buf.WriteByte(TC_NULL)
	_ = binary.Write(&buf, binary.BigEndian, int32(1))

	for i := 0; i < 200; i++ {
		buf.Write([]byte{TC_ARRAY, TC_REFERENCE})
		_ = binary.Write(&buf, binary.BigEndian, uint32(BaseWireHandle))
		_ = binary.Write(&buf, binary.BigEndian, int32(1))
	}
	buf.WriteByte(TC_NULL)

	dec := NewJOSDecoder(buf.Bytes())
	if _, err := dec.ReadObject(); err == nil {
		t.Fatalf("expected recursion depth limit error, got nil")
	}
}

func TestJOSDecoder_RejectsInflatedHashMapAndHashSetAndArrayList(t *testing.T) {
	// HashMap with size=0x7FFFFFFF in custom block data
	var hmBuf bytes.Buffer
	hmBuf.WriteByte(TC_BLOCKDATA)
	hmBuf.WriteByte(8)
	_ = binary.Write(&hmBuf, binary.BigEndian, uint32(16))
	_ = binary.Write(&hmBuf, binary.BigEndian, uint32(0x7FFFFFFF))
	decHM := NewJOSDecoder(hmBuf.Bytes())
	decHM.readHdr = true
	if err := metamodel.ClassDescHashMap.CustomReader(decHM, metamodel.NewJavaObject(metamodel.ClassDescHashMap)); err == nil {
		t.Fatalf("expected error for inflated HashMap size, got nil")
	}

	// HashSet with size=0x7FFFFFFF
	var hsBuf bytes.Buffer
	hsBuf.WriteByte(TC_BLOCKDATA)
	hsBuf.WriteByte(12)
	_ = binary.Write(&hsBuf, binary.BigEndian, uint32(16))
	_ = binary.Write(&hsBuf, binary.BigEndian, uint32(0x3F400000))
	_ = binary.Write(&hsBuf, binary.BigEndian, uint32(0x7FFFFFFF))
	decHS := NewJOSDecoder(hsBuf.Bytes())
	decHS.readHdr = true
	if err := metamodel.ClassDescHashSet.CustomReader(decHS, metamodel.NewJavaObject(metamodel.ClassDescHashSet)); err == nil {
		t.Fatalf("expected error for inflated HashSet size, got nil")
	}

	// ArrayList with size=0x7FFFFFFF
	var alBuf bytes.Buffer
	alBuf.WriteByte(TC_BLOCKDATA)
	alBuf.WriteByte(4)
	_ = binary.Write(&alBuf, binary.BigEndian, uint32(0))
	decAL := NewJOSDecoder(alBuf.Bytes())
	decAL.readHdr = true
	alObj := metamodel.NewJavaObject(metamodel.ClassDescArrayList)
	alObj.Set("size", int32(0x7FFFFFFF))
	if err := metamodel.ClassDescArrayList.CustomReader(decAL, alObj); err == nil {
		t.Fatalf("expected error for inflated ArrayList size, got nil")
	}
}

func TestJOSEncoder_WriteBlockData_LongPayloadRoundtrip(t *testing.T) {
	payload := bytes.Repeat([]byte{0xAB}, 300) // > 255 bytes
	enc := NewJOSEncoder()
	enc.WriteBlockData(payload)

	dec := NewJOSDecoder(enc.Bytes())
	dec.readHdr = true
	got, err := dec.ReadBlockData()
	if err != nil {
		t.Fatalf("ReadBlockData failed for 300-byte payload: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("roundtrip mismatch: got %d bytes, want %d bytes", len(got), len(payload))
	}
}
