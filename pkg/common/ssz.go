// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)
//
// Package common: Canonical Ethereum 2.0 Simple Serialize (SSZ) implementation.
// Adheres strictly to the Ethereum Consensus Layer SSZ Specification:
// - All primitive scalars encoded in Little-Endian byte order.
// - Fixed-size fields placed in the fixed container prefix.
// - Variable-size fields referenced by 4-byte Little-Endian uint32 offsets.
// - Variable-size heaps serialized sequentially with strict monotonic offset validation.

package common

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Endian defines the canonical SSZ byte order (Little-Endian) per Ethereum 2.0 specification.
var Endian = binary.LittleEndian

// SSZ Encoding and Decoding Errors
var (
	ErrSSZBufferTooShort    = errors.New("ssz: buffer smaller than minimum expected container size")
	ErrSSZInvalidOffset     = errors.New("ssz: offset table violates monotonic ordering or bounds")
	ErrSSZStringOutOfBounds = errors.New("ssz: dynamic string range exceeds buffer boundary")
	ErrSSZDataOutOfBounds   = errors.New("ssz: dynamic byte slice range exceeds buffer boundary")
	ErrSSZListAlignment     = errors.New("ssz: dynamic list byte length is not a multiple of element size")
)

// PutUint8 encodes an 8-bit unsigned integer.
func PutUint8(dst []byte, v uint8) {
	dst[0] = v
}

// PutUint16 encodes a 16-bit unsigned integer in Little-Endian.
func PutUint16(dst []byte, v uint16) {
	Endian.PutUint16(dst, v)
}

// PutUint32 encodes a 32-bit unsigned integer in Little-Endian.
func PutUint32(dst []byte, v uint32) {
	Endian.PutUint32(dst, v)
}

// PutUint64 encodes a 64-bit unsigned integer in Little-Endian.
func PutUint64(dst []byte, v uint64) {
	Endian.PutUint64(dst, v)
}

// PutInt32 encodes a 32-bit signed integer in Little-Endian two's complement.
func PutInt32(dst []byte, v int32) {
	Endian.PutUint32(dst, uint32(v))
}

// PutFloat64 encodes an IEEE-754 64-bit floating point number in Little-Endian.
func PutFloat64(dst []byte, v float64) {
	Endian.PutUint64(dst, math.Float64bits(v))
}

// PutOffset writes a 4-byte dynamic field offset into the container's offset table.
func PutOffset(dst []byte, offset uint32) {
	Endian.PutUint32(dst, offset)
}

// Uint8FromBytes reads an 8-bit unsigned integer.
func Uint8FromBytes(b []byte) uint8 {
	if len(b) < 1 {
		return 0
	}
	return b[0]
}

// Uint16FromBytes decodes a 16-bit unsigned integer from Little-Endian bytes.
func Uint16FromBytes(b []byte) uint16 {
	if len(b) < 2 {
		return 0
	}
	return Endian.Uint16(b)
}

// Uint32FromBytes decodes a 32-bit unsigned integer from Little-Endian bytes.
func Uint32FromBytes(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return Endian.Uint32(b)
}

// Uint64FromBytes decodes a 64-bit unsigned integer from Little-Endian bytes.
func Uint64FromBytes(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return Endian.Uint64(b)
}

// Int32FromBytes decodes a 32-bit signed integer from Little-Endian bytes.
func Int32FromBytes(b []byte) int32 {
	if len(b) < 4 {
		return 0
	}
	return int32(Endian.Uint32(b))
}

// Float64FromBytes decodes an IEEE-754 64-bit floating point number from Little-Endian bytes.
func Float64FromBytes(b []byte) float64 {
	if len(b) < 8 {
		return 0
	}
	return math.Float64frombits(Endian.Uint64(b))
}

// GetOffset reads a 4-byte dynamic field offset from the container's offset table.
func GetOffset(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return Endian.Uint32(b)
}

// ValidateOffsets validates a sequence of SSZ dynamic offsets for strict monotonicity,
// anti-overlap integrity, and total container length bounds.
func ValidateOffsets(offsets []uint32, minDataPos uint32, totalLen uint32) error {
	if len(offsets) == 0 {
		return nil
	}

	if offsets[0] != minDataPos {
		return fmt.Errorf("%w: first offset (%d) does not match fixed header size (%d)",
			ErrSSZInvalidOffset, offsets[0], minDataPos)
	}

	for i := 0; i < len(offsets)-1; i++ {
		if offsets[i+1] < offsets[i] {
			return fmt.Errorf("%w: offset[%d]=%d exceeds offset[%d]=%d (monotonic violation)",
				ErrSSZInvalidOffset, i, offsets[i], i+1, offsets[i+1])
		}
	}

	lastOffset := offsets[len(offsets)-1]
	if lastOffset > totalLen {
		return fmt.Errorf("%w: last offset (%d) exceeds container size (%d)",
			ErrSSZInvalidOffset, lastOffset, totalLen)
	}

	return nil
}

// EncodeSSZString copies string bytes into dst at offset and returns the updated offset.
func EncodeSSZString(dst []byte, offset uint32, s string) uint32 {
	copy(dst[offset:], s)
	return offset + uint32(len(s))
}

