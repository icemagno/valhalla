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

func TestOMTRegistry_ClassNamingCollision_DifferentBranches(t *testing.T) {
	reg := NewOMTRegistry()

	branchA, err := reg.AddObjectClass("BranchA", HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("falha ao criar BranchA: %v", err)
	}
	branchB, err := reg.AddObjectClass("BranchB", HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("falha ao criar BranchB: %v", err)
	}

	// 1. Criar classe com mesmo nome simples "Sensor" sob ramos diferentes
	sensorA, err := reg.AddObjectClass("Sensor", branchA.Handle)
	if err != nil {
		t.Fatalf("falha ao criar Sensor sob BranchA: %v", err)
	}
	sensorB, err := reg.AddObjectClass("Sensor", branchB.Handle)
	if err != nil {
		t.Fatalf("falha ao criar Sensor sob BranchB (colisão indevida): %v", err)
	}

	if sensorA.Handle == sensorB.Handle {
		t.Fatalf("SensorA e SensorB devem ter handles distintos; obtido: %d e %d", sensorA.Handle, sensorB.Handle)
	}
	if sensorA.QualifiedName != "HLAobjectRoot.BranchA.Sensor" {
		t.Fatalf("FQCN inesperado para SensorA: %s", sensorA.QualifiedName)
	}
	if sensorB.QualifiedName != "HLAobjectRoot.BranchB.Sensor" {
		t.Fatalf("FQCN inesperado para SensorB: %s", sensorB.QualifiedName)
	}

	// 2. Atributos isolados
	attrA, err := reg.AddAttribute(sensorA.Handle, "Range", "HLAfloat32BE")
	if err != nil {
		t.Fatalf("falha ao adicionar Range em SensorA: %v", err)
	}
	attrB, err := reg.AddAttribute(sensorB.Handle, "Temperature", "HLAfloat32BE")
	if err != nil {
		t.Fatalf("falha ao adicionar Temperature em SensorB: %v", err)
	}

	if _, err := reg.ResolveAttribute(sensorA.Handle, attrA.Handle); err != nil {
		t.Fatalf("Range deve existir em SensorA: %v", err)
	}
	if _, err := reg.ResolveAttribute(sensorA.Handle, attrB.Handle); err == nil {
		t.Fatalf("Temperature NÃO deve existir em SensorA")
	}
	if _, err := reg.ResolveAttribute(sensorB.Handle, attrB.Handle); err != nil {
		t.Fatalf("Temperature deve existir em SensorB: %v", err)
	}
	if _, err := reg.ResolveAttribute(sensorB.Handle, attrA.Handle); err == nil {
		t.Fatalf("Range NÃO deve existir em SensorB")
	}

	// 3. Subclasses Euler Tour
	if !reg.IsSubclassOfObject(sensorA.Handle, branchA.Handle) {
		t.Fatalf("SensorA deve ser subclasse de BranchA")
	}
	if reg.IsSubclassOfObject(sensorA.Handle, branchB.Handle) {
		t.Fatalf("SensorA NÃO deve ser subclasse de BranchB")
	}
	if !reg.IsSubclassOfObject(sensorB.Handle, branchB.Handle) {
		t.Fatalf("SensorB deve ser subclasse de BranchB")
	}
	if reg.IsSubclassOfObject(sensorB.Handle, branchA.Handle) {
		t.Fatalf("SensorB NÃO deve ser subclasse de BranchA")
	}

	// 4. Resolução por FQCN canônico
	cls1, ok1 := reg.GetObjectClassByName("HLAobjectRoot.BranchA.Sensor")
	if !ok1 || cls1.Handle != sensorA.Handle {
		t.Fatalf("falha ao resolver por FQCN completo BranchA.Sensor")
	}
	cls2, ok2 := reg.GetObjectClassByName("HLAobjectRoot.BranchB.Sensor")
	if !ok2 || cls2.Handle != sensorB.Handle {
		t.Fatalf("falha ao resolver por FQCN completo BranchB.Sensor")
	}

	// 5. Resolução por sufixo qualificado sem HLAobjectRoot
	clsRel1, okRel1 := reg.GetObjectClassByName("BranchA.Sensor")
	if !okRel1 || clsRel1.Handle != sensorA.Handle {
		t.Fatalf("falha ao resolver por nome qualificado relativo BranchA.Sensor")
	}
	clsRel2, okRel2 := reg.GetObjectClassByName("BranchB.Sensor")
	if !okRel2 || clsRel2.Handle != sensorB.Handle {
		t.Fatalf("falha ao resolver por nome qualificado relativo BranchB.Sensor")
	}

	// 6. Resolução por nome simples ambíguo DEVE falhar
	if _, okAmb := reg.GetObjectClassByName("Sensor"); okAmb {
		t.Fatalf("GetObjectClassByName('Sensor') deveria falhar por ambiguidade")
	}
	if _, errAmb := reg.ResolveObjectClass("Sensor"); errAmb != ErrAmbiguousClass {
		t.Fatalf("ResolveObjectClass('Sensor') esperado ErrAmbiguousClass, obtido: %v", errAmb)
	}

	// 7. Resolução por nome simples não-ambíguo DEVE suceder
	clsBranchA, okBranchA := reg.GetObjectClassByName("BranchA")
	if !okBranchA || clsBranchA.Handle != branchA.Handle {
		t.Fatalf("GetObjectClassByName('BranchA') deveria resolver unicamente")
	}
}

