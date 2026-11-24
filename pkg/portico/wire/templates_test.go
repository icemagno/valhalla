package wire

import (
	"testing"
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
}
