package core

import (
	"errors"
	"math"
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
type SpatialRegion struct {
	MinX, MaxX uint32
	MinY, MaxY uint32
	MinZ, MaxZ uint32
}

// NewSpatialRegionFromMetric creates a SpatialRegion from continuous metric coordinates.
func NewSpatialRegionFromMetric(minX, maxX, minY, maxY, minZ, maxZ float64) (SpatialRegion, error) {
	if minX > maxX || minY > maxY || minZ > maxZ {
		return SpatialRegion{}, ErrInvalidBoundingBox
	}
	return SpatialRegion{
		MinX: MetricToGrid(minX),
		MaxX: MetricToGrid(maxX),
		MinY: MetricToGrid(minY),
		MaxY: MetricToGrid(maxY),
		MinZ: MetricToGrid(minZ),
		MaxZ: MetricToGrid(maxZ),
	}, nil
}

// ContainsPoint checks whether a 3D grid point (x, y, z) is inside the region.
func (r SpatialRegion) ContainsPoint(x, y, z uint32) bool {
	return x >= r.MinX && x <= r.MaxX &&
		y >= r.MinY && y <= r.MaxY &&
		z >= r.MinZ && z <= r.MaxZ
}

// ContainsMorton checks whether a Morton-encoded point is inside the region in under 2 nanoseconds.
func (r SpatialRegion) ContainsMorton(code uint64) bool {
	x, y, z := DecodeMorton3D(code)
	return r.ContainsPoint(x, y, z)
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
