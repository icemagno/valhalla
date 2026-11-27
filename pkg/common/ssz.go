// Project Valhalla - Sovereign HLA-4 Run-Time Infrastructure
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)
//
// Canonical Ethereum 2.0 Simple Serialize (SSZ) Implementation.
// Strictly adheres to the Ethereum Consensus Layer SSZ Specification:
// - All primitive integers and floats in Little-Endian byte order.
// - Strict boolean verification (0x00 = false, 0x01 = true; all other values rejected).
// - Fixed-size fields located in container prefix; dynamic fields indexed by 4-byte uint32 offsets.
// - Strict monotonic offset validation; zero trailing bytes permitted.
// - Canonical SSZ Merkleization (hash_tree_root): Pack, Merkleize, MixInLength.
// - Anti-DoS: strict bounds checks preventing memory amplification (OOM) and uint32 wraparound.

package common

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"sort"
)

// Endian defines the canonical SSZ byte order (Little-Endian) per Ethereum 2.0 specification.
var Endian = binary.LittleEndian

// SSZ Specification Errors
var (
	ErrSSZBufferTooShort    = errors.New("ssz: buffer smaller than minimum expected container size")
	ErrSSZInvalidSize       = errors.New("ssz: buffer size does not match exact fixed container size")
	ErrSSZTrailingBytes     = errors.New("ssz: unconsumed trailing bytes at end of container")
	ErrSSZInvalidOffset     = errors.New("ssz: offset table violates monotonic ordering or bounds")
	ErrSSZInvalidBoolean    = errors.New("ssz: invalid boolean value (must be 0x00 or 0x01)")
	ErrSSZStringOutOfBounds = errors.New("ssz: dynamic string range exceeds buffer boundary")
	ErrSSZDataOutOfBounds   = errors.New("ssz: dynamic byte slice range exceeds buffer boundary")
	ErrSSZListAlignment     = errors.New("ssz: dynamic list byte length is not a multiple of element size")
	ErrSSZMemoryOverflow    = errors.New("ssz: item count exceeds maximum capacity of input buffer (OOM DoS prevented)")
)

// ============================================================================
// 1. PRIMITIVE SCALAR CODECS
// ============================================================================

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

// PutBool encodes a boolean strictly as 0x01 (true) or 0x00 (false).
func PutBool(dst []byte, v bool) {
	if v {
		dst[0] = 0x01
	} else {
		dst[0] = 0x00
	}
}

// DecodeUint8 strictly reads an 8-bit unsigned integer, failing if the buffer is too short.
func DecodeUint8(b []byte) (uint8, error) {
	if len(b) < 1 {
		return 0, ErrSSZBufferTooShort
	}
	return b[0], nil
}

// DecodeUint16 strictly decodes a 16-bit Little-Endian unsigned integer.
func DecodeUint16(b []byte) (uint16, error) {
	if len(b) < 2 {
		return 0, ErrSSZBufferTooShort
	}
	return Endian.Uint16(b), nil
}

// DecodeUint32 strictly decodes a 32-bit Little-Endian unsigned integer.
func DecodeUint32(b []byte) (uint32, error) {
	if len(b) < 4 {
		return 0, ErrSSZBufferTooShort
	}
	return Endian.Uint32(b), nil
}

// DecodeUint64 strictly decodes a 64-bit Little-Endian unsigned integer.
func DecodeUint64(b []byte) (uint64, error) {
	if len(b) < 8 {
		return 0, ErrSSZBufferTooShort
	}
	return Endian.Uint64(b), nil
}

// DecodeInt32 strictly decodes a 32-bit Little-Endian signed integer.
func DecodeInt32(b []byte) (int32, error) {
	if len(b) < 4 {
		return 0, ErrSSZBufferTooShort
	}
	return int32(Endian.Uint32(b)), nil
}

// DecodeFloat64 strictly decodes an IEEE-754 64-bit Little-Endian floating point number.
func DecodeFloat64(b []byte) (float64, error) {
	if len(b) < 8 {
		return 0, ErrSSZBufferTooShort
	}
	return math.Float64frombits(Endian.Uint64(b)), nil
}

