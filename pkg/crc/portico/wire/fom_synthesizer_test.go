package wire_test

import (
	"testing"

	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

func TestFOMSynthesizer_EndToEndRoundtrip(t *testing.T) {
	omt := core.NewOMTRegistry()

	// 1. Add some classes and attributes to OMT
	restaurantClass, err := omt.AddObjectClass("Restaurant", core.HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("failed to add Restaurant class: %v", err)
	}

	drinkClass, err := omt.AddObjectClass("Drink", restaurantClass.Handle)
	if err != nil {
		t.Fatalf("failed to add Drink class: %v", err)
	}

	flavorAttr, err := omt.AddAttribute(drinkClass.Handle, "Flavor", "HLAASCIIstring")
	if err != nil {
		t.Fatalf("failed to add Flavor attribute: %v", err)
	}

	// 2. Synthesize ObjectModel
	fomObj := wire.SynthesizePorticoObjectModel(omt)
	if fomObj == nil {
		t.Fatal("expected non-nil synthesized ObjectModel")
	}

	// 3. Assemble WelcomePack with the synthesized FOM
	wp := metamodel.NewWelcomePack(1, 42, "RestaurantFederation", "ThorFederate", "JavaClient")
	wp.Set("fom", fomObj)

	resp := metamodel.NewExtendedSuccessResponse(wp)

	// 4. Serialize to wire format
	payload, err := wire.SerializePorticoControlMessage(resp)
	if err != nil {
		t.Fatalf("failed to serialize synthesized WelcomePack: %v", err)
	}
	t.Logf("Synthesized WelcomePack serialized size: %d bytes", len(payload))

	// 5. Deserialize to simulate Portico Java client decoding
	decoded, err := wire.DeserializePorticoControlMessage(payload)
	if err != nil {
		t.Fatalf("failed to deserialize synthesized WelcomePack: %v", err)
	}

	results := decoded.Get("results").(*metamodel.JavaObject)
	entries := results.Get("entries").(map[any]any)
	decodedWP := entries["result"].(*metamodel.JavaObject)
	decodedFOM := decodedWP.Get("fom").(*metamodel.JavaObject)

	if decodedFOM == nil {
		t.Fatal("decoded FOM is nil")
	}

	// 6. Verify class hierarchy in decoded FOM
	oclasses := decodedFOM.Get("oclasses").(*metamodel.JavaObject)
	ocEntries := oclasses.Get("entries").(map[any]any)

	// Should have HLAobjectRoot, Restaurant, Drink
	if len(ocEntries) < 3 {
		t.Errorf("expected at least 3 object classes, got %d", len(ocEntries))
	}

	// Look for Drink class
	var foundDrink *metamodel.JavaObject
	for _, v := range ocEntries {
		if oc, ok := v.(*metamodel.JavaObject); ok {
			if oc.Get("name") == "Drink" {
				foundDrink = oc
				break
			}
		}
	}

	if foundDrink == nil {
		t.Fatal("Drink class not found in synthesized FOM")
	}

	if foundDrink.Get("handle") != int32(drinkClass.Handle) {
		t.Errorf("expected Drink handle %d, got %v", drinkClass.Handle, foundDrink.Get("handle"))
	}

	if foundDrink.Get("qualifiedName") != "HLAobjectRoot.Restaurant.Drink" {
		t.Errorf("expected qualifiedName 'HLAobjectRoot.Restaurant.Drink', got '%v'", foundDrink.Get("qualifiedName"))
	}

	if foundDrink.Get("vsafeQualifiedName") != "Restaurant.Drink" {
		t.Errorf("expected vsafeQualifiedName 'Restaurant.Drink', got '%v'", foundDrink.Get("vsafeQualifiedName"))
	}

	// Verify Drink's Flavor attribute
	attrs := foundDrink.Get("attributes").(*metamodel.JavaObject)
	attrEntries := attrs.Get("entries").(map[any]any)
	if len(attrEntries) != 1 {
		t.Fatalf("expected 1 attribute on Drink, got %d", len(attrEntries))
	}

	for _, av := range attrEntries {
		ac := av.(*metamodel.JavaObject)
		if ac.Get("name") != "Flavor" {
			t.Errorf("expected attribute name 'Flavor', got '%v'", ac.Get("name"))
		}
		if ac.Get("handle") != int32(flavorAttr.Handle) {
			t.Errorf("expected attribute handle %d, got %v", flavorAttr.Handle, ac.Get("handle"))
		}
	}

	t.Logf("Synthesized FOM validated with 100%% precision!")
}
