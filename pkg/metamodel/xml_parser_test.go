package metamodel_test

import (
	"path/filepath"
	"testing"

	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/metamodel"
)

func TestXMLParser_RestaurantFOMs(t *testing.T) {
	omt := core.NewOMTRegistry()

	fomDir := filepath.Join("..", "..", "test", "foms")
	foodFile := filepath.Join(fomDir, "RestaurantFood.xml")
	drinksFile := filepath.Join(fomDir, "RestaurantDrinks.xml")
	procFile := filepath.Join(fomDir, "RestaurantProcesses.xml")

	// 1. Ingest RestaurantFood.xml
	if err := metamodel.ParseFOMFile(omt, foodFile); err != nil {
		t.Fatalf("failed to parse RestaurantFood.xml: %v", err)
	}

	// Verify Food, Drink, MainCourse exist
	drinkCls, ok := omt.GetObjectClassByName("Drink")
	if !ok {
		t.Fatal("expected 'Drink' class in OMT")
	}
	t.Logf("Drink class handle: %d, parent: %d", drinkCls.Handle, drinkCls.ParentHandle)

	// Verify NumberCups attribute on Drink
	attr, err := omt.ResolveAttributeByName(drinkCls.Handle, "NumberCups")
	if err != nil {
		t.Fatalf("expected attribute 'NumberCups' on Drink: %v", err)
	}
	t.Logf("NumberCups attribute handle: %d, datatype: %s", attr.Handle, attr.Datatype)

	// 2. Ingest RestaurantDrinks.xml (merging into the same hierarchy)
	if err := metamodel.ParseFOMFile(omt, drinksFile); err != nil {
		t.Fatalf("failed to parse RestaurantDrinks.xml: %v", err)
	}

	// Verify Soda, Coffee exist
	sodaCls, ok := omt.GetObjectClassByName("Soda")
	if !ok {
		t.Fatal("expected 'Soda' class in OMT")
	}
	t.Logf("Soda class handle: %d, parent: %d", sodaCls.Handle, sodaCls.ParentHandle)

	// Verify Euler Tour inheritance check: Soda should be subclass of Drink!
	if !omt.IsSubclassOfObject(sodaCls.Handle, drinkCls.Handle) {
		t.Fatalf("expected Soda to be subclass of Drink via Euler Tour!")
	}
	t.Logf("Euler Tour verified: Soda (%d) is subclass of Drink (%d)", sodaCls.Handle, drinkCls.Handle)

	// 3. Ingest RestaurantProcesses.xml (interactions)
	if err := metamodel.ParseFOMFile(omt, procFile); err != nil {
		t.Fatalf("failed to parse RestaurantProcesses.xml: %v", err)
	}

	seatedInter, ok := omt.GetInteractionClassByName("CustomerSeated")
	if !ok {
		t.Fatal("expected 'CustomerSeated' interaction class in OMT")
	}
	t.Logf("CustomerSeated interaction handle: %d, parent: %d", seatedInter.Handle, seatedInter.ParentHandle)

	custTransInter, ok := omt.GetInteractionClassByName("CustomerTransactions")
	if !ok {
		t.Fatal("expected 'CustomerTransactions' interaction class in OMT")
	}

	if !omt.IsSubclassOfInteraction(seatedInter.Handle, custTransInter.Handle) {
		t.Fatalf("expected CustomerSeated to be subclass of CustomerTransactions!")
	}
	t.Logf("Euler Tour verified: CustomerSeated (%d) is subclass of CustomerTransactions (%d)",
		seatedInter.Handle, custTransInter.Handle)
}

func TestXMLParser_RPR_FOM_BaseAndPhysical(t *testing.T) {
	omt := core.NewOMTRegistry()

	rprDir := filepath.Join("..", "..", "test", "resources", "rpr-fom-v2.0")
	baseFile := filepath.Join(rprDir, "RPR-Base_v2.0.xml")
	physFile := filepath.Join(rprDir, "RPR-Physical_v2.0.xml")

	// 1. Ingest RPR-Base
	if err := metamodel.ParseFOMFile(omt, baseFile); err != nil {
		t.Fatalf("failed to parse RPR-Base_v2.0.xml: %v", err)
	}

	baseEntity, ok := omt.GetObjectClassByName("BaseEntity")
	if !ok {
		t.Fatal("expected 'BaseEntity' in OMT")
	}

	// 2. Ingest RPR-Physical (extends BaseEntity with PhysicalEntity -> Platform -> Aircraft)
	if err := metamodel.ParseFOMFile(omt, physFile); err != nil {
		t.Fatalf("failed to parse RPR-Physical_v2.0.xml: %v", err)
	}

	aircraft, ok := omt.GetObjectClassByName("Aircraft")
	if !ok {
		t.Fatal("expected 'Aircraft' in OMT")
	}

	// Euler Tour verification: Aircraft is subclass of BaseEntity!
	if !omt.IsSubclassOfObject(aircraft.Handle, baseEntity.Handle) {
		t.Fatalf("expected Aircraft to be subclass of BaseEntity!")
	}
	t.Logf("RPR-FOM verified: Aircraft (%d) is a BaseEntity (%d) in $O(1)$ Euler Tour!",
		aircraft.Handle, baseEntity.Handle)
}

