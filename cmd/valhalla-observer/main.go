// Project Valhalla - Native HLA-4 Observer Federate (IEEE 1516-2025)
// Passive Telemetry & Event Monitoring Federate over Native VAL4 Protocol
// Lead Developer: Carlos Magno Oliveira de Abreu (@icemagno / magno.mabreu@gmail.com)

package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/icemagno/valhalla/pkg/lrc"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

type ObserverCallbackHandler struct {
	lrc.NullFederateAmbassador
	amb   lrc.RTIambassador
	mu    sync.Mutex
	count int
}

func NewObserverCallbackHandler() *ObserverCallbackHandler {
	return &ObserverCallbackHandler{}
}

func (h *ObserverCallbackHandler) SetAmbassador(amb lrc.RTIambassador) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.amb = amb
}

func (h *ObserverCallbackHandler) AnnounceSynchronizationPoint(label string, tag []byte) {
	now := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s] %s[OBSERVER]%s %s📢 SYNC POINT ANNOUNCED:%s '%s' -> Auto-achieving to unblock federation...\n",
		now, colorCyan, colorReset, colorYellow, colorReset, label)

	h.mu.Lock()
	amb := h.amb
	h.mu.Unlock()

	if amb != nil {
		if err := amb.SynchronizationPointAchieved(label); err != nil {
			fmt.Printf("[%s] %s[OBSERVER]%s [!] Warning achieving sync point '%s': %v\n", now, colorCyan, colorReset, label, err)
		} else {
			fmt.Printf("[%s] %s[OBSERVER]%s %s✓ Sync point '%s' achieved successfully!%s\n", now, colorCyan, colorReset, colorGreen, label, colorReset)
		}
	}
}

func (h *ObserverCallbackHandler) FederationSynchronized(label string) {
	now := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s] %s[OBSERVER]%s %s🌐 FEDERATION FULLY SYNCHRONIZED at point '%s'!%s\n",
		now, colorCyan, colorReset, colorGreen+colorBold, label, colorReset)
}

func (h *ObserverCallbackHandler) DiscoverObjectInstance(theObject int32, theObjectClass int32, objectName string) {
	now := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s] %s[OBSERVER]%s %s🎯 DISCOVER OBJECT%s -> Handle: %d | Class: %d | Name: '%s'\n",
		now, colorCyan, colorReset, colorGreen+colorBold, colorReset, theObject, theObjectClass, objectName)
}

func formatBytes(b []byte) string {
	if len(b) == 0 {
		return "[] (empty)"
	}
	// Try big-endian numbers for common HLA sizes
	numStr := ""
	switch len(b) {
	case 2:
		numStr = fmt.Sprintf(" (int16: %d)", int16(binary.BigEndian.Uint16(b)))
	case 4:
		numStr = fmt.Sprintf(" (int32: %d)", int32(binary.BigEndian.Uint32(b)))
	case 8:
		numStr = fmt.Sprintf(" (int64: %d)", int64(binary.BigEndian.Uint64(b)))
	}

	// Check if printable ASCII/UTF-8 string
	isPrintable := true
	for _, c := range string(b) {
		if !unicode.IsPrint(c) && c != '\r' && c != '\n' && c != '\t' {
			isPrintable = false
			break
		}
	}
	if isPrintable && len(b) > 0 {
		return fmt.Sprintf("'%s'%s", string(b), numStr)
	}
	return fmt.Sprintf("hex:%x%s", b, numStr)
}

func (h *ObserverCallbackHandler) ReflectAttributeValues(theObject int32, theAttributes map[int32][]byte, tag []byte) {
	h.mu.Lock()
	h.count++
	evNum := h.count
	h.mu.Unlock()

	now := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s] %s[OBSERVER]%s %s📦 REFLECT ATTRIBUTES [#%02d]%s for Object #%d (%d attrs):\n",
		now, colorCyan, colorReset, colorPurple+colorBold, evNum, colorReset, theObject, len(theAttributes))
	for k, v := range theAttributes {
		fmt.Printf("           %s-> Attr[%d]%s = %s\n", colorBlue, k, colorReset, formatBytes(v))
	}
}

func (h *ObserverCallbackHandler) ReceiveInteraction(interactionClass int32, theParameters map[int32][]byte, tag []byte) {
	now := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s] %s[OBSERVER]%s %s⚡ RECEIVE INTERACTION%s -> Class: %d (%d params):\n",
		now, colorCyan, colorReset, colorYellow+colorBold, colorReset, interactionClass, len(theParameters))
	for k, v := range theParameters {
		fmt.Printf("           %s-> Param[%d]%s = %s\n", colorBlue, k, colorReset, formatBytes(v))
	}
}

