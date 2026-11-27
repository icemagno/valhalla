package metamodel

// ClassDescSendInteraction represents the official Portico interaction sending request
var ClassDescSendInteraction = &ClassDesc{
	Name:             "org.portico2.common.services.object.msg.SendInteraction",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "interactionId", Type: TypeInt},
		{Name: "parameters", Type: TypeObject, ClassName: "Ljava/util/Map;"},
		{Name: "tag", Type: TypeArray, ClassName: "[B"},
	},
	Super: ClassDescPorticoMessage,
}

// ClassDescReceiveInteraction represents the official Portico interaction receipt notification
var ClassDescReceiveInteraction = &ClassDesc{
	Name:             "org.portico2.common.services.object.msg.ReceiveInteraction",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "interactionId", Type: TypeInt},
		{Name: "parameters", Type: TypeObject, ClassName: "Ljava/util/Map;"},
		{Name: "tag", Type: TypeArray, ClassName: "[B"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescSendInteraction)
	RegisterClassDesc(ClassDescReceiveInteraction)
}

// NewSendInteraction builds a SendInteraction request
func NewSendInteraction(interactionId int32, params map[any]any, tag []byte) *JavaObject {
	obj := NewJavaObject(ClassDescSendInteraction)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("interactionId", interactionId)
	obj.Set("parameters", NewJavaHashMap(params))
	obj.Set("tag", tag)
	return obj
}

// NewReceiveInteraction builds a ReceiveInteraction notification ready for delivery to federates
func NewReceiveInteraction(interactionId int32, params map[any]any, tag []byte) *JavaObject {
	obj := NewJavaObject(ClassDescReceiveInteraction)
	obj.Set("isAsync", true)
	obj.Set("isFromRti", true)
	obj.Set("sourceFederate", int32(0))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("interactionId", interactionId)
	obj.Set("parameters", NewJavaHashMap(params))
	obj.Set("tag", tag)
	return obj
}