// DecodeOffset strictly reads a 4-byte dynamic field offset.
func DecodeOffset(b []byte) (uint32, error) {
	if len(b) < 4 {
		return 0, ErrSSZBufferTooShort
	}
	return Endian.Uint32(b), nil
}

// CheckedSSZUint32 converts a uint64 to uint32 rejecting any wraparound.
func CheckedSSZUint32(n uint64) (uint32, error) {
	if n > math.MaxUint32 {
		return 0, ErrSSZMemoryOverflow
	}
	return uint32(n), nil
}

// CheckedSSZSize verifies that an SSZ payload size fits in uint32 and does not exceed MaxNativePayloadLen.
func CheckedSSZSize(n uint64) (uint32, error) {
	if n > uint64(MaxNativePayloadLen) {
		return 0, ErrSSZMemoryOverflow
	}
	return uint32(n), nil
}

// CheckedSSZStringListSize computes the SSZ StringList byte size using overflow-safe uint64 math.
func CheckedSSZStringListSize(strs []string) (uint32, error) {
	if len(strs) == 0 {
		return 0, nil
	}
	total := uint64(len(strs)) * 4
	if total > uint64(MaxNativePayloadLen) {
		return 0, ErrSSZMemoryOverflow
	}
	for _, s := range strs {
		total += uint64(len(s))
		if total > uint64(MaxNativePayloadLen) {
			return 0, ErrSSZMemoryOverflow
		}
	}
	return uint32(total), nil
}

type sszHandleKey interface {
	~int32 | ~uint32
}

func checkedSSZHandleMapSize[K sszHandleKey](m map[K][]byte) (uint32, error) {
	if len(m) == 0 {
		return 0, nil
	}
	total := uint64(len(m)) * 4
	if total > uint64(MaxNativePayloadLen) {
		return 0, ErrSSZMemoryOverflow
	}
	for _, val := range m {
		total += 8 + uint64(len(val))
		if total > uint64(MaxNativePayloadLen) {
			return 0, ErrSSZMemoryOverflow
		}
	}
	return uint32(total), nil
}

// CheckedSSZAttributeListSize computes the SSZ AttributeList byte size using overflow-safe uint64 math.
func CheckedSSZAttributeListSize(attributes map[AttributeHandle][]byte) (uint32, error) {
	return checkedSSZHandleMapSize(attributes)
}

// CheckedSSZParameterListSize computes the SSZ ParameterList byte size using overflow-safe uint64 math.
func CheckedSSZParameterListSize(params map[ParameterHandle][]byte) (uint32, error) {
	return checkedSSZHandleMapSize(params)
}

// Uint8FromBytes reads an 8-bit unsigned integer (caller must verify len(b) >= 1).
func Uint8FromBytes(b []byte) uint8 {
	v, err := DecodeUint8(b)
	if err != nil {
		return 0
	}
	return v
}

// Uint16FromBytes decodes a 16-bit unsigned integer from Little-Endian bytes.
func Uint16FromBytes(b []byte) uint16 {
	v, err := DecodeUint16(b)
	if err != nil {
		return 0
	}
	return v
}

// Uint32FromBytes decodes a 32-bit unsigned integer from Little-Endian bytes.
func Uint32FromBytes(b []byte) uint32 {
	v, err := DecodeUint32(b)
	if err != nil {
		return 0
	}
	return v
}

// Uint64FromBytes decodes a 64-bit unsigned integer from Little-Endian bytes.
func Uint64FromBytes(b []byte) uint64 {
	v, err := DecodeUint64(b)
	if err != nil {
		return 0
	}
	return v
}

// Int32FromBytes decodes a 32-bit signed integer from Little-Endian bytes.
func Int32FromBytes(b []byte) int32 {
	v, err := DecodeInt32(b)
	if err != nil {
		return 0
	}
	return v
}

