package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// JOSDecoder é o motor genérico agnóstico de deserialização Java em Go (equivalente ao ObjectInputStream)
type JOSDecoder struct {
	r           *bytes.Reader
	readHdr     bool
	handleTable map[uint32]any
	nextHandle  uint32
}

// NewJOSDecoder instancia um novo deserializador a partir de um slice de bytes
func NewJOSDecoder(data []byte) *JOSDecoder {
	return &JOSDecoder{
		r:           bytes.NewReader(data),
		handleTable: make(map[uint32]any),
		nextHandle:  BaseWireHandle,
	}
}

// ReadStreamHeader valida o cabeçalho oficial do stream Java (0xACED 0x0005)
func (d *JOSDecoder) ReadStreamHeader() error {
	if d.readHdr {
		return nil
	}

	var magic, version uint16
	if err := binary.Read(d.r, binary.BigEndian, &magic); err != nil {
		return fmt.Errorf("jos decoder: falha ao ler STREAM_MAGIC: %w", err)
	}
	if magic != 0xACED {
		return fmt.Errorf("jos decoder: STREAM_MAGIC invalido: 0x%04X (esperado 0xACED)", magic)
	}

	if err := binary.Read(d.r, binary.BigEndian, &version); err != nil {
		return fmt.Errorf("jos decoder: falha ao ler STREAM_VERSION: %w", err)
	}
	if version != 0x0005 {
		return fmt.Errorf("jos decoder: STREAM_VERSION invalido: 0x%04X (esperado 0x0005)", version)
	}

	d.readHdr = true
	return nil
}

// ReadByte lê 1 byte do stream
func (d *JOSDecoder) ReadByte() (byte, error) {
	return d.r.ReadByte()
}

// ReadInt16 lê um int16 Big-Endian
func (d *JOSDecoder) ReadInt16() (int16, error) {
	var v int16
	err := binary.Read(d.r, binary.BigEndian, &v)
	return v, err
}

// ReadInt32 lê um int32 Big-Endian
func (d *JOSDecoder) ReadInt32() (int32, error) {
	var v int32
	err := binary.Read(d.r, binary.BigEndian, &v)
	return v, err
}

// ReadUint16 lê um uint16 Big-Endian
func (d *JOSDecoder) ReadUint16() (uint16, error) {
	var v uint16
	err := binary.Read(d.r, binary.BigEndian, &v)
	return v, err
}

// ReadUint64 lê um uint64 Big-Endian
func (d *JOSDecoder) ReadUint64() (uint64, error) {
	var v uint64
	err := binary.Read(d.r, binary.BigEndian, &v)
	return v, err
}

// ReadDouble lê um float64 Big-Endian
func (d *JOSDecoder) ReadDouble() (float64, error) {
	var bits uint64
	if err := binary.Read(d.r, binary.BigEndian, &bits); err != nil {
		return 0, err
	}
	return math.Float64frombits(bits), nil
}

