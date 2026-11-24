package wire

import "fmt"

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

// ClassDesc representa a árvore de metadados de uma classe serializável Java
type ClassDesc struct {
	Name             string
	SerialVersionUID uint64
	Flags            byte
	Fields           []FieldDesc
	Super            *ClassDesc

	// CustomWriter permite que classes com writeObject/writeExternal injetem seus blocos de dados
	CustomWriter func(enc *JOSEncoder, obj *JavaObject)
}

// JavaObject representa uma instância genérica de um objeto Java serializável
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

// Set atribui valor a um campo com validação fluente
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
