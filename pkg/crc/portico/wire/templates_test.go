package wire

import (
	"testing"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

func TestTemplates_BuildWelcomePackAndSuccess(t *testing.T) {
	succ := BuildSuccessResponsePayload()
	if len(succ) != 83 {
		t.Errorf("tamanho esperado de ResponseMessage 83, obtido %d", len(succ))
	}

	wp, err := BuildWelcomePackPayload(1, 42)
	if err != nil {
		t.Fatalf("falha ao gerar WelcomePack: %v", err)
	}

	info, err := ParseWelcomePack(wp)
	if err != nil {
		t.Fatalf("falha ao parsear WelcomePack gerado: %v", err)
	}

	if info.FederationHandle != 1 {
		t.Errorf("FederationHandle esperado 1, obtido %d", info.FederationHandle)
	}
	if info.FederateHandle != 42 {
		t.Errorf("FederateHandle esperado 42, obtido %d", info.FederateHandle)
	}

	cfResp := BuildCreateFederationResponsePayload(7)
	if len(cfResp) == 0 {
		t.Fatal("esperava payload nao vazio para CreateFederationResponse")
	}

	errResp := BuildErrorResponsePayload("test error")
	if len(errResp) != 83 {
		t.Errorf("tamanho esperado de ErrorResponse (ResponseMessage) 83, obtido %d", len(errResp))
	}
	regResp := BuildRegisterObjectResponsePayload(10, 1034, "Thor_Soda")
	if len(regResp) == 0 {
		t.Fatal("esperava payload nao vazio para RegisterObjectResponse")
	}
	decReg, err := DeserializePorticoControlMessage(regResp)
	if err != nil {
		t.Fatalf("falha ao deserializar RegisterObjectResponse: %v", err)
	}
	results := decReg.Get("results").(*metamodel.JavaObject)
	var handleVal any
	var nameVal any
	switch m := results.Get("entries").(type) {
	case map[string]any:
		handleVal = m["object-handle"]
		nameVal = m["object-name"]
	case map[any]any:
		handleVal = m["object-handle"]
		nameVal = m["object-name"]
	}
	if handleVal == nil {
		t.Errorf("chave 'object-handle' nao encontrada em RegisterObjectResponse")
	}
	if nameVal != "Thor_Soda" {
		t.Errorf("chave 'object-name' inesperada: %v", nameVal)
	}
}

func TestHashSet_MetamodelRoundtrip(t *testing.T) {
	hs := metamodel.NewJavaHashSet([]any{"item1", "item2"})
	enc := NewJOSEncoder()
	if err := enc.WriteObject(hs); err != nil {
		t.Fatalf("falha ao codificar HashSet: %v", err)
	}

	dec := NewJOSDecoder(enc.Bytes())
	objAny, err := dec.ReadObject()
	if err != nil {
		t.Fatalf("falha ao decodificar HashSet: %v", err)
	}

	obj, ok := objAny.(*metamodel.JavaObject)
	if !ok {
		t.Fatalf("esperava *JavaObject, obtido %T", objAny)
	}

	elements, ok := obj.Get("elements").([]any)
	if !ok || len(elements) != 2 {
		t.Fatalf("esperava 2 elementos no HashSet, obtido %v", elements)
	}
}

func TestTimeManagement_MetamodelPayloads(t *testing.T) {
	dblResp := BuildDoubleResponsePayload(0.0)
	if len(dblResp) == 0 {
		t.Fatal("esperava payload nao vazio para DoubleResponse")
	}

	obj, err := DeserializePorticoControlMessage(dblResp)
	if err != nil {
		t.Fatalf("falha ao decodificar DoubleResponse: %v", err)
	}
	resultsMap, ok := obj.Get("results").(*metamodel.JavaObject)
	if !ok {
		t.Fatalf("esperava results HashMap, obtido %T", obj.Get("results"))
	}
	entries, ok := resultsMap.Get("entries").(map[any]any)
	if !ok {
		t.Fatalf("esperava entries map, obtido %T", resultsMap.Get("entries"))
	}
	resObj, ok := entries["result"].(*metamodel.JavaObject)
	if !ok {
		t.Fatalf("esperava resObj *JavaObject, obtido %T", entries["result"])
	}
	val, ok := resObj.Get("value").(float64)
	if !ok || val != 0.0 {
		t.Fatalf("esperava double 0.0, obtido %v", resObj.Get("value"))
	}

	tagPayload, err := BuildTimeAdvanceGrantPayload(1, 1.0)
	if err != nil {
		t.Fatalf("falha ao gerar TimeAdvanceGrant: %v", err)
	}
	if len(tagPayload) == 0 {
		t.Fatal("esperava payload nao vazio para TimeAdvanceGrant")
	}
}
