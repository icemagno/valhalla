package common

import (
	"encoding/binary"
	"errors"
	"math"
)

// Binary payload decoding errors
var (
	ErrPayloadTooShort       = errors.New("valhalla payload: buffer smaller than minimum expected length")
	ErrPayloadStringOverflow = errors.New("valhalla payload: string length exceeds buffer")
	ErrPayloadDataOverflow   = errors.New("valhalla payload: data length exceeds buffer")
)

// ============================================================================
// 1. FEDERATION MANAGEMENT PAYLOADS
// ============================================================================

// FederationExecutionPayload carries the federation name and optional FOM modules for creation or destruction
type FederationExecutionPayload struct {
	FederationName string
	FOMModules     []string
}

func (p FederationExecutionPayload) MarshalBinary() ([]byte, error) {
	totalLen := 2 + len(p.FederationName)
	if len(p.FOMModules) > 0 {
		totalLen += 2 // uint16 module count
		for _, m := range p.FOMModules {
			totalLen += 2 + len(m)
		}
	}
	buf := make([]byte, totalLen)
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(p.FederationName)))
	copy(buf[2:], p.FederationName)

	offset := 2 + len(p.FederationName)
	if len(p.FOMModules) > 0 {
		binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(len(p.FOMModules)))
		offset += 2
		for _, m := range p.FOMModules {
			binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(len(m)))
			offset += 2
			copy(buf[offset:], m)
			offset += len(m)
		}
	}
	return buf, nil
}

func (p *FederationExecutionPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 2 {
		return ErrPayloadTooShort
	}
	fedLen := int(binary.BigEndian.Uint16(data[0:2]))
	if 2+fedLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.FederationName = string(data[2 : 2+fedLen])
	p.FOMModules = nil

	offset := 2 + fedLen
	if len(data) >= offset+2 {
		numMods := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
		p.FOMModules = make([]string, 0, numMods)
		for i := 0; i < numMods; i++ {
			if len(data) < offset+2 {
				break
			}
			mLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2
			if len(data) < offset+mLen {
				break
			}
			p.FOMModules = append(p.FOMModules, string(data[offset:offset+mLen]))
			offset += mLen
		}
	}
	return nil
}

// JoinFederationPayload represents a federate join request
type JoinFederationPayload struct {
	FederateName   string
	FederateType   string
	FederationName string
}

func (p JoinFederationPayload) MarshalBinary() ([]byte, error) {
	totalLen := 2 + len(p.FederateName) + 2 + len(p.FederateType) + 2 + len(p.FederationName)
	buf := make([]byte, totalLen)
	offset := 0

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(p.FederateName)))
	offset += 2
	copy(buf[offset:], p.FederateName)
	offset += len(p.FederateName)

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(p.FederateType)))
	offset += 2
	copy(buf[offset:], p.FederateType)
	offset += len(p.FederateType)

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(p.FederationName)))
	offset += 2
	copy(buf[offset:], p.FederationName)

	return buf, nil
}

func (p *JoinFederationPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 6 {
		return ErrPayloadTooShort
	}
	offset := 0

	// FederateName
	nameLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2
	if offset+nameLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.FederateName = string(data[offset : offset+nameLen])
	offset += nameLen

	// FederateType
	if offset+2 > len(data) {
		return ErrPayloadTooShort
	}
	typeLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2
	if offset+typeLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.FederateType = string(data[offset : offset+typeLen])
	offset += typeLen

	// FederationName
	if offset+2 > len(data) {
		return ErrPayloadTooShort
	}
	fedLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2
	if offset+fedLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.FederationName = string(data[offset : offset+fedLen])

	return nil
}

// JoinFederationResponsePayload contains granted federate and federation identifiers.
type JoinFederationResponsePayload struct {
	FederateHandle   FederateHandle
	FederationHandle int32
}

func (p JoinFederationResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.FederateHandle))
	binary.BigEndian.PutUint32(buf[4:8], uint32(p.FederationHandle))
	return buf, nil
}

func (p *JoinFederationResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 8 {
		return ErrPayloadTooShort
	}
	p.FederateHandle = FederateHandle(binary.BigEndian.Uint32(data[0:4]))
	p.FederationHandle = int32(binary.BigEndian.Uint32(data[4:8]))
	return nil
}

// ResignFederationPayload represents federate resignation
type ResignFederationPayload struct {
	ResignAction uint8 // 1=UnconditionallyDivest, 2=DeleteObjects, etc.
}

func (p ResignFederationPayload) MarshalBinary() ([]byte, error) {
	return []byte{p.ResignAction}, nil
}

func (p *ResignFederationPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 1 {
		return ErrPayloadTooShort
	}
	p.ResignAction = data[0]
	return nil
}

// RegisterSyncPointPayload represents synchronization point registration
type RegisterSyncPointPayload struct {
	Label string
	Tag   []byte
}

