package common

// Normative IEEE 1516-2025 (HLA-4) Handle and Identifier Types

// FederateHandle uniquely identifies a federate within a federation execution
type FederateHandle int32

// ObjectClassHandle identifies an object class in the OMT hierarchy
type ObjectClassHandle int32

// AttributeHandle identifies an attribute within an object class
type AttributeHandle int32

// InteractionClassHandle identifies an interaction class in the OMT hierarchy
type InteractionClassHandle int32

// ParameterHandle identifies a parameter of an interaction
type ParameterHandle int32

// ObjectInstanceHandle uniquely identifies a live object instance in the federation
type ObjectInstanceHandle int32

// DimensionHandle identifies a routing space dimension for DDM
type DimensionHandle int32

// RegionHandle identifies a routing region for DDM
type RegionHandle int32

// Standard MIM Normative Constants (IEEE 1516-2025)
const (
	HLAobjectRootHandle              ObjectClassHandle      = 1
	HLAprivilegeToDeleteObjectHandle AttributeHandle        = 1
	HLAinteractionRootHandle         InteractionClassHandle = 1

	HLAobjectRootName              = "HLAobjectRoot"
	HLAprivilegeToDeleteObjectName = "HLAprivilegeToDeleteObject"
	HLAinteractionRootName         = "HLAinteractionRoot"
)

// ProtocolType identifies the wire protocol dialect used by the federate connection
type ProtocolType uint8

const (
	ProtocolUnknown ProtocolType = 0
	ProtocolNative  ProtocolType = 1 // Valhalla Native (SSZ / Merkle / VAL4)
	ProtocolPortico ProtocolType = 2 // Portico Legacy (JOS / Header 12b / 0xCAFE / 0xBEEF)
	ProtocolWeb     ProtocolType = 3 // HLA-4 Web Profile (IEEE 1516-2025 WebSockets / JSON)
)

func (p ProtocolType) String() string {
	switch p {
	case ProtocolNative:
		return "Valhalla-Native(VAL4)"
	case ProtocolPortico:
		return "Portico-Legacy(JOS)"
	case ProtocolWeb:
		return "HLA4-WebProfile(JSON-WS)"
	default:
		return "Unknown"
	}
}
