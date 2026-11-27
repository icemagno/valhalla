package core

import (
	"sync"
	"testing"
)

func TestSpatialRegion_ContainsMorton_ConcurrentRaceFree(t *testing.T) {
	// Struct literal with boundsComputed == false (previously mutated r on first read!)
	r := &SpatialRegion{
		MinX: 10, MaxX: 100,
		MinY: 10, MaxY: 100,
		MinZ: 10, MaxZ: 100,
	}
	inside := EncodeMorton3D(50, 50, 50)
	outside := EncodeMorton3D(500, 500, 500)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if !r.ContainsMorton(inside) {
					t.Errorf("expected inside point to be contained")
				}
				if r.ContainsMorton(outside) {
					t.Errorf("expected outside point not to be contained")
				}
			}
		}()
	}
	wg.Wait()
}
