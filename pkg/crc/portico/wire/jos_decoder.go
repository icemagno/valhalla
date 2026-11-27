package wire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
)

const (
	maxJOSAllocation      = 16 * 1024 * 1024 // 16 MB safety limit against OOM DoS
	maxJOSCollectionItems = 100000
	maxJOSDepth           = 64
)

// JOSDecoder é o motor genérico agnóstico de deserialização Java em Go (equivalente ao ObjectInputStream)
type JOSDecoder struct {
	r           *bytes.Reader
	readHdr     bool
	handleTable map[uint32]any
	nextHandle  uint32
	depth       int
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
		if iLen < 0 || int(iLen) > maxJOSAllocation {
			return nil, fmt.Errorf("jos decoder: TC_BLOCKDATALONG length %d out of bounds [0, %d]", iLen, maxJOSAllocation)
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
	if d.depth >= maxJOSDepth {
		return nil, fmt.Errorf("jos decoder: max recursion depth (%d) exceeded", maxJOSDepth)
	}
	d.depth++
	defer func() { d.depth-- }()

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

	case TC_ARRAY:
		return d.readArray()

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

	case 0x7A: // TC_BLOCKDATALONG
		var iLen int32
		if err := binary.Read(d.r, binary.BigEndian, &iLen); err != nil {
			return nil, err
		}
		if iLen < 0 || int(iLen) > maxJOSAllocation {
			return nil, fmt.Errorf("jos decoder: TC_BLOCKDATALONG length %d out of bounds [0, %d]", iLen, maxJOSAllocation)
		}
		buf := make([]byte, iLen)
		if _, err := io.ReadFull(d.r, buf); err != nil {
			return nil, err
		}
		return buf, nil

	case TC_ENDBLOCKDATA:
		return nil, io.EOF

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
	if d.depth >= maxJOSDepth {
		return nil, fmt.Errorf("jos decoder: max recursion depth (%d) exceeded in readClassDesc", maxJOSDepth)
	}
	d.depth++
	defer func() { d.depth-- }()

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
			cls.CustomOnly = known.CustomOnly
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

// readArray lê um array de primitivos ou objetos Java (TC_ARRAY)
func (d *JOSDecoder) readArray() (any, error) {
	cls, err := d.readClassDesc()
	if err != nil {
		return nil, fmt.Errorf("jos decoder: falha ao ler descritor de classe do array: %w", err)
	}

	var size int32
	if err := binary.Read(d.r, binary.BigEndian, &size); err != nil {
		return nil, fmt.Errorf("jos decoder: falha ao ler tamanho do array: %w", err)
	}
	if size < 0 {
		return nil, fmt.Errorf("jos decoder: tamanho de array negativo: %d", size)
	}

	if cls != nil && cls.Name == "[B" {
		if int(size) > maxJOSAllocation {
			return nil, fmt.Errorf("jos decoder: byte array size %d exceeds maximum %d", size, maxJOSAllocation)
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(d.r, buf); err != nil {
			return nil, fmt.Errorf("jos decoder: falha ao ler bytes do byte array: %w", err)
		}
		d.assignHandle(buf)
		return buf, nil
	}

	if int(size) > maxJOSCollectionItems {
		return nil, fmt.Errorf("jos decoder: object array size %d exceeds maximum %d", size, maxJOSCollectionItems)
	}
	// Array genérico de objetos
	arr := make([]any, size)
	d.assignHandle(arr)
	for i := int32(0); i < size; i++ {
		el, err := d.ReadObject()
		if err != nil {
			return nil, fmt.Errorf("jos decoder: falha ao ler elemento %d do array: %w", i, err)
		}
		arr[i] = el
	}
	return arr, nil
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

	// Se a classe for Externalizable, apenas seu próprio CustomReader (readExternal) é executado
	if cls.Flags&SC_EXTERNALIZABLE != 0 {
		if cls.CustomReader != nil {
			if err := cls.CustomReader(d, obj); err != nil {
				return nil, fmt.Errorf("falha no CustomReader da classe %s: %w", cls.Name, err)
			}
		}
		return obj, nil
	}

	// Objeto Serializable padrão: percorrer hierarquia da classe base até a concreta
	for _, c := range chain {
		// 1. Ler campos declarados da classe (defaultReadObject) se não for CustomOnly
		if !c.CustomOnly {
			for _, field := range c.Fields {
				val, err := d.readFieldValue(field)
				if err != nil {
					return nil, fmt.Errorf("falha ao ler campo %q da classe %s: %w", field.Name, c.Name, err)
				}
				obj.Fields[field.Name] = val
			}
		}

		// 2. Se a classe possuir readObject customizado (SC_WRITE_METHOD), executá-lo ou pular custom data
		if c.Flags&SC_WRITE_METHOD != 0 {
			if c.CustomReader != nil {
				if err := c.CustomReader(d, obj); err != nil {
					return nil, fmt.Errorf("falha no CustomReader da classe %s: %w", c.Name, err)
				}
			} else {
				if err := d.skipCustomData(); err != nil {
					return nil, fmt.Errorf("falha ao pular dados customizados da classe %s: %w", c.Name, err)
				}
			}
		}
	}

	return obj, nil
}

// skipCustomData avança o stream ignorando blocos primitivos e objetos customizados até TC_ENDBLOCKDATA (0x78)
func (d *JOSDecoder) skipCustomData() error {
	for {
		tc, err := d.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch tc {
		case TC_ENDBLOCKDATA:
			return nil
		case TC_BLOCKDATA:
			bLen, err := d.ReadByte()
			if err != nil {
				return err
			}
			if _, err := d.r.Seek(int64(bLen), io.SeekCurrent); err != nil {
				return err
			}
		case 0x7A: // TC_BLOCKDATALONG
			var iLen int32
			if err := binary.Read(d.r, binary.BigEndian, &iLen); err != nil {
				return err
			}
			if iLen < 0 || int(iLen) > maxJOSAllocation {
				return fmt.Errorf("jos decoder: invalid TC_BLOCKDATALONG length %d in skipCustomData", iLen)
			}
			if _, err := d.r.Seek(int64(iLen), io.SeekCurrent); err != nil {
				return err
			}
		default:
			// Objeto gravado em writeObject
			if _, err := d.r.Seek(-1, io.SeekCurrent); err != nil {
				return err
			}
			if _, err := d.ReadObject(); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		}
	}
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
// tratando tanto mensagens padrão Java Serialization (com manualMarshal = false)
// quanto mensagens manuais rápidas do Portico (manualMarshal = true, ex: UpdateAttributes e SendInteraction)
func DeserializePorticoControlMessage(data []byte) (*JavaObject, error) {
	dec := NewJOSDecoder(data)
	if err := dec.ReadStreamHeader(); err != nil {
		return nil, err
	}

	// Verificar se inicia com TC_BLOCKDATA
	peek, err := dec.ReadByte()
	if err != nil {
		return nil, err
	}

	if peek == TC_BLOCKDATA {
		bLen, err := dec.ReadByte()
		if err != nil {
			return nil, err
		}
		buf := make([]byte, bLen)
		if _, err := io.ReadFull(dec.r, buf); err != nil {
			return nil, err
		}

		// Se manualMarshal == true (buf[0] == 1), decodificar mensagem manual do Portico
		if len(buf) >= 3 && buf[0] == 1 {
			msgType := binary.BigEndian.Uint16(buf[1:3])
			switch msgType {
			case 254: // UpdateAttributes
				if len(buf) >= 25 {
					obj := metamodel.NewJavaObject(metamodel.ClassDescUpdateAttributes)
					objectId := int32(binary.BigEndian.Uint32(buf[3:7]))
					obj.Set("objectId", objectId)
					obj.Set("sourceFederate", int32(binary.BigEndian.Uint32(buf[7:11])))
					obj.Set("targetFederation", int32(binary.BigEndian.Uint32(buf[11:15])))
					obj.Set("timestamp", math.Float64frombits(binary.BigEndian.Uint64(buf[15:23])))
					tagLen := int(binary.BigEndian.Uint16(buf[23:25]))
					idx := 25
					if idx+tagLen <= len(buf) {
						obj.Set("tag", buf[idx:idx+tagLen])
						idx += tagLen
					}
					if idx+4 <= len(buf) {
						attrCount := binary.BigEndian.Uint32(buf[idx : idx+4])
						idx += 4
						remaining := uint64(len(buf) - idx)
						if uint64(attrCount) > remaining/8 || attrCount > maxJOSCollectionItems {
							return nil, fmt.Errorf("jos decoder: invalid UpdateAttributes attrCount %d for remaining %d bytes", attrCount, remaining)
						}
						attrs := make(map[any]any, attrCount)
						for i := uint32(0); i < attrCount; i++ {
							if idx+8 > len(buf) {
								return nil, fmt.Errorf("jos decoder: truncated UpdateAttributes entry %d", i)
							}
							handle := int32(binary.BigEndian.Uint32(buf[idx : idx+4]))
							vLen := binary.BigEndian.Uint32(buf[idx+4 : idx+8])
							idx += 8
							if uint64(vLen) > uint64(len(buf)-idx) {
								return nil, fmt.Errorf("jos decoder: truncated UpdateAttributes value length %d", vLen)
							}
							val := make([]byte, vLen)
							copy(val, buf[idx:idx+int(vLen)])
							attrs[metamodel.NewJavaInteger(handle)] = val
							idx += int(vLen)
						}
						obj.Set("attributes", metamodel.NewJavaHashMap(attrs))
					}
					return obj, nil
				}
			case 255: // SendInteraction
				if len(buf) >= 29 {
					obj := metamodel.NewJavaObject(metamodel.ClassDescSendInteraction)
					interactionId := int32(binary.BigEndian.Uint32(buf[3:7]))
					obj.Set("interactionId", interactionId)
					obj.Set("sourceFederate", int32(binary.BigEndian.Uint32(buf[7:11])))
					obj.Set("targetFederation", int32(binary.BigEndian.Uint32(buf[11:15])))
					obj.Set("timestamp", math.Float64frombits(binary.BigEndian.Uint64(buf[15:23])))
					tagLen := int(binary.BigEndian.Uint16(buf[23:25]))
					idx := 25
					if idx+tagLen <= len(buf) {
						obj.Set("tag", buf[idx:idx+tagLen])
						idx += tagLen
					}
					if idx+4 <= len(buf) {
						obj.Set("regionToken", int32(binary.BigEndian.Uint32(buf[idx:idx+4])))
						idx += 4
					}
					if idx+4 <= len(buf) {
						paramCount := binary.BigEndian.Uint32(buf[idx : idx+4])
						idx += 4
						remaining := uint64(len(buf) - idx)
						if uint64(paramCount) > remaining/8 || paramCount > maxJOSCollectionItems {
							return nil, fmt.Errorf("jos decoder: invalid SendInteraction paramCount %d for remaining %d bytes", paramCount, remaining)
						}
						params := make(map[any]any, paramCount)
						for i := uint32(0); i < paramCount; i++ {
							if idx+8 > len(buf) {
								return nil, fmt.Errorf("jos decoder: truncated SendInteraction entry %d", i)
							}
							handle := int32(binary.BigEndian.Uint32(buf[idx : idx+4]))
							vLen := binary.BigEndian.Uint32(buf[idx+4 : idx+8])
							idx += 8
							if uint64(vLen) > uint64(len(buf)-idx) {
								return nil, fmt.Errorf("jos decoder: truncated SendInteraction value length %d", vLen)
							}
							val := make([]byte, vLen)
							copy(val, buf[idx:idx+int(vLen)])
							params[metamodel.NewJavaInteger(handle)] = val
							idx += int(vLen)
						}
						obj.Set("parameters", metamodel.NewJavaHashMap(params))
					}
					return obj, nil
				}
			}
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
