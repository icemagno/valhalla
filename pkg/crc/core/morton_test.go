package core

import (
	"math/rand"
	"testing"
)

func TestMortonEncodeDecode(t *testing.T) {
	testCases := []struct {
		x, y, z uint32
	}{
		{0, 0, 0},
		{1, 1, 1},
		{MaxCoordinateValue, MaxCoordinateValue, MaxCoordinateValue},
		{12345, 67890, 54321},
		{1048576, 524288, 262144},
		{MaxCoordinateValue, 0, MaxCoordinateValue},
	}

	for _, tc := range testCases {
		code := EncodeMorton3D(tc.x, tc.y, tc.z)
		rx, ry, rz := DecodeMorton3D(code)

		if rx != tc.x || ry != tc.y || rz != tc.z {
			t.Fatalf("Morton mismatch: expected (%d, %d, %d), got (%d, %d, %d) for code %016x",
				tc.x, tc.y, tc.z, rx, ry, rz, code)
		}
	}
}

func TestECEFConversion(t *testing.T) {
	coords := []struct {
		x, y, z float64
	}{
		{0.0, 0.0, 0.0},
		{6371000.0, 0.0, 0.0},             // Surface of Earth at Equator
		{0.0, 6371000.0, 400000.0},        // ISS LEO orbit (400 km)
		{-6371000.0, -6371000.0, 10000.0}, // Aircraft altitude (10 km)
	}

	for _, c := range coords {
		code := EncodeECEF(c.x, c.y, c.z)
		rx, ry, rz := DecodeECEF(code)

		// Max coordinate resolution across 20,000 km with 21 bits is ~9.53 m
		diffX := rx - c.x
		diffY := ry - c.y
		diffZ := rz - c.z
		if diffX < 0 {
			diffX = -diffX
		}
		if diffY < 0 {
			diffY = -diffY
		}
		if diffZ < 0 {
			diffZ = -diffZ
		}

		if diffX > 10.0 || diffY > 10.0 || diffZ > 10.0 {
			t.Fatalf("ECEF quantization error too high: expected (%.1f, %.1f, %.1f), got (%.1f, %.1f, %.1f)",
				c.x, c.y, c.z, rx, ry, rz)
		}
	}
}

func TestSpatialRegionContainment(t *testing.T) {
	reg, err := NewSpatialRegionFromMetric(-100000.0, 100000.0, -100000.0, 100000.0, 0.0, 50000.0)
	if err != nil {
		t.Fatalf("failed to create region: %v", err)
	}

	insideCode := EncodeECEF(0.0, 0.0, 25000.0)
	outsideCode := EncodeECEF(200000.0, 0.0, 25000.0)

	if !reg.ContainsMorton(insideCode) {
		t.Errorf("expected point to be inside region")
	}
	if reg.ContainsMorton(outsideCode) {
		t.Errorf("expected point to be outside region")
	}
}

