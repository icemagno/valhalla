package core

import (
	"errors"
	"math"
	"math/bits"
)

const (
	// MortonBitsPerAxis represents 21 bits per axis, totaling 63 bits packed into a uint64
	MortonBitsPerAxis = 21
	// MaxCoordinateValue represents (2^21 - 1) = 2,097,151
	MaxCoordinateValue = (1 << MortonBitsPerAxis) - 1
	// ECEFMinBound defines the minimum metric extent (-10,000,000 m, covering Earth + LEO)
	ECEFMinBound = -10000000.0
	// ECEFMaxBound defines the maximum metric extent (+10,000,000 m)
	ECEFMaxBound = 10000000.0
	// ECEFExtent is the total 20,000 km span
	ECEFExtent = ECEFMaxBound - ECEFMinBound

	// MortonXMask extracts every 3rd bit starting at bit 0 (the dilated X bits)
	MortonXMask uint64 = 0x1249249249249249
)

var (
	ErrCoordinateOutOfBounds = errors.New("ddm morton: metric coordinate out of ECEF bounds [-10M, +10M]")
	ErrInvalidBoundingBox    = errors.New("ddm morton: min bound exceeds max bound")
)

// Part1By2 spreads the lowest 21 bits of x across 63 bits, inserting 2 zeros between each bit.
func Part1By2(x uint64) uint64 {
	x &= 0x1fffff // Mask to 21 bits
	x = (x | (x << 32)) & 0x1f00000000ffff
	x = (x | (x << 16)) & 0x1f0000ff0000ff
	x = (x | (x << 8)) & 0x100f00f00f00f00f
	x = (x | (x << 4)) & 0x10c30c30c30c30c3
	x = (x | (x << 2)) & 0x1249249249249249
	return x
}

// Compact1By2 compresses every 3rd bit of x into the lowest 21 bits (inverse of Part1By2).
func Compact1By2(x uint64) uint32 {
	x &= 0x1249249249249249
	x = (x ^ (x >> 2)) & 0x10c30c30c30c30c3
	x = (x ^ (x >> 4)) & 0x100f00f00f00f00f
	x = (x ^ (x >> 8)) & 0x1f0000ff0000ff
	x = (x ^ (x >> 16)) & 0x1f00000000ffff
	x = (x ^ (x >> 32)) & 0x1fffff
	return uint32(x)
}

// EncodeMorton3D interleaves three 21-bit integers (X, Y, Z) into a 63-bit Morton code.
// Bits are ordered: ...Z2 Y2 X2 Z1 Y1 X1 Z0 Y0 X0
func EncodeMorton3D(x, y, z uint32) uint64 {
	return (Part1By2(uint64(z)) << 2) | (Part1By2(uint64(y)) << 1) | Part1By2(uint64(x))
}

// DecodeMorton3D de-interleaves a 63-bit Morton code back into three 21-bit integers (X, Y, Z).
func DecodeMorton3D(code uint64) (uint32, uint32, uint32) {
	x := Compact1By2(code)
	y := Compact1By2(code >> 1)
	z := Compact1By2(code >> 2)
	return x, y, z
}

// MetricToGrid converts a continuous metric ECEF coordinate (in meters) to a 21-bit integer grid.
func MetricToGrid(val float64) uint32 {
	if val <= ECEFMinBound {
		return 0
	}
	if val >= ECEFMaxBound {
		return MaxCoordinateValue
	}
	normalized := (val - ECEFMinBound) / ECEFExtent
	grid := uint32(math.Round(normalized * float64(MaxCoordinateValue)))
	if grid > MaxCoordinateValue {
		grid = MaxCoordinateValue
	}
	return grid
}

// GridToMetric converts a 21-bit integer grid index back to continuous metric ECEF (in meters).
func GridToMetric(grid uint32) float64 {
	if grid >= MaxCoordinateValue {
		return ECEFMaxBound
	}
	ratio := float64(grid) / float64(MaxCoordinateValue)
	return ECEFMinBound + ratio*ECEFExtent
}

// EncodeECEF converts metric (X, Y, Z) coordinates directly into a 64-bit Morton code.
func EncodeECEF(x, y, z float64) uint64 {
	gx := MetricToGrid(x)
	gy := MetricToGrid(y)
	gz := MetricToGrid(z)
	return EncodeMorton3D(gx, gy, gz)
}

// DecodeECEF converts a 64-bit Morton code directly back into metric (X, Y, Z) coordinates.
func DecodeECEF(code uint64) (float64, float64, float64) {
	gx, gy, gz := DecodeMorton3D(code)
	return GridToMetric(gx), GridToMetric(gy), GridToMetric(gz)
}

// SpatialRegion represents an orthogonal 3D bounding box for Data Distribution Management (DDM).
// It maintains precomputed dilated coordinate bounds for direct bitwise containment in Morton space.
type SpatialRegion struct {
	MinX, MaxX uint32
	MinY, MaxY uint32
	MinZ, MaxZ uint32

	// Precomputed dilated Morton coordinates for zero-decode bitwise containment
	partMinX, partMaxX uint64
	partMinY, partMaxY uint64
	partMinZ, partMaxZ uint64

	// Common octree ancestor prefix and mask for O(1) branch rejection
	Prefix     uint64
	PrefixMask uint64
}

