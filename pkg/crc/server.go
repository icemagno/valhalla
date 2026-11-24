package crc

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// Server representa o executivo central (CRC) do Valhalla com suporte a multiplexação de protocolos
type Server struct {
	addr        string
	listener    net.Listener
	fedManager  *FederationManager
	peekTimeout time.Duration

	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	running  bool
}

// NewServer instancia o executivo CRC do Valhalla
func NewServer(addr string) *Server {
	if addr == "" {
		addr = ":52295"
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	return &Server{
		addr:        addr,
		fedManager:  NewFederationManager(),
		peekTimeout: 50 * time.Millisecond,
		stopChan:    make(chan struct{}),
	}
}

// FederationManager retorna o gerenciador central de federações
func (s *Server) FederationManager() *FederationManager {
	return s.fedManager
}

// Start inicia o escutador TCP do CRC na porta configurada e atende conexões concorrentes
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return errors.New("crc server: ja em execucao")
	}

	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("crc server: falha ao escutar em %s: %w", s.addr, err)
	}

	s.listener = l
	s.running = true

	log.Printf("[CRC] Servidor Executivo Valhalla RTI ativo em %s", l.Addr())
	log.Printf("[CRC] Multiplexador Ativo: Protocolo Nativo (VAL4) e Ponte Portico (52295) na mesma porta!")

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
				log.Printf("[CRC] Aviso no accept: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			s.handleConnection(c)
		}(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	// Detectar o protocolo examinando os primeiros bytes na porta unificada
	proto, wrappedConn, err := DetectProtocol(conn, s.peekTimeout)
	if err != nil {
		_ = conn.Close()
		return
	}

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("[CRC] Nova conexao de %s detectada como: %s", remoteAddr, proto)

	switch proto {
	case ProtocolNative:
		if err := HandleNativeConnection(wrappedConn, wrappedConn, s.fedManager); err != nil {
			log.Printf("[CRC-Native] Conexao %s finalizada: %v", remoteAddr, err)
		}
	case ProtocolPortico:
		if err := HandlePorticoConnection(wrappedConn, wrappedConn, s.fedManager); err != nil {
			log.Printf("[CRC-Portico] Conexao %s finalizada: %v", remoteAddr, err)
		}
	}
}

// Stop desliga o executivo CRC graciosamente
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	close(s.stopChan)
	err := s.listener.Close()
	s.mu.Unlock()

	s.wg.Wait()
	log.Printf("[CRC] Servidor Executivo Valhalla encerrado com sucesso.")
	return err
}

// Addr retorna o endereço real em que o servidor está escutando
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}
