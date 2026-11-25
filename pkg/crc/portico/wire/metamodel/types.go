package metamodel

import "fmt"

// Constantes canônicas da especificação Java Object Serialization (JOS)
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
	TC_BLOCKDATALONG byte = 0x7A
	TC_EXCEPTION    byte = 0x7B
	TC_LONGSTRING   byte = 0x7C
	TC_PROXYCLASSDESC byte = 0x7D
	TC_ENUM         byte = 0x7E

	SC_WRITE_METHOD   byte = 0x01
	SC_SERIALIZABLE   byte = 0x02
	SC_EXTERNALIZABLE byte = 0x04
	SC_BLOCK_DATA     byte = 0x08
	SC_ENUM           byte = 0x10

	BaseWireHandle uint32 = 0x007E0000
)

// FieldType representa o código de tipo primitivo ou objeto da especificação JOS
type FieldType byte

const (
	TypeBoolean FieldType = 'Z' // 0x5A
	TypeByte    FieldType = 'B' // 0x42
	TypeChar    FieldType = 'C' // 0x43
	TypeShort   FieldType = 'S' // 0x53
	TypeInt     FieldType = 'I' // 0x49
	TypeLong    FieldType = 'J' // 0x4A
	TypeFloat   FieldType = 'F' // 0x46
	TypeDouble  FieldType = 'D' // 0x44
	TypeObject  FieldType = 'L' // 0x4C
	TypeArray   FieldType = '[' // 0x5B
)

func (t FieldType) String() string {
	return string([]byte{byte(t)})
}

// FieldDesc descreve os metadados de um campo na declaração de classe Java
type FieldDesc struct {
	Name      string
	Type      FieldType
	ClassName string // Usado quando Type == TypeObject ou TypeArray (ex: "Ljava/lang/String;")
}

// JOSWriter define a interface de escrita para métodos customizados (writeObject / writeExternal)
type JOSWriter interface {
	WriteByte(b byte) error
	WriteInt16(v int16)
	WriteInt32(v int32)
	WriteUint16(v uint16)
	WriteUint64(v uint64)
	WriteDouble(v float64)
	WriteBlockData(data []byte)
	WriteObject(obj any) error
	WriteString(s string)
}

// JOSReader define a interface de leitura para métodos customizados (readObject / readExternal)
type JOSReader interface {
	ReadByte() (byte, error)
	ReadInt16() (int16, error)
	ReadInt32() (int32, error)
	ReadUint16() (uint16, error)
	ReadUint64() (uint64, error)
	ReadDouble() (float64, error)
	ReadBlockData() ([]byte, error)
	ReadObject() (any, error)
	ReadString() (string, error)
}

// ClassDesc representa a árvore de metadados de uma classe serializável Java
type ClassDesc struct {
	Name             string
	SerialVersionUID uint64
	Flags            byte
	Fields           []FieldDesc
	Super            *ClassDesc
	CustomOnly       bool // Se true, o writeObject da classe não executa defaultWriteObject (ex: PorticoMessage, ResponseMessage)

	// CustomWriter permite que classes com writeObject/writeExternal injetem seus blocos de dados
	CustomWriter func(w JOSWriter, obj *JavaObject) error

	// CustomReader permite que classes com readObject/readExternal consumam seus blocos de dados
	CustomReader func(r JOSReader, obj *JavaObject) error
}

// JavaObject representa uma instância viva de um objeto Java serializável
type JavaObject struct {
	Class  *ClassDesc
	Fields map[string]any
}

// NewJavaObject instancia um novo objeto genérico com mapa de campos inicializado
func NewJavaObject(class *ClassDesc) *JavaObject {
	return &JavaObject{
		Class:  class,
		Fields: make(map[string]any),
	}
}

// Set atribui valor a um campo com encadeamento fluente
func (o *JavaObject) Set(name string, value any) *JavaObject {
	o.Fields[name] = value
	return o
}

// Get obtém o valor de um campo
func (o *JavaObject) Get(name string) any {
	return o.Fields[name]
}

func (o *JavaObject) String() string {
	if o.Class == nil {
		return "JavaObject{Class: <nil>}"
	}
	return fmt.Sprintf("JavaObject{Class: %s, Fields: %v}", o.Class.Name, o.Fields)
}

// JavaEnum representa uma constante de enum Java serializável
type JavaEnum struct {
	Class *ClassDesc
	Name  string
}

// NewJavaEnum cria uma nova referência a constante enum
func NewJavaEnum(class *ClassDesc, name string) *JavaEnum {
	return &JavaEnum{
		Class: class,
		Name:  name,
	}
}

func (e *JavaEnum) String() string {
	if e.Class == nil {
		return fmt.Sprintf("JavaEnum{Class: <nil>, Name: %s}", e.Name)
	}
	return fmt.Sprintf("JavaEnum{Class: %s, Name: %s}", e.Class.Name, e.Name)
}
