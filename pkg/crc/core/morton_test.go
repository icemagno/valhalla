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
	for i := 0; i < b.N; i++ {
		_ = reg.ContainsMorton(code)
	}
}
