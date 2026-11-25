package metamodel

import (
	"encoding/binary"
	"math"
)

// ClassDescHashMap representa a coleção canônica java.util.HashMap
// SUID oficial da JVM: 362498820763181265L (0x0507DAC1C31660D1)
var ClassDescHashMap = &ClassDesc{
	Name:             "java.util.HashMap",
	SerialVersionUID: 0x0507DAC1C31660D1,
	Flags:            SC_WRITE_METHOD | SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "loadFactor", Type: TypeFloat},
		{Name: "threshold", Type: TypeInt},
	},
	Super: ClassDescObject,
	CustomWriter: func(w JOSWriter, obj *JavaObject) error {
		// HashMap serializa entries customizadas em bloco após defaultWriteObject
		var count int
		switch m := obj.Get("entries").(type) {
		case map[string]any:
			count = len(m)
		case map[any]any:
			count = len(m)
		}

		// 1. Bloco de cabeçalho da tabela: capacity (16) + size
		var blk [8]byte
		binary.BigEndian.PutUint32(blk[0:4], 16)
		binary.BigEndian.PutUint32(blk[4:8], uint32(count))
		w.WriteBlockData(blk[:])

		// 2. Escrever pares alternados (Key -> Value)
		switch m := obj.Get("entries").(type) {
		case map[string]any:
			for k, v := range m {
				if err := w.WriteObject(k); err != nil {
					return err
				}
				if err := w.WriteObject(v); err != nil {
					return err
				}
			}
		case map[any]any:
			for k, v := range m {
				if err := w.WriteObject(k); err != nil {
					return err
				}
				if err := w.WriteObject(v); err != nil {
					return err
				}
			}
		}

		// 3. Fim do bloco customizado do HashMap
		w.WriteByte(TC_ENDBLOCKDATA)
		return nil
	},
	CustomReader: func(r JOSReader, obj *JavaObject) error {
		blk, err := r.ReadBlockData()
		if err != nil {
			return err
		}

		size := 0
		if len(blk) >= 8 {
			size = int(binary.BigEndian.Uint32(blk[4:8]))
		}

		entries := make(map[any]any, size)
		for i := 0; i < size; i++ {
			k, err := r.ReadObject()
			if err != nil {
				return err
			}
			v, err := r.ReadObject()
			if err != nil {
				return err
			}
			entries[k] = v
		}

		// Consumir TC_ENDBLOCKDATA se presente
		b, err := r.ReadByte()
		if err == nil && b != TC_ENDBLOCKDATA {
			// Se o byte não for TC_ENDBLOCKDATA, não consome
		}

		obj.Set("entries", entries)
		return nil
	},
}

func init() {
	RegisterClassDesc(ClassDescHashMap)
}

// NewJavaHashMap cria uma nova instância de java.util.HashMap inicializada
func NewJavaHashMap(entries map[string]any) *JavaObject {
	if entries == nil {
		entries = make(map[string]any)
	}
	obj := NewJavaObject(ClassDescHashMap)
	obj.Set("loadFactor", math.Float32frombits(0x3F400000)) // 0.75f
	obj.Set("threshold", int32(12))
	obj.Set("entries", entries)
	return obj
}
