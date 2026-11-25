package core

import (
	"math/bits"

	"github.com/icemagno/valhalla/pkg/common"
)

// Bitset implementa um conjunto esparso e dinâmico de bits de alto desempenho,
// otimizado para associação e filtragem de interesses de federados em simulações massivas.
// Baseado em blocos uint64 contíguos com expansão sob demanda e busca vetorizada via bits.TrailingZeros64.
type Bitset struct {
	words []uint64
}

// NewBitset inicializa um bitset vazio com capacidade inicial
func NewBitset(initialWords ...int) *Bitset {
	capWords := 2 // 128 bits padrão (cobre até 128 federados em 16 bytes)
	if len(initialWords) > 0 && initialWords[0] > 0 {
		capWords = initialWords[0]
	}
	return &Bitset{
		words: make([]uint64, 0, capWords),
	}
}

// ensureCapacity garante que o slice de words comporta o índice solicitado
func (b *Bitset) ensureCapacity(wordIdx int) {
	if wordIdx >= len(b.words) {
		newLen := wordIdx + 1
		if newLen <= cap(b.words) {
			b.words = b.words[:newLen]
		} else {
			expanded := make([]uint64, newLen, (newLen*3)/2+2)
			copy(expanded, b.words)
			b.words = expanded
		}
	}
}

// Set marca o bit correspondente ao ID (1-based ou 0-based)
func (b *Bitset) Set(id uint32) {
	wordIdx := int(id >> 6) // id / 64
	bitIdx := id & 63       // id % 64
	b.ensureCapacity(wordIdx)
	b.words[wordIdx] |= (1 << bitIdx)
}

// Clear desmarca o bit correspondente ao ID
func (b *Bitset) Clear(id uint32) {
	wordIdx := int(id >> 6)
	if wordIdx < len(b.words) {
		bitIdx := id & 63
		b.words[wordIdx] &^= (1 << bitIdx)
	}
}

// Contains verifica se o bit correspondente ao ID está marcado
func (b *Bitset) Contains(id uint32) bool {
	wordIdx := int(id >> 6)
	if wordIdx >= len(b.words) {
		return false
	}
	bitIdx := id & 63
	return (b.words[wordIdx] & (1 << bitIdx)) != 0
}

// IsEmpty retorna true se nenhum bit estiver marcado
func (b *Bitset) IsEmpty() bool {
	for _, w := range b.words {
		if w != 0 {
			return false
		}
	}
	return true
}

// Count retorna o número total de bits ativos (popcount)
func (b *Bitset) Count() int {
	total := 0
	for _, w := range b.words {
		total += bits.OnesCount64(w)
	}
	return total
}

// Union realiza a união in-place b = b OR other
func (b *Bitset) Union(other *Bitset) {
	if other == nil || len(other.words) == 0 {
		return
	}
	if len(other.words) > len(b.words) {
		b.ensureCapacity(len(other.words) - 1)
	}
	for i := 0; i < len(other.words); i++ {
		b.words[i] |= other.words[i]
	}
}

// Intersect realiza a interseção in-place b = b AND other
func (b *Bitset) Intersect(other *Bitset) {
	if other == nil {
		b.Reset()
		return
	}
	minLen := len(b.words)
	if len(other.words) < minLen {
		minLen = len(other.words)
	}
	for i := 0; i < minLen; i++ {
		b.words[i] &= other.words[i]
	}
	// Bits além do tamanho de other tornam-se 0
	for i := minLen; i < len(b.words); i++ {
		b.words[i] = 0
	}
}

// Clone cria uma cópia profunda do Bitset
func (b *Bitset) Clone() *Bitset {
	clone := &Bitset{
		words: make([]uint64, len(b.words)),
	}
	copy(clone.words, b.words)
	return clone
}

// Reset limpa todos os bits sem desalocar o slice subjacente
func (b *Bitset) Reset() {
	for i := range b.words {
		b.words[i] = 0
	}
}

// ToFederateHandles extrai todos os IDs ativos como um slice de FederateHandle.
// Utiliza bits.TrailingZeros64 para saltar diretamente para os bits 1 em O(k),
// onde k é o número de bits ativos, sem iterar por bits zero.
func (b *Bitset) ToFederateHandles() []common.FederateHandle {
	count := b.Count()
	if count == 0 {
		return nil
	}

	handles := make([]common.FederateHandle, 0, count)
	for wordIdx, w := range b.words {
		for w != 0 {
			tz := bits.TrailingZeros64(w)
			id := uint32(wordIdx<<6) + uint32(tz)
			handles = append(handles, common.FederateHandle(id))
			w &^= (1 << tz) // Limpa o bit processado mais baixo
		}
	}
	return handles
}
