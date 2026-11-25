package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
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
func (e *JOSEncoder) WriteByte(b byte) error {
	return e.w.WriteByte(b)
}

// WriteInt16 grava um inteiro de 16 bits Big-Endian no stream
func (e *JOSEncoder) WriteInt16(v int16) {
	binary.Write(e.w, binary.BigEndian, v)
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

// WriteString escreve uma string Java como TC_STRING ou TC_REFERENCE
func (e *JOSEncoder) WriteString(s string) {
	e.writeString(s)
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
	case *JavaEnum:
		return e.writeJavaEnum(v)
	case []byte:
		if v == nil {
			e.w.WriteByte(TC_NULL)
			return nil
		}
		return e.writeByteArray(v)
	default:
		return fmt.Errorf("jos encoder: tipo nao suportado para serializacao direta: %T", obj)
	}
}

// writeJavaEnum serializa uma constante Enum Java (TC_ENUM)
func (e *JOSEncoder) writeJavaEnum(en *JavaEnum) error {
	if en == nil || en.Class == nil {
		e.w.WriteByte(TC_NULL)
		return nil
	}

	refKey := fmt.Sprintf("enum:%p", en)
	if handle, exists := e.refMap[refKey]; exists {
		e.writeReference(handle)
		return nil
	}

	e.w.WriteByte(TC_ENUM)
	if err := e.writeClassDesc(en.Class); err != nil {
		return err
	}
	e.assignHandle(refKey)
	// Java readEnum strictly requires TC_STRING (0x74) or TC_LONGSTRING (0x7C), never TC_REFERENCE (0x71)
	e.w.WriteByte(TC_STRING)
	e.writeUTF(en.Name)
	e.assignHandle(fmt.Sprintf("enum_name:%d", e.nextHandle))
	return nil
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

	refKey := fmt.Sprintf("obj:%p", obj)
	if handle, exists := e.refMap[refKey]; exists {
		e.writeReference(handle)
		return nil
	}

	e.w.WriteByte(TC_OBJECT)
	if err := e.writeClassDesc(obj.Class); err != nil {
		return err
	}
	e.assignHandle(refKey)

	// Coletar hierarquia de classes da raiz (Object) até a folha (classe concreta)
	chain := getClassHierarchy(obj.Class)

	// Se o objeto for Externalizable, apenas seu próprio CustomWriter (writeExternal) é executado
	if obj.Class.Flags&SC_EXTERNALIZABLE != 0 {
		if obj.Class.CustomWriter != nil {
			if err := obj.Class.CustomWriter(e, obj); err != nil {
				return err
			}
		}
		return nil
	}

	// Objeto Serializable padrão: percorrer da classe base até a classe concreta
	for _, cls := range chain {
		// 1. Escrever campos declarados da classe (defaultWriteObject) se não for CustomOnly
		if !cls.CustomOnly {
			for _, field := range cls.Fields {
				val := obj.Fields[field.Name]
				if err := e.writeFieldValue(field, val); err != nil {
					return fmt.Errorf("falha ao gravar campo %q da classe %s: %w", field.Name, cls.Name, err)
				}
			}
		}

		// 2. Se a classe possuir writeObject customizado (SC_WRITE_METHOD), executá-lo
		if cls.Flags&SC_WRITE_METHOD != 0 && cls.CustomWriter != nil {
			if err := cls.CustomWriter(e, obj); err != nil {
				return err
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
	case TypeFloat:
		var f float32
		switch v := val.(type) {
		case float32:
			f = v
		case float64:
			f = float32(v)
		}
		binary.Write(e.w, binary.BigEndian, math.Float32bits(f))
	case TypeDouble:
		d, _ := val.(float64)
		binary.Write(e.w, binary.BigEndian, math.Float64bits(d))
	case TypeChar:
		var c uint16
		switch v := val.(type) {
		case rune:
			c = uint16(v)
		case uint16:
			c = v
		}
		binary.Write(e.w, binary.BigEndian, c)
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

func (e *JOSEncoder) writeByteArray(data []byte) error {
	refKey := fmt.Sprintf("arr:%p", data)
	if handle, exists := e.refMap[refKey]; exists {
		e.writeReference(handle)
		return nil
	}

	e.w.WriteByte(TC_ARRAY)
	byteArrDesc := &ClassDesc{
		Name:             "[B",
		SerialVersionUID: 0,
		Flags:            SC_SERIALIZABLE,
		Fields:           nil,
		Super:            nil,
	}
	if err := e.writeClassDesc(byteArrDesc); err != nil {
		return err
	}
	e.assignHandle(refKey)
	binary.Write(e.w, binary.BigEndian, int32(len(data)))
	e.w.Write(data)
	return nil
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