// DecodeSSZString extracts a UTF-8 string from a bounded slice of container data.
func DecodeSSZString(data []byte, start, end uint32) (string, error) {
	if start > end || end > uint32(len(data)) {
		return "", ErrSSZStringOutOfBounds
	}
	if start == end {
		return "", nil
	}
	return string(data[start:end]), nil
}

// EncodeSSZBytes copies raw bytes into dst at offset and returns the updated offset.
func EncodeSSZBytes(dst []byte, offset uint32, b []byte) uint32 {
	copy(dst[offset:], b)
	return offset + uint32(len(b))
}

// DecodeSSZBytes extracts a byte slice from a bounded slice of container data.
func DecodeSSZBytes(data []byte, start, end uint32) ([]byte, error) {
	if start > end || end > uint32(len(data)) {
		return nil, ErrSSZDataOutOfBounds
	}
	if start == end {
		return nil, nil
	}
	res := make([]byte, end-start)
	copy(res, data[start:end])
	return res, nil
}

// EncodeSSZUint32List serializes a slice of uint32 integers in contiguous Little-Endian bytes.
func EncodeSSZUint32List(dst []byte, offset uint32, list []uint32) uint32 {
	for _, v := range list {
		Endian.PutUint32(dst[offset:offset+4], v)
		offset += 4
	}
	return offset
}

// DecodeSSZUint32List deserializes a slice of uint32 integers from a bounded container slice.
func DecodeSSZUint32List(data []byte, start, end uint32) ([]uint32, error) {
	if start > end || end > uint32(len(data)) {
		return nil, ErrSSZDataOutOfBounds
	}
	byteLen := end - start
	if byteLen%4 != 0 {
		return nil, ErrSSZListAlignment
	}
	count := byteLen / 4
	if count == 0 {
		return nil, nil
	}
	res := make([]uint32, count)
	idx := start
	for i := uint32(0); i < count; i++ {
		res[i] = Endian.Uint32(data[idx : idx+4])
		idx += 4
	}
	return res, nil
}

// EncodeSSZInt32List serializes a slice of int32 integers in contiguous Little-Endian bytes.
func EncodeSSZInt32List(dst []byte, offset uint32, list []int32) uint32 {
	for _, v := range list {
		Endian.PutUint32(dst[offset:offset+4], uint32(v))
		offset += 4
	}
	return offset
}

// DecodeSSZInt32List deserializes a slice of int32 integers from a bounded container slice.
func DecodeSSZInt32List(data []byte, start, end uint32) ([]int32, error) {
	if start > end || end > uint32(len(data)) {
		return nil, ErrSSZDataOutOfBounds
	}
	byteLen := end - start
	if byteLen%4 != 0 {
		return nil, ErrSSZListAlignment
	}
	count := byteLen / 4
	if count == 0 {
		return nil, nil
	}
	res := make([]int32, count)
	idx := start
	for i := uint32(0); i < count; i++ {
		res[i] = int32(Endian.Uint32(data[idx : idx+4]))
		idx += 4
	}
	return res, nil
}

// EncodeSSZStringList serializes a list of dynamic strings as an SSZ sub-container.
// Each string receives a 4-byte offset in a local offset table, followed by its bytes.
func EncodeSSZStringList(dst []byte, offset uint32, list []string) uint32 {
	count := uint32(len(list))
	if count == 0 {
		return offset
	}

	offsetTableSize := count * 4
	heapOffset := offset + offsetTableSize

	for i, s := range list {
		relOffset := heapOffset - offset
		Endian.PutUint32(dst[offset+uint32(i*4):offset+uint32(i*4)+4], relOffset)
		copy(dst[heapOffset:], s)
		heapOffset += uint32(len(s))
	}

	return heapOffset
}

// SizeSSZStringList computes the total SSZ byte size required to serialize a list of strings.
func SizeSSZStringList(list []string) uint32 {
	if len(list) == 0 {
		return 0
	}
	total := uint32(len(list) * 4) // 4-byte offset per item
	for _, s := range list {
		total += uint32(len(s))
	}
	return total
}

// DecodeSSZStringList deserializes a list of strings from an SSZ sub-container slice.
func DecodeSSZStringList(data []byte, start, end uint32) ([]string, error) {
	if start > end || end > uint32(len(data)) {
		return nil, ErrSSZDataOutOfBounds
	}
	byteLen := end - start
	if byteLen == 0 {
		return nil, nil
	}
	if byteLen < 4 {
		return nil, ErrSSZBufferTooShort
	}

	subData := data[start:end]
	firstOffset := Endian.Uint32(subData[0:4])
	if firstOffset%4 != 0 || firstOffset > byteLen {
		return nil, ErrSSZInvalidOffset
	}
	count := firstOffset / 4

	offsets := make([]uint32, count)
	for i := uint32(0); i < count; i++ {
		offsets[i] = Endian.Uint32(subData[i*4 : (i+1)*4])
	}

	if err := ValidateOffsets(offsets, firstOffset, byteLen); err != nil {
		return nil, err
	}

	res := make([]string, count)
	for i := uint32(0); i < count; i++ {
		sStart := offsets[i]
		sEnd := byteLen
		if i+1 < count {
			sEnd = offsets[i+1]
		}
		res[i] = string(subData[sStart:sEnd])
	}

	return res, nil
}