func TestOMTRegistry_InteractionNamingCollision_DifferentBranches(t *testing.T) {
	reg := NewOMTRegistry()

	sysA, err := reg.AddInteractionClass("SystemA", HLAinteractionRootHandle)
	if err != nil {
		t.Fatalf("falha ao criar SystemA: %v", err)
	}
	sysB, err := reg.AddInteractionClass("SystemB", HLAinteractionRootHandle)
	if err != nil {
		t.Fatalf("falha ao criar SystemB: %v", err)
	}

	// Criar interação com mesmo nome simples "Ping" sob ramos diferentes
	pingA, err := reg.AddInteractionClass("Ping", sysA.Handle)
	if err != nil {
		t.Fatalf("falha ao criar Ping sob SystemA: %v", err)
	}
	pingB, err := reg.AddInteractionClass("Ping", sysB.Handle)
	if err != nil {
		t.Fatalf("falha ao criar Ping sob SystemB (colisão indevida): %v", err)
	}

	if pingA.Handle == pingB.Handle {
		t.Fatalf("PingA e PingB devem ter handles distintos; obtido: %d e %d", pingA.Handle, pingB.Handle)
	}
	if pingA.QualifiedName != "HLAinteractionRoot.SystemA.Ping" {
		t.Fatalf("FQCN inesperado para PingA: %s", pingA.QualifiedName)
	}
	if pingB.QualifiedName != "HLAinteractionRoot.SystemB.Ping" {
		t.Fatalf("FQCN inesperado para PingB: %s", pingB.QualifiedName)
	}

	// Resolução FQCN
	p1, ok1 := reg.GetInteractionClassByName("HLAinteractionRoot.SystemA.Ping")
	if !ok1 || p1.Handle != pingA.Handle {
		t.Fatalf("falha ao resolver HLAinteractionRoot.SystemA.Ping")
	}
	p2, ok2 := reg.GetInteractionClassByName("SystemB.Ping")
	if !ok2 || p2.Handle != pingB.Handle {
		t.Fatalf("falha ao resolver SystemB.Ping")
	}

	// Ambiguidade
	if _, okAmb := reg.GetInteractionClassByName("Ping"); okAmb {
		t.Fatalf("GetInteractionClassByName('Ping') deveria falhar por ambiguidade")
	}
	if _, errAmb := reg.ResolveInteractionClass("Ping"); errAmb != ErrAmbiguousInteraction {
		t.Fatalf("ResolveInteractionClass('Ping') esperado ErrAmbiguousInteraction, obtido: %v", errAmb)
	}
}

func TestOMTRegistry_TrueDuplicate_SameParent(t *testing.T) {
	reg := NewOMTRegistry()

	branch, err := reg.AddObjectClass("Branch", HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("falha ao criar Branch: %v", err)
	}

	_, err = reg.AddObjectClass("Sensor", branch.Handle)
	if err != nil {
		t.Fatalf("primeira criação de Sensor falhou: %v", err)
	}

	// Segunda tentativa de criar Sensor sob o MESMO pai DEVE falhar com ErrDuplicateClass
	_, err = reg.AddObjectClass("Sensor", branch.Handle)
	if err != ErrDuplicateClass {
		t.Fatalf("esperava ErrDuplicateClass para duplicata sob mesmo pai, obtido: %v", err)
	}

	sys, err := reg.AddInteractionClass("System", HLAinteractionRootHandle)
	if err != nil {
		t.Fatalf("falha ao criar System: %v", err)
	}

	_, err = reg.AddInteractionClass("Ping", sys.Handle)
	if err != nil {
		t.Fatalf("primeira criação de Ping falhou: %v", err)
	}

	_, err = reg.AddInteractionClass("Ping", sys.Handle)
	if err != ErrDuplicateInteraction {
		t.Fatalf("esperava ErrDuplicateInteraction para duplicata sob mesmo pai, obtido: %v", err)
	}
}
