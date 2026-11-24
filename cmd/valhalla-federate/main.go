// Project Valhalla - Native HLA-4 Federate (IEEE 1516-2025)
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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

	// Instantiate LRC and Callback Handler
	handler := NewCallbackHandler()
	amb := lrc.NewRTIambassador()

	// 1. Connect to CRC
	log.Printf("[1/5] Connecting to Central RTI Component (CRC)...")
	if err := amb.Connect(handler, *addr); err != nil {
		log.Fatalf("[-] Failed to connect to CRC: %v", err)
	}
	defer amb.Disconnect()
	log.Printf("[+] Native connection established successfully via VAL4 preamble!")

	// Background callback evocation goroutine
	stopCallbacks := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopCallbacks:
				return
			default:
				_, _ = amb.EvokeCallback(100 * time.Millisecond)
			}
		}
	}()
	defer close(stopCallbacks)

	// Graceful signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 2. Join Federation Execution
	log.Printf("[2/5] Joining federation execution '%s'...", *fedName)
	fedHandle, err := amb.JoinFederationExecution(*name, *fedName)
	if err != nil {
		log.Fatalf("[-] Failed to join federation: %v", err)
	}
	log.Printf("[+] Joined successfully! Assigned FederateHandle: %d", fedHandle)

	// 3. Synchronization Point
	if *syncPoint != "" {
		log.Printf("[3/5] Achieving synchronization point '%s'...", *syncPoint)
		_ = amb.RegisterFederationSynchronizationPoint(*syncPoint, nil)
		if err := amb.SynchronizationPointAchieved(*syncPoint); err != nil {
			log.Printf("[!] Warning achieving synchronization point: %v", err)
		} else {
			log.Printf("[+] Synchronization point '%s' achieved successfully!", *syncPoint)
		}
	}

	// 4. Object Instance Registration
	log.Printf("[4/5] Registering object instance in simulation...")
	objHandle, err := amb.RegisterObjectInstance(561, fmt.Sprintf("%s_Obj", *name))
	if err != nil {
		log.Printf("[!] Error registering object: %v", err)
	} else {
		log.Printf("[+] Object registered successfully! ObjectHandle: %d", objHandle)
	}

	// 5. Simulation Loop
	log.Printf("[5/5] Federate ready and active in simulation cycle!")
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
			// Publish attribute updates
			if objHandle > 0 {
				x := 100.0 + float64(step)*10.5
				y := 200.0 + float64(step)*5.2
				attrs := map[int32][]byte{
					562: []byte(fmt.Sprintf("X:%.2f", x)),
					551: []byte(fmt.Sprintf("Y:%.2f", y)),
				}
				_ = amb.UpdateAttributeValues(objHandle, attrs, nil)
				log.Printf("[Simulation #%d] Object %d attributes updated (X=%.2f, Y=%.2f)", step, objHandle, x, y)
			}

			// Send periodic interaction
			_ = amb.SendInteraction(525, map[int32][]byte{1: []byte(fmt.Sprintf("HeartbeatStep_%d", step))}, nil)

			// Request logical time advance
			logicalTime := float64(step) * 1.0
			_ = amb.TimeAdvanceRequest(logicalTime)

			step++
		}
	}
}
