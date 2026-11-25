package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc"
	"github.com/icemagno/valhalla/pkg/logger"
)

func main() {
	defer logger.Flush()

	port := flag.String("port", ":52295", "Listening TCP port (e.g. 52295 or :52295)")
	webPort := flag.String("web-port", "", "Listening HTTP/WebSocket port for HLA-4 Web Profile (defaults to port+1)")
	flag.Parse()

	addr := *port
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}

	fmt.Println("====================================================================")
	fmt.Println("               VALHALLA - CENTRAL RTI COMPONENT (CRC)")
	fmt.Printf("       Version: %s (%s) | Standard: %s\n", common.Version, common.ReleaseTag, common.StandardVersion)
	fmt.Println("====================================================================")

	logMain := logger.NewLogger("CRC")

	server := crc.NewServer(addr)
	if *webPort != "" {
		server.SetWebAddr(*webPort)
	}
	if err := server.Start(); err != nil {
		logMain.Critical("Fatal error starting Valhalla CRC: %v", err)
		logger.Flush()
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n[!] Interrupt signal received. Shutting down Valhalla CRC...")
	if err := server.Stop(); err != nil {
		logMain.Error("Error during shutdown: %v", err)
	}
	logger.Flush()
	fmt.Println("[+] Valhalla CRC shut down successfully.")
}
