package metamodel

// ClassDescResponseMessage representa a resposta canônica padrão do Portico
// SUID oficial: 98121116105109L
var ClassDescResponseMessage = &ClassDesc{
	Name:             "org.portico2.common.messaging.ResponseMessage",
	SerialVersionUID: 98121116105109,
	Flags:            SC_WRITE_METHOD | SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "isError", Type: TypeBoolean},
	},
	Super: ClassDescObject,
	CustomWriter: func(w JOSWriter, obj *JavaObject) error {
		// ResponseMessage.writeObject grava byte de controle adicional e fecha bloco
		w.WriteBlockData([]byte{0x00})
		w.WriteByte(TC_ENDBLOCKDATA)
		return nil
	},
	CustomReader: func(r JOSReader, obj *JavaObject) error {
		_, _ = r.ReadBlockData()
		_, _ = r.ReadByte() // Consumir TC_ENDBLOCKDATA se presente
		return nil
	},
}

func init() {
	RegisterClassDesc(ClassDescResponseMessage)
}

// NewResponseMessage cria uma instância de ResponseMessage
func NewResponseMessage(isError bool) *JavaObject {
	obj := NewJavaObject(ClassDescResponseMessage)
	obj.Set("isError", isError)
	return obj
}