func (p RegisterSyncPointPayload) MarshalBinary() ([]byte, error) {
	totalLen := 2 + len(p.Label) + 2 + len(p.Tag)
	buf := make([]byte, totalLen)
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(p.Label)))
	copy(buf[2:], p.Label)
	offset := 2 + len(p.Label)
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(p.Tag)))
	copy(buf[offset+2:], p.Tag)
	return buf, nil
}

func (p *RegisterSyncPointPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return ErrPayloadTooShort
	}
	labelLen := int(binary.BigEndian.Uint16(data[0:2]))
	offset := 2
	if offset+labelLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.Label = string(data[offset : offset+labelLen])
	offset += labelLen

	if offset+2 > len(data) {
		return ErrPayloadTooShort
	}
	tagLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2
	if offset+tagLen > len(data) {
		return ErrPayloadDataOverflow
	}
	p.Tag = make([]byte, tagLen)
	copy(p.Tag, data[offset:offset+tagLen])
	return nil
}

// SyncPointLabelPayload represents messages carrying only a label string (Achieve, Announce, Synchronized).
type SyncPointLabelPayload struct {
	Label string
}

func (p SyncPointLabelPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 2+len(p.Label))
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(p.Label)))
	copy(buf[2:], p.Label)
	return buf, nil
}

func (p *SyncPointLabelPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 2 {
		return ErrPayloadTooShort
	}
	labelLen := int(binary.BigEndian.Uint16(data[0:2]))
	if 2+labelLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.Label = string(data[2 : 2+labelLen])
	return nil
}

// ============================================================================
// 2. DECLARATION MANAGEMENT (PUB/SUB) PAYLOADS
// ============================================================================

// PublishObjectClassPayload publishes attributes of an object class.
type PublishObjectClassPayload struct {
	ClassHandle      ObjectClassHandle
	AttributeHandles []AttributeHandle
}

func (p PublishObjectClassPayload) MarshalBinary() ([]byte, error) {
	totalLen := 4 + 2 + (len(p.AttributeHandles) * 4)
	buf := make([]byte, totalLen)
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.ClassHandle))
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(p.AttributeHandles)))
	offset := 6
	for _, attr := range p.AttributeHandles {
		binary.BigEndian.PutUint32(buf[offset:], uint32(attr))
		offset += 4
	}
	return buf, nil
}

func (p *PublishObjectClassPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 6 {
		return ErrPayloadTooShort
	}
	p.ClassHandle = ObjectClassHandle(binary.BigEndian.Uint32(data[0:4]))
	count := int(binary.BigEndian.Uint16(data[4:6]))
	expectedLen := 6 + (count * 4)
	if len(data) < expectedLen {
		return ErrPayloadTooShort
	}

	p.AttributeHandles = make([]AttributeHandle, count)
	offset := 6
	for i := 0; i < count; i++ {
		p.AttributeHandles[i] = AttributeHandle(binary.BigEndian.Uint32(data[offset:]))
		offset += 4
	}
	return nil
}

// SubscribeObjectClassPayload subscribes to attributes of an object class.
type SubscribeObjectClassPayload struct {
	ClassHandle      ObjectClassHandle
	AttributeHandles []AttributeHandle
	Active           bool
}

func (p SubscribeObjectClassPayload) MarshalBinary() ([]byte, error) {
	totalLen := 4 + 1 + 2 + (len(p.AttributeHandles) * 4)
	buf := make([]byte, totalLen)
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.ClassHandle))
	if p.Active {
		buf[4] = 1
	} else {
		buf[4] = 0
	}
	binary.BigEndian.PutUint16(buf[5:7], uint16(len(p.AttributeHandles)))
	offset := 7
	for _, attr := range p.AttributeHandles {
		binary.BigEndian.PutUint32(buf[offset:], uint32(attr))
		offset += 4
	}
	return buf, nil
}

func (p *SubscribeObjectClassPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 7 {
		return ErrPayloadTooShort
	}
	p.ClassHandle = ObjectClassHandle(binary.BigEndian.Uint32(data[0:4]))
	p.Active = data[4] == 1
	count := int(binary.BigEndian.Uint16(data[5:7]))
	expectedLen := 7 + (count * 4)
	if len(data) < expectedLen {
		return ErrPayloadTooShort
	}

	p.AttributeHandles = make([]AttributeHandle, count)
	offset := 7
	for i := 0; i < count; i++ {
		p.AttributeHandles[i] = AttributeHandle(binary.BigEndian.Uint32(data[offset:]))
		offset += 4
	}
	return nil
}

// InteractionClassPayload publishes or unpublishes an interaction class
type InteractionClassPayload struct {
	ClassHandle InteractionClassHandle
}

func (p InteractionClassPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.ClassHandle))
	return buf, nil
}

