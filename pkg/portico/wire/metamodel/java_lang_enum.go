package metamodel

// ClassDescJavaEnum é a superclasse canônica de enumerações da linguagem Java
var ClassDescJavaEnum = &ClassDesc{
	Name:             "java.lang.Enum",
	SerialVersionUID: 0,
	Flags:            SC_SERIALIZABLE | SC_ENUM,
	Fields:           nil,
	Super:            ClassDescObject,
}

func init() {
	RegisterClassDesc(ClassDescJavaEnum)
}