// ReadUTF lê uma string UTF codificada com prefixo uint16 de tamanho
func (d *JOSDecoder) ReadUTF() (string, error) {
	length, err := d.ReadUint16()
	if err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(d.r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// ReadString lê um objeto string (TC_STRING ou TC_REFERENCE) do stream
func (d *JOSDecoder) ReadString() (string, error) {
	obj, err := d.ReadObject()
	if err != nil {
		return "", err
	}
	if s, ok := obj.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", obj), nil
}

// ReadBlockData lê um bloco de dados opaco (TC_BLOCKDATA ou TC_BLOCKDATALONG)
func (d *JOSDecoder) ReadBlockData() ([]byte, error) {
	tc, err := d.ReadByte()
	if err != nil {
		return nil, err
	}

	var length int
	if tc == TC_BLOCKDATA {
		bLen, err := d.ReadByte()
		if err != nil {
			return nil, err
		}
		length = int(bLen)
	} else if tc == 0x7A { // TC_BLOCKDATALONG
		iLen, err := d.ReadInt32()
		if err != nil {
			return nil, err
		}
		length = int(iLen)
	} else {
		return nil, fmt.Errorf("jos decoder: esperado TC_BLOCKDATA (0x77/0x7A), recebido 0x%02X", tc)
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(d.r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// ReadObject deserializa o próximo objeto do stream
func (d *JOSDecoder) ReadObject() (any, error) {
	if err := d.ReadStreamHeader(); err != nil {
		return nil, err
	}

	tc, err := d.ReadByte()
	if err != nil {
		return nil, err
	}

	switch tc {
	case TC_NULL:
		return nil, nil

	case TC_REFERENCE:
		handle, err := d.readHandle()
		if err != nil {
			return nil, err
		}
		obj, ok := d.handleTable[handle]
		if !ok {
			return nil, fmt.Errorf("jos decoder: handle de referencia nao encontrado: 0x%08X", handle)
		}
		return obj, nil

	case TC_STRING:
		s, err := d.ReadUTF()
		if err != nil {
			return nil, err
		}
		d.assignHandle(s)
		return s, nil

	case TC_ENUM:
		return d.readEnum()

	case TC_OBJECT:
		return d.readObjectInstance()

	case TC_BLOCKDATA:
		// Se encontrado solto, lê como bloco de dados
		bLen, err := d.ReadByte()
		if err != nil {
			return nil, err
		}
		buf := make([]byte, bLen)
		if _, err := io.ReadFull(d.r, buf); err != nil {
			return nil, err
		}
		return buf, nil

	default:
		return nil, fmt.Errorf("jos decoder: token desconhecido ou nao suportado: 0x%02X", tc)
	}
}

func (d *JOSDecoder) readHandle() (uint32, error) {
	var handle uint32
	err := binary.Read(d.r, binary.BigEndian, &handle)
	return handle, err
}

func (d *JOSDecoder) assignHandle(obj any) uint32 {
	h := d.nextHandle
	d.handleTable[h] = obj
	d.nextHandle++
	return h
}

// readClassDesc lê um descritor de classe Java (TC_CLASSDESC ou TC_REFERENCE ou TC_NULL)
func (d *JOSDecoder) readClassDesc() (*ClassDesc, error) {
	tc, err := d.ReadByte()
	if err != nil {
		return nil, err
	}

	switch tc {
	case TC_NULL:
		return nil, nil

	case TC_REFERENCE:
		handle, err := d.readHandle()
		if err != nil {
			return nil, err
		}
		cached, ok := d.handleTable[handle]
		if !ok {
			return nil, fmt.Errorf("jos decoder: handle de classe nao encontrado: 0x%08X", handle)
		}
		cls, ok := cached.(*ClassDesc)
		if !ok {
			return nil, fmt.Errorf("jos decoder: handle 0x%08X nao aponta para ClassDesc", handle)
		}
		return cls, nil

	case TC_CLASSDESC:
		name, err := d.ReadUTF()
		if err != nil {
			return nil, err
		}
		suid, err := d.ReadUint64()
		if err != nil {
			return nil, err
		}
		flags, err := d.ReadByte()
		if err != nil {
			return nil, err
		}
		numFields, err := d.ReadUint16()
		if err != nil {
			return nil, err
		}

		cls := &ClassDesc{
			Name:             name,
			SerialVersionUID: suid,
			Flags:            flags,
			Fields:           make([]FieldDesc, numFields),
		}

		if known, ok := LookupKnownClassDesc(name); ok {
			cls.CustomReader = known.CustomReader
			cls.CustomWriter = known.CustomWriter
		}

		// Registrar o ClassDesc na tabela de handles imediatamente antes de ler os tipos dos campos
		d.assignHandle(cls)

		for i := uint16(0); i < numFields; i++ {
			fType, err := d.ReadByte()
			if err != nil {
				return nil, err
			}
			fName, err := d.ReadUTF()
			if err != nil {
				return nil, err
			}
			cls.Fields[i] = FieldDesc{
				Name: fName,
				Type: FieldType(fType),
			}
			if FieldType(fType) == TypeObject || FieldType(fType) == TypeArray {
				// Nome da classe do objeto pode vir como TC_STRING ou TC_REFERENCE
				cNameObj, err := d.ReadObject()
				if err != nil {
					return nil, err
				}
				if cName, ok := cNameObj.(string); ok {
					cls.Fields[i].ClassName = cName
				}
			}
		}

		// Ler anotações de classe até TC_ENDBLOCKDATA
		endMarker, err := d.ReadByte()
		if err != nil {
			return nil, err
		}
		if endMarker != TC_ENDBLOCKDATA {
			return nil, fmt.Errorf("jos decoder: esperado TC_ENDBLOCKDATA (0x78) no ClassDesc, recebido 0x%02X", endMarker)
		}

		// Ler superclasse recursivamente
		superCls, err := d.readClassDesc()
		if err != nil {
			return nil, err
		}
		cls.Super = superCls

		return cls, nil

	default:
		return nil, fmt.Errorf("jos decoder: esperado descritor de classe (0x72/0x71/0x70), recebido 0x%02X", tc)
	}
}

// readEnum lê uma constante enumerada Java (TC_ENUM)
func (d *JOSDecoder) readEnum() (*JavaEnum, error) {
	cls, err := d.readClassDesc()
	if err != nil {
		return nil, err
	}

	en := &JavaEnum{Class: cls}
	d.assignHandle(en)

	// Ler nome da constante enum
	nameObj, err := d.ReadObject()
	if err != nil {
		return nil, err
	}
	if nameStr, ok := nameObj.(string); ok {
		en.Name = nameStr
	} else {
		return nil, fmt.Errorf("jos decoder: nome de constante enum esperado string, recebido %T", nameObj)
	}

	return en, nil
}

// readObjectInstance lê uma instância de objeto Java (TC_OBJECT)
func (d *JOSDecoder) readObjectInstance() (*JavaObject, error) {
	cls, err := d.readClassDesc()
	if err != nil {
		return nil, err
	}
	if cls == nil {
		return nil, fmt.Errorf("jos decoder: classe nula ao ler instancia de objeto")
	}

	obj := NewJavaObject(cls)
	// Registrar handle imediatamente para permitir grafos cíclicos
	d.assignHandle(obj)

	// Coletar hierarquia de classes da raiz (Object) até a folha (concreta)
	chain := getClassHierarchy(cls)

	// Ler os valores dos campos de cada classe na cadeia
	for _, c := range chain {
		if (c.Flags&SC_WRITE_METHOD != 0 || c.Flags&SC_EXTERNALIZABLE != 0) && c.CustomReader != nil {
			// Classe possui deserialização customizada
			if err := c.CustomReader(d, obj); err != nil {
				return nil, fmt.Errorf("falha no CustomReader da classe %s: %w", c.Name, err)
			}
		} else {
			for _, field := range c.Fields {
				val, err := d.readFieldValue(field)
				if err != nil {
					return nil, fmt.Errorf("falha ao ler campo %q da classe %s: %w", field.Name, c.Name, err)
				}
				obj.Fields[field.Name] = val
			}
		}
	}

	return obj, nil
}

func (d *JOSDecoder) readFieldValue(field FieldDesc) (any, error) {
	switch field.Type {
	case TypeBoolean:
		b, err := d.ReadByte()
		if err != nil {
			return nil, err
		}
		return b != 0, nil

	case TypeByte:
		return d.ReadByte()

	case TypeShort:
		var s int16
		err := binary.Read(d.r, binary.BigEndian, &s)
		return s, err

	case TypeInt:
		return d.ReadInt32()

	case TypeLong:
		var l int64
		err := binary.Read(d.r, binary.BigEndian, &l)
		return l, err

	case TypeFloat:
		var bits uint32
		if err := binary.Read(d.r, binary.BigEndian, &bits); err != nil {
			return nil, err
		}
		return math.Float32frombits(bits), nil

	case TypeDouble:
		return d.ReadDouble()

	case TypeObject, TypeArray:
		return d.ReadObject()

	default:
		return nil, fmt.Errorf("tipo de campo desconhecido: %s", field.Type)
	}
}

// ExtractErrorMessage extrai a mensagem legível de erro (detailMessage) de um payload de exceção Java/Portico
func ExtractErrorMessage(payload []byte) string {
	idx := bytes.Index(payload, []byte("detailMessage"))
	if idx == -1 {
		return "erro retornado pelo CRTI sem detailMessage"
	}

	sub := payload[idx:]
	// O primeiro TC_STRING (0x74) é a assinatura "Ljava/lang/String;"
	firstTC := bytes.IndexByte(sub, TC_STRING)
	if firstTC == -1 {
		return "erro retornado pelo CRTI"
	}

	// O segundo TC_STRING após os dados do objeto é o valor da mensagem
	rest := sub[firstTC+1:]
	secondTC := bytes.IndexByte(rest, TC_STRING)
	if secondTC != -1 && secondTC+3 <= len(rest) {
		strLen := int(binary.BigEndian.Uint16(rest[secondTC+1 : secondTC+3]))
		if secondTC+3+strLen <= len(rest) {
			return string(rest[secondTC+3 : secondTC+3+strLen])
		}
	}

	return "erro retornado pelo CRTI"
}

// DeserializePorticoControlMessage deserializa um payload de controle do Portico
// descartando o byte de bloco do cabeçalho manualMarshal = false se presente
func DeserializePorticoControlMessage(data []byte) (*JavaObject, error) {
	dec := NewJOSDecoder(data)
	if err := dec.ReadStreamHeader(); err != nil {
		return nil, err
	}

	// Verificar se inicia com TC_BLOCKDATA (0x77 0x01 0x00 para manualMarshal=false)
	peek, err := dec.ReadByte()
	if err != nil {
		return nil, err
	}

	if peek == TC_BLOCKDATA {
		bLen, err := dec.ReadByte()
		if err != nil {
			return nil, err
		}
		// Descartar bytes do bloco
		buf := make([]byte, bLen)
		if _, err := io.ReadFull(dec.r, buf); err != nil {
			return nil, err
		}
	} else {
		// Reposicionar 1 byte se não era TC_BLOCKDATA
		dec.r.Seek(-1, io.SeekCurrent)
	}

	objAny, err := dec.ReadObject()
	if err != nil {
		return nil, err
	}

	obj, ok := objAny.(*JavaObject)
	if !ok {
		return nil, fmt.Errorf("objeto deserializado nao e *JavaObject: %T", objAny)
	}

	return obj, nil
}
