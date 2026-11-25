package core

import (
	"reflect"
	"sort"
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
)

func TestInterestMatcher_PolymorphicObjectSubscriptions(t *testing.T) {
	reg := NewOMTRegistry()

	// Hierarquia: HLAobjectRoot -> BaseEntity -> Platform -> Aircraft -> FighterJet
	baseEntity, err := reg.AddObjectClass("BaseEntity", HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("falha ao criar BaseEntity: %v", err)
	}
	platform, err := reg.AddObjectClass("Platform", baseEntity.Handle)
	if err != nil {
		t.Fatalf("falha ao criar Platform: %v", err)
	}
	aircraft, err := reg.AddObjectClass("Aircraft", platform.Handle)
	if err != nil {
		t.Fatalf("falha ao criar Aircraft: %v", err)
	}
	fighterJet, err := reg.AddObjectClass("FighterJet", aircraft.Handle)
	if err != nil {
		t.Fatalf("falha ao criar FighterJet: %v", err)
	}

	// Atributos
	posAttr, _ := reg.AddAttribute(baseEntity.Handle, "Position", "Spatial")
	altAttr, _ := reg.AddAttribute(aircraft.Handle, "Altitude", "Float32")

	router := NewInterestMatcher(reg)

	// Fed 1 assina Position na classe abstrata BaseEntity
	err = router.SubscribeObjectClass(1, baseEntity.Handle, []common.AttributeHandle{posAttr.Handle}, true)
	if err != nil {
		t.Fatalf("falha ao assinar BaseEntity.Position: %v", err)
	}

	// Fed 2 assina Altitude na classe Aircraft
	err = router.SubscribeObjectClass(2, aircraft.Handle, []common.AttributeHandle{altAttr.Handle}, true)
	if err != nil {
		t.Fatalf("falha ao assinar Aircraft.Altitude: %v", err)
	}

	// Fed 3 assina Position na classe folha FighterJet
	err = router.SubscribeObjectClass(3, fighterJet.Handle, []common.AttributeHandle{posAttr.Handle}, true)
	if err != nil {
		t.Fatalf("falha ao assinar FighterJet.Position: %v", err)
	}

	// 1. Consulta assinantes de Position no FighterJet -> Deve retornar Fed 1 (ancestral) e Fed 3 (direto)
	posSubs := router.GetAttributeSubscribers(fighterJet.Handle, posAttr.Handle)
	sort.Slice(posSubs, func(i, j int) bool { return posSubs[i] < posSubs[j] })
	expectedPos := []common.FederateHandle{1, 3}
	if !reflect.DeepEqual(posSubs, expectedPos) {
		t.Fatalf("Assinantes de FighterJet.Position incorretos: esperado %v, obteve %v", expectedPos, posSubs)
	}

	// 2. Consulta assinantes de Altitude no FighterJet -> Deve retornar Fed 2 (ancestral Aircraft)
	altSubs := router.GetAttributeSubscribers(fighterJet.Handle, altAttr.Handle)
	expectedAlt := []common.FederateHandle{2}
	if !reflect.DeepEqual(altSubs, expectedAlt) {
		t.Fatalf("Assinantes de FighterJet.Altitude incorretos: esperado %v, obteve %v", expectedAlt, altSubs)
	}

	// 3. Consulta assinantes de Altitude no BaseEntity -> Não tem nenhum assinante (apenas Aircraft e filhos)
	baseAltSubs := router.GetAttributeSubscribers(baseEntity.Handle, altAttr.Handle)
	if len(baseAltSubs) != 0 {
		t.Fatalf("BaseEntity não deveria ter assinantes de Altitude, obteve %v", baseAltSubs)
	}

	// 4. Testar cancelamento de assinatura (Unsubscribe)
	err = router.UnsubscribeObjectClass(1, baseEntity.Handle)
	if err != nil {
		t.Fatalf("falha no Unsubscribe: %v", err)
	}
	posSubsAfter := router.GetAttributeSubscribers(fighterJet.Handle, posAttr.Handle)
	expectedAfter := []common.FederateHandle{3}
	if !reflect.DeepEqual(posSubsAfter, expectedAfter) {
		t.Fatalf("após Unsubscribe de Fed 1, esperava %v, obteve %v", expectedAfter, posSubsAfter)
	}
}

