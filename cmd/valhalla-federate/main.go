// Project Valhalla - Native HLA-4 Federate (IEEE 1516-2025)
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/lrc"
)

type FederateCallbackHandler struct {
	lrc.NullFederateAmbassador
	mu             sync.Mutex
	isSynchronized bool
	syncedCond     *sync.Cond
	timeGranted    float64
	timeCond       *sync.Cond
}

func NewCallbackHandler() *FederateCallbackHandler {
	h := &FederateCallbackHandler{}
	h.syncedCond = sync.NewCond(&h.mu)
	h.timeCond = sync.NewCond(&h.mu)
	return h
}

func (h *FederateCallbackHandler) FederationSynchronized(label string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.isSynchronized = true
	log.Printf("[Callback] >>> FEDERATION FULLY SYNCHRONIZED at point '%s'!", label)
	h.syncedCond.Broadcast()
}

func (h *FederateCallbackHandler) DiscoverObjectInstance(theObject int32, theObjectClass int32, objectName string) {
	log.Printf("[Callback] >>> OBJECT DISCOVERED: handle=%d, class=%d, name='%s'", theObject, theObjectClass, objectName)
}

func (h *FederateCallbackHandler) ReflectAttributeValues(theObject int32, theAttributes map[int32][]byte, tag []byte) {
	log.Printf("[Callback] >>> ATTRIBUTES REFLECTED for object %d: %d attributes updated", theObject, len(theAttributes))
	for k, v := range theAttributes {
		log.Printf("           -> Attr[%d] = %s", k, string(v))
	}
}

func (h *FederateCallbackHandler) ReceiveInteraction(interactionClass int32, theParameters map[int32][]byte, tag []byte) {
	log.Printf("[Callback] >>> INTERACTION RECEIVED for class %d: %d parameters", interactionClass, len(theParameters))
	for k, v := range theParameters {
		log.Printf("           -> Param[%d] = %s", k, string(v))
	}
}

func (h *FederateCallbackHandler) TimeAdvanceGrant(grantedTime float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.timeGranted = grantedTime
	log.Printf("[Callback] >>> TIME ADVANCE GRANTED (TAG): %.2f", grantedTime)
	h.timeCond.Broadcast()
}

func (h *FederateCallbackHandler) WaitForGrant(target float64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	h.mu.Lock()
	defer h.mu.Unlock()
	for h.timeGranted+1e-9 < target {
		if time.Now().After(deadline) {
			return false
		}
		h.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		h.mu.Lock()
	}
	return true
}

