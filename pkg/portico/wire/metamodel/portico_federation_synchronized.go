package metamodel

// ClassDescFederationSynchronized representa a notificação de que todos alcançaram o ponto
// SUID oficial: 98121116105109L
var ClassDescFederationSynchronized = &ClassDesc{
	Name:             "org.portico2.common.services.sync.msg.FederationSynchronized",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "label", Type: TypeObject, ClassName: "Ljava/lang/String;"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescFederationSynchronized)
}

// NewFederationSynchronized instancia a notificação FederationSynchronized
func NewFederationSynchronized(label string) *JavaObject {
	obj := NewJavaObject(ClassDescFederationSynchronized)
	obj.Set("immediate", true)
	obj.Set("isFromRti", true)
	obj.Set("sourceFederate", int32(0))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("label", label)
	return obj
}
