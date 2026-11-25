package metamodel

// ClassDescJResignAction é a enumeração oficial de ação de desingresso do Portico
var ClassDescJResignAction = &ClassDesc{
	Name:             "org.portico.lrc.compat.JResignAction",
	SerialVersionUID: 0,
	Flags:            SC_SERIALIZABLE | SC_ENUM,
	Fields:           nil,
	Super:            ClassDescJavaEnum,
}

func init() {
	RegisterClassDesc(ClassDescJResignAction)
}