func main() {
	addr := flag.String("address", "127.0.0.1:52295", "CRC Server address (ip:port)")
	fedName := flag.String("federation", "ExampleFederation", "Federation Execution name")
	name := flag.String("name", fmt.Sprintf("Valhalla_%d", os.Getpid()), "Federate name")
	syncPoint := flag.String("sync", "ReadyToRun", "Initial synchronization point label")
	flag.Parse()

	fmt.Println("====================================================================")
	fmt.Println("           PROJECT VALHALLA - NATIVE HLA-4 FEDERATE (LRC)")
	fmt.Println("====================================================================")
	fmt.Printf("Connecting to CRC at: %s\n", *addr)
	fmt.Printf("Federation: %s | Federate: %s (Native VAL4)\n\n", *fedName, *name)

	handler := NewCallbackHandler()
	amb := lrc.NewRTIambassador()

	// 1. Connect to CRC
	log.Printf("[1/6] Connecting to Central RTI Component (CRC)...")
	if err := amb.Connect(handler, *addr); err != nil {
		log.Fatalf("[-] Failed to connect to CRC: %v", err)
	}
	defer func() {
		if err := amb.Disconnect(); err != nil {
			log.Printf("[!] Disconnect warning: %v", err)
		}
	}()
	log.Printf("[+] Native connection established successfully via VAL4 preamble!")

	// Background callback evocation goroutine
	stopCallbacks := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopCallbacks:
				return
			default:
				if _, err := amb.EvokeCallback(100 * time.Millisecond); err != nil && !errors.Is(err, lrc.ErrNotConnected) {
					log.Printf("[!] EvokeCallback error: %v", err)
				}
			}
		}
	}()
	defer close(stopCallbacks)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 2. Create (if needed) & Join Federation Execution
	log.Printf("[2/6] Ensuring federation execution '%s' exists...", *fedName)
	if err := amb.CreateFederationExecution(*fedName); err != nil && !errors.Is(err, common.ErrFederationExecutionAlreadyExists) {
		log.Fatalf("[-] Failed to create federation: %v", err)
	}

	fedHandle, err := amb.JoinFederationExecution(*name, *fedName)
	if err != nil {
		log.Fatalf("[-] Failed to join federation: %v", err)
	}
	log.Printf("[+] Joined successfully! Assigned FederateHandle: %d", fedHandle)

	// 3. Synchronization Point
	if *syncPoint != "" {
		log.Printf("[3/6] Registering/achieving synchronization point '%s'...", *syncPoint)
		if err := amb.RegisterFederationSynchronizationPoint(*syncPoint, nil); err != nil {
			log.Printf("[i] Sync point registration notice: %v", err)
		}
		if err := amb.SynchronizationPointAchieved(*syncPoint); err != nil {
			log.Printf("[!] Warning achieving synchronization point: %v", err)
		} else {
			log.Printf("[+] Synchronization point '%s' achieved successfully!", *syncPoint)
		}
	}

	// 4. Enable Time Management (Regulating + Constrained)
	log.Printf("[4/6] Enabling HLA Time Management (lookahead=1.0)...")
	if err := amb.EnableTimeRegulation(1.0); err != nil {
		log.Fatalf("[-] Failed to enable time regulation: %v", err)
	}
	if err := amb.EnableTimeConstrained(); err != nil {
		log.Fatalf("[-] Failed to enable time constrained: %v", err)
	}

	// 5. Publish & Register Object Instance using canonical Standard MIM handles
	rootClass := int32(common.HLAobjectRootHandle)
	privAttr := int32(common.HLAprivilegeToDeleteObjectHandle)
	interClass := int32(common.HLAinteractionRootHandle)

	if err := amb.PublishObjectClassAttributes(rootClass, []int32{privAttr}); err != nil {
		log.Fatalf("[-] Failed to publish HLAobjectRoot: %v", err)
	}
	if err := amb.PublishInteractionClass(interClass); err != nil {
		log.Fatalf("[-] Failed to publish HLAinteractionRoot: %v", err)
	}

	log.Printf("[5/6] Registering object instance in simulation...")
	objHandle, err := amb.RegisterObjectInstance(rootClass, fmt.Sprintf("%s_Obj", *name))
	if err != nil {
		log.Fatalf("[-] Error registering object: %v", err)
	}
	log.Printf("[+] Object registered successfully! ObjectHandle: %d", objHandle)

	// 6. Simulation Loop
	log.Printf("[6/6] Federate ready and active in simulation cycle!")
	fmt.Println(">>> Press Ctrl+C to resign and terminate federate gracefully <<<")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	step := 1
	for {
		select {
		case <-sigChan:
			fmt.Println("\n[!] Termination signal received. Resigning from federation...")
			if err := amb.ResignFederationExecution(); err != nil {
				log.Printf("[-] Error resigning: %v", err)
			} else {
				log.Printf("[+] Resigned successfully!")
			}
			return

		case <-ticker.C:
			attrs := map[int32][]byte{
				privAttr: []byte(fmt.Sprintf("Step:%d", step)),
			}
			if err := amb.UpdateAttributeValues(objHandle, attrs, nil); err != nil {
				log.Printf("[-] UpdateAttributeValues failed: %v", err)
			} else {
				log.Printf("[Simulation #%d] Object %d attributes updated", step, objHandle)
			}

			if err := amb.SendInteraction(interClass, nil, []byte(fmt.Sprintf("HeartbeatStep_%d", step))); err != nil {
				log.Printf("[-] SendInteraction failed: %v", err)
			}

			logicalTime := float64(step) * 1.0
			if err := amb.TimeAdvanceRequest(logicalTime); err != nil {
				log.Printf("[-] TimeAdvanceRequest(%.2f) failed: %v", logicalTime, err)
			} else if !handler.WaitForGrant(logicalTime, 5*time.Second) {
				log.Printf("[!] Timeout waiting for TimeAdvanceGrant(%.2f)", logicalTime)
			}

			step++
		}
	}
}
