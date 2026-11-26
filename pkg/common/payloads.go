// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)
//
// Canonical Simple Serialize (SSZ) Payloads for Valhalla Native Wire Protocol (VAL4).
// Conforms strictly to the Ethereum 2.0 Consensus Layer Specification (SSZ):
// - All primitive integers and floats encoded in Little-Endian.
// - Fixed-size fields located in the container's static header.
// - Dynamic fields (strings, byte slices, lists) indexed by 4-byte uint32 offsets.
// - Strict monotonic offset validation and anti-overlap boundary checking.

package common

// ============================================================================
// 1. FEDERATION MANAGEMENT PAYLOADS
// ============================================================================

// FederationExecutionPayload carries the federation name and optional FOM modules
// Layout: [Offset0:4 (Name), Offset1:4 (FOMModules)] + Heap [Name bytes, StringList bytes]
type FederationExecutionPayload struct {
	FederationName string
	FOMModules     []string
}

func (p FederationExecutionPayload) MarshalBinary() ([]byte, error) {
	nameLen := uint32(len(p.FederationName))
	fomSize := SizeSSZStringList(p.FOMModules)
	const fixedHeaderSize = 8

	totalLen := fixedHeaderSize + nameLen + fomSize
	buf := make([]byte, totalLen)

	off0 := uint32(fixedHeaderSize)
	off1 := off0 + nameLen

	PutOffset(buf[0:4], off0)
	PutOffset(buf[4:8], off1)

	EncodeSSZString(buf, off0, p.FederationName)
	if fomSize > 0 {
		EncodeSSZStringList(buf, off1, p.FOMModules)
	}

	return buf, nil
}

func (p *FederationExecutionPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 8
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	off0 := GetOffset(data[0:4])
	off1 := GetOffset(data[4:8])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0, off1}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.FederationName, err = DecodeSSZString(data, off0, off1)
	if err != nil {
		return err
	}

	p.FOMModules, err = DecodeSSZStringList(data, off1, totalLen)
	if err != nil {
		return err
	}

	return nil
}

// JoinFederationPayload represents a federate join request
// Layout: [Offset0:4 (FedName), Offset1:4 (FedType), Offset2:4 (FederationName)] + Heap strings
type JoinFederationPayload struct {
	FederateName   string
	FederateType   string
	FederationName string
}

func (p JoinFederationPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 12
	nameLen := uint32(len(p.FederateName))
	typeLen := uint32(len(p.FederateType))
	fedLen := uint32(len(p.FederationName))

	totalLen := fixedHeaderSize + nameLen + typeLen + fedLen
	buf := make([]byte, totalLen)

	off0 := uint32(fixedHeaderSize)
	off1 := off0 + nameLen
	off2 := off1 + typeLen

	PutOffset(buf[0:4], off0)
	PutOffset(buf[4:8], off1)
	PutOffset(buf[8:12], off2)

	EncodeSSZString(buf, off0, p.FederateName)
	EncodeSSZString(buf, off1, p.FederateType)
	EncodeSSZString(buf, off2, p.FederationName)

	return buf, nil
}

func (p *JoinFederationPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 12
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	off0 := GetOffset(data[0:4])
	off1 := GetOffset(data[4:8])
	off2 := GetOffset(data[8:12])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0, off1, off2}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.FederateName, err = DecodeSSZString(data, off0, off1)
	if err != nil {
		return err
	}
	p.FederateType, err = DecodeSSZString(data, off1, off2)
	if err != nil {
		return err
	}
	p.FederationName, err = DecodeSSZString(data, off2, totalLen)
	if err != nil {
		return err
	}

	return nil
}

// JoinFederationResponsePayload contains granted federate and federation identifiers
// Layout: [FederateHandle:4, FederationHandle:4] (All Fixed)
type JoinFederationResponsePayload struct {
	FederateHandle   FederateHandle
	FederationHandle int32
}

func (p JoinFederationResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 8)
	PutUint32(buf[0:4], uint32(p.FederateHandle))
	PutInt32(buf[4:8], p.FederationHandle)
	return buf, nil
}

