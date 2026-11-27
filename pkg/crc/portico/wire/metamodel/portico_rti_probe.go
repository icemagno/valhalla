package metamodel

// ClassDescRtiProbe representa a sonda de presença do executivo RTI
// SUID oficial: 98121116105109L
var ClassDescRtiProbe = &ClassDesc{
	Name:             "org.portico2.common.services.federation.msg.RtiProbe",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields:           nil, // Sem campos adicionais na subclasse
	Super:            ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescRtiProbe)
}

// NewRtiProbe constrói uma instância de RtiProbe
func NewRtiProbe() *JavaObject {
	obj := NewJavaObject(ClassDescRtiProbe)
	obj.Set("isAsync", false)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(0))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	return obj
}
