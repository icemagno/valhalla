package metamodel

// ClassDescJoinFederation representa a requisição oficial de ingresso na federação
// SUID oficial: 98121116105109L
var ClassDescJoinFederation = &ClassDesc{
	Name:             "org.portico2.common.services.federation.msg.JoinFederation",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "federateName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "federateType", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "federationName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "parsedJoinObjectModels", Type: TypeObject, ClassName: "Ljava/util/List;"},
		{Name: "rawJoinObjectModels", Type: TypeObject, ClassName: "Ljava/util/List;"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescJoinFederation)
}

// NewJoinFederation constrói uma instância canônica de JoinFederation
func NewJoinFederation(federationName, federateName, federateType string) *JavaObject {
	obj := NewJavaObject(ClassDescJoinFederation)
	obj.Set("isAsync", true)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(0))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("federateName", federateName)
	obj.Set("federateType", federateType)
	obj.Set("federationName", federationName)
	obj.Set("parsedJoinObjectModels", NewArrayList())
	obj.Set("rawJoinObjectModels", NewArrayList())
	return obj
}