// TestMortonBitwiseEquivalence rigorously validates that ContainsMorton produces
// the exact same result as ContainsPoint across 100,000 random points and boundary edges,
// proving that parallel bitwise coordinate comparison in Morton space is mathematically identical.
func TestMortonBitwiseEquivalence(t *testing.T) {
	// Create regions of various sizes and positions
	testRegions := []SpatialRegion{
		func() SpatialRegion { r, _ := NewSpatialRegion(100, 500, 200, 800, 300, 900); return r }(),
		func() SpatialRegion { r, _ := NewSpatialRegion(0, 10000, 0, 10000, 0, 10000); return r }(),
		func() SpatialRegion {
			r, _ := NewSpatialRegion(500000, 1500000, 200000, 800000, 100000, 900000)
			return r
		}(),
		func() SpatialRegion {
			r, _ := NewSpatialRegionFromMetric(-50000.0, 50000.0, -50000.0, 50000.0, 1000.0, 20000.0)
			return r
		}(),
	}

	for regIdx, reg := range testRegions {
		// 1. Test exact boundaries (corners, faces, edges)
		boundaryPoints := [][3]uint32{
			{reg.MinX, reg.MinY, reg.MinZ},
			{reg.MaxX, reg.MaxY, reg.MaxZ},
			{reg.MinX, reg.MaxY, reg.MinZ},
			{reg.MaxX, reg.MinY, reg.MaxZ},
			{(reg.MinX + reg.MaxX) / 2, (reg.MinY + reg.MaxY) / 2, (reg.MinZ + reg.MaxZ) / 2},
		}

		for _, pt := range boundaryPoints {
			code := EncodeMorton3D(pt[0], pt[1], pt[2])
			expected := reg.ContainsPoint(pt[0], pt[1], pt[2])
			actual := reg.ContainsMorton(code)
			if actual != expected {
				t.Fatalf("[Reg %d] Boundary point (%d, %d, %d) mismatch: expected %v, got %v",
					regIdx, pt[0], pt[1], pt[2], expected, actual)
			}
		}

		// 2. Test off-by-one points outside boundary
		offPoints := [][3]uint32{}
		if reg.MinX > 0 {
			offPoints = append(offPoints, [3]uint32{reg.MinX - 1, reg.MinY, reg.MinZ})
		}
		if reg.MaxX < MaxCoordinateValue {
			offPoints = append(offPoints, [3]uint32{reg.MaxX + 1, reg.MinY, reg.MinZ})
		}
		if reg.MinY > 0 {
			offPoints = append(offPoints, [3]uint32{reg.MinX, reg.MinY - 1, reg.MinZ})
		}
		if reg.MaxY < MaxCoordinateValue {
			offPoints = append(offPoints, [3]uint32{reg.MinX, reg.MaxY + 1, reg.MinZ})
		}
		if reg.MinZ > 0 {
			offPoints = append(offPoints, [3]uint32{reg.MinX, reg.MinY, reg.MinZ - 1})
		}
		if reg.MaxZ < MaxCoordinateValue {
			offPoints = append(offPoints, [3]uint32{reg.MinX, reg.MinY, reg.MaxZ + 1})
		}

		for _, pt := range offPoints {
			code := EncodeMorton3D(pt[0], pt[1], pt[2])
			expected := reg.ContainsPoint(pt[0], pt[1], pt[2])
			actual := reg.ContainsMorton(code)
			if actual != expected {
				t.Fatalf("[Reg %d] Off-by-one point (%d, %d, %d) mismatch: expected %v, got %v",
					regIdx, pt[0], pt[1], pt[2], expected, actual)
			}
		}

		// 3. Test 50,000 random points
		rng := rand.New(rand.NewSource(int64(42 + regIdx)))
		for i := 0; i < 50000; i++ {
			x := rng.Uint32() & MaxCoordinateValue
			y := rng.Uint32() & MaxCoordinateValue
			z := rng.Uint32() & MaxCoordinateValue

			code := EncodeMorton3D(x, y, z)
			expected := reg.ContainsPoint(x, y, z)
			actual := reg.ContainsMorton(code)

			if actual != expected {
				t.Fatalf("[Reg %d] Random point (%d, %d, %d) mismatch: expected %v, got %v for code %016x",
					regIdx, x, y, z, expected, actual, code)
			}
		}
	}
}

func TestSpatialRegionOverlap(t *testing.T) {
	r1, _ := NewSpatialRegionFromMetric(0, 10000, 0, 10000, 0, 1000)
	r2, _ := NewSpatialRegionFromMetric(5000, 15000, 5000, 15000, 500, 1500)
	r3, _ := NewSpatialRegionFromMetric(20000, 30000, 20000, 30000, 0, 1000)

	if !r1.Overlaps(r2) {
		t.Errorf("expected r1 and r2 to overlap")
	}
	if r1.Overlaps(r3) {
		t.Errorf("expected r1 and r3 not to overlap")
	}
}

func BenchmarkMortonEncode(b *testing.B) {
	x, y, z := uint32(500000), uint32(1000000), uint32(750000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = EncodeMorton3D(x, y, z)
	}
}

func BenchmarkMortonDecode(b *testing.B) {
	code := EncodeMorton3D(500000, 1000000, 750000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = DecodeMorton3D(code)
	}
}

func BenchmarkSpatialRegionContainsMorton(b *testing.B) {
	reg, _ := NewSpatialRegionFromMetric(-100000.0, 100000.0, -100000.0, 100000.0, 0.0, 50000.0)
	code := EncodeECEF(rand.Float64()*100000, rand.Float64()*100000, 25000.0)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = reg.ContainsMorton(code)
	}
}

func BenchmarkSpatialRegionContainsMorton_BranchReject(b *testing.B) {
	reg, _ := NewSpatialRegionFromMetric(-100000.0, 100000.0, -100000.0, 100000.0, 0.0, 50000.0)
	// Point in completely different octant
	code := EncodeECEF(5000000.0, 5000000.0, 5000000.0)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = reg.ContainsMorton(code)
	}
}

func BenchmarkSpatialRegionContainsPoint(b *testing.B) {
	reg, _ := NewSpatialRegionFromMetric(-100000.0, 100000.0, -100000.0, 100000.0, 0.0, 50000.0)
	x, y, z := MetricToGrid(50000.0), MetricToGrid(50000.0), MetricToGrid(25000.0)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = reg.ContainsPoint(x, y, z)
	}
}
