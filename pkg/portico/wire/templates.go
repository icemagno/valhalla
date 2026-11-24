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
