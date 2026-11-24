package wire

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Definições canônicas das classes Portico registradas no ecossistema JOS
var (
	// ClassDescPorticoMessage é a classe base de todas as requisições Portico
	ClassDescPorticoMessage = &ClassDesc{
		Name:             "org.portico.utils.messaging.PorticoMessage",
		SerialVersionUID: 98121116105109,
		Flags:            SC_SERIALIZABLE | SC_WRITE_METHOD,
		Fields: []FieldDesc{
			{Name: "immediate", Type: TypeBoolean},
			{Name: "isFromRti", Type: TypeBoolean},
			{Name: "sourceFederate", Type: TypeInt},
			{Name: "targetFederate", Type: TypeInt},
			{Name: "targetFederation", Type: TypeInt},
			{Name: "timestamp", Type: TypeDouble},
		},
		CustomWriter: func(enc *JOSEncoder, obj *JavaObject) {
			// Escreve os 22 bytes do método writeExternal/writeObject do PorticoMessage
			var block [22]byte
			// 0: isFromRti (boolean)
			if isRti, _ := obj.Fields["isFromRti"].(bool); isRti {
				block[0] = 0x01
			} else {
				block[0] = 0x00
			}
			// 1-4: sourceFederate (int32, default -1)
			src, ok := obj.Fields["sourceFederate"].(int32)
			if !ok {
				src = -1
			}
			binary.BigEndian.PutUint32(block[1:5], uint32(src))

			// 5-8: targetFederate (int32, default 0 para RTI)
			tgt, ok := obj.Fields["targetFederate"].(int32)
			if !ok {
				tgt = 0
			}
			binary.BigEndian.PutUint32(block[5:9], uint32(tgt))

			// 9-12: targetFederation (int32, default -1)
			fed, ok := obj.Fields["targetFederation"].(int32)
			if !ok {
				fed = -1
			}
			binary.BigEndian.PutUint32(block[9:13], uint32(fed))

			// 13-20: timestamp (double, default -1.0)
			ts, ok := obj.Fields["timestamp"].(float64)
			if !ok {
				ts = -1.0
			}
			binary.BigEndian.PutUint64(block[13:21], math.Float64bits(ts))

			// 21: immediate (boolean)
			if imm, _ := obj.Fields["immediate"].(bool); imm {
				block[21] = 0x01
			} else {
				block[21] = 0x00
			}

			enc.WriteBlockData(block[:])
			enc.WriteByte(TC_ENDBLOCKDATA)
		},
		CustomReader: func(dec *JOSDecoder, obj *JavaObject) error {
			block, err := dec.ReadBlockData()
			if err != nil {
				return err
			}
			if len(block) >= 22 {
				obj.Set("isFromRti", block[0] != 0)
				obj.Set("sourceFederate", int32(binary.BigEndian.Uint32(block[1:5])))
				obj.Set("targetFederate", int32(binary.BigEndian.Uint32(block[5:9])))
				obj.Set("targetFederation", int32(binary.BigEndian.Uint32(block[9:13])))
				obj.Set("timestamp", math.Float64frombits(binary.BigEndian.Uint64(block[13:21])))
				obj.Set("immediate", block[21] != 0)
			}
			endMarker, err := dec.ReadByte()
			if err != nil {
				return err
			}
			if endMarker != TC_ENDBLOCKDATA {
				return fmt.Errorf("esperado TC_ENDBLOCKDATA (0x78), recebido 0x%02X", endMarker)
			}
			return nil
		},
	}

	// ClassDescArrayList representa java.util.ArrayList
	ClassDescArrayList = &ClassDesc{
		Name:             "java.util.ArrayList",
		SerialVersionUID: 0x7881D21D99C7619D,
		Flags:            SC_SERIALIZABLE | SC_WRITE_METHOD,
		Fields: []FieldDesc{
			{Name: "size", Type: TypeInt},
		},
		CustomWriter: func(enc *JOSEncoder, obj *JavaObject) {
			enc.WriteInt32(0) // size = 0
			// ArrayList writeObject escreve capacidade no bloco:
			enc.WriteBlockData([]byte{0x00, 0x00, 0x00, 0x00})
			enc.WriteByte(TC_ENDBLOCKDATA)
		},
		CustomReader: func(dec *JOSDecoder, obj *JavaObject) error {
			size, err := dec.ReadInt32()
			if err != nil {
				return err
			}
			obj.Set("size", size)
			if _, err := dec.ReadBlockData(); err != nil {
				return err
			}
			endMarker, err := dec.ReadByte()
			if err != nil {
				return err
			}
			if endMarker != TC_ENDBLOCKDATA {
				return fmt.Errorf("esperado TC_ENDBLOCKDATA (0x78), recebido 0x%02X", endMarker)
			}
			return nil
		},
	}

	// ClassDescJoinFederation representa a requisição oficial de ingresso em federação
	ClassDescJoinFederation = &ClassDesc{
		Name:             "org.portico2.common.services.federation.msg.JoinFederation",
		SerialVersionUID: 98121116105109,
		Flags:            SC_SERIALIZABLE,
		Fields: []FieldDesc{
			{Name: "federateName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "federateType", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "federationName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "parsedJoinObjectModels", Type: TypeObject, ClassName: "Ljava/util/List;"},
			{Name: "rawJoinObjectModels", Type: TypeObject, ClassName: "Ljava/util/List;"},
		},
		Super: ClassDescPorticoMessage,
	}

	// ClassDescCreateFederation representa a requisição oficial de criação de federação
	ClassDescCreateFederation = &ClassDesc{
		Name:             "org.portico2.common.services.federation.msg.CreateFederation",
		SerialVersionUID: 98121116105109,
		Flags:            SC_SERIALIZABLE,
		Fields: []FieldDesc{
			{Name: "federationName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		},
		Super: ClassDescPorticoMessage,
	}

	// ClassDescJavaEnum é a superclasse base de enums Java
	ClassDescJavaEnum = &ClassDesc{
		Name:             "java.lang.Enum",
		SerialVersionUID: 0,
		Flags:            SC_SERIALIZABLE | SC_ENUM,
		Fields:           nil,
	}

	// ClassDescJResignAction é o enum oficial de ação de desligamento do Portico
	ClassDescJResignAction = &ClassDesc{
		Name:             "org.portico.lrc.compat.JResignAction",
		SerialVersionUID: 0,
		Flags:            SC_SERIALIZABLE | SC_ENUM,
		Fields:           nil,
		Super:            ClassDescJavaEnum,
	}

	// ClassDescResignFederation representa a saída voluntária de um federado
	ClassDescResignFederation = &ClassDesc{
		Name:             "org.portico2.common.services.federation.msg.ResignFederation",
		SerialVersionUID: 98121116105109,
		Flags:            SC_SERIALIZABLE,
		Fields: []FieldDesc{
			{Name: "federateName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "federateType", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "federationName", Type: TypeObject, ClassName: "Ljava/lang/String;"},
			{Name: "resignAction", Type: TypeObject, ClassName: "Lorg/portico/lrc/compat/JResignAction;"},
		},
		Super: ClassDescPorticoMessage,
	}

	// ClassDescSyncPointAchieved sinaliza que um ponto de sincronização foi alcançado
	ClassDescSyncPointAchieved = &ClassDesc{
		Name:             "org.portico2.common.services.sync.msg.SyncPointAchieved",
		SerialVersionUID: 98121116105109,
		Flags:            SC_SERIALIZABLE,
		Fields: []FieldDesc{
			{Name: "label", Type: TypeObject, ClassName: "Ljava/lang/String;"},
		},
		Super: ClassDescPorticoMessage,
	}

	// ClassDescRtiProbe representa a sonda de presença do executivo RTI
	ClassDescRtiProbe = &ClassDesc{
		Name:             "org.portico2.common.services.federation.msg.RtiProbe",
		SerialVersionUID: 98121116105109,
		Flags:            SC_SERIALIZABLE,
		Fields:           nil, // Sem campos próprios
		Super:            ClassDescPorticoMessage,
	}
)

// Registro global de classes Java conhecidas pelo Portico
var knownClassRegistry = make(map[string]*ClassDesc)

// RegisterClassDesc registra metadados de uma classe no catálogo global
func RegisterClassDesc(cls *ClassDesc) {
	if cls != nil {
		knownClassRegistry[cls.Name] = cls
	}
}

// LookupKnownClassDesc busca metadados e hooks registrados para um nome de classe
func LookupKnownClassDesc(name string) (*ClassDesc, bool) {
	cls, ok := knownClassRegistry[name]
	return cls, ok
}

func init() {
	RegisterClassDesc(ClassDescPorticoMessage)
	RegisterClassDesc(ClassDescArrayList)
	RegisterClassDesc(ClassDescJoinFederation)
	RegisterClassDesc(ClassDescCreateFederation)
	RegisterClassDesc(ClassDescResignFederation)
	RegisterClassDesc(ClassDescRtiProbe)
	RegisterClassDesc(ClassDescSyncPointAchieved)
	RegisterClassDesc(ClassDescJResignAction)
	RegisterClassDesc(ClassDescJavaEnum)
}

// EmptyArrayList cria uma instância genérica de ArrayList vazio
func EmptyArrayList() *JavaObject {
	return NewJavaObject(ClassDescArrayList).Set("size", int32(0))
}

// NewJoinFederationObject constrói uma instância de JoinFederation pronta para serialização
func NewJoinFederationObject(federationName, federateName, federateType string) *JavaObject {
	obj := NewJavaObject(ClassDescJoinFederation)
	// Campos da superclasse PorticoMessage
	obj.Set("immediate", true)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(0))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))

	// Campos de JoinFederation
	obj.Set("federateName", federateName)
	obj.Set("federateType", federateType)
	obj.Set("federationName", federationName)
	obj.Set("parsedJoinObjectModels", EmptyArrayList())
	obj.Set("rawJoinObjectModels", EmptyArrayList())

	return obj
}