func (p *InteractionClassPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return ErrPayloadTooShort
	}
	p.ClassHandle = InteractionClassHandle(binary.BigEndian.Uint32(data[0:4]))
	return nil
}

// SubscribeInteractionClassPayload subscribes to an interaction class
type SubscribeInteractionClassPayload struct {
	ClassHandle InteractionClassHandle
	Active      bool
}

func (p SubscribeInteractionClassPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 5)
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.ClassHandle))
	if p.Active {
		buf[4] = 1
	} else {
		buf[4] = 0
	}
	return buf, nil
}

func (p *SubscribeInteractionClassPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 5 {
		return ErrPayloadTooShort
	}
	p.ClassHandle = InteractionClassHandle(binary.BigEndian.Uint32(data[0:4]))
	p.Active = data[4] == 1
	return nil
}

// ============================================================================
// 3. OBJECT MANAGEMENT PAYLOADS
// ============================================================================

// RegisterObjectInstancePayload registers a new object instance
type RegisterObjectInstancePayload struct {
	ClassHandle ObjectClassHandle
	ObjectName  string
}

func (p RegisterObjectInstancePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4+2+len(p.ObjectName))
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.ClassHandle))
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(p.ObjectName)))
	copy(buf[6:], p.ObjectName)
	return buf, nil
}

func (p *RegisterObjectInstancePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 6 {
		return ErrPayloadTooShort
	}
	p.ClassHandle = ObjectClassHandle(binary.BigEndian.Uint32(data[0:4]))
	nameLen := int(binary.BigEndian.Uint16(data[4:6]))
	if 6+nameLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.ObjectName = string(data[6 : 6+nameLen])
	return nil
}

// DiscoverObjectInstancePayload conveys a discovered remote object instance
type DiscoverObjectInstancePayload struct {
	ObjectHandle ObjectInstanceHandle
	ClassHandle  ObjectClassHandle
	ObjectName   string
}

func (p DiscoverObjectInstancePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4+4+2+len(p.ObjectName))
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.ObjectHandle))
	binary.BigEndian.PutUint32(buf[4:8], uint32(p.ClassHandle))
	binary.BigEndian.PutUint16(buf[8:10], uint16(len(p.ObjectName)))
	copy(buf[10:], p.ObjectName)
	return buf, nil
}

func (p *DiscoverObjectInstancePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 10 {
		return ErrPayloadTooShort
	}
	p.ObjectHandle = ObjectInstanceHandle(binary.BigEndian.Uint32(data[0:4]))
	p.ClassHandle = ObjectClassHandle(binary.BigEndian.Uint32(data[4:8]))
	nameLen := int(binary.BigEndian.Uint16(data[8:10]))
	if 10+nameLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.ObjectName = string(data[10 : 10+nameLen])
	return nil
}

// DeleteObjectInstancePayload represents an object instance deletion request
type DeleteObjectInstancePayload struct {
	ObjectHandle ObjectInstanceHandle
	Tag          []byte
}

func (p DeleteObjectInstancePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4+2+len(p.Tag))
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.ObjectHandle))
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(p.Tag)))
	copy(buf[6:], p.Tag)
	return buf, nil
}

func (p *DeleteObjectInstancePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 6 {
		return ErrPayloadTooShort
	}
	p.ObjectHandle = ObjectInstanceHandle(binary.BigEndian.Uint32(data[0:4]))
	tagLen := int(binary.BigEndian.Uint16(data[4:6]))
	if 6+tagLen > len(data) {
		return ErrPayloadDataOverflow
	}
	p.Tag = make([]byte, tagLen)
	copy(p.Tag, data[6:6+tagLen])
	return nil
}

// RemoveObjectInstancePayload notifies federates of object instance removal
type RemoveObjectInstancePayload struct {
	ObjectHandle ObjectInstanceHandle
	Tag          []byte
}

func (p RemoveObjectInstancePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4+2+len(p.Tag))
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.ObjectHandle))
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(p.Tag)))
	copy(buf[6:], p.Tag)
	return buf, nil
}

func (p *RemoveObjectInstancePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 6 {
		return ErrPayloadTooShort
	}
	p.ObjectHandle = ObjectInstanceHandle(binary.BigEndian.Uint32(data[0:4]))
	tagLen := int(binary.BigEndian.Uint16(data[4:6]))
	if 6+tagLen > len(data) {
		return ErrPayloadDataOverflow
	}
	p.Tag = make([]byte, tagLen)
	copy(p.Tag, data[6:6+tagLen])
	return nil
}

// UpdateAttributesPayload carries attribute updates (UAV) or reflections (RAV)
type UpdateAttributesPayload struct {
	ObjectHandle ObjectInstanceHandle
	Attributes   map[AttributeHandle][]byte
	Tag          []byte
	HasTime      bool
	Time         float64
}

