package test

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

// MinimalObjectModel valida a raiz dos arquivos FOM segundo IEEE 1516-2010
type MinimalObjectModel struct {
	XMLName xml.Name `xml:"objectModel"`
	ModelID struct {
		Name    string `xml:"name"`
		Type    string `xml:"type"`
		Version string `xml:"version"`
	} `xml:"modelIdentification"`
}

func TestSprint0_RPR_FOM_Files_Integrity(t *testing.T) {
	requiredModules := []string{
		"RPR-Base_v2.0.xml",
		"RPR-Physical_v2.0.xml",
		"RPR-Warfare_v2.0.xml",
		"RPR-Communication_v2.0.xml",
		"RPR-Enumerations_v2.0.xml",
		"RPR-Foundation_v2.0.xml",
		"RPR-Logistics_v2.0.xml",
		"RPR-Minefield_v2.0.xml",
		"RPR-SE_v2.0.xml",
		"RPR-SIMAN_v2.0.xml",
		"RPR-Switches_v2.0.xml",
		"RPR-UA_v2.0.xml",
		"RPR-Aggregate_v2.0.xml",
		"RPR-DER_v2.0.xml",
	}

	resourcesDir := filepath.Join("resources", "rpr-fom-v2.0")

	for _, module := range requiredModules {
		t.Run(module, func(t *testing.T) {
			path := filepath.Join(resourcesDir, module)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("falha ao abrir modulo RPR-FOM obrigatorio %s: %v", module, err)
			}

			if len(data) == 0 {
				t.Fatalf("o modulo %s esta vazio", module)
			}

			var fom MinimalObjectModel
			if err := xml.Unmarshal(data, &fom); err != nil {
				t.Fatalf("falha ao decodificar XML do modulo %s: %v", module, err)
			}

			if fom.XMLName.Local != "objectModel" {
				t.Errorf("modulo %s: esperava tag raiz <objectModel>, obteve <%s>", module, fom.XMLName.Local)
			}

			t.Logf("Modulo validado: %s [Nome: %s, Versao: %s, Tamanho: %d bytes]",
				module, fom.ModelID.Name, fom.ModelID.Version, len(data))
		})
	}
}
