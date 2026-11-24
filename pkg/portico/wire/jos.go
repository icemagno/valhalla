package wire

import (
	"bytes"
	"encoding/binary"
	"math"
)

// Constantes do Java Object Serialization Stream Protocol (JOS - RFC/JDK Spec)
const (
	JOSMagic   uint16 = 0xACED // STREAM_MAGIC
	JOSVersion uint16 = 0x0005 // STREAM_VERSION

	// Marcadores de Tipo (Type Tokens)
	TC_NULL         byte = 0x70
	TC_REFERENCE    byte = 0x71
	TC_CLASSDESC    byte = 0x72
	TC_OBJECT       byte = 0x73
	TC_STRING       byte = 0x74
	TC_ARRAY        byte = 0x75
	TC_CLASS        byte = 0x76
	TC_BLOCKDATA    byte = 0x77
	TC_ENDBLOCKDATA byte = 0x78
	TC_RESET        byte = 0x79

	// Flags de Descritor de Classe
	SC_WRITE_METHOD byte = 0x01
	SC_SERIALIZABLE byte = 0x02

	// TypeCodes de Campos
	TypeBoolean byte = 'Z' // 0x5A
	TypeInt     byte = 'I' // 0x49
	TypeDouble  byte = 'D' // 0x44
	TypeObject  byte = 'L' // 0x4C

	// Identificador canônico de versão das classes Portico (serialVersionUID)
	PorticoSerialVersionUID uint64 = 98121116105109 // 0x0000593D9A453995
	ArrayListSerialVersionUID uint64 = 0x7881D21D99C7619D
)

// Nomes completos de classe segundo o pacote org.portico2 e org.portico
const (
	ClassPorticoMessage = "org.portico.utils.messaging.PorticoMessage"
	ClassRtiProbe       = "org.portico2.common.services.federation.msg.RtiProbe"
	ClassJoinFederation = "org.portico2.common.services.federation.msg.JoinFederation"
	ClassArrayList      = "java.util.ArrayList"
	TypeStringDesc      = "Ljava/lang/String;"
	TypeListDesc        = "Ljava/util/List;"
)

// JOSWriter é um construtor de streams Java Object Serialization binário em Go
type JOSWriter struct {
	buf bytes.Buffer
}

// NewJOSWriter inicializa o buffer gravando o cabeçalho obrigatório do Java (0xACED 0x0005)
// seguido pelo boolean falso do Portico (indicando que manual marshal não foi utilizado)
func NewJOSWriter() *JOSWriter {
	w := &JOSWriter{}
	binary.Write(&w.buf, binary.BigEndian, JOSMagic)
	binary.Write(&w.buf, binary.BigEndian, JOSVersion)
	// Portico MessageHelpers.deflate2: out.writeBoolean(false) -> bloco de 1 byte
	w.WriteBlockData([]byte{0x00})
	return w
}

func (w *JOSWriter) WriteByte(b byte) {
	w.buf.WriteByte(b)
}

func (w *JOSWriter) WriteUint16(v uint16) {
	binary.Write(&w.buf, binary.BigEndian, v)
}

func (w *JOSWriter) WriteInt32(v int32) {
	binary.Write(&w.buf, binary.BigEndian, v)
}

func (w *JOSWriter) WriteUint64(v uint64) {
	binary.Write(&w.buf, binary.BigEndian, v)
}

func (w *JOSWriter) WriteDouble(v float64) {
	binary.Write(&w.buf, binary.BigEndian, math.Float64bits(v))
}

func (w *JOSWriter) WriteUTF(s string) {
	w.WriteUint16(uint16(len(s)))
	w.buf.WriteString(s)
}

func (w *JOSWriter) WriteBlockData(data []byte) {
	w.WriteByte(TC_BLOCKDATA)
	w.WriteByte(byte(len(data)))
	w.buf.Write(data)
}

func (w *JOSWriter) Bytes() []byte {
	return w.buf.Bytes()
}

// writePorticoMessageClassDesc escreve o descritor de classe completo da classe pai PorticoMessage
func (w *JOSWriter) writePorticoMessageClassDesc() {
	w.WriteByte(TC_CLASSDESC)
	w.WriteUTF(ClassPorticoMessage)
	w.WriteUint64(PorticoSerialVersionUID)
	w.WriteByte(SC_SERIALIZABLE | SC_WRITE_METHOD) // 0x03
	w.WriteUint16(6)                               // 6 campos primitivos ordenados

	// 1. immediate (Z)
	w.WriteByte(TypeBoolean)
	w.WriteUTF("immediate")

	// 2. isFromRti (Z)
	w.WriteByte(TypeBoolean)
	w.WriteUTF("isFromRti")

	// 3. sourceFederate (I)
	w.WriteByte(TypeInt)
	w.WriteUTF("sourceFederate")

	// 4. targetFederate (I)
	w.WriteByte(TypeInt)
	w.WriteUTF("targetFederate")

	// 5. targetFederation (I)
	w.WriteByte(TypeInt)
	w.WriteUTF("targetFederation")

	// 6. timestamp (D)
	w.WriteByte(TypeDouble)
	w.WriteUTF("timestamp")

	w.WriteByte(TC_ENDBLOCKDATA)
	w.WriteByte(TC_NULL) // Sem superclasse Java além de Object
}

