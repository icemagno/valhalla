package metamodel

// ClassDescPublishObjectClass represents the official Portico object class publication request
var ClassDescPublishObjectClass = &ClassDesc{
	Name:             "org.portico2.common.services.pubsub.msg.PublishObjectClass",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "classHandle", Type: TypeInt},
		{Name: "attributes", Type: TypeObject, ClassName: "Ljava/util/Set;"},
	},
	Super: ClassDescPorticoMessage,
}

// ClassDescSubscribeObjectClass represents the official Portico object class subscription request
var ClassDescSubscribeObjectClass = &ClassDesc{
	Name:             "org.portico2.common.services.pubsub.msg.SubscribeObjectClass",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "classHandle", Type: TypeInt},
		{Name: "attributes", Type: TypeObject, ClassName: "Ljava/util/Set;"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescPublishObjectClass)
	RegisterClassDesc(ClassDescSubscribeObjectClass)
}

// NewPublishObjectClass builds a PublishObjectClass request
func NewPublishObjectClass(classHandle int32, attributes []any) *JavaObject {
	obj := NewJavaObject(ClassDescPublishObjectClass)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("classHandle", classHandle)
	obj.Set("attributes", NewJavaHashSet(attributes))
	return obj
}

// NewSubscribeObjectClass builds a SubscribeObjectClass request
func NewSubscribeObjectClass(classHandle int32, attributes []any) *JavaObject {
	obj := NewJavaObject(ClassDescSubscribeObjectClass)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("classHandle", classHandle)
	obj.Set("attributes", NewJavaHashSet(attributes))
	return obj
}