func (p *JoinFederationResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 8 {
		return ErrSSZBufferTooShort
	}
	p.FederateHandle = FederateHandle(Uint32FromBytes(data[0:4]))
	p.FederationHandle = Int32FromBytes(data[4:8])
	return nil
}

// ResignFederationPayload represents federate resignation
// Layout: [ResignAction:1]
type ResignFederationPayload struct {
	ResignAction uint8
}

func (p ResignFederationPayload) MarshalBinary() ([]byte, error) {
	return []byte{p.ResignAction}, nil
}

func (p *ResignFederationPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 1 {
		return ErrSSZBufferTooShort
	}
	p.ResignAction = data[0]
	return nil
}

// RegisterSyncPointPayload represents synchronization point registration
// Layout: [Offset0:4 (Label), Offset1:4 (Tag)] + Heap [Label, Tag]
type RegisterSyncPointPayload struct {
	Label string
	Tag   []byte
}

func (p RegisterSyncPointPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 8
	labelLen := uint32(len(p.Label))
	tagLen := uint32(len(p.Tag))

	totalLen := fixedHeaderSize + labelLen + tagLen
	buf := make([]byte, totalLen)

	off0 := uint32(fixedHeaderSize)
	off1 := off0 + labelLen

	PutOffset(buf[0:4], off0)
	PutOffset(buf[4:8], off1)

	EncodeSSZString(buf, off0, p.Label)
	EncodeSSZBytes(buf, off1, p.Tag)

	return buf, nil
}

func (p *RegisterSyncPointPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 8
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	off0 := GetOffset(data[0:4])
	off1 := GetOffset(data[4:8])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0, off1}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Label, err = DecodeSSZString(data, off0, off1)
	if err != nil {
		return err
	}
	p.Tag, err = DecodeSSZBytes(data, off1, totalLen)
	if err != nil {
		return err
	}

	return nil
}

// SyncPointLabelPayload represents messages carrying only a label string
// Layout: [Offset0:4 (Label)] + Heap [Label]
type SyncPointLabelPayload struct {
	Label string
}

func (p SyncPointLabelPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 4
	labelLen := uint32(len(p.Label))
	totalLen := fixedHeaderSize + labelLen

	buf := make([]byte, totalLen)
	PutOffset(buf[0:4], fixedHeaderSize)
	EncodeSSZString(buf, fixedHeaderSize, p.Label)
	return buf, nil
}

func (p *SyncPointLabelPayload) UnmarshalBinary(data []byte) error {
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
	p.Label, err = DecodeSSZString(data, off0, totalLen)
	return err
}

// ============================================================================
// 2. DECLARATION MANAGEMENT (PUB/SUB) PAYLOADS
// ============================================================================

// PublishObjectClassPayload publishes attributes of an object class
// Layout: [ClassHandle:4, Offset0:4 (AttributeHandles)] + Heap [AttributeHandles uint32 list]
type PublishObjectClassPayload struct {
	ClassHandle      ObjectClassHandle
	AttributeHandles []AttributeHandle
}

func (p PublishObjectClassPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 8
	attrListSize := uint32(len(p.AttributeHandles) * 4)
	totalLen := fixedHeaderSize + attrListSize

	buf := make([]byte, totalLen)
	PutUint32(buf[0:4], uint32(p.ClassHandle))
	PutOffset(buf[4:8], fixedHeaderSize)

	for i, h := range p.AttributeHandles {
		PutUint32(buf[fixedHeaderSize+uint32(i*4):fixedHeaderSize+uint32(i*4)+4], uint32(h))
	}

	return buf, nil
}

func (p *PublishObjectClassPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 8
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ClassHandle = ObjectClassHandle(Uint32FromBytes(data[0:4]))
	off0 := GetOffset(data[4:8])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	rawList, err := DecodeSSZUint32List(data, off0, totalLen)
	if err != nil {
		return err
	}

	p.AttributeHandles = make([]AttributeHandle, len(rawList))
	for i, v := range rawList {
		p.AttributeHandles[i] = AttributeHandle(v)
	}

	return nil
}

