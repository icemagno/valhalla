package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/icemagno/valhalla/pkg/crc"
)

func main() {
	port := flag.String("port", ":52295", "Listening TCP port (e.g. 52295 or :52295)")
	flag.Parse()

	addr := *port
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}

	fmt.Println("====================================================================")
	fmt.Println("               VALHALLA - CENTRAL RTI COMPONENT (CRC)")
	fmt.Println("       IEEE 1516-2025 (HLA-4) | High-Performance Simulation Executive")
	fmt.Println("====================================================================")

	server := crc.NewServer(addr)
	if err := server.Start(); err != nil {
		log.Fatalf("[-] Fatal error starting Valhalla CRC: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n[!] Interrupt signal received. Shutting down Valhalla CRC...")
	if err := server.Stop(); err != nil {
		log.Printf("[-] Error during shutdown: %v", err)
	}
	fmt.Println("[+] Valhalla CRC shut down successfully.")
}
