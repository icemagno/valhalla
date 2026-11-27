package common

// Layout: [ObjectHandle:4, Offset0:4 (Attributes)] + Heap [Attributes uint32 list]
type DivestOwnershipPayload struct {
	ObjectHandle uint32
	Attributes   []uint32
}

func (p DivestOwnershipPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 8
	attrLen := uint32(len(p.Attributes) * 4)
	totalLen := fixedHeaderSize + attrLen

	buf := make([]byte, totalLen)
	PutUint32(buf[0:4], p.ObjectHandle)
	PutOffset(buf[4:8], fixedHeaderSize)
	EncodeSSZUint32List(buf, fixedHeaderSize, p.Attributes)

	return buf, nil
}

func (p *DivestOwnershipPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 8
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ObjectHandle = Uint32FromBytes(data[0:4])
	off0 := GetOffset(data[4:8])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Attributes, err = DecodeSSZUint32List(data, off0, totalLen)
	return err
}

// AcquireOwnershipPayload carries the object handle and attributes to acquire
// Layout: [ObjectHandle:4, Offset0:4 (Attributes)] + Heap [Attributes uint32 list]
type AcquireOwnershipPayload struct {
	ObjectHandle uint32
	Attributes   []uint32
}

func (p AcquireOwnershipPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 8
	attrLen := uint32(len(p.Attributes) * 4)
	totalLen := fixedHeaderSize + attrLen

	buf := make([]byte, totalLen)
	PutUint32(buf[0:4], p.ObjectHandle)
	PutOffset(buf[4:8], fixedHeaderSize)
	EncodeSSZUint32List(buf, fixedHeaderSize, p.Attributes)

	return buf, nil
}

func (p *AcquireOwnershipPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 8
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ObjectHandle = Uint32FromBytes(data[0:4])
	off0 := GetOffset(data[4:8])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Attributes, err = DecodeSSZUint32List(data, off0, totalLen)
	return err
}

// QueryOwnershipPayload carries the object handle and attribute handle to query
// Layout: [ObjectHandle:4, AttributeHandle:4] (Exact 8 bytes fixed)
type QueryOwnershipPayload struct {
	ObjectHandle    uint32
	AttributeHandle uint32
}

func (p QueryOwnershipPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 8)
	PutUint32(buf[0:4], p.ObjectHandle)
	PutUint32(buf[4:8], p.AttributeHandle)
	return buf, nil
}

func (p *QueryOwnershipPayload) UnmarshalBinary(data []byte) error {
	if len(data) != 8 {
		if len(data) < 8 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
	}
	p.ObjectHandle = Uint32FromBytes(data[0:4])
	p.AttributeHandle = Uint32FromBytes(data[4:8])
	return nil
}

// QueryOwnershipResponsePayload returns ownership status
// Layout: [ObjectHandle:4, AttributeHandle:4, OwnerHandle:4, IsOwned:1] (Exact 13 bytes fixed)
type QueryOwnershipResponsePayload struct {
	ObjectHandle    uint32
	AttributeHandle uint32
	OwnerHandle     uint32
	IsOwned         bool
}

func (p QueryOwnershipResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 13)
	PutUint32(buf[0:4], p.ObjectHandle)
	PutUint32(buf[4:8], p.AttributeHandle)
	PutUint32(buf[8:12], p.OwnerHandle)
	PutBool(buf[12:13], p.IsOwned)
	return buf, nil
}

func (p *QueryOwnershipResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) != 13 {
		if len(data) < 13 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
	}
	p.ObjectHandle = Uint32FromBytes(data[0:4])
	p.AttributeHandle = Uint32FromBytes(data[4:8])
	p.OwnerHandle = Uint32FromBytes(data[8:12])
	var err error
	p.IsOwned, err = DecodeBool(data[12])
	return err
}

// ============================================================================
// 7. DATA DISTRIBUTION MANAGEMENT (DDM) PAYLOADS
// ============================================================================

// CreateRegionPayload carries dimensions for region creation
// Layout: [Offset0:4 (Dimensions)] + Heap [Dimensions uint32 list]
type CreateRegionPayload struct {
	Dimensions []uint32
}

func (p CreateRegionPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 4
	dimLen := uint32(len(p.Dimensions) * 4)
	totalLen := fixedHeaderSize + dimLen

	buf := make([]byte, totalLen)
	PutOffset(buf[0:4], fixedHeaderSize)
	EncodeSSZUint32List(buf, fixedHeaderSize, p.Dimensions)

	return buf, nil
}

func (p *CreateRegionPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 4
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	off0 := GetOffset(data[0:4])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Dimensions, err = DecodeSSZUint32List(data, off0, totalLen)
	return err
}

