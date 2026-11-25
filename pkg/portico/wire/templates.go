package wire

import (
	"github.com/icemagno/valhalla/pkg/portico/wire/metamodel"
)

// BuildSuccessResponsePayload constrói dinamicamente em Go a ResponseMessage de sucesso oficial
func BuildSuccessResponsePayload() []byte {
	resp := metamodel.NewResponseMessage(false)
	data, err := SerializePorticoControlMessage(resp)
	if err != nil {
		return nil
	}
	return data
}

// BuildCreateFederationResponsePayload constrói dinamicamente em Go a ExtendedSuccessResponse
// com o handle da federação empacotado em java.lang.Integer sob as chaves oficiais "result" e "federationHandle"
func BuildCreateFederationResponsePayload(fedHandle int32, optFedName ...string) []byte {
	fedName := "ExampleFederation"
	if len(optFedName) > 0 && optFedName[0] != "" {
		fedName = optFedName[0]
	}

	intObj := metamodel.NewJavaInteger(fedHandle)
	resp := metamodel.NewExtendedSuccessResponse(intObj)

	// O CreateFederationHandler do Portico consulta explicitamente context.getSuccessResultAsInt("federationHandle")
	if hm, ok := resp.Get("results").(*metamodel.JavaObject); ok {
		if entries, ok := hm.Get("entries").(map[string]any); ok {
			entries["federationHandle"] = intObj
			entries["federationName"] = fedName
		}
	}

	data, err := SerializePorticoControlMessage(resp)
	if err != nil {
		return nil
	}
	return data
}

// BuildWelcomePackPayload constrói dinamicamente em Go o WelcomePack oficial com handles atribuídos
func BuildWelcomePackPayload(fedHandle, federateHandle int32, optNames ...string) ([]byte, error) {
	fedName := "ExampleFederation"
	federateName := "Thor"
	federateType := "ieee1516e"
	if len(optNames) > 0 && optNames[0] != "" {
		fedName = optNames[0]
	}
	if len(optNames) > 1 && optNames[1] != "" {
		federateName = optNames[1]
	}
	if len(optNames) > 2 && optNames[2] != "" {
		federateType = optNames[2]
	}

	wp := metamodel.NewWelcomePack(fedHandle, federateHandle, fedName, federateName, federateType)
	resp := metamodel.NewExtendedSuccessResponse(wp)
	return SerializePorticoControlMessage(resp)
}

// BuildAnnounceSyncPointPayload constrói o payload de notificação AnnounceSyncPoint
func BuildAnnounceSyncPointPayload(label string) ([]byte, error) {
	obj := metamodel.NewAnnounceSyncPoint(label)
	return SerializePorticoControlMessage(obj)
}

// BuildFederationSynchronizedPayload constrói o payload de notificação FederationSynchronized
func BuildFederationSynchronizedPayload(label string) ([]byte, error) {
	obj := metamodel.NewFederationSynchronized(label)
	return SerializePorticoControlMessage(obj)
}

// BuildErrorResponsePayload constrói dinamicamente em Go a ResponseMessage de erro oficial
func BuildErrorResponsePayload(errMsg string) []byte {
	resp := metamodel.NewResponseMessage(true)
	data, err := SerializePorticoControlMessage(resp)
	if err != nil {
		return nil
	}
	return data
}

// BuildDoubleResponsePayload constrói dinamicamente em Go a ExtendedSuccessResponse
// empacotando um java.lang.Double como resultado (ex: EnableTimeRegulation)
func BuildDoubleResponsePayload(val float64) []byte {
	dblObj := metamodel.NewJavaDouble(val)
	resp := metamodel.NewExtendedSuccessResponse(dblObj)
	data, err := SerializePorticoControlMessage(resp)
	if err != nil {
		return nil
	}
	return data
}

// BuildTimeAdvanceGrantPayload constrói o payload de notificação TimeAdvanceGrant
func BuildTimeAdvanceGrantPayload(targetFederate int32, time float64) ([]byte, error) {
	tag := metamodel.NewTimeAdvanceGrant(targetFederate, time)
	return SerializePorticoControlMessage(tag)
}

// ExtractFederateNameFromJoinPayload extrai o federateName do payload cru de JoinFederation
// como mecanismo de resiliência caso estruturas complexas e opcionais do FOM falhem na árvore
func ExtractFederateNameFromJoinPayload(payload []byte) string {
	dec := NewJOSDecoder(payload)
	if err := dec.ReadStreamHeader(); err != nil {
		return ""
	}

	// Tenta ler o objeto raiz JoinFederation
	objAny, err := dec.ReadObject()
	if err != nil {
		return ""
	}

	if jo, ok := objAny.(*metamodel.JavaObject); ok {
		if fn, ok := jo.Get("federateName").(string); ok && fn != "" {
			return fn
		}
	}
	return ""
}
