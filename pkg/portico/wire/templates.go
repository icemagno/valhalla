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