// CreateRegionResponsePayload returns the assigned region handle
// Layout: [RegionHandle:4] (Exact 4 bytes fixed)
type CreateRegionResponsePayload struct {
	RegionHandle uint32
}

func (p CreateRegionResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	PutUint32(buf[0:4], p.RegionHandle)
	return buf, nil
}

func (p *CreateRegionResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) != 4 {
		if len(data) < 4 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
	}
	p.RegionHandle = Uint32FromBytes(data[0:4])
	return nil
}

// SetRangeBoundsPayload defines discrete grid bounds [lower, upper] for a dimension
// Layout: [RegionHandle:4, DimensionHandle:4, LowerBound:4, UpperBound:4] (Exact 16 bytes fixed)
type SetRangeBoundsPayload struct {
	RegionHandle    uint32
	DimensionHandle uint32
	LowerBound      uint32
	UpperBound      uint32
}

func (p SetRangeBoundsPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 16)
	PutUint32(buf[0:4], p.RegionHandle)
	PutUint32(buf[4:8], p.DimensionHandle)
	PutUint32(buf[8:12], p.LowerBound)
	PutUint32(buf[12:16], p.UpperBound)
	return buf, nil
}

func (p *SetRangeBoundsPayload) UnmarshalBinary(data []byte) error {
	if len(data) != 16 {
		if len(data) < 16 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
	}
	p.RegionHandle = Uint32FromBytes(data[0:4])
	p.DimensionHandle = Uint32FromBytes(data[4:8])
	p.LowerBound = Uint32FromBytes(data[8:12])
	p.UpperBound = Uint32FromBytes(data[12:16])
	return nil
}

// SetMetricBoundsPayload sets continuous ECEF metric bounds for a region
// Layout: [RegionHandle:4, MinX:8, MaxX:8, MinY:8, MaxY:8, MinZ:8, MaxZ:8] (Exact 52 bytes fixed)
type SetMetricBoundsPayload struct {
	RegionHandle uint32
	MinX, MaxX   float64
	MinY, MaxY   float64
	MinZ, MaxZ   float64
}

func (p SetMetricBoundsPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 52)
	PutUint32(buf[0:4], p.RegionHandle)
	PutFloat64(buf[4:12], p.MinX)
	PutFloat64(buf[12:20], p.MaxX)
	PutFloat64(buf[20:28], p.MinY)
	PutFloat64(buf[28:36], p.MaxY)
	PutFloat64(buf[36:44], p.MinZ)
	PutFloat64(buf[44:52], p.MaxZ)
	return buf, nil
}

func (p *SetMetricBoundsPayload) UnmarshalBinary(data []byte) error {
	if len(data) != 52 {
		if len(data) < 52 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
	}
	p.RegionHandle = Uint32FromBytes(data[0:4])
	p.MinX = Float64FromBytes(data[4:12])
	p.MaxX = Float64FromBytes(data[12:20])
	p.MinY = Float64FromBytes(data[20:28])
	p.MaxY = Float64FromBytes(data[28:36])
	p.MinZ = Float64FromBytes(data[36:44])
	p.MaxZ = Float64FromBytes(data[44:52])
	return nil
}

// SubscribeWithRegionPayload associates a subscription with a spatial region
// Layout: [ObjectClass:4, RegionHandle:4, Offset0:4 (Attributes)] + Heap [Attributes uint32 list]
type SubscribeWithRegionPayload struct {
	ObjectClass  uint32
	Attributes   []uint32
	RegionHandle uint32
}

func (p SubscribeWithRegionPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 12
	attrLen := uint32(len(p.Attributes) * 4)
	totalLen := fixedHeaderSize + attrLen

	buf := make([]byte, totalLen)
	PutUint32(buf[0:4], p.ObjectClass)
	PutUint32(buf[4:8], p.RegionHandle)
	PutOffset(buf[8:12], fixedHeaderSize)
	EncodeSSZUint32List(buf, fixedHeaderSize, p.Attributes)

	return buf, nil
}

func (p *SubscribeWithRegionPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 12
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ObjectClass = Uint32FromBytes(data[0:4])
	p.RegionHandle = Uint32FromBytes(data[4:8])
	off0 := GetOffset(data[8:12])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Attributes, err = DecodeSSZUint32List(data, off0, totalLen)
	return err
}

// UpdateAttributesSpatialPayload transmits attribute updates tagged with metric ECEF coordinates
// Layout: [ObjectHandle:4, X:8, Y:8, Z:8, Offset0:4 (Tag), Offset1:4 (Attributes)]
// Dynamic part: Tag bytes + Canonical SSZ List[Container] of Attributes
type UpdateAttributesSpatialPayload struct {
	ObjectHandle ObjectInstanceHandle
	X, Y, Z      float64
	Attributes   map[AttributeHandle][]byte
	Tag          []byte
}

func (p UpdateAttributesSpatialPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 36 // 4 + 8 + 8 + 8 + 4 + 4
	tagLen := uint32(len(p.Tag))
	attrListSize := SizeSSZAttributeList(p.Attributes)

	totalLen := fixedHeaderSize + tagLen + attrListSize
	buf := make([]byte, totalLen)

	PutUint32(buf[0:4], uint32(p.ObjectHandle))
	PutFloat64(buf[4:12], p.X)
	PutFloat64(buf[12:20], p.Y)
	PutFloat64(buf[20:28], p.Z)

	off0 := uint32(fixedHeaderSize)
	off1 := off0 + tagLen

	PutOffset(buf[28:32], off0)
	PutOffset(buf[32:36], off1)

	EncodeSSZBytes(buf, off0, p.Tag)
	EncodeSSZAttributeList(buf, off1, p.Attributes)

	return buf, nil
}

func (p *UpdateAttributesSpatialPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 36
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ObjectHandle = ObjectInstanceHandle(Uint32FromBytes(data[0:4]))
	p.X = Float64FromBytes(data[4:12])
	p.Y = Float64FromBytes(data[12:20])
	p.Z = Float64FromBytes(data[20:28])

	off0 := GetOffset(data[28:32])
	off1 := GetOffset(data[32:36])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0, off1}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Tag, err = DecodeSSZBytes(data, off0, off1)
	if err != nil {
		return err
	}

	p.Attributes, err = DecodeSSZAttributeList(data, off1, totalLen)
	if err != nil {
		return err
	}

	return nil
}

// ============================================================================
// 8. OMT / FOM HANDLE RESOLUTION PAYLOADS
// ============================================================================

// GetObjectClassHandlePayload carries the class name to resolve
// Layout: [Offset0:4 (ClassName)] + Heap [ClassName]
type GetObjectClassHandlePayload struct {
	ClassName string
}

func (p GetObjectClassHandlePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 4
	nameLen := uint32(len(p.ClassName))
	totalLen := fixedHeaderSize + nameLen

	buf := make([]byte, totalLen)
	PutOffset(buf[0:4], fixedHeaderSize)
	EncodeSSZString(buf, fixedHeaderSize, p.ClassName)

	return buf, nil
}

func (p *GetObjectClassHandlePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 4
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	off0 := GetOffset(data[0:4])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.ClassName, err = DecodeSSZString(data, off0, totalLen)
	return err
}

// HandleResponsePayload returns an int32 handle
// Layout: [Handle:4] (Exact 4 bytes fixed)
type HandleResponsePayload struct {
	Handle int32
}

func (p HandleResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	PutInt32(buf[0:4], p.Handle)
	return buf, nil
}

func (p *HandleResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) != 4 {
		if len(data) < 4 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
	}
	p.Handle = Int32FromBytes(data[0:4])
	return nil
}

// GetAttributeHandlePayload carries class handle and attribute name to resolve
// Layout: [ClassHandle:4, Offset0:4 (AttributeName)] + Heap [AttributeName]
type GetAttributeHandlePayload struct {
	ClassHandle   int32
	AttributeName string
}

func (p GetAttributeHandlePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 8
	nameLen := uint32(len(p.AttributeName))
	totalLen := fixedHeaderSize + nameLen

	buf := make([]byte, totalLen)
	PutInt32(buf[0:4], p.ClassHandle)
	PutOffset(buf[4:8], fixedHeaderSize)
	EncodeSSZString(buf, fixedHeaderSize, p.AttributeName)

	return buf, nil
}

func (p *GetAttributeHandlePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 8
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ClassHandle = Int32FromBytes(data[0:4])
	off0 := GetOffset(data[4:8])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.AttributeName, err = DecodeSSZString(data, off0, totalLen)
	return err
}

// GetDimensionHandlePayload carries dimension name to resolve
// Layout: [Offset0:4 (DimensionName)] + Heap [DimensionName]
type GetDimensionHandlePayload struct {
	DimensionName string
}

func (p GetDimensionHandlePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 4
	nameLen := uint32(len(p.DimensionName))
	totalLen := fixedHeaderSize + nameLen

	buf := make([]byte, totalLen)
	PutOffset(buf[0:4], fixedHeaderSize)
	EncodeSSZString(buf, fixedHeaderSize, p.DimensionName)

	return buf, nil
}