func TestInterestMatcher_PolymorphicInteractions(t *testing.T) {
	reg := NewOMTRegistry()

	// Hierarquia: HLAinteractionRoot -> WeaponFire -> MissileLaunch
	//                                -> Collision
	weaponFire, err := reg.AddInteractionClass("WeaponFire", HLAinteractionRootHandle)
	if err != nil {
		t.Fatalf("falha ao criar WeaponFire: %v", err)
	}
	missileLaunch, err := reg.AddInteractionClass("MissileLaunch", weaponFire.Handle)
	if err != nil {
		t.Fatalf("falha ao criar MissileLaunch: %v", err)
	}
	collision, err := reg.AddInteractionClass("Collision", HLAinteractionRootHandle)
	if err != nil {
		t.Fatalf("falha ao criar Collision: %v", err)
	}

	router := NewInterestMatcher(reg)

	// Fed 10 assina a raiz global HLAinteractionRoot (deve receber tudo)
	router.SubscribeInteractionClass(10, HLAinteractionRootHandle, true)

	// Fed 20 assina WeaponFire
	router.SubscribeInteractionClass(20, weaponFire.Handle, true)

	// Fed 30 assina apenas MissileLaunch
	router.SubscribeInteractionClass(30, missileLaunch.Handle, true)

	// Fed 40 assina Collision
	router.SubscribeInteractionClass(40, collision.Handle, true)

	// Disparo de MissileLaunch -> Destinatários: Fed 10, Fed 20, Fed 30 (Fed 40 NÃO recebe)
	recipients := router.GetInteractionSubscribers(missileLaunch.Handle)
	sort.Slice(recipients, func(i, j int) bool { return recipients[i] < recipients[j] })
	expected := []common.FederateHandle{10, 20, 30}
	if !reflect.DeepEqual(recipients, expected) {
		t.Fatalf("destinatários de MissileLaunch incorretos: esperado %v, obteve %v", expected, recipients)
	}

	// Disparo de Collision -> Destinatários: Fed 10, Fed 40
	colRecipients := router.GetInteractionSubscribers(collision.Handle)
	sort.Slice(colRecipients, func(i, j int) bool { return colRecipients[i] < colRecipients[j] })
	expectedCol := []common.FederateHandle{10, 40}
	if !reflect.DeepEqual(colRecipients, expectedCol) {
		t.Fatalf("destinatários de Collision incorretos: esperado %v, obteve %v", expectedCol, colRecipients)
	}

	// Testar RemoveFederate (Resign do Fed 10)
	router.RemoveFederate(10)
	afterResign := router.GetInteractionSubscribers(missileLaunch.Handle)
	sort.Slice(afterResign, func(i, j int) bool { return afterResign[i] < afterResign[j] })
	expectedAfter := []common.FederateHandle{20, 30}
	if !reflect.DeepEqual(afterResign, expectedAfter) {
		t.Fatalf("após resign do Fed 10, esperava %v, obteve %v", expectedAfter, afterResign)
	}
}

func TestInterestMatcher_NegativeCases(t *testing.T) {
	reg := NewOMTRegistry()
	router := NewInterestMatcher(reg)

	// Classe inexistente
	err := router.PublishObjectClass(1, 9999, []common.AttributeHandle{1})
	if err != ErrClassNotFound {
		t.Fatalf("esperava ErrClassNotFound, obteve %v", err)
	}

	// Atributo inexistente na hierarquia
	err = router.PublishObjectClass(1, HLAobjectRootHandle, []common.AttributeHandle{9999})
	if err != ErrAttributeNotFound {
		t.Fatalf("esperava ErrAttributeNotFound, obteve %v", err)
	}

	// Interação inexistente
	err = router.SubscribeInteractionClass(1, 9999, true)
	if err != ErrInteractionNotFound {
		t.Fatalf("esperava ErrInteractionNotFound, obteve %v", err)
	}
}

func BenchmarkInterestMatcher_GetAttributeSubscribers(b *testing.B) {
	reg := NewOMTRegistry()
	base, _ := reg.AddObjectClass("Base", HLAobjectRootHandle)
	mid, _ := reg.AddObjectClass("Mid", base.Handle)
	leaf, _ := reg.AddObjectClass("Leaf", mid.Handle)

	attr, _ := reg.AddAttribute(base.Handle, "Telemetry", "Raw")

	router := NewInterestMatcher(reg)

	// 50 federados assinando na raiz, 50 no meio, 50 na folha
	for i := common.FederateHandle(1); i <= 50; i++ {
		_ = router.SubscribeObjectClass(i, base.Handle, []common.AttributeHandle{attr.Handle}, true)
		_ = router.SubscribeObjectClass(i+50, mid.Handle, []common.AttributeHandle{attr.Handle}, true)
		_ = router.SubscribeObjectClass(i+100, leaf.Handle, []common.AttributeHandle{attr.Handle}, true)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = router.GetAttributeSubscribers(leaf.Handle, attr.Handle)
	}
}
