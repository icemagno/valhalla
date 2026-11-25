package metamodel

// ClassDescRegisterSyncPoint representa a requisição de registro de ponto de sincronização
// SUID oficial compatível com Portico: 98121116105108L
var ClassDescRegisterSyncPoint = &ClassDesc{
	Name:             "org.portico2.common.services.sync.msg.RegisterSyncPoint",
	SerialVersionUID: 98121116105108,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "label", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "tag", Type: TypeArray, ClassName: "[B"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescRegisterSyncPoint)
}

// NewRegisterSyncPoint instancia um objeto RegisterSyncPoint
func NewRegisterSyncPoint(label string, federateHandle int32) *JavaObject {
	obj := NewJavaObject(ClassDescRegisterSyncPoint)
	obj.Set("immediate", true)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", federateHandle)
	obj.Set("targetFederate", int32(0))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("label", label)
	obj.Set("tag", nil)
	return obj
}
