package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Constantes fundamentais do Java Object Serialization (JOS)
const (
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

	SC_WRITE_METHOD byte = 0x01
	SC_SERIALIZABLE byte = 0x02

	BaseWireHandle uint32 = 0x007E0000
)

// JOSEncoder é o motor genérico agnóstico de serialização Java em Go (equivalente ao ObjectOutputStream)
type JOSEncoder struct {
	w          *bytes.Buffer
	writtenHdr bool
	refMap     map[string]uint32
	nextHandle uint32
}

// NewJOSEncoder inicializa um novo codificador JOS
func NewJOSEncoder() *JOSEncoder {
	return &JOSEncoder{
		w:          &bytes.Buffer{},
		refMap:     make(map[string]uint32),
		nextHandle: BaseWireHandle,
	}
}

// WriteStreamHeader escreve o cabeçalho oficial do stream Java (0xACED 0x0005)
func (e *JOSEncoder) WriteStreamHeader() {
	if !e.writtenHdr {
		binary.Write(e.w, binary.BigEndian, uint16(0xACED))
		binary.Write(e.w, binary.BigEndian, uint16(0x0005))
		e.writtenHdr = true
	}
}

// WriteByte grava um byte no stream
func (e *JOSEncoder) WriteByte(b byte) {
	e.w.WriteByte(b)
}

// WriteInt32 grava um inteiro de 32 bits Big-Endian no stream
func (e *JOSEncoder) WriteInt32(v int32) {
	binary.Write(e.w, binary.BigEndian, v)
}

// WriteUint16 grava um inteiro de 16 bits Big-Endian no stream
func (e *JOSEncoder) WriteUint16(v uint16) {
	binary.Write(e.w, binary.BigEndian, v)
}

// WriteUint64 grava um inteiro de 64 bits Big-Endian no stream
func (e *JOSEncoder) WriteUint64(v uint64) {
	binary.Write(e.w, binary.BigEndian, v)
}

// WriteDouble grava um ponto flutuante de 64 bits no stream
func (e *JOSEncoder) WriteDouble(v float64) {
	binary.Write(e.w, binary.BigEndian, math.Float64bits(v))
}

// WriteBlockData escreve um bloco opaco de dados binários (TC_BLOCKDATA)
func (e *JOSEncoder) WriteBlockData(data []byte) {
	e.w.WriteByte(TC_BLOCKDATA)
	e.w.WriteByte(byte(len(data)))
	e.w.Write(data)
}

// WriteObject serializa qualquer objeto Java com base em sua árvore de descritores de classe
func (e *JOSEncoder) WriteObject(obj any) error {
	e.WriteStreamHeader()

	if obj == nil {
		e.w.WriteByte(TC_NULL)
		return nil
	}

	switch v := obj.(type) {
	case string:
		e.writeString(v)
		return nil
	case *JavaObject:
		return e.writeJavaObject(v)
	default:
		return fmt.Errorf("jos encoder: tipo nao suportado para serializacao direta: %T", obj)
	}
}

// writeString escreve uma string Java como TC_STRING ou TC_REFERENCE se já poolada
func (e *JOSEncoder) writeString(s string) {
	refKey := "str:" + s
	if handle, exists := e.refMap[refKey]; exists {
		e.writeReference(handle)
		return
	}

	e.w.WriteByte(TC_STRING)
	e.writeUTF(s)
	e.assignHandle(refKey)
}

func (e *JOSEncoder) writeReference(handle uint32) {
	e.w.WriteByte(TC_REFERENCE)
	binary.Write(e.w, binary.BigEndian, handle)
}

func (e *JOSEncoder) assignHandle(key string) uint32 {
	h := e.nextHandle
	e.refMap[key] = h
	e.nextHandle++
	return h
}

func (e *JOSEncoder) writeUTF(s string) {
	binary.Write(e.w, binary.BigEndian, uint16(len(s)))
	e.w.WriteString(s)
}

