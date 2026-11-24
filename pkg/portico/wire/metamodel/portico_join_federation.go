package metamodel

// ClassDescJoinFederation representa a requisição oficial de ingresso na federação
// SUID oficial: 98121116105109L
var ClassDescJoinFederation = &ClassDesc{
	Name:             "org.portico2.common.services.federation.msg.JoinFederation",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "federationName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "federateName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "federateType", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "modules", Type: TypeObject, ClassName: "Ljava/util/List;"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescJoinFederation)
}

// NewJoinFederation constrói uma instância de JoinFederation
func NewJoinFederation(federationName, federateName, federateType string) *JavaObject {
	obj := NewJavaObject(ClassDescJoinFederation)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("federationName", federationName)
	obj.Set("federateName", federateName)
	obj.Set("federateType", federateType)
	obj.Set("modules", NewJavaArrayList(nil))
	return obj
}