func TestXMLParser_All14_RPR_FOM_Modules(t *testing.T) {
	omt := core.NewOMTRegistry()

	rprDir := filepath.Join("..", "..", "test", "resources", "rpr-fom-v2.0")

	// The 14 official SISO RPR-FOM v2.0 modules
	modules := []string{
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

	for _, modName := range modules {
		filePath := filepath.Join(rprDir, modName)
		if err := metamodel.ParseFOMFile(omt, filePath); err != nil {
			t.Fatalf("failed to ingest RPR-FOM module %s: %v", modName, err)
		}
		t.Logf("Successfully ingested module: %s", modName)
	}

	// Verify key classes across the modules
	expectedClasses := []string{
		"HLAobjectRoot",
		"BaseEntity",                 // RPR-Base
		"PhysicalEntity",             // RPR-Physical
		"Platform",                   // RPR-Physical
		"Aircraft",                   // RPR-Physical
		"AggregateEntity",            // RPR-Aggregate
		"RadioTransmitter",           // RPR-Communication
		"RadioReceiver",              // RPR-Communication
		"Minefield",                  // RPR-Minefield
		"MinefieldObject",            // RPR-Minefield
		"MinefieldLaneMarkerObject",  // RPR-Minefield
		"Munition",                   // RPR-Warfare
		"ActiveSonarBeam",            // RPR-UA
	}

	for _, clsName := range expectedClasses {
		cls, ok := omt.GetObjectClassByName(clsName)
		if !ok {
			t.Errorf("expected class '%s' to be present after ingesting all 14 modules", clsName)
		} else {
			t.Logf("Found class '%s' -> Handle=%d, Subtree=[%d, %d]", cls.Name, cls.Handle, cls.IntervalLeft, cls.IntervalRight)
		}
	}

	// Verify key interactions across the modules
	expectedInteractions := []string{
		"HLAinteractionRoot",
		"WeaponFire",       // RPR-Warfare
		"MunitionDetonation", // RPR-Warfare
		"ActionRequest",    // RPR-SIMAN
		"ActionResponse",   // RPR-SIMAN
		"ResupplyOffer",    // RPR-Logistics
		"ResupplyReceived", // RPR-Logistics
		"RadioSignal",      // RPR-Communication
	}

	for _, interName := range expectedInteractions {
		inter, ok := omt.GetInteractionClassByName(interName)
		if !ok {
			t.Errorf("expected interaction '%s' to be present after ingesting all 14 modules", interName)
		} else {
			t.Logf("Found interaction '%s' -> Handle=%d, Subtree=[%d, %d]", inter.Name, inter.Handle, inter.IntervalLeft, inter.IntervalRight)
		}
	}

	// Verify Euler Tour Subclass Relationships spanning modules
	// Aircraft -> Platform -> PhysicalEntity -> BaseEntity -> HLAobjectRoot
	aircraft, _ := omt.GetObjectClassByName("Aircraft")
	baseEntity, _ := omt.GetObjectClassByName("BaseEntity")
	root, _ := omt.GetObjectClassByName("HLAobjectRoot")

	if !omt.IsSubclassOfObject(aircraft.Handle, baseEntity.Handle) {
		t.Errorf("expected Aircraft to be subclass of BaseEntity")
	}
	if !omt.IsSubclassOfObject(aircraft.Handle, root.Handle) {
		t.Errorf("expected Aircraft to be subclass of HLAobjectRoot")
	}

	// WeaponFire -> HLAinteractionRoot
	weaponFire, _ := omt.GetInteractionClassByName("WeaponFire")
	interRoot, _ := omt.GetInteractionClassByName("HLAinteractionRoot")
	if !omt.IsSubclassOfInteraction(weaponFire.Handle, interRoot.Handle) {
		t.Errorf("expected WeaponFire to be subclass of HLAinteractionRoot")
	}
}

func TestXMLParser_ClassNamingCollision_MultiBranch(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<objectModel xmlns="http://standards.ieee.org/IEEE1516-2010">
  <objects>
    <objectClass>
      <name>HLAobjectRoot</name>
      <objectClass>
        <name>Platform</name>
        <objectClass>
          <name>Sensor</name>
          <attribute>
            <name>SensorRange</name>
            <dataType>HLAfloat32BE</dataType>
          </attribute>
        </objectClass>
      </objectClass>
      <objectClass>
        <name>Environment</name>
        <objectClass>
          <name>Sensor</name>
          <attribute>
            <name>AmbientTemp</name>
            <dataType>HLAfloat32BE</dataType>
          </attribute>
        </objectClass>
      </objectClass>
    </objectClass>
  </objects>
  <interactions>
    <interactionClass>
      <name>HLAinteractionRoot</name>
      <interactionClass>
        <name>Radar</name>
        <interactionClass>
          <name>Scan</name>
          <parameter>
            <name>Frequency</name>
            <dataType>HLAfloat32BE</dataType>
          </parameter>
        </interactionClass>
      </interactionClass>
      <interactionClass>
        <name>Sonar</name>
        <interactionClass>
          <name>Scan</name>
          <parameter>
            <name>Depth</name>
            <dataType>HLAfloat32BE</dataType>
          </parameter>
        </interactionClass>
      </interactionClass>
    </interactionClass>
  </interactions>
</objectModel>`)

	omt := core.NewOMTRegistry()
	if err := metamodel.ParseFOMXML(omt, xmlData); err != nil {
		t.Fatalf("falha ao realizar parsing de XML com nomes colidentes em ramos distintos: %v", err)
	}

	// 1. Obter classes por FQCN
	platformSensor, ok := omt.GetObjectClassByName("HLAobjectRoot.Platform.Sensor")
	if !ok {
		t.Fatalf("classe HLAobjectRoot.Platform.Sensor não encontrada")
	}
	envSensor, ok := omt.GetObjectClassByName("HLAobjectRoot.Environment.Sensor")
	if !ok {
		t.Fatalf("classe HLAobjectRoot.Environment.Sensor não encontrada")
	}

	if platformSensor.Handle == envSensor.Handle {
		t.Fatalf("Platform.Sensor e Environment.Sensor não podem compartilhar o mesmo handle (%d)", platformSensor.Handle)
	}

	// 2. Atributos devem ser isolados em suas respectivas classes
	if _, err := omt.ResolveAttributeByName(platformSensor.Handle, "SensorRange"); err != nil {
		t.Fatalf("SensorRange deve existir em Platform.Sensor: %v", err)
	}
	if _, err := omt.ResolveAttributeByName(platformSensor.Handle, "AmbientTemp"); err == nil {
		t.Fatalf("AmbientTemp NÃO deve existir em Platform.Sensor")
	}
	if _, err := omt.ResolveAttributeByName(envSensor.Handle, "AmbientTemp"); err != nil {
		t.Fatalf("AmbientTemp deve existir em Environment.Sensor: %v", err)
	}
	if _, err := omt.ResolveAttributeByName(envSensor.Handle, "SensorRange"); err == nil {
		t.Fatalf("SensorRange NÃO deve existir em Environment.Sensor")
	}

	// 3. Interações por FQCN
	radarScan, ok := omt.GetInteractionClassByName("HLAinteractionRoot.Radar.Scan")
	if !ok {
		t.Fatalf("interação HLAinteractionRoot.Radar.Scan não encontrada")
	}
	sonarScan, ok := omt.GetInteractionClassByName("HLAinteractionRoot.Sonar.Scan")
	if !ok {
		t.Fatalf("interação HLAinteractionRoot.Sonar.Scan não encontrada")
	}

	if radarScan.Handle == sonarScan.Handle {
		t.Fatalf("Radar.Scan e Sonar.Scan não podem compartilhar o mesmo handle (%d)", radarScan.Handle)
	}
}

func TestXMLParser_ExtensionFOM_TargetedClassMerge(t *testing.T) {
	baseXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<objectModel xmlns="http://standards.ieee.org/IEEE1516-2010">
  <objects>
    <objectClass>
      <name>HLAobjectRoot</name>
      <objectClass>
        <name>Platform</name>
        <objectClass>
          <name>Sensor</name>
          <attribute>
            <name>BaseSensitivity</name>
            <dataType>HLAfloat32BE</dataType>
          </attribute>
        </objectClass>
      </objectClass>
    </objectClass>
  </objects>
</objectModel>`)

	extXML := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<objectModel xmlns="http://standards.ieee.org/IEEE1516-2010">
  <objects>
    <objectClass>
      <name>HLAobjectRoot</name>
      <objectClass>
        <name>Platform</name>
        <objectClass>
          <name>Sensor</name>
          <attribute>
            <name>ExtendedFrequency</name>
            <dataType>HLAfloat32BE</dataType>
          </attribute>
        </objectClass>
      </objectClass>
    </objectClass>
  </objects>
</objectModel>`)

	omt := core.NewOMTRegistry()
	if err := metamodel.ParseFOMXML(omt, baseXML); err != nil {
		t.Fatalf("falha ao ingerir base XML: %v", err)
	}
	if err := metamodel.ParseFOMXML(omt, extXML); err != nil {
		t.Fatalf("falha ao ingerir extension XML: %v", err)
	}

	sensor, ok := omt.GetObjectClassByName("Platform.Sensor")
	if !ok {
		t.Fatalf("Platform.Sensor não encontrado")
	}

	if _, err := omt.ResolveAttributeByName(sensor.Handle, "BaseSensitivity"); err != nil {
		t.Fatalf("BaseSensitivity deve existir em Platform.Sensor: %v", err)
	}
	if _, err := omt.ResolveAttributeByName(sensor.Handle, "ExtendedFrequency"); err != nil {
		t.Fatalf("ExtendedFrequency deve ser mesclado em Platform.Sensor: %v", err)
	}
}
