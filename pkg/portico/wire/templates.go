package wire

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
)

//go:embed testdata/welcome_pack_base.bin
var welcomePackBase []byte

//go:embed testdata/response_message_base.bin
var responseMessageBase []byte

// BuildSuccessResponsePayload retorna o payload canônico oficial de ResponseMessage (83 bytes)
func BuildSuccessResponsePayload() []byte {
	buf := make([]byte, len(responseMessageBase))
	copy(buf, responseMessageBase)
	return buf
}

// BuildWelcomePackPayload gera um WelcomePack válido para o Portico com handles atribuídos
func BuildWelcomePackPayload(fedHandle, federateHandle int32) ([]byte, error) {
	if len(welcomePackBase) < 300 {
		return nil, fmt.Errorf("jos wire: welcome_pack_base corrompido ou muito curto")
	}

	buf := make([]byte, len(welcomePackBase))
	copy(buf, welcomePackBase)

	idxWP := bytes.Index(buf, []byte("WelcomePack"))
	if idxWP == -1 {
		return nil, fmt.Errorf("jos wire: WelcomePack nao encontrado no template base")
	}

	// Pular nome da classe, SUID (8b), Flags (1b), Fields (2b), TC_ENDBLOCKDATA (1b), TC_NULL (1b), TC_BLOCKDATA (1b), BlockLen (1b)
	// Total apos "WelcomePack": 8 + 1 + 2 + 1 + 1 + 1 + 1 = 15 bytes
	offset := idxWP + len("WelcomePack") + 15
	if offset+8 > len(buf) {
		return nil, fmt.Errorf("jos wire: offset do WelcomePack excede tamanho do buffer")
	}

	binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(fedHandle))
	binary.BigEndian.PutUint32(buf[offset+4:offset+8], uint32(federateHandle))

	return buf, nil
}

// BuildCreateFederationResponsePayload gera uma ExtendedSuccessResponse contendo o handle da federação como java.lang.Integer
func BuildCreateFederationResponsePayload(fedHandle int32) []byte {
	idxResult := bytes.Index(welcomePackBase, []byte("result"))
	if idxResult == -1 {
		return BuildSuccessResponsePayload()
	}

	prefixLen := idxResult + len("result")
	var buf bytes.Buffer
	buf.Write(welcomePackBase[:prefixLen])

	intPayload := []byte{
		0x73,             // TC_OBJECT
		0x72, 0x00, 0x11, // TC_CLASSDESC "java.lang.Integer"
		'j', 'a', 'v', 'a', '.', 'l', 'a', 'n', 'g', '.', 'I', 'n', 't', 'e', 'g', 'e', 'r',
		0x12, 0xE2, 0xA0, 0xA4, 0xF7, 0x81, 0x87, 0x39, // serialVersionUID
		0x02,       // SC_SERIALIZABLE
		0x00, 0x01, // 1 field
		'I', 0x00, 0x05, 'v', 'a', 'l', 'u', 'e',
		0x78,             // TC_ENDBLOCKDATA
		0x72, 0x00, 0x10, // TC_CLASSDESC "java.lang.Number"
		'j', 'a', 'v', 'a', '.', 'l', 'a', 'n', 'g', '.', 'N', 'u', 'm', 'b', 'e', 'r',
		0x86, 0xAC, 0x95, 0x1D, 0x0B, 0x94, 0xE0, 0x8B, // serialVersionUID
		0x02,       // SC_SERIALIZABLE
		0x00, 0x00, // 0 fields
		0x78, // TC_ENDBLOCKDATA
		0x70, // TC_NULL
		byte(fedHandle >> 24), byte(fedHandle >> 16), byte(fedHandle >> 8), byte(fedHandle),
		0x78, // TC_ENDBLOCKDATA
		0x78, // TC_ENDBLOCKDATA
	}

	buf.Write(intPayload)
	return buf.Bytes()
}