func (p UpdateAttributesPayload) MarshalBinary() ([]byte, error) {
	// 4 bytes ObjectHandle + 1 byte HasTime + (8 bytes se HasTime) + 2 bytes TagLen + len(Tag) + 2 bytes AttrCount
	totalLen := 4 + 1 + 2 + len(p.Tag) + 2
	if p.HasTime {
		totalLen += 8
	}
	for _, val := range p.Attributes {
		totalLen += 4 + 4 + len(val) // AttrHandle (4) + ValLen (4) + Data
	}

	buf := make([]byte, totalLen)
	offset := 0

	binary.BigEndian.PutUint32(buf[offset:], uint32(p.ObjectHandle))
	offset += 4

	if p.HasTime {
		buf[offset] = 1
		offset++
		binary.BigEndian.PutUint64(buf[offset:], math.Float64bits(p.Time))
		offset += 8
	} else {
		buf[offset] = 0
		offset++
	}

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(p.Tag)))
	offset += 2
	copy(buf[offset:], p.Tag)
	offset += len(p.Tag)

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(p.Attributes)))
	offset += 2

	for attr, val := range p.Attributes {
		binary.BigEndian.PutUint32(buf[offset:], uint32(attr))
		offset += 4
		binary.BigEndian.PutUint32(buf[offset:], uint32(len(val)))
		offset += 4
		copy(buf[offset:], val)
		offset += len(val)
	}

	return buf, nil
}

func (p *UpdateAttributesPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 7 { // 4 (handle) + 1 (hasTime) + 2 (tagLen)
		return ErrPayloadTooShort
	}
	offset := 0

	p.ObjectHandle = ObjectInstanceHandle(binary.BigEndian.Uint32(data[offset:]))
	offset += 4

	p.HasTime = data[offset] == 1
	offset++

	if p.HasTime {
		if offset+8 > len(data) {
			return ErrPayloadTooShort
		}
		p.Time = math.Float64frombits(binary.BigEndian.Uint64(data[offset:]))
		offset += 8
	}

	if offset+2 > len(data) {
		return ErrPayloadTooShort
	}
	tagLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2
	if offset+tagLen > len(data) {
		return ErrPayloadDataOverflow
	}
	p.Tag = make([]byte, tagLen)
	copy(p.Tag, data[offset:offset+tagLen])
	offset += tagLen

	if offset+2 > len(data) {
		return ErrPayloadTooShort
	}
	attrCount := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2

	p.Attributes = make(map[AttributeHandle][]byte, attrCount)
	for i := 0; i < attrCount; i++ {
		if offset+8 > len(data) {
			return ErrPayloadTooShort
		}
		attr := AttributeHandle(binary.BigEndian.Uint32(data[offset:]))
		offset += 4
		valLen := int(binary.BigEndian.Uint32(data[offset:]))
		offset += 4
		if offset+valLen > len(data) {
			return ErrPayloadDataOverflow
		}
		val := make([]byte, valLen)
		copy(val, data[offset:offset+valLen])
		offset += valLen
		p.Attributes[attr] = val
	}

	return nil
}

// SendInteractionPayload carries interaction sending or reception
type SendInteractionPayload struct {
	ClassHandle InteractionClassHandle
	Parameters  map[ParameterHandle][]byte
	Tag         []byte
	HasTime     bool
	Time        float64
}

func (p SendInteractionPayload) MarshalBinary() ([]byte, error) {
	totalLen := 4 + 1 + 2 + len(p.Tag) + 2
	if p.HasTime {
		totalLen += 8
	}
	for _, val := range p.Parameters {
		totalLen += 4 + 4 + len(val) // ParamHandle (4) + ValLen (4) + Data
	}

	buf := make([]byte, totalLen)
	offset := 0

	binary.BigEndian.PutUint32(buf[offset:], uint32(p.ClassHandle))
	offset += 4

	if p.HasTime {
		buf[offset] = 1
		offset++
		binary.BigEndian.PutUint64(buf[offset:], math.Float64bits(p.Time))
		offset += 8
	} else {
		buf[offset] = 0
		offset++
	}

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(p.Tag)))
	offset += 2
	copy(buf[offset:], p.Tag)
	offset += len(p.Tag)

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(p.Parameters)))
	offset += 2

	for param, val := range p.Parameters {
		binary.BigEndian.PutUint32(buf[offset:], uint32(param))
		offset += 4
		binary.BigEndian.PutUint32(buf[offset:], uint32(len(val)))
		offset += 4
		copy(buf[offset:], val)
		offset += len(val)
	}

	return buf, nil
}