// Float64FromBytes decodes an IEEE-754 64-bit floating point number from Little-Endian bytes.
func Float64FromBytes(b []byte) float64 {
	v, err := DecodeFloat64(b)
	if err != nil {
		return 0
	}
	return v
}

// DecodeBool decodes a boolean with strict validation (only 0x00 and 0x01 allowed).
func DecodeBool(b byte) (bool, error) {
	switch b {
	case 0x00:
		return false, nil
	case 0x01:
		return true, nil
	default:
		return false, fmt.Errorf("%w: byte 0x%02X", ErrSSZInvalidBoolean, b)
	}
}

// GetOffset reads a 4-byte dynamic field offset from the container's offset table.
func GetOffset(b []byte) uint32 {
	v, err := DecodeOffset(b)
	if err != nil {
		return 0
	}
	return v
}

// ============================================================================
// 2. OFFSET & BOUNDS VALIDATION (ANTI-DOS & MONOTONICITY)
// ============================================================================

// ValidateOffsets validates a sequence of SSZ dynamic offsets:
// 1. First offset matches the exact fixed header size (no gaps).
// 2. Offsets are strictly non-decreasing (monotonic ordering).
// 3. Offsets do not exceed the container boundary.
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

// ============================================================================
// 3. STRINGS & RAW BYTE SLICES
// ============================================================================

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

// ============================================================================
// 4. LISTS (SCALARS & STRINGS)
// ============================================================================

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

// EncodeSSZStringList serializes a list of dynamic strings as a canonical SSZ List[String].
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
	if firstOffset == 0 || firstOffset%4 != 0 || firstOffset > byteLen {
		return nil, ErrSSZInvalidOffset
	}
	count := firstOffset / 4

	// Anti-DoS check: each string requires at least 4 offset bytes
	if count > byteLen/4 {
		return nil, ErrSSZMemoryOverflow
	}

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

// ============================================================================
// 5. CANONICAL SSZ LIST[CONTAINER] FOR ATTRIBUTES & PARAMETERS
// ============================================================================

// SSZAttributeEntry represents an attribute handle and its binary value.
type SSZAttributeEntry struct {
	Handle uint32
	Value  []byte
}

