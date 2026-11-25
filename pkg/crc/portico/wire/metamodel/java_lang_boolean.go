package metamodel

// ClassDescBoolean representa o wrapper java.lang.Boolean
// SUID oficial da JVM: -3665804199014368530L (0xCD207280D59CFAEE)
var ClassDescBoolean = &ClassDesc{
	Name:             "java.lang.Boolean",
	SerialVersionUID: 0xCD207280D59CFAEE,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "value", Type: TypeBoolean},
	},
	Super: ClassDescObject,
}

func init() {
	RegisterClassDesc(ClassDescBoolean)
}

// NewJavaBoolean instancia um objeto java.lang.Boolean serializável
func NewJavaBoolean(val bool) *JavaObject {
	obj := NewJavaObject(ClassDescBoolean)
	obj.Set("value", val)
	return obj
}