func (p *SendInteractionPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 7 {
		return ErrPayloadTooShort
	}
	offset := 0

	p.ClassHandle = InteractionClassHandle(binary.BigEndian.Uint32(data[offset:]))
	offset += 4

	p.HasTime = data[offset] == 1
	offset++

	if p.HasTime {
		if offset+8 > len(data) {
			return ErrPayloadTooShort
		}
		p.Time = math.Float64frombits(binary.BigEndian.Uint64(data[offset:]))
		offset += 8
	}

	if offset+2 > len(data) {
		return ErrPayloadTooShort
	}
	tagLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2
	if offset+tagLen > len(data) {
		return ErrPayloadDataOverflow
	}
	p.Tag = make([]byte, tagLen)
	copy(p.Tag, data[offset:offset+tagLen])
	offset += tagLen

	if offset+2 > len(data) {
		return ErrPayloadTooShort
	}
	paramCount := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2

	p.Parameters = make(map[ParameterHandle][]byte, paramCount)
	for i := 0; i < paramCount; i++ {
		if offset+8 > len(data) {
			return ErrPayloadTooShort
		}
		param := ParameterHandle(binary.BigEndian.Uint32(data[offset:]))
		offset += 4
		valLen := int(binary.BigEndian.Uint32(data[offset:]))
		offset += 4
		if offset+valLen > len(data) {
			return ErrPayloadDataOverflow
		}
		val := make([]byte, valLen)
		copy(val, data[offset:offset+valLen])
		offset += valLen
		p.Parameters[param] = val
	}

	return nil
}

// ============================================================================
// 4. TIME MANAGEMENT PAYLOADS
// ============================================================================

// TimeFloat64Payload common payload conveying a single float64 value (e.g., Lookahead, LogicalTime, GrantTime)
type TimeFloat64Payload struct {
	Time float64
}

func (p TimeFloat64Payload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf[0:8], math.Float64bits(p.Time))
	return buf, nil
}

func (p *TimeFloat64Payload) UnmarshalBinary(data []byte) error {
	if len(data) < 8 {
		return ErrPayloadTooShort
	}
	p.Time = math.Float64frombits(binary.BigEndian.Uint64(data[0:8]))
	return nil
}

// ============================================================================
// 5. GENERIC CONTROL & ERROR PAYLOADS
// ============================================================================

// ErrorResponsePayload represents a formal RTI error response
type ErrorResponsePayload struct {
	ErrorCode uint16
	Message   string
}

func (p ErrorResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 2+2+len(p.Message))
	binary.BigEndian.PutUint16(buf[0:2], p.ErrorCode)
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(p.Message)))
	copy(buf[4:], p.Message)
	return buf, nil
}

func (p *ErrorResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return ErrPayloadTooShort
	}
	p.ErrorCode = binary.BigEndian.Uint16(data[0:2])
	msgLen := int(binary.BigEndian.Uint16(data[2:4]))
	if 4+msgLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.Message = string(data[4 : 4+msgLen])
	return nil
}

// SuccessOKPayload carries confirmation with an optional message
type SuccessOKPayload struct {
	Message string
}

func (p SuccessOKPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 2+len(p.Message))
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(p.Message)))
	copy(buf[2:], p.Message)
	return buf, nil
}

func (p *SuccessOKPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 2 {
		return ErrPayloadTooShort
	}
	msgLen := int(binary.BigEndian.Uint16(data[0:2]))
	if 2+msgLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.Message = string(data[2 : 2+msgLen])
	return nil
}

// ============================================================================
// 6. OWNERSHIP MANAGEMENT PAYLOADS
// ============================================================================

// DivestOwnershipPayload carries the object handle and attribute set to divest
type DivestOwnershipPayload struct {
	ObjectHandle uint32
	Attributes   []uint32
}

func (p DivestOwnershipPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4+2+len(p.Attributes)*4)
	binary.BigEndian.PutUint32(buf[0:4], p.ObjectHandle)
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(p.Attributes)))
	offset := 6
	for _, attr := range p.Attributes {
		binary.BigEndian.PutUint32(buf[offset:offset+4], attr)
		offset += 4
	}
	return buf, nil
}

func (p *DivestOwnershipPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 6 {
		return ErrPayloadTooShort
	}
	p.ObjectHandle = binary.BigEndian.Uint32(data[0:4])
	count := int(binary.BigEndian.Uint16(data[4:6]))
	if 6+count*4 > len(data) {
		return ErrPayloadDataOverflow
	}
	p.Attributes = make([]uint32, count)
	offset := 6
	for i := 0; i < count; i++ {
		p.Attributes[i] = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
	}
	return nil
}

// AcquireOwnershipPayload carries the object handle and attributes to acquire
type AcquireOwnershipPayload struct {
	ObjectHandle uint32
	Attributes   []uint32
}

func (p AcquireOwnershipPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4+2+len(p.Attributes)*4)
	binary.BigEndian.PutUint32(buf[0:4], p.ObjectHandle)
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(p.Attributes)))
	offset := 6
	for _, attr := range p.Attributes {
		binary.BigEndian.PutUint32(buf[offset:offset+4], attr)
		offset += 4
	}
	return buf, nil
}

