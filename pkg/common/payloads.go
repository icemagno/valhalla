// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)
//
// Canonical Simple Serialize (SSZ) Payloads for Valhalla Native Wire Protocol (VAL4).
// Conforms strictly to the Ethereum 2.0 Consensus Layer Specification (SSZ):
// - All primitive integers and floats in Little-Endian byte order.
// - Strict boolean verification (0x00 and 0x01 only; all other values rejected).
// - Zero trailing bytes permitted; exact container length enforcement.
// - Canonical SSZ List[Container] for attributes and parameters with strict ascending handle sorting.
// - Anti-DoS bounds checking preventing memory amplification (OOM) and uint32 wraparound.

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
// Layout: [FederateHandle:4, FederationHandle:4] (Exact 8 bytes fixed)
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
	if len(data) != 8 {
		if len(data) < 8 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
	}
	p.FederateHandle = FederateHandle(Uint32FromBytes(data[0:4]))
	p.FederationHandle = Int32FromBytes(data[4:8])
	return nil
}

// ResignFederationPayload represents federate resignation
// Layout: [ResignAction:1] (Exact 1 byte fixed)
type ResignFederationPayload struct {
	ResignAction uint8
}

func (p ResignFederationPayload) MarshalBinary() ([]byte, error) {
	return []byte{p.ResignAction}, nil
}

func (p *ResignFederationPayload) UnmarshalBinary(data []byte) error {
	if len(data) != 1 {
		if len(data) < 1 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
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

// SyncPointFailurePayload represents synchronization point registration failure
// Layout: [Offset0:4 (Label), Offset1:4 (Reason)] + Heap [Label, Reason]
type SyncPointFailurePayload struct {
	Label  string
	Reason string
}

func (p SyncPointFailurePayload) MarshalBinary() ([]byte, error) {
	const fixedHeaderSize = 8
	labelLen := uint32(len(p.Label))
	reasonLen := uint32(len(p.Reason))

	off0 := uint32(fixedHeaderSize)
	off1 := off0 + labelLen
	totalLen := off1 + reasonLen

	buf := make([]byte, totalLen)
	PutOffset(buf[0:4], off0)
	PutOffset(buf[4:8], off1)
	EncodeSSZString(buf, off0, p.Label)
	EncodeSSZString(buf, off1, p.Reason)
	return buf, nil
}

func (p *SyncPointFailurePayload) UnmarshalBinary(data []byte) error {
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
	p.Reason, err = DecodeSSZString(data, off1, totalLen)
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
	PutBool(buf[4:5], p.Active)
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
	var err error
	p.Active, err = DecodeBool(data[4])
	if err != nil {
		return err
	}

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
// Layout: [ClassHandle:4] (Exact 4 bytes fixed)
type InteractionClassPayload struct {
	ClassHandle InteractionClassHandle
}

func (p InteractionClassPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	PutUint32(buf[0:4], uint32(p.ClassHandle))
	return buf, nil
}

func (p *InteractionClassPayload) UnmarshalBinary(data []byte) error {
	if len(data) != 4 {
		if len(data) < 4 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
	}
	p.ClassHandle = InteractionClassHandle(Uint32FromBytes(data[0:4]))
	return nil
}

// SubscribeInteractionClassPayload subscribes or unsubscribes to an interaction class
// Layout: [ClassHandle:4, Active:1] (Exact 5 bytes fixed)
type SubscribeInteractionClassPayload struct {
	ClassHandle InteractionClassHandle
	Active      bool
}

func (p SubscribeInteractionClassPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 5)
	PutUint32(buf[0:4], uint32(p.ClassHandle))
	PutBool(buf[4:5], p.Active)
	return buf, nil
}

func (p *SubscribeInteractionClassPayload) UnmarshalBinary(data []byte) error {
	if len(data) != 5 {
		if len(data) < 5 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
	}
	p.ClassHandle = InteractionClassHandle(Uint32FromBytes(data[0:4]))
	var err error
	p.Active, err = DecodeBool(data[4])
	return err
}

// ============================================================================
// 3. OBJECT MANAGEMENT PAYLOADS
// ============================================================================

// RegisterObjectInstancePayload registers an object instance