// writeJavaObject serializa a instância do objeto Java (TC_OBJECT) e sua hierarquia
func (e *JOSEncoder) writeJavaObject(obj *JavaObject) error {
	if obj.Class == nil {
		return fmt.Errorf("jos encoder: objeto sem descritor de classe (ClassDesc nil)")
	}

	e.w.WriteByte(TC_OBJECT)
	if err := e.writeClassDesc(obj.Class); err != nil {
		return err
	}
	e.assignHandle(fmt.Sprintf("obj:%p", obj))

	// Coletar hierarquia de classes da raiz (Object) até a folha (classe concreta)
	chain := getClassHierarchy(obj.Class)

	// Escrever os valores de dados de cada classe na hierarquia (da base para a filha)
	for _, cls := range chain {
		if cls.Flags&SC_WRITE_METHOD != 0 && cls.CustomWriter != nil {
			// Classe possui serialização customizada (writeObject / writeExternal)
			cls.CustomWriter(e, obj)
		} else {
			// Serialização de campos declarados da classe
			for _, field := range cls.Fields {
				val := obj.Fields[field.Name]
				if err := e.writeFieldValue(field, val); err != nil {
					return fmt.Errorf("falha ao gravar campo %q da classe %s: %w", field.Name, cls.Name, err)
				}
			}
		}
	}

	return nil
}

// writeClassDesc escreve a cadeia de metadados da classe (TC_CLASSDESC)
func (e *JOSEncoder) writeClassDesc(cls *ClassDesc) error {
	if cls == nil {
		e.w.WriteByte(TC_NULL)
		return nil
	}

	refKey := "cls:" + cls.Name
	if handle, exists := e.refMap[refKey]; exists {
		e.writeReference(handle)
		return nil
	}

	e.w.WriteByte(TC_CLASSDESC)
	e.writeUTF(cls.Name)
	binary.Write(e.w, binary.BigEndian, cls.SerialVersionUID)
	e.w.WriteByte(cls.Flags)
	binary.Write(e.w, binary.BigEndian, uint16(len(cls.Fields)))

	// Registrar handle do descritor de classe
	e.assignHandle(refKey)

	// Descrever campos declarados
	for _, field := range cls.Fields {
		e.w.WriteByte(byte(field.Type))
		e.writeUTF(field.Name)
		if field.Type == TypeObject || field.Type == TypeArray {
			e.writeString(field.ClassName)
		}
	}

	e.w.WriteByte(TC_ENDBLOCKDATA)

	// Escrever superclasse recursivamente
	return e.writeClassDesc(cls.Super)
}

func (e *JOSEncoder) writeFieldValue(field FieldDesc, val any) error {
	switch field.Type {
	case TypeBoolean:
		b, _ := val.(bool)
		if b {
			e.w.WriteByte(1)
		} else {
			e.w.WriteByte(0)
		}
	case TypeByte:
		b, _ := val.(byte)
		e.w.WriteByte(b)
	case TypeShort:
		s, _ := val.(int16)
		binary.Write(e.w, binary.BigEndian, s)
	case TypeInt:
		var i int32
		switch v := val.(type) {
		case int:
			i = int32(v)
		case int32:
			i = v
		case uint32:
			i = int32(v)
		}
		binary.Write(e.w, binary.BigEndian, i)
	case TypeLong:
		var l int64
		switch v := val.(type) {
		case int64:
			l = v
		case uint64:
			l = int64(v)
		case int:
			l = int64(v)
		}
		binary.Write(e.w, binary.BigEndian, l)
	case TypeDouble:
		d, _ := val.(float64)
		binary.Write(e.w, binary.BigEndian, math.Float64bits(d))
	case TypeObject, TypeArray:
		return e.WriteObject(val)
	default:
		return fmt.Errorf("tipo de campo desconhecido: %s", field.Type)
	}
	return nil
}

func getClassHierarchy(cls *ClassDesc) []*ClassDesc {
	var list []*ClassDesc
	for c := cls; c != nil; c = c.Super {
		list = append([]*ClassDesc{c}, list...) // Insere no início para inverter (base primeiro)
	}
	return list
}

// Bytes retorna o slice de bytes completo gerado
func (e *JOSEncoder) Bytes() []byte {
	return e.w.Bytes()
}

// WriteTo descarrega os bytes serializados para um writer arbitrário
func (e *JOSEncoder) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(e.w.Bytes())
	return int64(n), err
}
