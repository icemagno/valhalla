package metamodel

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
)

func canonicalMapSortKey(v any, depth int) string {
	if depth > 8 {
		return fmt.Sprintf("%T", v)
	}
	switch x := v.(type) {
	case nil:
		return "nil"
	case string:
		return "str:" + x
	case int32:
		return fmt.Sprintf("int32:%011d", int64(x)+2147483648)
	case int64:
		return fmt.Sprintf("int64:%020d", uint64(x)^uint64(1<<63))
	case *JavaObject:
		if x == nil {
			return "obj:nil"
		}
		if iv, ok := x.Get("value").(int32); ok && len(x.Fields) == 1 {
			return fmt.Sprintf("int32:%011d", int64(iv)+2147483648)
		}
		var b strings.Builder
		b.WriteString("obj:")
		if x.Class != nil {
			b.WriteString(x.Class.Name)
		}
		b.WriteByte('{')
		fkeys := make([]string, 0, len(x.Fields))
		for fk := range x.Fields {
			fkeys = append(fkeys, fk)
		}
		sort.Strings(fkeys)
		for i, fk := range fkeys {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(fk)
			b.WriteByte('=')
			b.WriteString(canonicalMapSortKey(x.Fields[fk], depth+1))
		}
		b.WriteByte('}')
		return b.String()
	case *JavaEnum:
		if x == nil {
			return "enum:nil"
		}
		clsName := ""
		if x.Class != nil {
			clsName = x.Class.Name
		}
		return "enum:" + clsName + ":" + x.Name
	default:
		return fmt.Sprintf("%T:%v", v, v)
	}
}

const maxMetamodelCollectionSize = 100000

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

		// 2. Escrever pares alternados (Key -> Value) em ordem determinística
		switch m := obj.Get("entries").(type) {
		case map[string]any:
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if err := w.WriteObject(k); err != nil {
					return err
				}
				if err := w.WriteObject(m[k]); err != nil {
					return err
				}
			}
		case map[any]any:
			type kvEntry struct {
				sortKey string
				key     any
				val     any
			}
			items := make([]kvEntry, 0, len(m))
			for k, v := range m {
				sk := canonicalMapSortKey(k, 0)
				items = append(items, kvEntry{sortKey: sk, key: k, val: v})
			}
			sort.Slice(items, func(i, j int) bool { return items[i].sortKey < items[j].sortKey })
			for _, it := range items {
				if err := w.WriteObject(it.key); err != nil {
					return err
				}
				if err := w.WriteObject(it.val); err != nil {
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
			rawSize := binary.BigEndian.Uint32(blk[4:8])
			if rawSize > maxMetamodelCollectionSize {
				return fmt.Errorf("jos metamodel: HashMap size %d exceeds maximum %d", rawSize, maxMetamodelCollectionSize)
			}
			size = int(rawSize)
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

		// Consumir TC_ENDBLOCKDATA obrigatório
		b, err := r.ReadByte()
		if err != nil {
			return fmt.Errorf("jos metamodel: HashMap missing TC_ENDBLOCKDATA: %w", err)
		}
		if b != TC_ENDBLOCKDATA {
			return fmt.Errorf("jos metamodel: HashMap expected TC_ENDBLOCKDATA (0x78), got 0x%02X", b)
		}

		obj.Set("entries", entries)
		return nil
	},
}

func init() {
	RegisterClassDesc(ClassDescHashMap)
}

// NewJavaHashMap cria uma nova instância de java.util.HashMap inicializada
func NewJavaHashMap(entries any) *JavaObject {
	if entries == nil {
		entries = make(map[string]any)
	}
	obj := NewJavaObject(ClassDescHashMap)
	obj.Set("loadFactor", math.Float32frombits(0x3F400000)) // 0.75f
	obj.Set("threshold", int32(12))
	obj.Set("entries", entries)
	return obj
}
