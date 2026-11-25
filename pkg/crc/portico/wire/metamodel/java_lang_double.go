package metamodel

// ClassDescDouble representa o wrapper java.lang.Double
// SUID oficial da JVM: -9155950456994860284L (0x80B3C24A296BFB04)
var ClassDescDouble = &ClassDesc{
	Name:             "java.lang.Double",
	SerialVersionUID: 0x80B3C24A296BFB04,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "value", Type: TypeDouble},
	},
	Super: ClassDescNumber,
}

func init() {
	RegisterClassDesc(ClassDescDouble)
}

// NewJavaDouble instancia um objeto java.lang.Double serializável
func NewJavaDouble(val float64) *JavaObject {
	obj := NewJavaObject(ClassDescDouble)
	obj.Set("value", val)
	return obj
}
