package wire

import (
	"testing"
)

func TestHeader_Roundtrip(t *testing.T) {
	orig := Header{
		Flags:         FlagManualMarshal | FlagValhallaExtended,
		PayloadLength: 1024,
		FederationID:  7,
		CallType:      CallTypeControlRequest,
		MessageType:   MsgCreateFederation,
		RequestId:     42,
		SourceHandle:  100,
		TargetHandle:  200,
	}

	data, err := orig.MarshalBinary()
	if err != nil {
		t.Fatalf("falha ao serializar cabecalho: %v", err)
	}

	if len(data) != HeaderLength {
		t.Fatalf("tamanho incorreto: esperado %d, obtido %d", HeaderLength, len(data))
	}

	// Verificar byte a byte
	// Byte 0: Flags (0x08 | 0x80 = 0x88)
	if data[0] != 0x88 {
		t.Errorf("Flags byte incorreto: esperado 0x88, obtido 0x%02X", data[0])
	}

	// Bytes 1-3: PayloadLength 1024 (0x00, 0x04, 0x00)
	if data[1] != 0 || data[2] != 4 || data[3] != 0 {
		t.Errorf("PayloadLength bytes incorretos: %02X %02X %02X", data[1], data[2], data[3])
	}

	// Byte 4: FederationID (7 << 4 = 0x70) | CallType (2 = 0x02) = 0x72
	if data[4] != 0x72 {
		t.Errorf("Byte 4 nibbles incorretos: esperado 0x72, obtido 0x%02X", data[4])
	}

	// Byte 5: MsgCreateFederation (3)
	if data[5] != 3 {
		t.Errorf("Byte 5 MessageType incorreto: esperado 3, obtido %d", data[5])
	}

	var parsed Header
	if err := parsed.UnmarshalBinary(data); err != nil {
		t.Fatalf("falha ao decodificar cabecalho: %v", err)
	}

	if parsed != orig {
		t.Errorf("cabecalho decodificado difere do original:\nOriginal:    %+v\nDecodificado: %+v", orig, parsed)
	}
}

func TestHeader_Bitfield_Nibbles(t *testing.T) {
	// Testar limites de FederationID (0..15) e CallType (0..4)
	for fedID := uint8(0); fedID <= 15; fedID++ {
		for cType := CallType(0); cType <= 4; cType++ {
			h := Header{
				FederationID: fedID,
				CallType:     cType,
				MessageType:  MsgRtiProbe,
			}
			buf, err := h.MarshalBinary()
			if err != nil {
				t.Fatalf("falha ao serializar: %v", err)
			}

			var parsed Header
			if err := parsed.UnmarshalBinary(buf); err != nil {
				t.Fatalf("falha ao deserializar: %v", err)
			}

			if parsed.FederationID != fedID {
				t.Errorf("FederationID corrompido: esperado %d, obtido %d", fedID, parsed.FederationID)
			}
			if parsed.CallType != cType {
				t.Errorf("CallType corrompido: esperado %d, obtido %d", cType, parsed.CallType)
			}
		}
	}
}

func TestHeader_PayloadLength_Limits(t *testing.T) {
	h := Header{
		PayloadLength: 0x00FFFFFF, // Maximo suportado por 24 bits
		MessageType:   MsgUpdateAttributes,
	}

	buf, err := h.MarshalBinary()
	if err != nil {
		t.Fatalf("falha ao serializar limite maximo de 24 bits: %v", err)
	}

	var parsed Header
	if err := parsed.UnmarshalBinary(buf); err != nil {
		t.Fatalf("falha ao decodificar: %v", err)
	}
	if parsed.PayloadLength != 0x00FFFFFF {
		t.Errorf("esperado %d, obtido %d", 0x00FFFFFF, parsed.PayloadLength)
	}

	// Testar overflow (> 24 bits)
	hOver := Header{
		PayloadLength: 0x01000000, // 16 MB + 1 byte
	}
	err = hOver.MarshalTo(buf)
	if err != ErrPayloadTooLarge {
		t.Errorf("esperava ErrPayloadTooLarge, obtido %v", err)
	}
}

func TestHeader_BufferTooShort(t *testing.T) {
	buf := make([]byte, 11) // menor que 12 bytes
	var h Header
	if err := h.UnmarshalBinary(buf); err != ErrBufferTooShort {
		t.Errorf("esperava ErrBufferTooShort, obtido %v", err)
	}

	if err := h.MarshalTo(buf); err != ErrBufferTooShort {
		t.Errorf("esperava ErrBufferTooShort, obtido %v", err)
	}
}

func BenchmarkHeader_MarshalTo(b *testing.B) {
	h := NewDataMessage(MsgUpdateAttributes, 1, 100, 200, 512)
	buf := make([]byte, HeaderLength)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := h.MarshalTo(buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHeader_UnmarshalBinary(b *testing.B) {
	h := NewDataMessage(MsgUpdateAttributes, 1, 100, 200, 512)
	buf, _ := h.MarshalBinary()

	var parsed Header
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := parsed.UnmarshalBinary(buf); err != nil {
			b.Fatal(err)
		}
	}
}
