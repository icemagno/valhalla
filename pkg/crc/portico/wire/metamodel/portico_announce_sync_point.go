package metamodel

// ClassDescAnnounceSyncPoint representa a notificação oficial de anúncio de ponto de sincronização
// SUID oficial: 98121116105109L
var ClassDescAnnounceSyncPoint = &ClassDesc{
	Name:             "org.portico2.common.services.sync.msg.AnnounceSyncPoint",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "label", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		{Name: "tag", Type: TypeArray, ClassName: "[B"},
		{Name: "federates", Type: TypeObject, ClassName: "Ljava/util/Set;"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescAnnounceSyncPoint)
}

// NewAnnounceSyncPoint cria uma nova notificação AnnounceSyncPoint pronta para envio ao federado
func NewAnnounceSyncPoint(label string) *JavaObject {
	obj := NewJavaObject(ClassDescAnnounceSyncPoint)
	obj.Set("isAsync", true)
	obj.Set("isFromRti", true)
	obj.Set("sourceFederate", int32(0))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("label", label)
	obj.Set("tag", nil)
	obj.Set("federates", nil)
	return obj
}
