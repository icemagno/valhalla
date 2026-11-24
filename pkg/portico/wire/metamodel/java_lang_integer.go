package metamodel

// ClassDescInteger representa o wrapper java.lang.Integer
// SUID oficial da JVM: 1360196720211904840L (0x12E2A0A4F7818738)
var ClassDescInteger = &ClassDesc{
	Name:             "java.lang.Integer",
	SerialVersionUID: 0x12E2A0A4F7818738,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "value", Type: TypeInt},
	},
	Super: ClassDescNumber,
}

func init() {
	RegisterClassDesc(ClassDescInteger)
}

// NewJavaInteger instancia um objeto java.lang.Integer serializável
func NewJavaInteger(val int32) *JavaObject {
	obj := NewJavaObject(ClassDescInteger)
	obj.Set("value", val)
	return obj
}
