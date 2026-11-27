package wire

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

func TestJOSDecoder_Adversarial_BlockDataLongBounds(t *testing.T) {
	cases := []struct {
		name string
		iLen int32
	}{
		{"NegativeLength", -1},
		{"MinInt32", -2147483648},
		{"MaxInt32_2GiB", 0x7FFFFFFF},
		{"ExceedsMaxAllocation", int32(maxJOSAllocation + 1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			buf.Write([]byte{0xAC, 0xED, 0x00, 0x05}) // STREAM_MAGIC + STREAM_VERSION
			buf.WriteByte(0x7A)                       // TC_BLOCKDATALONG
			_ = binary.Write(&buf, binary.BigEndian, tc.iLen)

			dec := NewJOSDecoder(buf.Bytes())
			_, err := dec.ReadObject()
			if err == nil {
				t.Fatalf("expected error for TC_BLOCKDATALONG iLen=%d, got nil", tc.iLen)
			}
		})
	}
}

func TestJOSDecoder_Adversarial_ClassDescRecursionDepth(t *testing.T) {
	// Build a deeply nested TC_CLASSDESC superclass chain exceeding maxJOSDepth
	var buf bytes.Buffer
	buf.Write([]byte{0xAC, 0xED, 0x00, 0x05}) // STREAM_MAGIC + STREAM_VERSION
	buf.WriteByte(TC_OBJECT)

	depth := maxJOSDepth + 20
	for i := 0; i < depth; i++ {
		buf.WriteByte(TC_CLASSDESC)
		name := "C"
		_ = binary.Write(&buf, binary.BigEndian, uint16(len(name)))
		buf.WriteString(name)
		_ = binary.Write(&buf, binary.BigEndian, uint64(1)) // SUID
		buf.WriteByte(SC_SERIALIZABLE)                       // flags
		_ = binary.Write(&buf, binary.BigEndian, uint16(0))  // numFields = 0
		buf.WriteByte(TC_ENDBLOCKDATA)                       // classAnnotation end
	}
	buf.WriteByte(TC_NULL) // terminate superclass chain

	dec := NewJOSDecoder(buf.Bytes())
	_, err := dec.ReadObject()
	if err == nil {
		t.Fatalf("expected max recursion depth error for %d nested TC_CLASSDESC, got nil", depth)
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Fatalf("expected recursion depth error message, got: %v", err)
	}
}

func TestJOSEncoder_Adversarial_StringOverflowUint16(t *testing.T) {
	oversized := strings.Repeat("A", 65536) // 65536 overflows uint16 -> 0 if unchecked
	enc := NewJOSEncoder()
	err := enc.WriteObject(oversized)
	if err == nil {
		t.Fatalf("expected error when encoding string of length %d (> uint16 max), got nil", len(oversized))
	}
}

func TestClassDescHashMap_Determinism100Iterations(t *testing.T) {
	// Custom JavaObject class with multiple fields to trigger Go map iteration randomization
	keyClass := &metamodel.ClassDesc{
		Name:             "test.CompositeKey",
		SerialVersionUID: 0x123456789ABCDEF0,
		Flags:            metamodel.SC_SERIALIZABLE,
		Fields: []metamodel.FieldDesc{
			{Name: "alpha", Type: metamodel.TypeInt},
			{Name: "beta", Type: metamodel.TypeInt},
			{Name: "gamma", Type: metamodel.TypeInt},
			{Name: "delta", Type: metamodel.TypeInt},
		},
	}

	buildHashMap := func() *metamodel.JavaObject {
		entries := make(map[any]any)
		for i := int32(0); i < 8; i++ {
			k := metamodel.NewJavaObject(keyClass).
				Set("alpha", i).
				Set("beta", i*10).
				Set("gamma", i*100).
				Set("delta", i*1000)
			entries[k] = metamodel.NewJavaInteger(i)
		}
		hm := metamodel.NewJavaObject(metamodel.ClassDescHashMap).
			Set("loadFactor", float32(0.75)).
			Set("threshold", int32(12)).
			Set("entries", entries)
		return hm
	}

	enc0 := NewJOSEncoder()
	if err := enc0.WriteObject(buildHashMap()); err != nil {
		t.Fatalf("initial encode failed: %v", err)
	}
	baseline := append([]byte(nil), enc0.Bytes()...)

	for iter := 0; iter < 100; iter++ {
		enc := NewJOSEncoder()
		if err := enc.WriteObject(buildHashMap()); err != nil {
			t.Fatalf("iter %d encode failed: %v", iter, err)
		}
		if !bytes.Equal(baseline, enc.Bytes()) {
			t.Fatalf("non-deterministic HashMap serialization detected at iteration %d", iter)
		}
	}
}
