package metamodel

// ClassDescTimeAdvanceGrant representa a mensagem oficial TimeAdvanceGrant do Portico
// SUID oficial: 98121116105109L
var ClassDescTimeAdvanceGrant = &ClassDesc{
	Name:             "org.portico2.common.services.time.msg.TimeAdvanceGrant",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "time", Type: TypeDouble},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescTimeAdvanceGrant)
}

// NewTimeAdvanceGrant constrói uma instância de TimeAdvanceGrant para o federado alvo
func NewTimeAdvanceGrant(targetFederate int32, time float64) *JavaObject {
	obj := NewJavaObject(ClassDescTimeAdvanceGrant)
	obj.Set("immediate", true)
	obj.Set("isFromRti", true)
	obj.Set("sourceFederate", int32(0))
	obj.Set("targetFederate", targetFederate)
	obj.Set("targetFederation", int32(1))
	obj.Set("timestamp", time)
	obj.Set("time", time)
	return obj
}