func (p *AcquireOwnershipPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 6 {
		return ErrPayloadTooShort
	}
	p.ObjectHandle = binary.BigEndian.Uint32(data[0:4])
	count := int(binary.BigEndian.Uint16(data[4:6]))
	if 6+count*4 > len(data) {
		return ErrPayloadDataOverflow
	}
	p.Attributes = make([]uint32, count)
	offset := 6
	for i := 0; i < count; i++ {
		p.Attributes[i] = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
	}
	return nil
}

// QueryOwnershipPayload carries the object handle and attribute handle to query
type QueryOwnershipPayload struct {
	ObjectHandle    uint32
	AttributeHandle uint32
}

func (p QueryOwnershipPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], p.ObjectHandle)
	binary.BigEndian.PutUint32(buf[4:8], p.AttributeHandle)
	return buf, nil
}

func (p *QueryOwnershipPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 8 {
		return ErrPayloadTooShort
	}
	p.ObjectHandle = binary.BigEndian.Uint32(data[0:4])
	p.AttributeHandle = binary.BigEndian.Uint32(data[4:8])
	return nil
}

// QueryOwnershipResponsePayload returns ownership status
type QueryOwnershipResponsePayload struct {
	ObjectHandle    uint32
	AttributeHandle uint32
	OwnerHandle     uint32
	IsOwned         bool
}

func (p QueryOwnershipResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 13)
	binary.BigEndian.PutUint32(buf[0:4], p.ObjectHandle)
	binary.BigEndian.PutUint32(buf[4:8], p.AttributeHandle)
	binary.BigEndian.PutUint32(buf[8:12], p.OwnerHandle)
	if p.IsOwned {
		buf[12] = 1
	} else {
		buf[12] = 0
	}
	return buf, nil
}

func (p *QueryOwnershipResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 13 {
		return ErrPayloadTooShort
	}
	p.ObjectHandle = binary.BigEndian.Uint32(data[0:4])
	p.AttributeHandle = binary.BigEndian.Uint32(data[4:8])
	p.OwnerHandle = binary.BigEndian.Uint32(data[8:12])
	p.IsOwned = data[12] == 1
	return nil
}

// ============================================================================
// 7. DATA DISTRIBUTION MANAGEMENT (DDM) PAYLOADS
// ============================================================================

// CreateRegionPayload carries dimensions for region creation
type CreateRegionPayload struct {
	Dimensions []uint32
}

func (p CreateRegionPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 2+len(p.Dimensions)*4)
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(p.Dimensions)))
	offset := 2
	for _, dim := range p.Dimensions {
		binary.BigEndian.PutUint32(buf[offset:offset+4], dim)
		offset += 4
	}
	return buf, nil
}

func (p *CreateRegionPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 2 {
		return ErrPayloadTooShort
	}
	count := int(binary.BigEndian.Uint16(data[0:2]))
	if 2+count*4 > len(data) {
		return ErrPayloadDataOverflow
	}
	p.Dimensions = make([]uint32, count)
	offset := 2
	for i := 0; i < count; i++ {
		p.Dimensions[i] = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
	}
	return nil
}

// CreateRegionResponsePayload returns the assigned region handle
type CreateRegionResponsePayload struct {
	RegionHandle uint32
}

func (p CreateRegionResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf[0:4], p.RegionHandle)
	return buf, nil
}

func (p *CreateRegionResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return ErrPayloadTooShort
	}
	p.RegionHandle = binary.BigEndian.Uint32(data[0:4])
	return nil
}

// SetRangeBoundsPayload defines discrete grid bounds [lower, upper] for a dimension
type SetRangeBoundsPayload struct {
	RegionHandle    uint32
	DimensionHandle uint32
	LowerBound      uint32
	UpperBound      uint32
}

func (p SetRangeBoundsPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint32(buf[0:4], p.RegionHandle)
	binary.BigEndian.PutUint32(buf[4:8], p.DimensionHandle)
	binary.BigEndian.PutUint32(buf[8:12], p.LowerBound)
	binary.BigEndian.PutUint32(buf[12:16], p.UpperBound)
	return buf, nil
}

func (p *SetRangeBoundsPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 16 {
		return ErrPayloadTooShort
	}
	p.RegionHandle = binary.BigEndian.Uint32(data[0:4])
	p.DimensionHandle = binary.BigEndian.Uint32(data[4:8])
	p.LowerBound = binary.BigEndian.Uint32(data[8:12])
	p.UpperBound = binary.BigEndian.Uint32(data[12:16])
	return nil
}

// SetMetricBoundsPayload sets continuous ECEF metric bounds for a region
type SetMetricBoundsPayload struct {
	RegionHandle uint32
	MinX, MaxX   float64
	MinY, MaxY   float64
	MinZ, MaxZ   float64
}

