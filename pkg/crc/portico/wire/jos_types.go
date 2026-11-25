package wire

import "github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"

// Reexportação dos tipos fundamentais do metamodelo para compatibilidade
type (
	FieldType  = metamodel.FieldType
	FieldDesc  = metamodel.FieldDesc
	ClassDesc  = metamodel.ClassDesc
	JavaObject = metamodel.JavaObject
	JavaEnum   = metamodel.JavaEnum
)

// Constantes do Java Object Serialization (JOS)
const (
	TypeBoolean = metamodel.TypeBoolean
	TypeByte    = metamodel.TypeByte
	TypeChar    = metamodel.TypeChar
	TypeShort   = metamodel.TypeShort
	TypeInt     = metamodel.TypeInt
	TypeLong    = metamodel.TypeLong
	TypeFloat   = metamodel.TypeFloat
	TypeDouble  = metamodel.TypeDouble
	TypeObject  = metamodel.TypeObject
	TypeArray   = metamodel.TypeArray

	TC_NULL         = metamodel.TC_NULL
	TC_REFERENCE    = metamodel.TC_REFERENCE
	TC_CLASSDESC    = metamodel.TC_CLASSDESC
	TC_OBJECT       = metamodel.TC_OBJECT
	TC_STRING       = metamodel.TC_STRING
	TC_ARRAY        = metamodel.TC_ARRAY
	TC_CLASS        = metamodel.TC_CLASS
	TC_BLOCKDATA    = metamodel.TC_BLOCKDATA
	TC_ENDBLOCKDATA = metamodel.TC_ENDBLOCKDATA
	TC_RESET        = metamodel.TC_RESET
	TC_ENUM         = metamodel.TC_ENUM

	SC_WRITE_METHOD   = metamodel.SC_WRITE_METHOD
	SC_SERIALIZABLE   = metamodel.SC_SERIALIZABLE
	SC_EXTERNALIZABLE = metamodel.SC_EXTERNALIZABLE
	SC_BLOCK_DATA     = metamodel.SC_BLOCK_DATA
	SC_ENUM           = metamodel.SC_ENUM

	BaseWireHandle = metamodel.BaseWireHandle
)

// Funções de instanciação delegadas para o pacote metamodel
var (
	NewJavaObject     = metamodel.NewJavaObject
	NewJavaEnum       = metamodel.NewJavaEnum
	RegisterClassDesc = metamodel.RegisterClassDesc
	LookupClassDesc   = metamodel.LookupClassDesc
)