// SubscribeObjectClassPayload subscribes to attributes of an object class
// Layout: [ClassHandle:4, Active:1, Offset0:4 (AttributeHandles)] + Heap [AttributeHandles uint32 list]
type SubscribeObjectClassPayload struct {
	ClassHandle      ObjectClassHandle
	AttributeHandles []AttributeHandle
	Active           bool
}

func (p SubscribeObjectClassPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 9
	attrListSize := uint32(len(p.AttributeHandles) * 4)
	totalLen := fixedHeaderSize + attrListSize

	buf := make([]byte, totalLen)
	PutUint32(buf[0:4], uint32(p.ClassHandle))
	if p.Active {
		buf[4] = 1
	} else {
		buf[4] = 0
	}
	PutOffset(buf[5:9], fixedHeaderSize)

	for i, h := range p.AttributeHandles {
		PutUint32(buf[fixedHeaderSize+uint32(i*4):fixedHeaderSize+uint32(i*4)+4], uint32(h))
	}

	return buf, nil
}

func (p *SubscribeObjectClassPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 9
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ClassHandle = ObjectClassHandle(Uint32FromBytes(data[0:4]))
	p.Active = data[4] == 1
	off0 := GetOffset(data[5:9])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	rawList, err := DecodeSSZUint32List(data, off0, totalLen)
	if err != nil {
		return err
	}

	p.AttributeHandles = make([]AttributeHandle, len(rawList))
	for i, v := range rawList {
		p.AttributeHandles[i] = AttributeHandle(v)
	}

	return nil
}

// InteractionClassPayload publishes or unpublishes an interaction class
// Layout: [ClassHandle:4]
type InteractionClassPayload struct {
	ClassHandle InteractionClassHandle
}

func (p InteractionClassPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	PutUint32(buf[0:4], uint32(p.ClassHandle))
	return buf, nil
}

func (p *InteractionClassPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return ErrSSZBufferTooShort
	}
	p.ClassHandle = InteractionClassHandle(Uint32FromBytes(data[0:4]))
	return nil
}

// SubscribeInteractionClassPayload subscribes or unsubscribes to an interaction class
// Layout: [ClassHandle:4, Active:1]
type SubscribeInteractionClassPayload struct {
	ClassHandle InteractionClassHandle
	Active      bool
}

func (p SubscribeInteractionClassPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 5)
	PutUint32(buf[0:4], uint32(p.ClassHandle))
	if p.Active {
		buf[4] = 1
	} else {
		buf[4] = 0
	}
	return buf, nil
}

func (p *SubscribeInteractionClassPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 5 {
		return ErrSSZBufferTooShort
	}
	p.ClassHandle = InteractionClassHandle(Uint32FromBytes(data[0:4]))
	p.Active = data[4] == 1
	return nil
}

// ============================================================================
// 3. OBJECT MANAGEMENT PAYLOADS
// ============================================================================

// RegisterObjectInstancePayload registers an object instance
// Layout: [ClassHandle:4, Offset0:4 (ObjectName)] + Heap [ObjectName]
type RegisterObjectInstancePayload struct {
	ClassHandle ObjectClassHandle
	ObjectName  string
}

func (p RegisterObjectInstancePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 8
	nameLen := uint32(len(p.ObjectName))
	totalLen := fixedHeaderSize + nameLen

	buf := make([]byte, totalLen)
	PutUint32(buf[0:4], uint32(p.ClassHandle))
	PutOffset(buf[4:8], fixedHeaderSize)
	EncodeSSZString(buf, fixedHeaderSize, p.ObjectName)

	return buf, nil
}

func (p *RegisterObjectInstancePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 8
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ClassHandle = ObjectClassHandle(Uint32FromBytes(data[0:4]))
	off0 := GetOffset(data[4:8])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.ObjectName, err = DecodeSSZString(data, off0, totalLen)
	return err
}

// DiscoverObjectInstancePayload conveys a discovered remote object instance
// Layout: [ObjectHandle:4, ClassHandle:4, Offset0:4 (ObjectName)] + Heap [ObjectName]
type DiscoverObjectInstancePayload struct {
	ObjectHandle ObjectInstanceHandle
	ClassHandle  ObjectClassHandle
	ObjectName   string
}

