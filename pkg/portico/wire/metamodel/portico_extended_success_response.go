package metamodel

// ClassDescExtendedSuccessResponse representa a resposta estendida com mapa de resultados do Portico
// SUID oficial: 98121116105109L
var ClassDescExtendedSuccessResponse = &ClassDesc{
	Name:             "org.portico2.common.messaging.ExtendedSuccessResponse",
	SerialVersionUID: 98121116105109,
	Flags:            SC_EXTERNALIZABLE | SC_BLOCK_DATA,
	Fields:           nil,
	Super:            ClassDescResponseMessage,
	CustomWriter: func(w JOSWriter, obj *JavaObject) error {
		// ExtendedSuccessResponse.writeExternal escreve o HashMap results
		resultsObj := obj.Get("results")
		if resultsObj == nil {
			resultsObj = NewJavaHashMap(nil)
		}
		if err := w.WriteObject(resultsObj); err != nil {
			return err
		}
		w.WriteByte(TC_ENDBLOCKDATA)
		return nil
	},
	CustomReader: func(r JOSReader, obj *JavaObject) error {
		results, err := r.ReadObject()
		if err != nil {
			return err
		}
		obj.Set("results", results)
		_, _ = r.ReadByte() // Consumir TC_ENDBLOCKDATA do Externalizable
		return nil
	},
}

func init() {
	RegisterClassDesc(ClassDescExtendedSuccessResponse)
}

// NewExtendedSuccessResponse cria uma resposta estendida contendo o resultado informado
func NewExtendedSuccessResponse(result any) *JavaObject {
	entries := make(map[string]any)
	if result != nil {
		entries["result"] = result
	}
	hashMap := NewJavaHashMap(entries)

	obj := NewJavaObject(ClassDescExtendedSuccessResponse)
	obj.Set("isError", false)
	obj.Set("results", hashMap)
	return obj
}
