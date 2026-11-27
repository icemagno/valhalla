package metamodel

import (
	"encoding/binary"
	"math"
)

// ClassDescHashSet representa o conjunto canônico java.util.HashSet
// SUID oficial da JVM: -5024744406713321676L (0xBA44859596B8B734)
var ClassDescHashSet = &ClassDesc{
	Name:             "java.util.HashSet",
	SerialVersionUID: 0xBA44859596B8B734,
	Flags:            SC_WRITE_METHOD | SC_SERIALIZABLE,
	Fields:           nil,
	Super:            ClassDescObject,
	CustomWriter: func(w JOSWriter, obj *JavaObject) error {
		elements, _ := obj.Get("elements").([]any)

		// 1. Bloco de cabeçalho: capacity (int32), loadFactor (float32), size (int32) = 12 bytes
		var blk [12]byte
		binary.BigEndian.PutUint32(blk[0:4], 16)
		binary.BigEndian.PutUint32(blk[4:8], math.Float32bits(0.75))
		binary.BigEndian.PutUint32(blk[8:12], uint32(len(elements)))
		w.WriteBlockData(blk[:])

		// 2. Elementos individuais
		for _, el := range elements {
			if err := w.WriteObject(el); err != nil {
				return err
			}
		}

		// 3. Fim do bloco customizado do HashSet
		w.WriteByte(TC_ENDBLOCKDATA)
		return nil
	},
	CustomReader: func(r JOSReader, obj *JavaObject) error {
		blk, err := r.ReadBlockData()
		if err != nil {
			return err
		}

		size := 0
		if len(blk) >= 12 {
			size = int(binary.BigEndian.Uint32(blk[8:12]))
		}

		elements := make([]any, size)
		for i := 0; i < size; i++ {
			el, err := r.ReadObject()
			if err != nil {
				return err
			}
			elements[i] = el
		}

		// Consumir TC_ENDBLOCKDATA (0x78)
		b, err := r.ReadByte()
		if err == nil && b != TC_ENDBLOCKDATA {
			// Se não era TC_ENDBLOCKDATA, reposicionar se possível
		}

		obj.Set("elements", elements)
		return nil
	},
}

func init() {
	RegisterClassDesc(ClassDescHashSet)
}

// NewJavaHashSet cria uma nova instância de java.util.HashSet
func NewJavaHashSet(elements []any) *JavaObject {
	if elements == nil {
		elements = make([]any, 0)
	}
	boxed := make([]any, len(elements))
	for i, el := range elements {
		switch v := el.(type) {
		case int32:
			boxed[i] = NewJavaInteger(v)
		case int:
			boxed[i] = NewJavaInteger(int32(v))
		case float64:
			boxed[i] = NewJavaDouble(v)
		case bool:
			boxed[i] = NewJavaBoolean(v)
		default:
			boxed[i] = v
		}
	}
	obj := NewJavaObject(ClassDescHashSet)
	obj.Set("elements", boxed)
	return obj
}