func (p DiscoverObjectInstancePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 12
	nameLen := uint32(len(p.ObjectName))
	totalLen := fixedHeaderSize + nameLen

	buf := make([]byte, totalLen)
	PutUint32(buf[0:4], uint32(p.ObjectHandle))
	PutUint32(buf[4:8], uint32(p.ClassHandle))
	PutOffset(buf[8:12], fixedHeaderSize)
	EncodeSSZString(buf, fixedHeaderSize, p.ObjectName)

	return buf, nil
}

func (p *DiscoverObjectInstancePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 12
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ObjectHandle = ObjectInstanceHandle(Uint32FromBytes(data[0:4]))
	p.ClassHandle = ObjectClassHandle(Uint32FromBytes(data[4:8]))
	off0 := GetOffset(data[8:12])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.ObjectName, err = DecodeSSZString(data, off0, totalLen)
	return err
}

// DeleteObjectInstancePayload represents an object instance deletion request
// Layout: [ObjectHandle:4, Offset0:4 (Tag)] + Heap [Tag]
type DeleteObjectInstancePayload struct {
	ObjectHandle ObjectInstanceHandle
	Tag          []byte
}

func (p DeleteObjectInstancePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 8
	tagLen := uint32(len(p.Tag))
	totalLen := fixedHeaderSize + tagLen

	buf := make([]byte, totalLen)
	PutUint32(buf[0:4], uint32(p.ObjectHandle))
	PutOffset(buf[4:8], fixedHeaderSize)
	EncodeSSZBytes(buf, fixedHeaderSize, p.Tag)

	return buf, nil
}

func (p *DeleteObjectInstancePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 8
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ObjectHandle = ObjectInstanceHandle(Uint32FromBytes(data[0:4]))
	off0 := GetOffset(data[4:8])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Tag, err = DecodeSSZBytes(data, off0, totalLen)
	return err
}

// RemoveObjectInstancePayload notifies federates of object instance removal
// Layout: [ObjectHandle:4, Offset0:4 (Tag)] + Heap [Tag]
type RemoveObjectInstancePayload struct {
	ObjectHandle ObjectInstanceHandle
	Tag          []byte
}

func (p RemoveObjectInstancePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 8
	tagLen := uint32(len(p.Tag))
	totalLen := fixedHeaderSize + tagLen

	buf := make([]byte, totalLen)
	PutUint32(buf[0:4], uint32(p.ObjectHandle))
	PutOffset(buf[4:8], fixedHeaderSize)
	EncodeSSZBytes(buf, fixedHeaderSize, p.Tag)

	return buf, nil
}

func (p *RemoveObjectInstancePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 8
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ObjectHandle = ObjectInstanceHandle(Uint32FromBytes(data[0:4]))
	off0 := GetOffset(data[4:8])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Tag, err = DecodeSSZBytes(data, off0, totalLen)
	return err
}

// UpdateAttributesPayload carries attribute updates (UAV) or reflections (RAV)
// Layout: [ObjectHandle:4, HasTime:1, Time:8, Offset0:4 (Tag), Offset1:4 (Attributes)]
// Heap: Tag bytes + Attributes table [Count:4 + (Handle:4, Len:4, Data:N)...]
type UpdateAttributesPayload struct {
	ObjectHandle ObjectInstanceHandle
	Attributes   map[AttributeHandle][]byte
	Tag          []byte
	HasTime      bool
	Time         float64
}

