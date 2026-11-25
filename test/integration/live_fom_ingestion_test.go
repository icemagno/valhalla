package integration_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/crc/portico/transport"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

func TestIntegration_PorticoXMLFOMIngestionAndWelcomePack(t *testing.T) {
	// 1. Start sovereign Valhalla CRC
	server := crc.NewServer("127.0.0.1:0")
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start CRC server: %v", err)
	}
	defer server.Stop()

	crcAddr := server.Addr().String()

	// 2. Connect legacy Portico client (Thor)
	clientThor := transport.NewTCPClient(crcAddr)
	if err := clientThor.Connect(3 * time.Second); err != nil {
		t.Fatalf("failed to connect Thor to CRC: %v", err)
	}
	defer clientThor.Close()

	// 3. Thor prepares CreateFederation containing RestaurantFood.xml
	fomPath := filepath.Join("..", "foms", "RestaurantFood.xml")
	xmlBytes, err := os.ReadFile(fomPath)
	if err != nil {
		t.Fatalf("failed to read RestaurantFood.xml: %v", err)
	}

	createObj := wire.NewCreateFederationObject("RestaurantFederation")
	modulesList := metamodel.NewJavaArrayList([]any{xmlBytes})
	createObj.Set("modules", modulesList)

	createPayload, err := wire.SerializePorticoControlMessage(createObj)
	if err != nil {
		t.Fatalf("failed to serialize CreateFederation: %v", err)
	}

	createHdr := wire.NewControlRequest(wire.MsgCreateFederation, 1, 0, 0, uint32(len(createPayload)))
	if err := clientThor.SendPacket(createHdr, createPayload); err != nil {
		t.Fatalf("failed to send CreateFederation: %v", err)
	}

	_, cfRespPayload, err := clientThor.ReadPacketTimeout(3 * time.Second)
	if err != nil {
		t.Fatalf("failed to read CreateFederation response: %v", err)
	}
	if len(cfRespPayload) == 0 {
		t.Fatal("empty CreateFederation response")
	}

	// 4. Thor joins the federation
	joinObj := wire.NewJoinFederationObject("RestaurantFederation", "ThorWaitstaff", "JavaWaitstaff")
	joinPayload, err := wire.SerializePorticoControlMessage(joinObj)
	if err != nil {
		t.Fatalf("failed to serialize JoinFederation: %v", err)
	}
	joinHdr := wire.NewControlRequest(wire.MsgJoinFederation, 2, 0, 0, uint32(len(joinPayload)))
	if err := clientThor.SendPacket(joinHdr, joinPayload); err != nil {
		t.Fatalf("failed to send JoinFederation: %v", err)
	}

	// 5. Read WelcomePack containing the dynamically synthesized ObjectModel from the XML
	_, wpPayload, err := clientThor.ReadPacketTimeout(3 * time.Second)
	if err != nil {
		t.Fatalf("failed to read WelcomePack: %v", err)
	}

	wpObj, err := wire.DeserializePorticoControlMessage(wpPayload)
	if err != nil {
		t.Fatalf("failed to deserialize WelcomePack: %v", err)
	}

	results := wpObj.Get("results").(*metamodel.JavaObject)
	entries := results.Get("entries").(map[any]any)
	wpObjActual := entries["result"].(*metamodel.JavaObject)
	fomObj, ok := wpObjActual.Get("fom").(*metamodel.JavaObject)
	if !ok || fomObj == nil {
		t.Fatal("expected synthesized FOM in WelcomePack")
	}

	// 6. Inspect the dynamically parsed classes from the XML in the WelcomePack!
	oclasses := fomObj.Get("oclasses").(*metamodel.JavaObject)
	ocEntries := oclasses.Get("entries").(map[any]any)

	var foundDrink *metamodel.JavaObject
	var foundFood *metamodel.JavaObject

	for _, v := range ocEntries {
		if oc, ok := v.(*metamodel.JavaObject); ok {
			if oc.Get("name") == "Drink" {
				foundDrink = oc
			}
			if oc.Get("name") == "Food" {
				foundFood = oc
			}
		}
	}

	if foundFood == nil {
		t.Fatal("class 'Food' parsed from XML was not found in WelcomePack!")
	}
	if foundDrink == nil {
		t.Fatal("class 'Drink' parsed from XML was not found in WelcomePack!")
	}

	t.Logf(">>> XML PARSER SUCCESS: Found Food (handle %v) and Drink (handle %v) in WelcomePack!",
		foundFood.Get("handle"), foundDrink.Get("handle"))

	// Verify Drink's NumberCups attribute parsed from XML
	attrs := foundDrink.Get("attributes").(*metamodel.JavaObject)
	attrEntries := attrs.Get("entries").(map[any]any)
	var foundNumberCups *metamodel.JavaObject
	for _, av := range attrEntries {
		if ac, ok := av.(*metamodel.JavaObject); ok {
			if ac.Get("name") == "NumberCups" {
				foundNumberCups = ac
			}
		}
	}

	if foundNumberCups == nil {
		t.Fatal("attribute 'NumberCups' parsed from XML was not found in Drink class!")
	}
	t.Logf(">>> XML ATTRIBUTE SUCCESS: Found NumberCups (handle %v) on Drink!", foundNumberCups.Get("handle"))

	// 7. Thor registers an instance of Drink using the handle received in WelcomePack
	drinkHandle := foundDrink.Get("handle").(int32)
	regPayload, err := wire.NewRegisterObjectMsg(drinkHandle, "ThorDrink_01")
	if err != nil {
		t.Fatalf("failed to serialize RegisterObject: %v", err)
	}

	regHdr := wire.NewControlRequest(wire.MsgRegisterObject, 3, 1, 0, uint32(len(regPayload)))
	if err := clientThor.SendPacket(regHdr, regPayload); err != nil {
		t.Fatalf("failed to send RegisterObjectInstance: %v", err)
	}

	_, regRespPayload, err := clientThor.ReadPacketTimeout(3 * time.Second)
	if err != nil {
		t.Fatalf("failed to read RegisterObject response: %v", err)
	}
	if len(regRespPayload) == 0 {
		t.Fatal("empty RegisterObject response")
	}

	t.Logf(">>> REGISTRATION SUCCESS: ThorDrink_01 registered successfully against dynamically parsed XML class!")

	// 8. Graceful resign
	resignObj := wire.NewResignFederationObject("RestaurantFederation", "ThorWaitstaff", "ieee1516e", "NO_ACTION")
	resignPayload, _ := wire.SerializePorticoControlMessage(resignObj)
	resignHdr := wire.NewControlRequest(wire.MsgResignFederation, 4, 1, 0, uint32(len(resignPayload)))
	_ = clientThor.SendPacket(resignHdr, resignPayload)
	_, _, _ = clientThor.ReadPacketTimeout(2 * time.Second)
}
