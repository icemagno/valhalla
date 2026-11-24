package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/icemagno/valhalla/pkg/crc"
)

func main() {
	port := flag.String("port", ":52295", "Porta TCP de escuta unificada (Nativo VAL4 + Ponte Portico)")
	flag.Parse()

	fmt.Println("====================================================================")
	fmt.Println("       PROJETO VALHALLA - CENTRAL RTI COMPONENT (CRC)")
	fmt.Println("       IEEE 1516-2025 (HLA-4) | Executivo de Simulacao Soberano")
	fmt.Println("====================================================================")
	fmt.Printf("[+] Inicializando Valhalla CRC na porta unificada: %s\n", *port)
	fmt.Println("[+] Protocolos Suportados na MESMA PORTA:")
	fmt.Println("    1. Valhalla Nativo (HLA-4 / SSZ / Merkle Root / PQC Kyber)")
	fmt.Println("    2. Portico Legado  (JOS / Header 12b / 0xCAFE / 0xBEEF)")
	fmt.Println("====================================================================")

	server := crc.NewServer(*port)
	if err := server.Start(); err != nil {
		log.Fatalf("[-] Erro fatal iniciando Valhalla CRC: %v", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n[!] Sinal de interrupcao recebido. Desligando Valhalla CRC...")
	if err := server.Stop(); err != nil {
		log.Printf("[-] Erro durante desligamento: %v", err)
	}
	fmt.Println("[+] Valhalla CRC encerrado com sucesso.")
}
