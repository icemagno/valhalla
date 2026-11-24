package metamodel

// ClassDescSyncPointAchieved representa o aviso de alcance de ponto de sincronização
// SUID oficial: 98121116105109L
var ClassDescSyncPointAchieved = &ClassDesc{
	Name:             "org.portico2.common.services.sync.msg.SyncPointAchieved",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "label", Type: TypeObject, ClassName: "Ljava/lang/String;"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescSyncPointAchieved)
}

// NewSyncPointAchieved instancia um objeto SyncPointAchieved
func NewSyncPointAchieved(label string, federateHandle int32) *JavaObject {
	obj := NewJavaObject(ClassDescSyncPointAchieved)
	obj.Set("immediate", true)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", federateHandle)
	obj.Set("targetFederate", int32(0))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("label", label)
	return obj
}