// writePorticoMessageValues escreve os valores do método writeExternal/writeObject do PorticoMessage
func (w *JOSWriter) writePorticoMessageValues(immediate bool, targetFederate int32) {
	var block [22]byte
	// 0: isFromRti (false)
	block[0] = 0x00
	// 1-4: sourceFederate (-1 = 0xFFFFFFFF)
	binary.BigEndian.PutUint32(block[1:5], 0xFFFFFFFF)
	// 5-8: targetFederate
	binary.BigEndian.PutUint32(block[5:9], uint32(targetFederate))
	// 9-12: targetFederation (-1 = 0xFFFFFFFF)
	binary.BigEndian.PutUint32(block[9:13], 0xFFFFFFFF)
	// 13-20: timestamp (-1.0 = 0xBFF0000000000000)
	binary.BigEndian.PutUint64(block[13:21], math.Float64bits(-1.0))
	// 21: immediate
	if immediate {
		block[21] = 0x01
	} else {
		block[21] = 0x00
	}

	w.WriteBlockData(block[:])
	w.WriteByte(TC_ENDBLOCKDATA)
}

// NewRtiProbePayload gera o payload binário exato para o RtiProbe no padrão JOS
func NewRtiProbePayload() []byte {
	w := NewJOSWriter()

	w.WriteByte(TC_OBJECT)
	w.WriteByte(TC_CLASSDESC)
	w.WriteUTF(ClassRtiProbe)
	w.WriteUint64(PorticoSerialVersionUID)
	w.WriteByte(SC_SERIALIZABLE)
	w.WriteUint16(0) // RtiProbe não adiciona campos próprios
	w.WriteByte(TC_ENDBLOCKDATA)

	// Superclasse PorticoMessage
	w.writePorticoMessageClassDesc()

	// Valores: targetFederate = 0 (RTI_HANDLE), immediate = false
	w.writePorticoMessageValues(false, 0)

	return w.Bytes()
}

// NewJoinFederationPayload gera o payload binário exato para o JoinFederation no padrão JOS
func NewJoinFederationPayload(federationName, federateName, federateType string) []byte {
	w := NewJOSWriter()

	w.WriteByte(TC_OBJECT)
	w.WriteByte(TC_CLASSDESC)
	w.WriteUTF(ClassJoinFederation)
	w.WriteUint64(PorticoSerialVersionUID)
	w.WriteByte(SC_SERIALIZABLE)
	w.WriteUint16(5) // 5 campos de objeto ordenados alfabeticamente

	// 1. federateName (Ljava/lang/String;)
	w.WriteByte(TypeObject)
	w.WriteUTF("federateName")
	w.WriteByte(TC_STRING)
	w.WriteUTF(TypeStringDesc)

	// 2. federateType (referencia ao descritor de String anterior)
	w.WriteByte(TypeObject)
	w.WriteUTF("federateType")
	w.WriteByte(TC_REFERENCE)
	w.WriteUint16(0x007E)
	w.WriteUint16(0x0001) // Handle de referencia 0x007E0001

	// 3. federationName (referencia ao descritor de String anterior)
	w.WriteByte(TypeObject)
	w.WriteUTF("federationName")
	w.WriteByte(TC_REFERENCE)
	w.WriteUint16(0x007E)
	w.WriteUint16(0x0001)

	// 4. parsedJoinObjectModels (Ljava/util/List;)
	w.WriteByte(TypeObject)
	w.WriteUTF("parsedJoinObjectModels")
	w.WriteByte(TC_STRING)
	w.WriteUTF(TypeListDesc)

	// 5. rawJoinObjectModels (referencia ao descritor de List anterior)
	w.WriteByte(TypeObject)
	w.WriteUTF("rawJoinObjectModels")
	w.WriteByte(TC_REFERENCE)
	w.WriteUint16(0x007E)
	w.WriteUint16(0x0002)

	w.WriteByte(TC_ENDBLOCKDATA)

	// Superclasse PorticoMessage
	w.writePorticoMessageClassDesc()

	// Valores da superclasse PorticoMessage: targetFederate = 0, immediate = true
	w.writePorticoMessageValues(true, 0)

	// Valores dos campos de JoinFederation:
	// String federateName
	w.WriteByte(TC_STRING)
	w.WriteUTF(federateName)

	// String federateType
	w.WriteByte(TC_STRING)
	w.WriteUTF(federateType)

	// String federationName
	w.WriteByte(TC_STRING)
	w.WriteUTF(federationName)

	// parsedJoinObjectModels = new ArrayList(0)
	w.WriteByte(TC_OBJECT)
	w.WriteByte(TC_CLASSDESC)
	w.WriteUTF(ClassArrayList)
	w.WriteUint64(ArrayListSerialVersionUID)
	w.WriteByte(SC_SERIALIZABLE | SC_WRITE_METHOD)
	w.WriteUint16(1) // 1 campo: size (I)
	w.WriteByte(TypeInt)
	w.WriteUTF("size")
	w.WriteByte(TC_ENDBLOCKDATA)
	w.WriteByte(TC_NULL) // Superclasse do ArrayList
	w.WriteInt32(0)      // size = 0
	// ArrayList writeObject escreve a capacidade no bloco:
	w.WriteBlockData([]byte{0x00, 0x00, 0x00, 0x00})
	w.WriteByte(TC_ENDBLOCKDATA)

	// rawJoinObjectModels = new ArrayList(0)
	w.WriteByte(TC_OBJECT)
	w.WriteByte(TC_REFERENCE)
	w.WriteUint16(0x007E)
	w.WriteUint16(0x0008) // Referencia a classe ArrayList descrita acima
	w.WriteInt32(0)       // size = 0
	w.WriteBlockData([]byte{0x00, 0x00, 0x00, 0x00})
	w.WriteByte(TC_ENDBLOCKDATA)

	return w.Bytes()
}
