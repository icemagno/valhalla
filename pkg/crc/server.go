package crc

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/logger"
)

var (
	logCRC        = logger.NewLogger("CRC")
	logCRCNative  = logger.NewLogger("CRC-Native")
	logCRCPortico = logger.NewLogger("CRC-Portico")
)

// Server representa o executivo central (CRC) do Valhalla com suporte a multiplexação de protocolos
type Server struct {
	addr        string
	listener    net.Listener
	fedManager  *FederationManager
	peekTimeout time.Duration

	conns   map[net.Conn]struct{}
	connsMu sync.Mutex

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
		conns:       make(map[net.Conn]struct{}),
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
		return errors.New("crc server: already running")
	}

	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("crc server: failed to listen on %s: %w", s.addr, err)
	}

	s.listener = l
	s.running = true

	logCRC.Info("Valhalla RTI Executive active on %s", l.Addr())

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
				logCRC.Warn("Warning on accept: %v", err)
				continue
			}
		}

		s.connsMu.Lock()
		s.conns[conn] = struct{}{}
		s.connsMu.Unlock()

		s.wg.Add(1)
		go func(c net.Conn) {
			defer func() {
				s.connsMu.Lock()
				delete(s.conns, c)
				s.connsMu.Unlock()
				_ = c.Close()
				s.wg.Done()
			}()
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
	logCRC.Info("Connection from %s identified as: %s", remoteAddr, proto)

	switch proto {
	case ProtocolNative:
		if err := HandleNativeConnection(wrappedConn, wrappedConn, s.fedManager); err != nil {
			logCRCNative.Info("Connection %s terminated: %v", remoteAddr, err)
		}
	case ProtocolPortico:
		if err := HandlePorticoConnection(wrappedConn, wrappedConn, s.fedManager); err != nil {
			logCRCPortico.Info("Connection %s terminated: %v", remoteAddr, err)
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

	// Fechar proativamente todas as conexões ativas para desbloquear os loops de leitura
	s.connsMu.Lock()
	for c := range s.conns {
		_ = c.Close()
	}
	s.connsMu.Unlock()

	s.mu.Unlock()

	s.wg.Wait()
	logCRC.Info("Valhalla RTI Executive shut down successfully.")
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