func (p UpdateAttributesPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 21 // 4 + 1 + 8 + 4 + 4
	tagLen := uint32(len(p.Tag))

	attrDataLen := uint32(4) // count
	for _, val := range p.Attributes {
		attrDataLen += 8 + uint32(len(val)) // handle(4) + len(4) + data
	}

	totalLen := fixedHeaderSize + tagLen + attrDataLen
	buf := make([]byte, totalLen)

	PutUint32(buf[0:4], uint32(p.ObjectHandle))
	if p.HasTime {
		buf[4] = 1
		PutFloat64(buf[5:13], p.Time)
	} else {
		buf[4] = 0
		PutFloat64(buf[5:13], 0)
	}

	off0 := uint32(fixedHeaderSize)
	off1 := off0 + tagLen

	PutOffset(buf[13:17], off0)
	PutOffset(buf[17:21], off1)

	EncodeSSZBytes(buf, off0, p.Tag)

	// Serialize attribute table
	PutUint32(buf[off1:off1+4], uint32(len(p.Attributes)))
	curOffset := off1 + 4
	for attr, val := range p.Attributes {
		PutUint32(buf[curOffset:curOffset+4], uint32(attr))
		PutUint32(buf[curOffset+4:curOffset+8], uint32(len(val)))
		curOffset += 8
		copy(buf[curOffset:], val)
		curOffset += uint32(len(val))
	}

	return buf, nil
}

func (p *UpdateAttributesPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 21
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ObjectHandle = ObjectInstanceHandle(Uint32FromBytes(data[0:4]))
	p.HasTime = data[4] == 1
	p.Time = Float64FromBytes(data[5:13])

	off0 := GetOffset(data[13:17])
	off1 := GetOffset(data[17:21])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0, off1}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Tag, err = DecodeSSZBytes(data, off0, off1)
	if err != nil {
		return err
	}

	// Parse attributes table
	if off1+4 > totalLen {
		p.Attributes = make(map[AttributeHandle][]byte)
		return nil
	}

	count := Uint32FromBytes(data[off1 : off1+4])
	p.Attributes = make(map[AttributeHandle][]byte, count)
	curOffset := off1 + 4

	for i := uint32(0); i < count; i++ {
		if curOffset+8 > totalLen {
			return ErrSSZBufferTooShort
		}
		handle := AttributeHandle(Uint32FromBytes(data[curOffset : curOffset+4]))
		valLen := Uint32FromBytes(data[curOffset+4 : curOffset+8])
		curOffset += 8
		if curOffset+valLen > totalLen {
			return ErrSSZDataOutOfBounds
		}
		val := make([]byte, valLen)
		copy(val, data[curOffset:curOffset+valLen])
		curOffset += valLen
		p.Attributes[handle] = val
	}

	return nil
}

// SendInteractionPayload carries interaction sending or reception
// Layout: [ClassHandle:4, HasTime:1, Time:8, Offset0:4 (Tag), Offset1:4 (Parameters)]
// Heap: Tag bytes + Parameters table [Count:4 + (Handle:4, Len:4, Data:N)...]
type SendInteractionPayload struct {
	ClassHandle InteractionClassHandle
	Parameters  map[ParameterHandle][]byte
	Tag         []byte
	HasTime     bool
	Time        float64
}

func (p SendInteractionPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 21 // 4 + 1 + 8 + 4 + 4
	tagLen := uint32(len(p.Tag))

	paramDataLen := uint32(4) // count
	for _, val := range p.Parameters {
		paramDataLen += 8 + uint32(len(val))
	}

	totalLen := fixedHeaderSize + tagLen + paramDataLen
	buf := make([]byte, totalLen)

	PutUint32(buf[0:4], uint32(p.ClassHandle))
	if p.HasTime {
		buf[4] = 1
		PutFloat64(buf[5:13], p.Time)
	} else {
		buf[4] = 0
		PutFloat64(buf[5:13], 0)
	}

	off0 := uint32(fixedHeaderSize)
	off1 := off0 + tagLen

	PutOffset(buf[13:17], off0)
	PutOffset(buf[17:21], off1)

	EncodeSSZBytes(buf, off0, p.Tag)

	// Serialize parameter table
	PutUint32(buf[off1:off1+4], uint32(len(p.Parameters)))
	curOffset := off1 + 4
	for param, val := range p.Parameters {
		PutUint32(buf[curOffset:curOffset+4], uint32(param))
		PutUint32(buf[curOffset+4:curOffset+8], uint32(len(val)))
		curOffset += 8
		copy(buf[curOffset:], val)
		curOffset += uint32(len(val))
	}

	return buf, nil
}

