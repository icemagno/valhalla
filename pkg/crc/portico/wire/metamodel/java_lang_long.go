package metamodel

// ClassDescLong representa o wrapper java.lang.Long
// SUID oficial da JVM: 4294967296L (0x3B8BAA2444859841)
var ClassDescLong = &ClassDesc{
	Name:             "java.lang.Long",
	SerialVersionUID: 0x3B8BAA2444859841,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "value", Type: TypeLong},
	},
	Super: ClassDescNumber,
}

func init() {
	RegisterClassDesc(ClassDescLong)
}

// NewJavaLong instancia um objeto java.lang.Long serializável
func NewJavaLong(val int64) *JavaObject {
	obj := NewJavaObject(ClassDescLong)
	obj.Set("value", val)
	return obj
}
