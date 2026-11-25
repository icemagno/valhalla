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

// FederationExecutionPayload carries the federation name for creation or destruction
type FederationExecutionPayload struct {
	FederationName string
}

func (p FederationExecutionPayload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 2+len(p.FederationName))
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(p.FederationName)))
	copy(buf[2:], p.FederationName)
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
	return nil
}

// JoinFederationPayload representa a requisição de ingresso do federado
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

// JoinFederationResponsePayload resposta com identificadores concedidos
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

// ResignFederationPayload desassociação do federado
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

// RegisterSyncPointPayload registro de ponto de sincronização
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

// SyncPointLabelPayload payload para mensagens que transmitem apenas o label (Achieve, Announce, Synchronized)
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

// PublishObjectClassPayload publica atributos de uma classe de objetos
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

// SubscribeObjectClassPayload subscreve atributos de uma classe de objetos
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

// InteractionClassPayload publica ou despublica uma classe de interação
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

// SubscribeInteractionClassPayload subscreve uma classe de interação
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

// RegisterObjectInstancePayload registra uma nova instância
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

// DiscoverObjectInstancePayload descobre uma instância remota
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

// DeleteObjectInstancePayload solicitação de exclusão de instância de objeto
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

// RemoveObjectInstancePayload notificação de remoção de instância de objeto para federados
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

// UpdateAttributesPayload atualização de atributos (UAV) ou reflexão (RAV)
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

// SendInteractionPayload envio ou recebimento de interação
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

// TimeFloat64Payload payload comum que transporta um valor float64 (ex: Lookahead, LogicalTime, GrantTime)
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

// ErrorResponsePayload representa uma resposta de erro formal do RTI
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

// SuccessOKPayload confirmação com mensagem opcional
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
