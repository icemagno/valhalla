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
