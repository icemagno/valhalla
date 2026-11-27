package wire

import (
	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
	"github.com/icemagno/valhalla/pkg/logger"
)

var logWireTemplates = logger.NewLogger("Portico-Wire")

func mustSerializeControl(op string, obj *metamodel.JavaObject) []byte {
	data, err := SerializePorticoControlMessage(obj)
	if err != nil {
		logWireTemplates.Error("failed to serialize %s: %v", op, err)
		return nil
	}
	return data
}

// BuildSuccessResponsePayload constrói dinamicamente em Go a ResponseMessage de sucesso oficial
func BuildSuccessResponsePayload() []byte {
	return mustSerializeControl("SuccessResponse", metamodel.NewResponseMessage(false))
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

	return mustSerializeControl("CreateFederationResponse", resp)
}

// BuildWelcomePackPayload constrói dinamicamente em Go o WelcomePack oficial com handles atribuídos
func BuildWelcomePackPayload(fedHandle, federateHandle int32, optNames ...string) ([]byte, error) {
	return BuildWelcomePackPayloadWithFOM(fedHandle, federateHandle, nil, optNames...)
}

// BuildWelcomePackPayloadWithFOM constrói dinamicamente em Go o WelcomePack oficial com handles atribuídos e ObjectModel sintetizado
func BuildWelcomePackPayloadWithFOM(fedHandle, federateHandle int32, omt *core.OMTRegistry, optNames ...string) ([]byte, error) {
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
	if omt != nil {
		fomObj := SynthesizePorticoObjectModel(omt)
		wp.Set("fom", fomObj)
	}
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
	if errMsg != "" {
		resp.Set("exception", errMsg)
	}
	return mustSerializeControl("ErrorResponse", resp)
}

// BuildDoubleResponsePayload constrói dinamicamente em Go a ExtendedSuccessResponse
// empacotando um java.lang.Double como resultado (ex: EnableTimeRegulation)
func BuildDoubleResponsePayload(val float64) []byte {
	dblObj := metamodel.NewJavaDouble(val)
	resp := metamodel.NewExtendedSuccessResponse(dblObj)
	return mustSerializeControl("DoubleResponse", resp)
}

// BuildIntegerResponsePayload constrói dinamicamente em Go a ExtendedSuccessResponse
// empacotando um java.lang.Integer como resultado (ex: RegisterObjectInstance retornando o ObjectHandle)
func BuildIntegerResponsePayload(val int32) []byte {
	intObj := metamodel.NewJavaInteger(val)
	resp := metamodel.NewExtendedSuccessResponse(intObj)
	return mustSerializeControl("IntegerResponse", resp)
}

// BuildRegisterObjectResponsePayload constrói dinamicamente em Go a ExtendedSuccessResponse
// com as chaves exigidas pelo RegisterObjectHandler do Portico ("object-handle", "object-name", "object-class", "result")
func BuildRegisterObjectResponsePayload(objHandle, classHandle int32, objName string) []byte {
	intObj := metamodel.NewJavaInteger(objHandle)
	resp := metamodel.NewExtendedSuccessResponse(intObj)

	if hm, ok := resp.Get("results").(*metamodel.JavaObject); ok {
		if entries, ok := hm.Get("entries").(map[string]any); ok {
			entries["object-handle"] = intObj
			entries["object-name"] = objName
			entries["object-class"] = metamodel.NewJavaInteger(classHandle)
			entries["result"] = intObj
		}
	}

	return mustSerializeControl("RegisterObjectResponse", resp)
}

// BuildTimeAdvanceGrantPayload constrói o payload de notificação TimeAdvanceGrant
func BuildTimeAdvanceGrantPayload(targetFederate int32, time float64) ([]byte, error) {
	tag := metamodel.NewTimeAdvanceGrant(targetFederate, time)
	return SerializePorticoControlMessage(tag)
}

// BuildDiscoverObjectPayload constrói o payload de notificação DiscoverObject para federados Portico
func BuildDiscoverObjectPayload(objectHandle, classHandle int32, objectName string) ([]byte, error) {
	obj := metamodel.NewDiscoverObject(objectHandle, classHandle, objectName)
	return SerializePorticoControlMessage(obj)
}

// BuildDeleteObjectPayload constrói o payload de notificação DeleteObject para federados Portico
func BuildDeleteObjectPayload(objectHandle int32, tag []byte) ([]byte, error) {
	obj := metamodel.NewDeleteObject(objectHandle, tag)
	return SerializePorticoControlMessage(obj)
}

// BuildReflectAttributesPayload constrói o payload de notificação ReflectAttributes para federados Portico
func BuildReflectAttributesPayload(objectId int32, attrs map[int32][]byte, tag []byte) ([]byte, error) {
	return BuildReflectAttributesWithTimePayload(objectId, attrs, tag, false, 0)
}

// BuildReflectAttributesWithTimePayload constrói ReflectAttributes preservando timestamp lógico TSO quando hasTime=true
func BuildReflectAttributesWithTimePayload(objectId int32, attrs map[int32][]byte, tag []byte, hasTime bool, timestamp float64) ([]byte, error) {
	entries := make(map[any]any, len(attrs))
	for k, v := range attrs {
		entries[metamodel.NewJavaInteger(k)] = v
	}
	obj := metamodel.NewReflectAttributes(objectId, entries, tag)
	if hasTime {
		obj.Set("timestamp", timestamp)
	}
	return SerializePorticoControlMessage(obj)
}

// BuildReceiveInteractionPayload constrói o payload de notificação ReceiveInteraction para federados Portico
func BuildReceiveInteractionPayload(interactionId int32, params map[int32][]byte, tag []byte) ([]byte, error) {
	return BuildReceiveInteractionWithTimePayload(interactionId, params, tag, false, 0)
}

// BuildReceiveInteractionWithTimePayload constrói ReceiveInteraction preservando timestamp lógico TSO quando hasTime=true
func BuildReceiveInteractionWithTimePayload(interactionId int32, params map[int32][]byte, tag []byte, hasTime bool, timestamp float64) ([]byte, error) {
	entries := make(map[any]any, len(params))
	for k, v := range params {
		entries[metamodel.NewJavaInteger(k)] = v
	}
	obj := metamodel.NewReceiveInteraction(interactionId, entries, tag)
	if hasTime {
		obj.Set("timestamp", timestamp)
	}
	return SerializePorticoControlMessage(obj)
}

// NewRegisterObjectMsg constrói o payload de requisição RegisterObject do Portico
func NewRegisterObjectMsg(classHandle int32, objectName string) ([]byte, error) {
	obj := metamodel.NewRegisterObject(classHandle, objectName)
	return SerializePorticoControlMessage(obj)
}

// NewUpdateAttributesMsg constrói o payload de requisição UpdateAttributes do Portico
func NewUpdateAttributesMsg(objectId int32, attrs map[int32][]byte, tag []byte) ([]byte, error) {
	entries := make(map[any]any, len(attrs))
	for k, v := range attrs {
		entries[metamodel.NewJavaInteger(k)] = v
	}
	obj := metamodel.NewUpdateAttributes(objectId, entries, tag)
	return SerializePorticoControlMessage(obj)
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
