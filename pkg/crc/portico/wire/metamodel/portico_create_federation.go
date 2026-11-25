package metamodel

// ClassDescCreateFederation representa a requisição oficial de criação de federação
// SUID oficial: 98121116105109L
var ClassDescCreateFederation = &ClassDesc{
	Name:             "org.portico2.common.services.federation.msg.CreateFederation",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "federationName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "modules", Type: TypeObject, ClassName: "Ljava/util/List;"},
	},
	Super: ClassDescPorticoMessage,
}

// ClassDescDestroyFederation representa a requisição oficial de destruição de federação
// SUID oficial: 98121116105109L
var ClassDescDestroyFederation = &ClassDesc{
	Name:             "org.portico2.common.services.federation.msg.DestroyFederation",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "federationName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescCreateFederation)
	RegisterClassDesc(ClassDescDestroyFederation)
}

// NewCreateFederation constrói uma instância de CreateFederation
func NewCreateFederation(federationName string) *JavaObject {
	obj := NewJavaObject(ClassDescCreateFederation)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("federationName", federationName)
	obj.Set("modules", NewJavaArrayList(nil))
	return obj
}