// NewCreateFederationObject constrói uma instância de CreateFederation pronta para serialização
func NewCreateFederationObject(federationName string) *JavaObject {
	obj := NewJavaObject(ClassDescCreateFederation)
	obj.Set("immediate", true)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(0))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("federationName", federationName)
	return obj
}

// NewResignFederationObject constrói uma instância canônica de ResignFederation
func NewResignFederationObject(federationName, federateName, federateType, action string) *JavaObject {
	obj := NewJavaObject(ClassDescResignFederation)
	obj.Set("immediate", true)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(0))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("federateName", federateName)
	obj.Set("federateType", federateType)
	obj.Set("federationName", federationName)
	if action == "" {
		action = "NO_ACTION"
	}
	obj.Set("resignAction", NewJavaEnum(ClassDescJResignAction, action))
	return obj
}

// NewSyncPointAchievedObject constrói uma instância de SyncPointAchieved
func NewSyncPointAchievedObject(label string, federateHandle int32) *JavaObject {
	obj := NewJavaObject(ClassDescSyncPointAchieved)
	obj.Set("immediate", true)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", federateHandle)
	obj.Set("targetFederate", int32(0))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	obj.Set("label", label)
	return obj
}

// NewRtiProbeObject constrói uma instância de RtiProbe
func NewRtiProbeObject() *JavaObject {
	obj := NewJavaObject(ClassDescRtiProbe)
	obj.Set("immediate", false)
	obj.Set("isFromRti", false)
	obj.Set("sourceFederate", int32(-1))
	obj.Set("targetFederate", int32(0))
	obj.Set("targetFederation", int32(-1))
	obj.Set("timestamp", float64(-1.0))
	return obj
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
