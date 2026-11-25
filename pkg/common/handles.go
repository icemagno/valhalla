package common

// Tipos de identificadores e handles normativos da IEEE 1516-2025 (HLA-4)

// FederateHandle identifica unicamente um federado dentro de uma federação
type FederateHandle int32

// ObjectClassHandle identifica uma classe de objeto na árvore OMT
type ObjectClassHandle int32

// AttributeHandle identifica um atributo dentro de uma classe de objeto
type AttributeHandle int32

// InteractionClassHandle identifica uma classe de interação na árvore OMT
type InteractionClassHandle int32

// ParameterHandle identifica um parâmetro de uma interação
type ParameterHandle int32

// ObjectInstanceHandle identifica uma instância viva de objeto na simulação
type ObjectInstanceHandle int32

// DimensionHandle identifica uma dimensão de espaço de roteamento DDM
type DimensionHandle int32

// RegionHandle identifica uma região de roteamento DDM
type RegionHandle int32

// Constantes normativas do Standard MIM (IEEE 1516-2025)
const (
	HLAobjectRootHandle              ObjectClassHandle      = 1
	HLAprivilegeToDeleteObjectHandle AttributeHandle        = 1
	HLAinteractionRootHandle         InteractionClassHandle = 1

	HLAobjectRootName              = "HLAobjectRoot"
	HLAprivilegeToDeleteObjectName = "HLAprivilegeToDeleteObject"
	HLAinteractionRootName         = "HLAinteractionRoot"
)

// ProtocolType identifica o dialeto de transporte e serialização usado pelo federado
type ProtocolType uint8

const (
	ProtocolNative  ProtocolType = 1 // Valhalla Nativo (SSZ / Merkle / PQC / VAL4)
	ProtocolPortico ProtocolType = 2 // Portico Legado (JOS / Header 12b / 0xCAFE / 0xBEEF)
)

func (p ProtocolType) String() string {
	switch p {
	case ProtocolNative:
		return "Valhalla-Native(VAL4)"
	case ProtocolPortico:
		return "Portico-Legacy(JOS)"
	default:
		return "Unknown"
	}
}
