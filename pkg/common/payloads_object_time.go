package common

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
// Dynamic part: Tag bytes + Canonical SSZ List[Container] of Attributes
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
	attrListSize := SizeSSZAttributeList(p.Attributes)

	totalLen := fixedHeaderSize + tagLen + attrListSize
	buf := make([]byte, totalLen)

	PutUint32(buf[0:4], uint32(p.ObjectHandle))
	PutBool(buf[4:5], p.HasTime)
	if p.HasTime {
		PutFloat64(buf[5:13], p.Time)
	} else {
		PutFloat64(buf[5:13], 0)
	}

	off0 := uint32(fixedHeaderSize)
	off1 := off0 + tagLen

	PutOffset(buf[13:17], off0)
	PutOffset(buf[17:21], off1)

	EncodeSSZBytes(buf, off0, p.Tag)
	EncodeSSZAttributeList(buf, off1, p.Attributes)

	return buf, nil
}

func (p *UpdateAttributesPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 21
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ObjectHandle = ObjectInstanceHandle(Uint32FromBytes(data[0:4]))
	var err error
	p.HasTime, err = DecodeBool(data[4])
	if err != nil {
		return err
	}
	p.Time = Float64FromBytes(data[5:13])

	off0 := GetOffset(data[13:17])
	off1 := GetOffset(data[17:21])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0, off1}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

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

// SendInteractionPayload carries interaction sending or reception
// Layout: [ClassHandle:4, HasTime:1, Time:8, Offset0:4 (Tag), Offset1:4 (Parameters)]
// Dynamic part: Tag bytes + Canonical SSZ List[Container] of Parameters
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
	paramListSize := SizeSSZParameterList(p.Parameters)

	totalLen := fixedHeaderSize + tagLen + paramListSize
	buf := make([]byte, totalLen)

	PutUint32(buf[0:4], uint32(p.ClassHandle))
	PutBool(buf[4:5], p.HasTime)
	if p.HasTime {
		PutFloat64(buf[5:13], p.Time)
	} else {
		PutFloat64(buf[5:13], 0)
	}

	off0 := uint32(fixedHeaderSize)
	off1 := off0 + tagLen

	PutOffset(buf[13:17], off0)
	PutOffset(buf[17:21], off1)

	EncodeSSZBytes(buf, off0, p.Tag)
	EncodeSSZParameterList(buf, off1, p.Parameters)

	return buf, nil
}

func (p *SendInteractionPayload) UnmarshalBinary(data []byte) error {
	const fixedHeaderSize = 21
	if len(data) < fixedHeaderSize {
		return ErrSSZBufferTooShort
	}

	p.ClassHandle = InteractionClassHandle(Uint32FromBytes(data[0:4]))
	var err error
	p.HasTime, err = DecodeBool(data[4])
	if err != nil {
		return err
	}
	p.Time = Float64FromBytes(data[5:13])

	off0 := GetOffset(data[13:17])
	off1 := GetOffset(data[17:21])
	totalLen := uint32(len(data))

	if err := ValidateOffsets([]uint32{off0, off1}, fixedHeaderSize, totalLen); err != nil {
		return err
	}

	p.Tag, err = DecodeSSZBytes(data, off0, off1)
	if err != nil {
		return err
	}

	p.Parameters, err = DecodeSSZParameterList(data, off1, totalLen)
	if err != nil {
		return err
	}

	return nil
}

// ============================================================================
// 4. TIME MANAGEMENT PAYLOADS
// ============================================================================

// TimeFloat64Payload conveys a single float64 time value
// Layout: [Time:8] (Exact 8 bytes fixed)
type TimeFloat64Payload struct {
	Time float64
}

func (p TimeFloat64Payload) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 8)
	PutFloat64(buf[0:8], p.Time)
	return buf, nil
}

func (p *TimeFloat64Payload) UnmarshalBinary(data []byte) error {
	if len(data) != 8 {
		if len(data) < 8 {
			return ErrSSZBufferTooShort
		}
		return ErrSSZTrailingBytes
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