func (p *SendInteractionPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 21
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ClassHandle = InteractionClassHandle(Uint32FromBytes(data[0:4]))
	p.HasTime = data[4] == 1
	p.Time = Float64FromBytes(data[5:13])

	off0 := GetOffset(data[13:17])
	off1 := GetOffset(data[17:21])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0, off1}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Tag, err = DecodeSSZBytes(data, off0, off1)
	if err != nil {
		return err
	}

	if off1+4 > totalLen {
		p.Parameters = make(map[ParameterHandle][]byte)
		return nil
	}

	count := Uint32FromBytes(data[off1 : off1+4])
	p.Parameters = make(map[ParameterHandle][]byte, count)
	curOffset := off1 + 4

	for i := uint32(0); i < count; i++ {
		if curOffset+8 > totalLen {
			return ErrSSZBufferTooShort
		}
		handle := ParameterHandle(Uint32FromBytes(data[curOffset : curOffset+4]))
		valLen := Uint32FromBytes(data[curOffset+4 : curOffset+8])
		curOffset += 8
		if curOffset+valLen > totalLen {
			return ErrSSZDataOutOfBounds
		}
		val := make([]byte, valLen)
		copy(val, data[curOffset:curOffset+valLen])
		curOffset += valLen
		p.Parameters[handle] = val
	}

	return nil
}

// ============================================================================
// 4. TIME MANAGEMENT PAYLOADS
// ============================================================================

// TimeFloat64Payload conveys a single float64 time value
// Layout: [Time:8] (Fixed)
type TimeFloat64Payload struct {
	Time float64
}

func (p TimeFloat64Payload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 8)
	PutFloat64(buf[0:8], p.Time)
	return buf, nil
}

func (p *TimeFloat64Payload) UnmarshalBinary(data []byte) error {
	if len(data) < 8 {
		return ErrSSZBufferTooShort
	}
	p.Time = Float64FromBytes(data[0:8])
	return nil
}

// ============================================================================
// 5. GENERIC CONTROL & ERROR PAYLOADS
// ============================================================================

// ErrorResponsePayload represents a formal RTI error response
// Layout: [ErrorCode:2, Offset0:4 (Message)] + Heap [Message]
type ErrorResponsePayload struct {
	ErrorCode uint16
	Message   string
}

func (p ErrorResponsePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 6
	msgLen := uint32(len(p.Message))
	totalLen := fixedHeaderSize + msgLen

	buf := make([]byte, totalLen)
	PutUint16(buf[0:2], p.ErrorCode)
	PutOffset(buf[2:6], fixedHeaderSize)
	EncodeSSZString(buf, fixedHeaderSize, p.Message)

	return buf, nil
}

func (p *ErrorResponsePayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 6
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ErrorCode = Uint16FromBytes(data[0:2])
	off0 := GetOffset(data[2:6])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	var err error
	p.Message, err = DecodeSSZString(data, off0, totalLen)
	return err
}

// SuccessOKPayload carries confirmation with an optional message
// Layout: [Offset0:4 (Message)] + Heap [Message]
type SuccessOKPayload struct {
	Message string
}

func (p SuccessOKPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 4
	msgLen := uint32(len(p.Message))
	totalLen := fixedHeaderSize + msgLen

	buf := make([]byte, totalLen)
	PutOffset(buf[0:4], fixedHeaderSize)
	EncodeSSZString(buf, fixedHeaderSize, p.Message)

	return buf, nil
}

func (p *SuccessOKPayload) UnmarshalBinary(data []byte) error {
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
	p.Message, err = DecodeSSZString(data, off0, totalLen)
	return err
}

// ============================================================================
// 6. OWNERSHIP MANAGEMENT PAYLOADS
// ============================================================================

// DivestOwnershipPayload carries the object handle and attribute set to divest
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
// Layout: [ObjectHandle:4, AttributeHandle:4] (Fixed)
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
	if len(data) < 8 {
		return ErrSSZBufferTooShort
	}
	p.ObjectHandle = Uint32FromBytes(data[0:4])
	p.AttributeHandle = Uint32FromBytes(data[4:8])
	return nil
}

