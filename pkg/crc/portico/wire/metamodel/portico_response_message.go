package metamodel

import "io"

// ClassDescResponseMessage representa a resposta canônica padrão do Portico
// SUID oficial: 98121116105109L
var ClassDescResponseMessage = &ClassDesc{
	Name:             "org.portico2.common.messaging.ResponseMessage",
	SerialVersionUID: 98121116105109,
	Flags:            SC_WRITE_METHOD | SC_SERIALIZABLE,
	Fields: []FieldDesc{
		{Name: "isError", Type: TypeBoolean},
	},
	Super:      ClassDescObject,
	CustomOnly: true,
	CustomWriter: func(w JOSWriter, obj *JavaObject) error {
		// ResponseMessage.writeObject executa writeExternal gravando isError no bloco
		isErr, _ := obj.Get("isError").(bool)
		b := byte(0x00)
		if isErr {
			b = 0x01
		}
		w.WriteBlockData([]byte{b})
		if isErr {
			if exc, ok := obj.Get("exception").(string); ok && exc != "" {
				if err := w.WriteObject(exc); err != nil {
					return err
				}
			}
		}
		w.WriteByte(TC_ENDBLOCKDATA)
		return nil
	},
	CustomReader: func(r JOSReader, obj *JavaObject) error {
		blk, err := r.ReadBlockData()
		if err != nil {
			return err
		}
		isErr := false
		if len(blk) > 0 {
			isErr = blk[0] != 0
			obj.Set("isError", isErr)
		}
		if isErr {
			// Optional trailing exception string before TC_ENDBLOCKDATA
			val, err := r.ReadObject()
			if err == io.EOF {
				// TC_ENDBLOCKDATA already consumed by ReadObject
				return nil
			}
			if err != nil {
				return err
			}
			if s, ok := val.(string); ok {
				obj.Set("exception", s)
			}
		}
		_, _ = r.ReadByte() // Consumir TC_ENDBLOCKDATA
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