func encodeSSZHandleMap[K sszHandleKey](dst []byte, offset uint32, entries map[K][]byte) uint32 {
	count := uint32(len(entries))
	if count == 0 {
		return offset
	}

	keys := make([]uint32, 0, count)
	for k := range entries {
		keys = append(keys, uint32(k))
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	listOffsetTableSize := count * 4
	curElementOffset := offset + listOffsetTableSize

	for i, k := range keys {
		val := entries[K(k)]
		relElementOffset := curElementOffset - offset
		Endian.PutUint32(dst[offset+uint32(i*4):offset+uint32(i*4)+4], relElementOffset)

		// Container fixed part: Handle (4) + ValueOffset (4)
		Endian.PutUint32(dst[curElementOffset:curElementOffset+4], k)
		Endian.PutUint32(dst[curElementOffset+4:curElementOffset+8], 8)
		// Container dynamic part: Value bytes
		copy(dst[curElementOffset+8:], val)

		curElementOffset += 8 + uint32(len(val))
	}

	return curElementOffset
}

func sizeSSZHandleMap[K sszHandleKey](entries map[K][]byte) uint32 {
	count := uint32(len(entries))
	if count == 0 {
		return 0
	}
	total := count * 4
	for _, val := range entries {
		total += 8 + uint32(len(val))
	}
	return total
}

func decodeSSZHandleMap[K sszHandleKey](data []byte, start, end uint32) (map[K][]byte, error) {
	if start > end || end > uint32(len(data)) {
		return nil, ErrSSZDataOutOfBounds
	}
	byteLen := end - start
	if byteLen == 0 {
		return make(map[K][]byte), nil
	}
	if byteLen < 4 {
		return nil, ErrSSZBufferTooShort
	}

	subData := data[start:end]
	firstOffset := Endian.Uint32(subData[0:4])
	if firstOffset == 0 || firstOffset%4 != 0 || firstOffset > byteLen {
		return nil, ErrSSZInvalidOffset
	}
	count := firstOffset / 4

	// Anti-DoS Guard: Each element takes at least 4 bytes in offset table + 8 bytes container = 12 bytes
	if count > byteLen/12 {
		return nil, ErrSSZMemoryOverflow
	}

	offsets := make([]uint32, count)
	for i := uint32(0); i < count; i++ {
		offsets[i] = Endian.Uint32(subData[i*4 : (i+1)*4])
	}

	if err := ValidateOffsets(offsets, firstOffset, byteLen); err != nil {
		return nil, err
	}

	res := make(map[K][]byte, count)
	var lastHandle uint32
	hasLast := false

	for i := uint32(0); i < count; i++ {
		elemStart := offsets[i]
		elemEnd := byteLen
		if i+1 < count {
			elemEnd = offsets[i+1]
		}

		if elemEnd < elemStart || elemEnd-elemStart < 8 {
			return nil, ErrSSZBufferTooShort
		}

		elemBytes := subData[elemStart:elemEnd]
		handle := Endian.Uint32(elemBytes[0:4])
		valOffset := Endian.Uint32(elemBytes[4:8])

		if valOffset != 8 || valOffset > uint32(len(elemBytes)) {
			return nil, ErrSSZInvalidOffset
		}

		if hasLast && handle <= lastHandle {
			return nil, fmt.Errorf("%w: duplicate or unsorted handle %d <= %d", ErrSSZInvalidOffset, handle, lastHandle)
		}
		lastHandle = handle
		hasLast = true

		valLen := uint32(len(elemBytes)) - valOffset
		val := make([]byte, valLen)
		copy(val, elemBytes[valOffset:])
		res[K(handle)] = val
	}

	return res, nil
}

// EncodeSSZAttributeList encodes a map of attributes as a strictly deterministic,
// canonical SSZ List[Container]. Keys are sorted in strictly ascending numerical order.
func EncodeSSZAttributeList(dst []byte, offset uint32, attributes map[AttributeHandle][]byte) uint32 {
	return encodeSSZHandleMap(dst, offset, attributes)
}

// SizeSSZAttributeList computes the exact byte size required for an attribute map.
func SizeSSZAttributeList(attributes map[AttributeHandle][]byte) uint32 {
	return sizeSSZHandleMap(attributes)
}

// DecodeSSZAttributeList deserializes a canonical SSZ List[Container] of attributes.
func DecodeSSZAttributeList(data []byte, start, end uint32) (map[AttributeHandle][]byte, error) {
	return decodeSSZHandleMap[AttributeHandle](data, start, end)
}

// EncodeSSZParameterList encodes a map of parameters as a deterministic canonical SSZ List[Container].
func EncodeSSZParameterList(dst []byte, offset uint32, params map[ParameterHandle][]byte) uint32 {
	return encodeSSZHandleMap(dst, offset, params)
}

// SizeSSZParameterList computes the exact byte size required for a parameter map.
func SizeSSZParameterList(params map[ParameterHandle][]byte) uint32 {
	return sizeSSZHandleMap(params)
}

// DecodeSSZParameterList deserializes a canonical SSZ List[Container] of parameters.
func DecodeSSZParameterList(data []byte, start, end uint32) (map[ParameterHandle][]byte, error) {
	return decodeSSZHandleMap[ParameterHandle](data, start, end)
}

// ============================================================================
// 6. CANONICAL SSZ MERKLEIZATION (hash_tree_root)
// ============================================================================

// HashPair computes the SHA-256 hash of two 32-byte chunks: SHA256(left || right).
func HashPair(left, right [32]byte) [32]byte {
	var buf [64]byte
	copy(buf[:32], left[:])
	copy(buf[32:], right[:])
	return sha256.Sum256(buf[:])
}

// Pack chunks raw bytes into consecutive 32-byte chunks (padded with zeros).
func Pack(data []byte) [][32]byte {
	if len(data) == 0 {
		return [][32]byte{}
	}
	numChunks := (len(data) + 31) / 32
	chunks := make([][32]byte, numChunks)
	for i := 0; i < numChunks; i++ {
		start := i * 32
		end := start + 32
		if end > len(data) {
			end = len(data)
		}
		copy(chunks[i][:end-start], data[start:end])
	}
	return chunks
}

// NextPowerOfTwo returns the smallest power of 2 >= n (at least 1).
// Clamps at the largest representable power of two for int to prevent signed overflow/infinite loops.
func NextPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}
	const maxPow2 = 1 << (bits.UintSize - 2)
	if n >= maxPow2 {
		return maxPow2
	}
	return 1 << bits.Len(uint(n-1))
}

