package wire

import (
	"bytes"
	"testing"
)

func TestHeader_Roundtrip(t *testing.T) {
	orig := NewHeader(MsgJoinFederation, 101, -1, 512)
	orig.Flags = FlagValhallaExt | FlagTimestampOrder

	data, err := orig.MarshalBinary()
	if err != nil {
		t.Fatalf("falha ao serializar cabecalho: %v", err)
	}

	if len(data) != HeaderSize {
		t.Fatalf("tamanho incorreto: esperado %d, obtido %d", HeaderSize, len(data))
	}

	// Verificar se os 4 primeiros bytes sao "PORT" (0x504F5254)
	expectedMagic := []byte{'P', 'O', 'R', 'T'}
	if !bytes.Equal(data[0:4], expectedMagic) {
		t.Errorf("magic incorreto nos bytes brutos: %v", data[0:4])
	}

	var parsed Header
	if err := parsed.UnmarshalBinary(data); err != nil {
		t.Fatalf("falha ao decodificar cabecalho: %v", err)
	}

	if parsed != orig {
		t.Errorf("cabecalho decodificado difere do original:\nOriginal: %+v\nDecodificado: %+v", orig, parsed)
	}
}

func TestHeader_InvalidMagic(t *testing.T) {
	buf := make([]byte, HeaderSize)
	buf[0] = 'X' // quebra o Magic "PORT"

	var h Header
	err := h.UnmarshalBinary(buf)
	if err == nil {
		t.Fatal("esperava erro de magic invalido, mas passou com sucesso")
	}
}

func TestHeader_BufferTooShort(t *testing.T) {
	buf := make([]byte, 10) // menor que HeaderSize (24)

	var h Header
	err := h.UnmarshalBinary(buf)
	if err == nil {
		t.Fatal("esperava erro de buffer curto, mas passou com sucesso")
	}
}

func BenchmarkHeader_MarshalTo(b *testing.B) {
	h := NewHeader(MsgUpdateAttributes, 202, 0, 1024)
	h.Flags = FlagValhallaExt
	buf := make([]byte, HeaderSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := h.MarshalTo(buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHeader_UnmarshalBinary(b *testing.B) {
	h := NewHeader(MsgUpdateAttributes, 202, 0, 1024)
	data, _ := h.MarshalBinary()

	var parsed Header
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := parsed.UnmarshalBinary(data); err != nil {
			b.Fatal(err)
		}
	}
}
