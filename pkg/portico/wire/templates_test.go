package wire

import (
	"testing"

	"github.com/icemagno/valhalla/pkg/portico/wire/metamodel"
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