func (h *ObserverCallbackHandler) RemoveObjectInstance(theObject int32, tag []byte) {
	now := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s] %s[OBSERVER]%s %s🗑️ REMOVE OBJECT%s -> Handle #%d removed from simulation\n",
		now, colorCyan, colorReset, colorYellow+colorBold, colorReset, theObject)
}

func main() {
	addr := flag.String("address", "127.0.0.1:52295", "CRC Server address (ip:port)")
	fedName := flag.String("federation", "", "Federation Execution name to monitor (empty or 'auto' for dynamic discovery)")
	name := flag.String("name", "ValhallaObserver", "Federate name")
	flag.Parse()

	targetAddr := *addr
	if !strings.Contains(targetAddr, ":") {
		targetAddr = ":" + targetAddr
	}

	targetFedDisplay := *fedName
	if targetFedDisplay == "" || targetFedDisplay == "auto" {
		targetFedDisplay = "[DYNAMIC AUTO-DISCOVERY] Any Active Federation"
	}

	fmt.Println("====================================================================")
	fmt.Println("            PROJECT VALHALLA - NATIVE FEDERATE OBSERVER")
	fmt.Println("       Passive Real-Time Telemetry & Monitoring (IEEE 1516-2025)")
	fmt.Println("====================================================================")
	fmt.Printf("Connecting to CRC at: %s\n", targetAddr)
	fmt.Printf("Federation Target  : %s\n", targetFedDisplay)
	fmt.Printf("Observer Federate  : %s (Protocol: VAL4 Native)\n\n", *name)

	handler := NewObserverCallbackHandler()
	amb := lrc.NewRTIambassador()
	handler.SetAmbassador(amb)

	// 1. Connect to CRC via Native VAL4 Preamble
	fmt.Println("[1/4] Connecting to Valhalla Central RTI Component (CRC)...")
	if err := amb.Connect(handler, targetAddr); err != nil {
		fmt.Printf("[-] Failed to connect to CRC at %s: %v\n", targetAddr, err)
		os.Exit(1)
	}
	defer amb.Disconnect()
	fmt.Println("[+] Successfully connected to Valhalla CRC via VAL4 native channel!")

	// Background callback evoker loop
	stopCallbacks := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopCallbacks:
				return
			default:
				_, _ = amb.EvokeCallback(50 * time.Millisecond)
			}
		}
	}()
	defer close(stopCallbacks)

	// Graceful termination handling (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 2. Join Federation Execution (with auto-retry until a federation is created by simulator)
	fmt.Printf("[2/4] Joining federation execution '%s'...\n", targetFedDisplay)
	var fedHandle int32
	for {
		h, err := amb.JoinFederationExecution(*name, *fedName)
		if err == nil {
			fedHandle = h
			break
		}
		if *fedName == "" || *fedName == "auto" {
			fmt.Println("      Waiting for ANY active federation to be created by simulator... (retrying)")
		} else {
			fmt.Printf("      Waiting for federation '%s' to be created by simulator... (retrying)\n", *fedName)
		}
		select {
		case <-sigChan:
			fmt.Println("\n[!] Observer cancelled before federation was created.")
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
	fmt.Printf("[+] Successfully attached to federation! Assigned Federate Handle: %d\n", fedHandle)

	// 3. Subscribe to FOM classes (Universal dynamic subscriptions)
	fmt.Println("[3/4] Subscribing to simulation object and interaction classes...")
	// Root object class + TacticalEntity (1001) + Soda (1034)
	_ = amb.SubscribeObjectClassAttributes(1, []int32{1})
	_ = amb.SubscribeObjectClassAttributes(1001, []int32{1, 1001, 1002, 1003, 1004})
	_ = amb.SubscribeObjectClassAttributes(1034, []int32{1, 1008, 1009})
	// Root interaction class + SensorTrack (1001) + MissionControl (1002) + Interaction (1007)
	_ = amb.SubscribeInteractionClass(1)
	_ = amb.SubscribeInteractionClass(1001)
	_ = amb.SubscribeInteractionClass(1002)
	_ = amb.SubscribeInteractionClass(1007)
	fmt.Println("[+] Subscriptions active: [TacticalEntity (1001), Soda (1034), SensorTrack (1001), MissionControl (1002), Interaction (1007)]")

	// 4. Enter passive listening loop
	fmt.Println("\n[4/4] 🚀 Observer is ACTIVE and waiting for real-time simulation events!")
	fmt.Println(">>> Press Ctrl+C to resign and exit cleanly <<<")
	fmt.Println()

	<-sigChan
	fmt.Println("\n[!] Termination signal received. Resigning observer from federation...")
	if err := amb.ResignFederationExecution(); err != nil {
		fmt.Printf("[-] Error resigning: %v\n", err)
	} else {
		fmt.Println("[+] Observer resigned successfully. Disconnecting.")
	}
}