// zeroHashes stores precomputed Merkle roots of empty subtrees up to depth 64:
// zeroHashes[0] = [32]byte{} (empty chunk)
// zeroHashes[i] = HashPair(zeroHashes[i-1], zeroHashes[i-1])
var zeroHashes [65][32]byte

func init() {
	for i := 0; i < 64; i++ {
		zeroHashes[i+1] = HashPair(zeroHashes[i], zeroHashes[i])
	}
}

// Merkleize computes the Merkle root of a sequence of 32-byte chunks per Ethereum 2.0 SSZ.
// If limit > 0, the tree is expanded to the virtual depth required by limit using
// precomputed zero-hashes in O(N + log2(limit)) time and O(N) space, avoiding dense
// allocation and redundant hashing of empty subtrees.
func Merkleize(chunks [][32]byte, limit uint64) [32]byte {
	n := len(chunks)
	count := uint64(n)
	if limit > count {
		count = limit
	}
	if count == 0 {
		return [32]byte{}
	}

	depth := 0
	if count > 1 {
		depth = bits.Len64(count - 1)
	}
	if depth > 64 {
		depth = 64
	}

	if n == 0 {
		return zeroHashes[depth]
	}

	if n == 1 {
		root := chunks[0]
		for level := 0; level < depth; level++ {
			root = HashPair(root, zeroHashes[level])
		}
		return root
	}

	// Layer-by-layer sparse folding in O(N + log2(limit))
	layer := make([][32]byte, n)
	copy(layer, chunks)

	for h := 0; h < depth; h++ {
		curLen := len(layer)
		if curLen == 1 {
			root := layer[0]
			for level := h; level < depth; level++ {
				root = HashPair(root, zeroHashes[level])
			}
			return root
		}

		half := curLen / 2
		for i := 0; i < half; i++ {
			layer[i] = HashPair(layer[2*i], layer[2*i+1])
		}
		if curLen%2 != 0 {
			layer[half] = HashPair(layer[curLen-1], zeroHashes[h])
			layer = layer[:half+1]
		} else {
			layer = layer[:half]
		}
	}

	return layer[0]
}

// MixInLength mixes the length of a dynamic list into its Merkle root:
// HashPair(root, uint256(length)) per Ethereum 2.0 specification.
func MixInLength(root [32]byte, length uint64) [32]byte {
	var lenChunk [32]byte
	Endian.PutUint64(lenChunk[0:8], length)
	return HashPair(root, lenChunk)
}

// HashTreeRootUint32 computes the SSZ hash_tree_root of a uint32.
func HashTreeRootUint32(v uint32) [32]byte {
	var chunk [32]byte
	Endian.PutUint32(chunk[0:4], v)
	return chunk
}

// HashTreeRootByteList computes the SSZ hash_tree_root of a dynamic byte slice (ByteList).
func HashTreeRootByteList(data []byte, limit uint64) [32]byte {
	chunks := Pack(data)
	var chunkLimit uint64
	if limit > 0 {
		// Overflow-safe ceiling division: ceil(limit / 32) without (limit + 31) wraparound
		chunkLimit = (limit-1)/32 + 1
	}
	root := Merkleize(chunks, chunkLimit)
	return MixInLength(root, uint64(len(data)))
}
