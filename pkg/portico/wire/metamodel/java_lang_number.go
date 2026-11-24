package metamodel

// ClassDescNumber representa a superclasse abstrata de números em Java (java.lang.Number)
// SUID oficial da JVM: -8742448824652078972L (0x86AC951D0B94E08B)
var ClassDescNumber = &ClassDesc{
	Name:             "java.lang.Number",
	SerialVersionUID: 0x86AC951D0B94E08B,
	Flags:            SC_SERIALIZABLE,
	Fields:           nil,
	Super:            ClassDescObject,
}

func init() {
	RegisterClassDesc(ClassDescNumber)
}