func (p *GetDimensionHandlePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 4
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	off0 := GetOffset(data[0:4])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.DimensionName, err = DecodeSSZString(data, off0, totalLen)
	return err
}

// GetInteractionClassHandlePayload carries interaction name to resolve
// Layout: [Offset0:4 (InteractionName)] + Heap [InteractionName]
type GetInteractionClassHandlePayload struct {
	InteractionName string
}

func (p GetInteractionClassHandlePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 4
	nameLen := uint32(len(p.InteractionName))
	totalLen := fixedHeaderSize + nameLen

	buf := make([]byte, totalLen)
	PutOffset(buf[0:4], fixedHeaderSize)
	EncodeSSZString(buf, fixedHeaderSize, p.InteractionName)

	return buf, nil
}

func (p *GetInteractionClassHandlePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 4
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	off0 := GetOffset(data[0:4])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.InteractionName, err = DecodeSSZString(data, off0, totalLen)
	return err
}

// GetParameterHandlePayload carries interaction handle and parameter name to resolve
// Layout: [InteractionHandle:4, Offset0:4 (ParameterName)] + Heap [ParameterName]
type GetParameterHandlePayload struct {
	InteractionHandle int32
	ParameterName     string
}

func (p GetParameterHandlePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 8
	nameLen := uint32(len(p.ParameterName))
	totalLen := fixedHeaderSize + nameLen

	buf := make([]byte, totalLen)
	PutInt32(buf[0:4], p.InteractionHandle)
	PutOffset(buf[4:8], fixedHeaderSize)
	EncodeSSZString(buf, fixedHeaderSize, p.ParameterName)

	return buf, nil
}

func (p *GetParameterHandlePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 8
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.InteractionHandle = Int32FromBytes(data[0:4])
	off0 := GetOffset(data[4:8])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.ParameterName, err = DecodeSSZString(data, off0, totalLen)
	return err
}

// GetMerkleProofPayload requests a cryptographic Merkle inclusion proof for an entity
// Fixed layout: [ObjectHandle:4] = 4 bytes
type GetMerkleProofPayload struct {
	ObjectHandle ObjectInstanceHandle
}

func (p GetMerkleProofPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	PutUint32(buf[0:4], uint32(p.ObjectHandle))
	return buf, nil
}

func (p *GetMerkleProofPayload) UnmarshalBinary(data []byte) error {
	if len(data) != 4 {
		if len(data) < 4 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
	}
	p.ObjectHandle = ObjectInstanceHandle(Uint32FromBytes(data[0:4]))
	return nil
}

// MerkleProofResponsePayload carries cryptographic Merkle inclusion proof
// Layout: [ObjectHandle:4, LeafIndex:8, GlobalRoot:32, LeafHash:32, Offset0:4 (AuditPath)] = 80 bytes
// Dynamic part: AuditPath as contiguous 32-byte chunk hashes
type MerkleProofResponsePayload struct {
	ObjectHandle ObjectInstanceHandle
	LeafIndex    uint64
	GlobalRoot   [32]byte
	LeafHash     [32]byte
	AuditPath    [][32]byte
}

func (p MerkleProofResponsePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 80
	auditPathLen := uint32(len(p.AuditPath) * 32)
	totalLen := fixedHeaderSize + auditPathLen

	buf := make([]byte, totalLen)
	PutUint32(buf[0:4], uint32(p.ObjectHandle))
	PutUint64(buf[4:12], p.LeafIndex)
	copy(buf[12:44], p.GlobalRoot[:])
	copy(buf[44:76], p.LeafHash[:])
	PutOffset(buf[76:80], fixedHeaderSize)

	offset := fixedHeaderSize
	for _, chunk := range p.AuditPath {
		copy(buf[offset:offset+32], chunk[:])
		offset += 32
	}

	return buf, nil
}

func (p *MerkleProofResponsePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 80
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ObjectHandle = ObjectInstanceHandle(Uint32FromBytes(data[0:4]))
	p.LeafIndex = Uint64FromBytes(data[4:12])
	copy(p.GlobalRoot[:], data[12:44])
	copy(p.LeafHash[:], data[44:76])

	off0 := GetOffset(data[76:80])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	auditPathBytes := totalLen - off0
	if auditPathBytes%32 != 0 {
		return ErrSSZListAlignment
	}
	chunkCount := auditPathBytes / 32
	if chunkCount > 64 {
		return ErrSSZDataOutOfBounds
	}
	p.AuditPath = make([][32]byte, chunkCount)
	idx := off0
	for i := uint32(0); i < chunkCount; i++ {
		copy(p.AuditPath[i][:], data[idx:idx+32])
		idx += 32
	}

	return nil
}
