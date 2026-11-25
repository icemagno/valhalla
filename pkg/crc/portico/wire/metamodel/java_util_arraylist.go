package metamodel

// ClassDescArrayList representa a lista canônica java.util.ArrayList
// SUID oficial da JVM: 8683452581122892189L (0x7881D21D99C7619D)
var ClassDescArrayList = &ClassDesc{
	Name:             "java.util.ArrayList",
	SerialVersionUID: 0x7881D21D99C7619D,
	Flags:            SC_WRITE_METHOD | SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "size", Type: TypeInt},
	},
	Super: ClassDescObject,
	CustomWriter: func(w JOSWriter, obj *JavaObject) error {
		elements, _ := obj.Get("elements").([]any)
		w.WriteBlockData([]byte{0x00, 0x00, 0x00, 0x00}) // capacidade
		for _, el := range elements {
			if err := w.WriteObject(el); err != nil {
				return err
			}
		}
		w.WriteByte(TC_ENDBLOCKDATA)
		return nil
	},
	CustomReader: func(r JOSReader, obj *JavaObject) error {
		sizeVal, _ := obj.Get("size").(int32)
		size := int(sizeVal)

		// Consumir bloco de capacidade
		if _, err := r.ReadBlockData(); err != nil {
			return err
		}

		elements := make([]any, 0, size)
		for i := 0; i < size; i++ {
			el, err := r.ReadObject()
			if err != nil {
				return err
			}
			elements = append(elements, el)
		}

		// Consumir TC_ENDBLOCKDATA
		_, _ = r.ReadByte()

		obj.Set("elements", elements)
		return nil
	},
}

func init() {
	RegisterClassDesc(ClassDescArrayList)
}

// NewArrayList cria um novo objeto java.util.ArrayList vazio
func NewArrayList() *JavaObject {
	return NewJavaArrayList(nil)
}

// NewJavaArrayList cria um novo objeto java.util.ArrayList inicializado
func NewJavaArrayList(elements []any) *JavaObject {
	obj := NewJavaObject(ClassDescArrayList)
	obj.Set("size", int32(len(elements)))
	obj.Set("elements", elements)
	return obj
}
