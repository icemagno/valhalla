package core

import (
	"testing"
)

func TestOMTRegistry_StandardMIM(t *testing.T) {
	reg := NewOMTRegistry()

	// 1. Verificar HLAobjectRoot
	objRoot, exists := reg.GetObjectClass(HLAobjectRootHandle)
	if !exists {
		t.Fatalf("HLAobjectRoot não foi inicializado no MIM padrão")
	}
	if objRoot.Name != HLAobjectRootName {
		t.Fatalf("nome inválido para HLAobjectRoot: %s", objRoot.Name)
	}

	// 2. Verificar HLAprivilegeToDeleteObject
	privAttr, err := reg.ResolveAttribute(HLAobjectRootHandle, HLAprivilegeToDeleteObjectHandle)
	if err != nil {
		t.Fatalf("falha ao resolver HLAprivilegeToDeleteObject: %v", err)
	}
	if privAttr.Name != HLAprivilegeToDeleteObjectName {
		t.Fatalf("nome de atributo inesperado: %s", privAttr.Name)
	}

	// 3. Verificar HLAinteractionRoot
	intRoot, exists := reg.GetInteractionClass(HLAinteractionRootHandle)
	if !exists {
		t.Fatalf("HLAinteractionRoot não foi inicializado no MIM padrão")
	}
	if intRoot.Name != HLAinteractionRootName {
		t.Fatalf("nome inválido para HLAinteractionRoot: %s", intRoot.Name)
	}
}

func TestOMTRegistry_HierarchyAndEulerIntervals(t *testing.T) {
	reg := NewOMTRegistry()

	// Construir hierarquia:
	// HLAobjectRoot (1)
	//   -> BaseEntity (2)
	//        -> Platform (3)
	//             -> Aircraft (4)
	//                  -> FighterJet (5)
	//   -> CulturalFeature (6)

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
	cultural, err := reg.AddObjectClass("CulturalFeature", HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("falha ao criar CulturalFeature: %v", err)
	}

	// Adicionar atributos em diferentes níveis
	attrPos, err := reg.AddAttribute(baseEntity.Handle, "Position", "Spatial")
	if err != nil {
		t.Fatalf("falha ao criar Position: %v", err)
	}
	attrAlt, err := reg.AddAttribute(aircraft.Handle, "Altitude", "Float32")
	if err != nil {
		t.Fatalf("falha ao criar Altitude: %v", err)
	}
	attrBay, err := reg.AddAttribute(fighterJet.Handle, "WeaponBayOpen", "Boolean")
	if err != nil {
		t.Fatalf("falha ao criar WeaponBayOpen: %v", err)
	}

	// Testar Resolução de Herança de Atributos
	resolvedPos, err := reg.ResolveAttribute(fighterJet.Handle, attrPos.Handle)
	if err != nil || resolvedPos.Name != "Position" {
		t.Fatalf("FighterJet falhou ao herdar Position de BaseEntity: %v", err)
	}

	resolvedAlt, err := reg.ResolveAttribute(fighterJet.Handle, attrAlt.Handle)
	if err != nil || resolvedAlt.Name != "Altitude" {
		t.Fatalf("FighterJet falhou ao herdar Altitude de Aircraft: %v", err)
	}

	resolvedBay, err := reg.ResolveAttribute(fighterJet.Handle, attrBay.Handle)
	if err != nil || resolvedBay.Name != "WeaponBayOpen" {
		t.Fatalf("FighterJet falhou ao resolver seu próprio atributo WeaponBayOpen: %v", err)
	}

	// Atributo do FighterJet NÃO deve existir em BaseEntity
	if _, err := reg.ResolveAttribute(baseEntity.Handle, attrBay.Handle); err == nil {
		t.Fatalf("BaseEntity resolveu WeaponBayOpen incorretamente")
	}

	// Testar Subclass O(1) via Euler Tour Interval Encoding
	if !reg.IsSubclassOfObject(fighterJet.Handle, aircraft.Handle) {
		t.Fatalf("FighterJet deve ser subclasse de Aircraft")
	}
	if !reg.IsSubclassOfObject(fighterJet.Handle, platform.Handle) {
		t.Fatalf("FighterJet deve ser subclasse de Platform")
	}
	if !reg.IsSubclassOfObject(fighterJet.Handle, baseEntity.Handle) {
		t.Fatalf("FighterJet deve ser subclasse de BaseEntity")
	}
	if !reg.IsSubclassOfObject(fighterJet.Handle, HLAobjectRootHandle) {
		t.Fatalf("FighterJet deve ser subclasse de HLAobjectRoot")
	}

	// FighterJet NÃO é subclasse de CulturalFeature
	if reg.IsSubclassOfObject(fighterJet.Handle, cultural.Handle) {
		t.Fatalf("FighterJet NÃO deve ser subclasse de CulturalFeature")
	}
	// Platform NÃO é subclasse de FighterJet
	if reg.IsSubclassOfObject(platform.Handle, fighterJet.Handle) {
		t.Fatalf("Platform NÃO deve ser subclasse de FighterJet")
	}
}

func BenchmarkOMTRegistry_IsSubclassOfObject(b *testing.B) {
	reg := NewOMTRegistry()
	c1, _ := reg.AddObjectClass("C1", HLAobjectRootHandle)
	c2, _ := reg.AddObjectClass("C2", c1.Handle)
	c3, _ := reg.AddObjectClass("C3", c2.Handle)
	c4, _ := reg.AddObjectClass("C4", c3.Handle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.IsSubclassOfObject(c4.Handle, c1.Handle)
	}
}

func BenchmarkOMTRegistry_ResolveAttribute(b *testing.B) {
	reg := NewOMTRegistry()
	c1, _ := reg.AddObjectClass("C1", HLAobjectRootHandle)
	attr, _ := reg.AddAttribute(c1.Handle, "Pos", "Spatial")
	c2, _ := reg.AddObjectClass("C2", c1.Handle)
	c3, _ := reg.AddObjectClass("C3", c2.Handle)
	c4, _ := reg.AddObjectClass("C4", c3.Handle)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = reg.ResolveAttribute(c4.Handle, attr.Handle)
	}
}
