package metamodel

// ClassDescUpdateAttributes represents the official Portico attribute update request
var ClassDescUpdateAttributes = &ClassDesc{
	Name:             "org.portico2.common.services.object.msg.UpdateAttributes",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "objectId", Type: TypeInt},
		{Name: "attributes", Type: TypeObject, ClassName: "Ljava/util/Map;"},
		{Name: "tag", Type: TypeArray, ClassName: "[B"},
	},
	Super: ClassDescPorticoMessage,
}

// ClassDescReflectAttributes represents the official Portico attribute reflection notification
var ClassDescReflectAttributes = &ClassDesc{
	Name:             "org.portico2.common.services.object.msg.ReflectAttributes",
	SerialVersionUID: 98121116105109,
	Flags:            SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "objectId", Type: TypeInt},
		{Name: "attributes", Type: TypeObject, ClassName: "Ljava/util/Map;"},
		{Name: "tag", Type: TypeArray, ClassName: "[B"},
	},
	Super: ClassDescPorticoMessage,
}

func init() {
	RegisterClassDesc(ClassDescUpdateAttributes)
	RegisterClassDesc(ClassDescReflectAttributes)
}

// NewUpdateAttributes builds an UpdateAttributes request
func NewUpdateAttributes(objectId int32, attrs map[any]any, tag []byte) *JavaObject {
	obj := NewJavaObject(ClassDescUpdateAttributes)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("objectId", objectId)
	obj.Set("attributes", NewJavaHashMap(attrs))
	obj.Set("tag", tag)
	return obj
}

// NewReflectAttributes builds a ReflectAttributes notification ready for delivery to federates
func NewReflectAttributes(objectId int32, attrs map[any]any, tag []byte) *JavaObject {
	obj := NewJavaObject(ClassDescReflectAttributes)
	obj.Set("isAsync", true)
	obj.Set("isFromRti", true)
	obj.Set("sourceFederate", int32(0))
	obj.Set("targetFederate", int32(-1))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("objectId", objectId)
	obj.Set("attributes", NewJavaHashMap(attrs))
	obj.Set("tag", tag)
	return obj
}
