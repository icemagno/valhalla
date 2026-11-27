package metamodel

// ClassDescResignFederation representa a requisição oficial de desingresso da federação
// SUID oficial: 98121116105109L
var ClassDescResignFederation = &ClassDesc{
	Name:             "org.portico2.common.services.federation.msg.ResignFederation",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "federateName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "federateType", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "federationName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "resignAction", Type: TypeObject, ClassName: "Lorg/portico/lrc/compat/JResignAction;"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescResignFederation)
}

// NewResignFederation constrói uma instância de ResignFederation com ação de desingresso
func NewResignFederation(federationName, federateName, federateType, action string) *JavaObject {
	if action == "" {
		action = "NO_ACTION"
	}
	obj := NewJavaObject(ClassDescResignFederation)
	obj.Set("isAsync", true)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(0))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("federateName", federateName)
	obj.Set("federateType", federateType)
	obj.Set("federationName", federationName)
	obj.Set("resignAction", NewJavaEnum(ClassDescJResignAction, action))
	return obj
}
