package crc

import (
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

func TestPorticoSession_ConcurrentIDAndSend_RaceFree(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	// Drain peer side so writes never block
	go func() {
		_, _ = io.Copy(io.Discard, c2)
	}()

	sess := NewPorticoSession(c1)
	defer sess.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := int32(1); i <= 100; i++ {
			sess.SetFederateID(i)
			sess.SetFederationHandle(i % 10)
		}
	}()
	go func() {
		defer wg.Done()
		for i := uint16(1); i <= 100; i++ {
			_ = sess.SendControlResponseOK(i, wire.MsgRtiProbe, []byte{0x01})
			_ = sess.SendControlResponseErr(i, wire.MsgRtiProbe, "err")
		}
	}()
	wg.Wait()
}

func TestFederationExecution_ConcurrentOMTAndObjectReads_RaceFree(t *testing.T) {
	fed := NewFederationExecution(1, "RaceFed")
	fed.SetMerkleAuditEnabled(true)

	cls, err := fed.OMT().AddObjectClass("Aircraft", core.HLAobjectRootHandle)
	if err != nil {
		t.Fatalf("AddObjectClass failed: %v", err)
	}
	attr, err := fed.OMT().AddAttribute(cls.Handle, "Altitude", "HLAfloat64LE")
	if err != nil {
		t.Fatalf("AddAttribute failed: %v", err)
	}

	obj, err := fed.RegisterObjectInstance(int32(cls.Handle), "F16-01", 1)
	if err != nil {
		t.Fatalf("RegisterObjectInstance failed: %v", err)
	}

	var wg sync.WaitGroup
	// Goroutine 1: concurrently add attributes to OMT class (simulating Join-time FOM ingest)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = fed.OMT().AddAttribute(cls.Handle, fmt.Sprintf("DynAttr%d", i), "HLAoctet")
		}
	}()

	// Goroutine 2: concurrently register objects of that class
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = fed.RegisterObjectInstance(int32(cls.Handle), fmt.Sprintf("Obj-%d", i), 1)
		}
	}()

	// Goroutine 3: concurrently update attributes on obj
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = fed.UpdateObjectAttributes(obj.Handle, map[int32][]byte{
				int32(attr.Handle): []byte(fmt.Sprintf("val-%d", i)),
			}, [32]byte{})
		}
	}()

	// Goroutine 4: concurrently read obj via GetObjectInstance & GetAllObjects and inspect Attributes/MerkleRoot
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if snap, ok := fed.GetObjectInstance(obj.Handle); ok {
				_ = snap.MerkleRoot
				for k, v := range snap.Attributes {
					_ = k
					_ = len(v)
				}
			}
			for _, snap := range fed.GetAllObjects() {
				_ = snap.MerkleRoot
				for k, v := range snap.Attributes {
					_ = k
					_ = len(v)
				}
			}
		}
	}()

	wg.Wait()
}

func TestServer_ConcurrentPeekTimeout_RaceFree(t *testing.T) {
	srv := NewServer(":0")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 1; i <= 100; i++ {
			srv.SetPeekTimeout(time.Duration(i) * time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 1; i <= 100; i++ {
			_ = srv.PeekTimeout()
		}
	}()
	wg.Wait()
}
