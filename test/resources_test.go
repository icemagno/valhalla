package test

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/metamodel"
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

// TestResources_RPR_FOM_FullMetamodelIngestion realiza a ingestão completa ponta-a-ponta
// dos 14 módulos oficiais do RPR-FOM v2.0 (SISO-STD-001-2015) no OMTRegistry canônico do Valhalla,
// comprovando compilação do metamodelo, herança O(1) com Euler Tour, resolução de atributos e
// criação de federação com múltiplos módulos de FOM via FederationManager.
func TestResources_RPR_FOM_FullMetamodelIngestion(t *testing.T) {
	resourcesDir := filepath.Join("resources", "rpr-fom-v2.0")

	// Ordem de carregamento dos 14 módulos do RPR-FOM v2.0
	rprModules := []string{
		"RPR-Foundation_v2.0.xml",
		"RPR-Enumerations_v2.0.xml",
		"RPR-Base_v2.0.xml",
		"RPR-Physical_v2.0.xml",
		"RPR-Aggregate_v2.0.xml",
		"RPR-Communication_v2.0.xml",
		"RPR-DER_v2.0.xml",
		"RPR-Logistics_v2.0.xml",
		"RPR-Minefield_v2.0.xml",
		"RPR-SE_v2.0.xml",
		"RPR-SIMAN_v2.0.xml",
		"RPR-Switches_v2.0.xml",
		"RPR-UA_v2.0.xml",
		"RPR-Warfare_v2.0.xml",
	}

	// 1. Ingestão direta no OMTRegistry
	omt := core.NewOMTRegistry()
	for _, modName := range rprModules {
		path := filepath.Join(resourcesDir, modName)
		if err := metamodel.ParseFOMFile(omt, path); err != nil {
			t.Fatalf("falha ao ingerir modulo RPR-FOM %s: %v", modName, err)
		}
	}

	// 2. Validação estrutural de classes e herança Euler Tour entre módulos
	// Aircraft -> Platform -> PhysicalEntity -> BaseEntity -> HLAobjectRoot
	aircraft, ok := omt.GetObjectClassByName("Aircraft")
	if !ok {
		t.Fatal("classe 'Aircraft' não encontrada no OMTRegistry após ingestão dos 14 módulos")
	}
	baseEntity, ok := omt.GetObjectClassByName("BaseEntity")
	if !ok {
		t.Fatal("classe 'BaseEntity' não encontrada no OMTRegistry")
	}
	root, ok := omt.GetObjectClassByName("HLAobjectRoot")
	if !ok {
		t.Fatal("classe 'HLAobjectRoot' não encontrada no OMTRegistry")
	}

	if !omt.IsSubclassOfObject(aircraft.Handle, baseEntity.Handle) {
		t.Fatalf("Aircraft (%d) deveria ser subclasse de BaseEntity (%d)", aircraft.Handle, baseEntity.Handle)
	}
	if !omt.IsSubclassOfObject(aircraft.Handle, root.Handle) {
		t.Fatalf("Aircraft (%d) deveria ser subclasse de HLAobjectRoot (%d)", aircraft.Handle, root.Handle)
	}

	// 3. Validação de classes agregadas e de domínio
	domainClasses := []string{
		"AggregateEntity",         // RPR-Aggregate
		"RadioTransmitter",        // RPR-Communication
		"RadioReceiver",           // RPR-Communication
		"Minefield",               // RPR-Minefield
		"MinefieldObject",         // RPR-Minefield
		"MinefieldLaneMarkerObject", // RPR-Minefield
		"Munition",                // RPR-Warfare
		"ActiveSonarBeam",         // RPR-UA
	}
	for _, clsName := range domainClasses {
		cls, exists := omt.GetObjectClassByName(clsName)
		if !exists {
			t.Errorf("classe de domínio '%s' esperada no metamodelo completo", clsName)
		} else {
			if !omt.IsSubclassOfObject(cls.Handle, root.Handle) {
				t.Errorf("classe '%s' não descende de HLAobjectRoot", clsName)
			}
		}
	}

	// 4. Validação de interações de domínio e herança Euler Tour
	domainInteractions := []string{
		"WeaponFire",         // RPR-Warfare
		"MunitionDetonation", // RPR-Warfare
		"ActionRequest",      // RPR-SIMAN
		"ActionResponse",     // RPR-SIMAN
		"ResupplyOffer",      // RPR-Logistics
		"ResupplyReceived",   // RPR-Logistics
		"RadioSignal",        // RPR-Communication
	}
	interRoot, ok := omt.GetInteractionClassByName("HLAinteractionRoot")
	if !ok {
		t.Fatal("HLAinteractionRoot não encontrado no OMTRegistry")
	}
	for _, interName := range domainInteractions {
		inter, exists := omt.GetInteractionClassByName(interName)
		if !exists {
			t.Errorf("interação de domínio '%s' esperada no metamodelo completo", interName)
		} else {
			if !omt.IsSubclassOfInteraction(inter.Handle, interRoot.Handle) {
				t.Errorf("interação '%s' não descende de HLAinteractionRoot", interName)
			}
		}
	}

	// 5. Ingestão via FederationManager com todos os 14 módulos
	prevFOMDir := crc.CanonicalFOMDir
	crc.CanonicalFOMDir = "resources"
	defer func() { crc.CanonicalFOMDir = prevFOMDir }()

	fedMgr := crc.NewFederationManager()
	var modulePaths []string
	for _, m := range rprModules {
		modulePaths = append(modulePaths, filepath.Join(resourcesDir, m))
	}

	fed, err := fedMgr.CreateFederationWithFOMs("RPR_Tactical_Simulation", modulePaths)
	if err != nil {
		t.Fatalf("falha ao criar federação via FederationManager com os 14 módulos RPR-FOM: %v", err)
	}

	fedAircraft, exists := fed.OMT().GetObjectClassByName("Aircraft")
	if !exists {
		t.Fatal("classe 'Aircraft' não encontrada na federação criada com os 14 módulos")
	}
	fedWeaponFire, exists := fed.OMT().GetInteractionClassByName("WeaponFire")
	if !exists {
		t.Fatal("interação 'WeaponFire' não encontrada na federação criada com os 14 módulos")
	}

	t.Logf("Federação RPR completa instanciada com sucesso: %s (Handle Aircraft=%d, Handle WeaponFire=%d)",
		fed.Name, fedAircraft.Handle, fedWeaponFire.Handle)
}