func (p SetMetricBoundsPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4+6*8)
	binary.BigEndian.PutUint32(buf[0:4], p.RegionHandle)
	binary.BigEndian.PutUint64(buf[4:12], math.Float64bits(p.MinX))
	binary.BigEndian.PutUint64(buf[12:20], math.Float64bits(p.MaxX))
	binary.BigEndian.PutUint64(buf[20:28], math.Float64bits(p.MinY))
	binary.BigEndian.PutUint64(buf[28:36], math.Float64bits(p.MaxY))
	binary.BigEndian.PutUint64(buf[36:44], math.Float64bits(p.MinZ))
	binary.BigEndian.PutUint64(buf[44:52], math.Float64bits(p.MaxZ))
	return buf, nil
}

func (p *SetMetricBoundsPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 52 {
		return ErrPayloadTooShort
	}
	p.RegionHandle = binary.BigEndian.Uint32(data[0:4])
	p.MinX = math.Float64frombits(binary.BigEndian.Uint64(data[4:12]))
	p.MaxX = math.Float64frombits(binary.BigEndian.Uint64(data[12:20]))
	p.MinY = math.Float64frombits(binary.BigEndian.Uint64(data[20:28]))
	p.MaxY = math.Float64frombits(binary.BigEndian.Uint64(data[28:36]))
	p.MinZ = math.Float64frombits(binary.BigEndian.Uint64(data[36:44]))
	p.MaxZ = math.Float64frombits(binary.BigEndian.Uint64(data[44:52]))
	return nil
}

// SubscribeWithRegionPayload associates a subscription with a spatial region
type SubscribeWithRegionPayload struct {
	ObjectClass  uint32
	Attributes   []uint32
	RegionHandle uint32
}

func (p SubscribeWithRegionPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4+4+2+len(p.Attributes)*4)
	binary.BigEndian.PutUint32(buf[0:4], p.ObjectClass)
	binary.BigEndian.PutUint32(buf[4:8], p.RegionHandle)
	binary.BigEndian.PutUint16(buf[8:10], uint16(len(p.Attributes)))
	offset := 10
	for _, attr := range p.Attributes {
		binary.BigEndian.PutUint32(buf[offset:offset+4], attr)
		offset += 4
	}
	return buf, nil
}

func (p *SubscribeWithRegionPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 10 {
		return ErrPayloadTooShort
	}
	p.ObjectClass = binary.BigEndian.Uint32(data[0:4])
	p.RegionHandle = binary.BigEndian.Uint32(data[4:8])
	count := int(binary.BigEndian.Uint16(data[8:10]))
	if 10+count*4 > len(data) {
		return ErrPayloadDataOverflow
	}
	p.Attributes = make([]uint32, count)
	offset := 10
	for i := 0; i < count; i++ {
		p.Attributes[i] = binary.BigEndian.Uint32(data[offset : offset+4])
		offset += 4
	}
	return nil
}

// UpdateAttributesSpatialPayload transmits attribute updates tagged with metric ECEF (X, Y, Z) coordinates
type UpdateAttributesSpatialPayload struct {
	ObjectHandle ObjectInstanceHandle
	X, Y, Z      float64
	Attributes   map[AttributeHandle][]byte
	Tag          []byte
}

func (p UpdateAttributesSpatialPayload) MarshalBinary() ([]byte, error) {
	totalLen := 4 + 3*8 + 2 + 2 + len(p.Tag)
	for _, val := range p.Attributes {
		totalLen += 4 + 2 + len(val)
	}

	buf := make([]byte, totalLen)
	offset := 0

	binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(p.ObjectHandle))
	offset += 4
	binary.BigEndian.PutUint64(buf[offset:offset+8], math.Float64bits(p.X))
	offset += 8
	binary.BigEndian.PutUint64(buf[offset:offset+8], math.Float64bits(p.Y))
	offset += 8
	binary.BigEndian.PutUint64(buf[offset:offset+8], math.Float64bits(p.Z))
	offset += 8

	binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(len(p.Tag)))
	offset += 2
	copy(buf[offset:], p.Tag)
	offset += len(p.Tag)

	binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(len(p.Attributes)))
	offset += 2

	for attrHandle, val := range p.Attributes {
		binary.BigEndian.PutUint32(buf[offset:offset+4], uint32(attrHandle))
		offset += 4
		binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(len(val)))
		offset += 2
		copy(buf[offset:], val)
		offset += len(val)
	}

	return buf, nil
}