// QueryOwnershipResponsePayload returns ownership status
// Layout: [ObjectHandle:4, AttributeHandle:4, OwnerHandle:4, IsOwned:1] (Fixed)
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
	if p.IsOwned {
		buf[12] = 1
	} else {
		buf[12] = 0
	}
	return buf, nil
}

func (p *QueryOwnershipResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 13 {
		return ErrSSZBufferTooShort
	}
	p.ObjectHandle = Uint32FromBytes(data[0:4])
	p.AttributeHandle = Uint32FromBytes(data[4:8])
	p.OwnerHandle = Uint32FromBytes(data[8:12])
	p.IsOwned = data[12] == 1
	return nil
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
// Layout: [RegionHandle:4] (Fixed)
type CreateRegionResponsePayload struct {
	RegionHandle uint32
}

func (p CreateRegionResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	PutUint32(buf[0:4], p.RegionHandle)
	return buf, nil
}

func (p *CreateRegionResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return ErrSSZBufferTooShort
	}
	p.RegionHandle = Uint32FromBytes(data[0:4])
	return nil
}

// SetRangeBoundsPayload defines discrete grid bounds [lower, upper] for a dimension
// Layout: [RegionHandle:4, DimensionHandle:4, LowerBound:4, UpperBound:4] (Fixed)
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
	if len(data) < 16 {
		return ErrSSZBufferTooShort
	}
	p.RegionHandle = Uint32FromBytes(data[0:4])
	p.DimensionHandle = Uint32FromBytes(data[4:8])
	p.LowerBound = Uint32FromBytes(data[8:12])
	p.UpperBound = Uint32FromBytes(data[12:16])
	return nil
}

// SetMetricBoundsPayload sets continuous ECEF metric bounds for a region
// Layout: [RegionHandle:4, MinX:8, MaxX:8, MinY:8, MaxY:8, MinZ:8, MaxZ:8] (Fixed)
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
	if len(data) < 52 {
		return ErrSSZBufferTooShort
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
// Heap: Tag bytes + Attributes table
type UpdateAttributesSpatialPayload struct {
	ObjectHandle ObjectInstanceHandle
	X, Y, Z      float64
	Attributes   map[AttributeHandle][]byte
	Tag          []byte
}

func (p UpdateAttributesSpatialPayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 36 // 4 + 8 + 8 + 8 + 4 + 4
	tagLen := uint32(len(p.Tag))

	attrDataLen := uint32(4) // count
	for _, val := range p.Attributes {
		attrDataLen += 8 + uint32(len(val))
	}

	totalLen := fixedHeaderSize + tagLen + attrDataLen
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

	PutUint32(buf[off1:off1+4], uint32(len(p.Attributes)))
	curOffset := off1 + 4
	for attr, val := range p.Attributes {
		PutUint32(buf[curOffset:curOffset+4], uint32(attr))
		PutUint32(buf[curOffset+4:curOffset+8], uint32(len(val)))
		curOffset += 8
		copy(buf[curOffset:], val)
		curOffset += uint32(len(val))
	}

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

	if off1+4 > totalLen {
		p.Attributes = make(map[AttributeHandle][]byte)
		return nil
	}

	count := Uint32FromBytes(data[off1 : off1+4])
	p.Attributes = make(map[AttributeHandle][]byte, count)
	curOffset := off1 + 4

	for i := uint32(0); i < count; i++ {
		if curOffset+8 > totalLen {
			return ErrSSZBufferTooShort
		}
		handle := AttributeHandle(Uint32FromBytes(data[curOffset : curOffset+4]))
		valLen := Uint32FromBytes(data[curOffset+4 : curOffset+8])
		curOffset += 8
		if curOffset+valLen > totalLen {
			return ErrSSZDataOutOfBounds
		}
		val := make([]byte, valLen)
		copy(val, data[curOffset:curOffset+valLen])
		curOffset += valLen
		p.Attributes[handle] = val
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
// Layout: [Handle:4] (Fixed)
type HandleResponsePayload struct {
	Handle int32
}

func (p HandleResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	PutInt32(buf[0:4], p.Handle)
	return buf, nil
}

func (p *HandleResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return ErrSSZBufferTooShort
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