// RecomputeMortonBounds calculates the dilated coordinates and octree prefix mask.
func (r *SpatialRegion) RecomputeMortonBounds() {
	r.partMinX = Part1By2(uint64(r.MinX))
	r.partMaxX = Part1By2(uint64(r.MaxX))
	r.partMinY = Part1By2(uint64(r.MinY))
	r.partMaxY = Part1By2(uint64(r.MaxY))
	r.partMinZ = Part1By2(uint64(r.MinZ))
	r.partMaxZ = Part1By2(uint64(r.MaxZ))

	minCode := (r.partMinZ << 2) | (r.partMinY << 1) | r.partMinX
	maxCode := (r.partMaxZ << 2) | (r.partMaxY << 1) | r.partMaxX
	diff := minCode ^ maxCode

	if diff == 0 {
		r.PrefixMask = 0x7fffffffffffffff
		r.Prefix = minCode & r.PrefixMask
	} else {
		clz := bits.LeadingZeros64(diff)
		// Morton codes occupy 63 bits (bits 0..62). Bit 63 is zero.
		if clz > 1 {
			sharedBits := clz - 1
			octreeLevels := sharedBits / 3
			pBits := octreeLevels * 3
			if pBits > 0 && pBits <= 63 {
				r.PrefixMask = ((uint64(1) << pBits) - 1) << (63 - pBits)
				r.Prefix = minCode & r.PrefixMask
			} else {
				r.PrefixMask = 0
				r.Prefix = 0
			}
		} else {
			r.PrefixMask = 0
			r.Prefix = 0
		}
	}
}

// NewSpatialRegion creates a SpatialRegion from discrete grid coordinates and precomputes Morton bounds.
func NewSpatialRegion(minX, maxX, minY, maxY, minZ, maxZ uint32) (SpatialRegion, error) {
	if minX > maxX || minY > maxY || minZ > maxZ {
		return SpatialRegion{}, ErrInvalidBoundingBox
	}
	r := SpatialRegion{
		MinX: minX, MaxX: maxX,
		MinY: minY, MaxY: maxY,
		MinZ: minZ, MaxZ: maxZ,
	}
	r.RecomputeMortonBounds()
	return r, nil
}

// NewSpatialRegionFromMetric creates a SpatialRegion from continuous metric coordinates.
func NewSpatialRegionFromMetric(minX, maxX, minY, maxY, minZ, maxZ float64) (SpatialRegion, error) {
	if minX > maxX || minY > maxY || minZ > maxZ {
		return SpatialRegion{}, ErrInvalidBoundingBox
	}
	r := SpatialRegion{
		MinX: MetricToGrid(minX),
		MaxX: MetricToGrid(maxX),
		MinY: MetricToGrid(minY),
		MaxY: MetricToGrid(maxY),
		MinZ: MetricToGrid(minZ),
		MaxZ: MetricToGrid(maxZ),
	}
	r.RecomputeMortonBounds()
	return r, nil
}

// ContainsPoint checks whether a 3D grid point (x, y, z) is inside the region.
func (r SpatialRegion) ContainsPoint(x, y, z uint32) bool {
	return x >= r.MinX && x <= r.MaxX &&
		y >= r.MinY && y <= r.MaxY &&
		z >= r.MinZ && z <= r.MaxZ
}

// ContainsMorton evaluates spatial containment directly in Morton space without decoding coordinates.
// It executes a bitwise common-octree ancestor filter followed by parallel bitwise coordinate comparison.
// Because Part1By2 is strictly monotonic, comparing dilated bits preserves total ordering:
//
//	(code & MortonXMask) in [partMinX, partMaxX] <=> x in [MinX, MaxX]
//
// This eliminates coordinate decoding (Compact1By2) and floating-point conversions.
func (r SpatialRegion) ContainsMorton(code uint64) bool {
	if r.partMaxX == 0 && r.MaxX > 0 {
		r.RecomputeMortonBounds()
	}

	// 1. Bitwise common octree ancestor check (fast reject for points in other octants)
	if r.PrefixMask != 0 && (code&r.PrefixMask) != r.Prefix {
		return false
	}

	// 2. Parallel bitwise coordinate containment in Morton space
	cx := code & MortonXMask
	if cx < r.partMinX || cx > r.partMaxX {
		return false
	}

	cy := (code >> 1) & MortonXMask
	if cy < r.partMinY || cy > r.partMaxY {
		return false
	}

	cz := (code >> 2) & MortonXMask
	if cz < r.partMinZ || cz > r.partMaxZ {
		return false
	}

	return true
}

// MortonBounds returns the enclosing bounding interval [minCode, maxCode] in Morton space.
func (r SpatialRegion) MortonBounds() (uint64, uint64) {
	if r.partMaxX == 0 && r.MaxX > 0 {
		r.RecomputeMortonBounds()
	}
	minCode := (r.partMinZ << 2) | (r.partMinY << 1) | r.partMinX
	maxCode := (r.partMaxZ << 2) | (r.partMaxY << 1) | r.partMaxX
	return minCode, maxCode
}

// Overlaps checks whether two 3D spatial regions intersect.
func (r SpatialRegion) Overlaps(other SpatialRegion) bool {
	if r.MaxX < other.MinX || r.MinX > other.MaxX {
		return false
	}
	if r.MaxY < other.MinY || r.MinY > other.MaxY {
		return false
	}
	if r.MaxZ < other.MinZ || r.MinZ > other.MaxZ {
		return false
	}
	return true
}