func (p *UpdateAttributesSpatialPayload) UnmarshalBinary(data []byte) error {
	if len(data) < 32 {
		return ErrPayloadTooShort
	}
	offset := 0
	p.ObjectHandle = ObjectInstanceHandle(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4
	p.X = math.Float64frombits(binary.BigEndian.Uint64(data[offset : offset+8]))
	offset += 8
	p.Y = math.Float64frombits(binary.BigEndian.Uint64(data[offset : offset+8]))
	offset += 8
	p.Z = math.Float64frombits(binary.BigEndian.Uint64(data[offset : offset+8]))
	offset += 8

	tagLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if offset+tagLen > len(data) {
		return ErrPayloadDataOverflow
	}
	p.Tag = make([]byte, tagLen)
	copy(p.Tag, data[offset:offset+tagLen])
	offset += tagLen

	if offset+2 > len(data) {
		return ErrPayloadTooShort
	}
	attrCount := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2

	p.Attributes = make(map[AttributeHandle][]byte, attrCount)
	for i := 0; i < attrCount; i++ {
		if offset+6 > len(data) {
			return ErrPayloadTooShort
		}
		handle := AttributeHandle(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4
		vLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
		if offset+vLen > len(data) {
			return ErrPayloadDataOverflow
		}
		val := make([]byte, vLen)
		copy(val, data[offset:offset+vLen])
		offset += vLen
		p.Attributes[handle] = val
	}
	return nil
}

// ============================================================================
// 6. OMT / FOM HANDLE RESOLUTION PAYLOADS
// ============================================================================

// GetObjectClassHandlePayload carries the class name to resolve
type GetObjectClassHandlePayload struct {
	ClassName string
}

func (p GetObjectClassHandlePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 2+len(p.ClassName))
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(p.ClassName)))
	copy(buf[2:], p.ClassName)
	return buf, nil
}

func (p *GetObjectClassHandlePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 2 {
		return ErrPayloadTooShort
	}
	nLen := int(binary.BigEndian.Uint16(data[0:2]))
	if 2+nLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.ClassName = string(data[2 : 2+nLen])
	return nil
}

// HandleResponsePayload returns an int32 handle
type HandleResponsePayload struct {
	Handle int32
}

func (p HandleResponsePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.Handle))
	return buf, nil
}

func (p *HandleResponsePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return ErrPayloadTooShort
	}
	p.Handle = int32(binary.BigEndian.Uint32(data[0:4]))
	return nil
}

// GetAttributeHandlePayload carries class handle and attribute name to resolve
type GetAttributeHandlePayload struct {
	ClassHandle   int32
	AttributeName string
}

func (p GetAttributeHandlePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 6+len(p.AttributeName))
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.ClassHandle))
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(p.AttributeName)))
	copy(buf[6:], p.AttributeName)
	return buf, nil
}

func (p *GetAttributeHandlePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 6 {
		return ErrPayloadTooShort
	}
	p.ClassHandle = int32(binary.BigEndian.Uint32(data[0:4]))
	nLen := int(binary.BigEndian.Uint16(data[4:6]))
	if 6+nLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.AttributeName = string(data[6 : 6+nLen])
	return nil
}

// GetDimensionHandlePayload carries dimension name to resolve
type GetDimensionHandlePayload struct {
	DimensionName string
}

func (p GetDimensionHandlePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 2+len(p.DimensionName))
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(p.DimensionName)))
	copy(buf[2:], p.DimensionName)
	return buf, nil
}

func (p *GetDimensionHandlePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 2 {
		return ErrPayloadTooShort
	}
	nLen := int(binary.BigEndian.Uint16(data[0:2]))
	if 2+nLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.DimensionName = string(data[2 : 2+nLen])
	return nil
}

// GetInteractionClassHandlePayload carries interaction name to resolve
type GetInteractionClassHandlePayload struct {
	InteractionName string
}

func (p GetInteractionClassHandlePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 2+len(p.InteractionName))
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(p.InteractionName)))
	copy(buf[2:], p.InteractionName)
	return buf, nil
}

func (p *GetInteractionClassHandlePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 2 {
		return ErrPayloadTooShort
	}
	nLen := int(binary.BigEndian.Uint16(data[0:2]))
	if 2+nLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.InteractionName = string(data[2 : 2+nLen])
	return nil
}

// GetParameterHandlePayload carries interaction handle and parameter name to resolve
type GetParameterHandlePayload struct {
	InteractionHandle int32
	ParameterName     string
}

func (p GetParameterHandlePayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 6+len(p.ParameterName))
	binary.BigEndian.PutUint32(buf[0:4], uint32(p.InteractionHandle))
	binary.BigEndian.PutUint16(buf[4:6], uint16(len(p.ParameterName)))
	copy(buf[6:], p.ParameterName)
	return buf, nil
}

func (p *GetParameterHandlePayload) UnmarshalBinary(data []byte) error {
	if len(data) < 6 {
		return ErrPayloadTooShort
	}
	p.InteractionHandle = int32(binary.BigEndian.Uint32(data[0:4]))
	nLen := int(binary.BigEndian.Uint16(data[4:6]))
	if 6+nLen > len(data) {
		return ErrPayloadStringOverflow
	}
	p.ParameterName = string(data[6 : 6+nLen])
	return nil
}
