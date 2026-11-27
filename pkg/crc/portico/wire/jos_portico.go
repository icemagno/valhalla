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
	NewRegisterSyncPointObject = metamodel.NewRegisterSyncPoint
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

func extractWelcomePackFromObject(obj *JavaObject) *JavaObject {
	if obj == nil || obj.Class == nil {
		return nil
	}
	if obj.Class.Name == metamodel.ClassDescWelcomePack.Name {
		return obj
	}
	if obj.Class.Name == metamodel.ClassDescExtendedSuccessResponse.Name {
		if resultsObj, ok := obj.Get("results").(*JavaObject); ok && resultsObj != nil {
			switch entries := resultsObj.Get("entries").(type) {
			case map[string]any:
				if wp, ok := entries["result"].(*JavaObject); ok {
					return wp
				}
			case map[any]any:
				if wp, ok := entries["result"].(*JavaObject); ok {
					return wp
				}
			}
		}
		if res, ok := obj.Get("result").(*JavaObject); ok && res != nil {
			return res
		}
	}
	return nil
}

// ParseWelcomePack decodifica de forma canonica o WelcomePack (Externalizable) do Portico usando JOSDecoder
func ParseWelcomePack(payload []byte) (*WelcomePackInfo, error) {
	// Primary canonical path: full JOS stream (0xAC 0xED) decoded via JOSDecoder
	if len(payload) >= 2 && payload[0] == 0xAC && payload[1] == 0xED {
		decoded, err := DeserializePorticoControlMessage(payload)
		if err != nil {
			return nil, fmt.Errorf("jos wire: falha ao decodificar WelcomePack via JOSDecoder: %w", err)
		}
		wpObj := extractWelcomePackFromObject(decoded)
		if wpObj == nil || wpObj.Class == nil || wpObj.Class.Name != metamodel.ClassDescWelcomePack.Name {
			return nil, fmt.Errorf("jos wire: payload nao contem WelcomePack do Portico")
		}
		fedHandle, _ := wpObj.Get("federationHandle").(int32)
		federateHandle, _ := wpObj.Get("federateHandle").(int32)
		fedName, _ := wpObj.Get("federationName").(string)
		federateName, _ := wpObj.Get("federateName").(string)
		federateType, _ := wpObj.Get("federateType").(string)
		return &WelcomePackInfo{
			FederationHandle: fedHandle,
			FederateHandle:   federateHandle,
			FederationName:   fedName,
			FederateName:     federateName,
			FederateType:     federateType,
		}, nil
	}

	// Legacy raw WelcomePack fragment without JOS stream header (parse block data via JOSDecoder)
	idx := bytes.Index(payload, []byte("WelcomePack"))
	if idx == -1 {
		return nil, fmt.Errorf("jos wire: payload nao contem WelcomePack do Portico")
	}
	blockStart := idx + len("WelcomePack") + 8 + 1 + 2 + 1 + 1
	if blockStart >= len(payload) {
		return nil, fmt.Errorf("jos wire: payload truncado antes dos dados do WelcomePack")
	}
	dec := NewJOSDecoder(payload[blockStart:])
	data, err := dec.ReadBlockData()
	if err != nil {
		// Fall back to direct slice if fragment has shorter data than declared block length
		if blockStart+2 > len(payload) {
			return nil, fmt.Errorf("jos wire: bloco de dados truncado: %w", err)
		}
		data = payload[blockStart+2:]
	}
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
