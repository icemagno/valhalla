package core

import (
	"reflect"
	"sort"
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
)

func TestBitset_BasicOperations(t *testing.T) {
	b := NewBitset()

	if !b.IsEmpty() {
		t.Fatalf("esperava bitset vazio")
	}

	b.Set(1)
	b.Set(64)
	b.Set(100)
	b.Set(1000)

	if b.IsEmpty() {
		t.Fatalf("bitset não deveria estar vazio")
	}

	if b.Count() != 4 {
		t.Fatalf("esperava count=4, obteve %d", b.Count())
	}

	if !b.Contains(1) || !b.Contains(64) || !b.Contains(100) || !b.Contains(1000) {
		t.Fatalf("falha ao verificar bits marcados")
	}

	if b.Contains(0) || b.Contains(2) || b.Contains(63) || b.Contains(65) || b.Contains(999) {
		t.Fatalf("detectou bit não marcado incorretamente")
	}

	b.Clear(64)
	if b.Contains(64) {
		t.Fatalf("bit 64 ainda está presente após Clear")
	}
	if b.Count() != 3 {
		t.Fatalf("esperava count=3 após Clear, obteve %d", b.Count())
	}

	handles := b.ToFederateHandles()
	expected := []common.FederateHandle{1, 100, 1000}
	sort.Slice(handles, func(i, j int) bool { return handles[i] < handles[j] })
	if !reflect.DeepEqual(handles, expected) {
		t.Fatalf("ToFederateHandles inconsistente: esperado %v, obteve %v", expected, handles)
	}
}

func TestBitset_UnionAndIntersect(t *testing.T) {
	b1 := NewBitset()
	b1.Set(1)
	b1.Set(2)
	b1.Set(150)

	b2 := NewBitset()
	b2.Set(2)
	b2.Set(3)
	b2.Set(150)
	b2.Set(300)

	// Clone para testar interseção
	bInter := b1.Clone()
	bInter.Intersect(b2)

	expectedInter := []common.FederateHandle{2, 150}
	handlesInter := bInter.ToFederateHandles()
	sort.Slice(handlesInter, func(i, j int) bool { return handlesInter[i] < handlesInter[j] })
	if !reflect.DeepEqual(handlesInter, expectedInter) {
		t.Fatalf("Intersect incorreto: esperado %v, obteve %v", expectedInter, handlesInter)
	}

	// Testar União
	b1.Union(b2)
	expectedUnion := []common.FederateHandle{1, 2, 3, 150, 300}
	handlesUnion := b1.ToFederateHandles()
	sort.Slice(handlesUnion, func(i, j int) bool { return handlesUnion[i] < handlesUnion[j] })
	if !reflect.DeepEqual(handlesUnion, expectedUnion) {
		t.Fatalf("Union incorreto: esperado %v, obteve %v", expectedUnion, handlesUnion)
	}
}

func BenchmarkBitset_Union(b *testing.B) {
	b1 := NewBitset()
	b2 := NewBitset()

	for i := uint32(1); i <= 1000; i += 2 {
		b1.Set(i)
	}
	for i := uint32(2); i <= 1000; i += 2 {
		b2.Set(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		temp := b1.Clone()
		temp.Union(b2)
	}
}

func BenchmarkBitset_ToFederateHandles(b *testing.B) {
	bs := NewBitset()
	for i := uint32(1); i <= 500; i += 5 {
		bs.Set(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bs.ToFederateHandles()
	}
}
