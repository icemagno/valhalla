package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

// Definições canônicas das classes Portico delegadas ao metamodelo
var (
	ClassDescPorticoMessage          = metamodel.ClassDescPorticoMessage
	ClassDescArrayList               = metamodel.ClassDescArrayList
	ClassDescJoinFederation          = metamodel.ClassDescJoinFederation
	ClassDescCreateFederation        = metamodel.ClassDescCreateFederation
	ClassDescJavaEnum                = metamodel.ClassDescJavaEnum
	ClassDescJResignAction           = metamodel.ClassDescJResignAction
	ClassDescResignFederation        = metamodel.ClassDescResignFederation
	ClassDescSyncPointAchieved       = metamodel.ClassDescSyncPointAchieved
	ClassDescRtiProbe                = metamodel.ClassDescRtiProbe
	ClassDescWelcomePack             = metamodel.ClassDescWelcomePack
	ClassDescExtendedSuccessResponse = metamodel.ClassDescExtendedSuccessResponse
	ClassDescResponseMessage         = metamodel.ClassDescResponseMessage
	ClassDescHashMap                 = metamodel.ClassDescHashMap
	ClassDescInteger                 = metamodel.ClassDescInteger
	ClassDescLong                    = metamodel.ClassDescLong
	ClassDescBoolean                 = metamodel.ClassDescBoolean
	ClassDescNumber                  = metamodel.ClassDescNumber
	ClassDescObject                  = metamodel.ClassDescObject
)

// LookupKnownClassDesc busca metadados no catálogo global do metamodelo
func LookupKnownClassDesc(name string) (*ClassDesc, bool) {
	return metamodel.LookupClassDesc(name)
}

// Funções de instanciação compatíveis
var (
	EmptyArrayList             = metamodel.NewArrayList
	NewJoinFederationObject    = metamodel.NewJoinFederation
	NewCreateFederationObject  = metamodel.NewCreateFederation
	NewResignFederationObject  = metamodel.NewResignFederation
	NewSyncPointAchievedObject = metamodel.NewSyncPointAchieved
	NewRtiProbeObject          = metamodel.NewRtiProbe
)

// WelcomePackInfo consolida as informacoes essenciais retornadas pelo Portico apos o ingresso
type WelcomePackInfo struct {
	FederationHandle int32
	FederateHandle   int32
	FederationName   string
	FederateName     string
	FederateType     string
}

// ParseWelcomePack decodifica de forma canonica o WelcomePack (Externalizable) do Portico
func ParseWelcomePack(payload []byte) (*WelcomePackInfo, error) {
	idx := bytes.Index(payload, []byte("WelcomePack"))
	if idx == -1 {
		return nil, fmt.Errorf("jos wire: payload nao contem WelcomePack do Portico")
	}

	// Estrutura do WelcomePack no JOS:
	// - ClassName: "WelcomePack"
	// - SUID (8 bytes)
	// - Flags (1 byte: 0x0C = SC_EXTERNALIZABLE | SC_BLOCK_DATA)
	// - NumFields (2 bytes: 0x0000)
	// - TC_ENDBLOCKDATA (1 byte: 0x78)
	// - SuperClass TC_NULL (1 byte: 0x70)
	// - TC_BLOCKDATA (1 byte: 0x77)
	// - BlockLength (1 byte)
	headerOffset := idx + len("WelcomePack") + 8 + 1 + 2 + 1 + 1 + 1 + 1
	if headerOffset > len(payload) {
		return nil, fmt.Errorf("jos wire: payload truncado antes dos dados do WelcomePack")
	}

	data := payload[headerOffset:]
	if len(data) < 8 {
		return nil, fmt.Errorf("jos wire: bloco de dados insuficiente para handles")
	}

	info := &WelcomePackInfo{
		FederationHandle: int32(binary.BigEndian.Uint32(data[0:4])),
		FederateHandle:   int32(binary.BigEndian.Uint32(data[4:8])),
	}

	offset := 8
	readUTF := func() (string, error) {
		if offset+2 > len(data) {
			return "", fmt.Errorf("dados insuficientes para UTF length")
		}
		sLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
		if offset+sLen > len(data) {
			return "", fmt.Errorf("dados insuficientes para string UTF")
		}
		str := string(data[offset : offset+sLen])
		offset += sLen
		return str, nil
	}

	var err error
	if info.FederationName, err = readUTF(); err != nil {
		return nil, fmt.Errorf("falha ao ler federationName: %w", err)
	}
	if info.FederateName, err = readUTF(); err != nil {
		return nil, fmt.Errorf("falha ao ler federateName: %w", err)
	}
	if info.FederateType, err = readUTF(); err != nil {
		return nil, fmt.Errorf("falha ao ler federateType: %w", err)
	}

	return info, nil
}

// SerializePorticoControlMessage serializa genericamente qualquer objeto de controle Portico
// inserindo o prefixo de 1 byte de bloco com false (ManualMarshal = false) exigido por MessageHelpers.inflate2
func SerializePorticoControlMessage(msg *JavaObject) ([]byte, error) {
	enc := NewJOSEncoder()
	enc.WriteStreamHeader()

	// Inserir bloco de false exigido por MessageHelpers.deflate2 do Portico:
	enc.WriteBlockData([]byte{0x00})

	// Serializar o objeto genérico
	if err := enc.WriteObject(msg); err != nil {
		return nil, err
	}

	return enc.Bytes(), nil
}
