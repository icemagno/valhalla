package crc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/logger"
)

var (
	logCRC        = logger.NewLogger("CRC")
	logCRCNative  = logger.NewLogger("CRC-Native")
	logCRCDomain  = logger.NewLogger("CRC-Domain")
	logCRCPortico = logger.NewLogger("CRC-Portico")
)

// DefaultPeekTimeout is the default duration to wait for client preamble on the multiplexed port
const DefaultPeekTimeout = 15 * time.Millisecond

// Server represents the Valhalla Central RTI Component (CRC) executive supporting tri-protocol multiplexing
type Server struct {
	addr            string
	porticoAddr     string
	nativeAddr      string
	webAddr         string
	webAddrExplicit bool
	listener        net.Listener
	porticoListener net.Listener
	nativeListener  net.Listener
	fedManager      *FederationManager
	peekTimeout     time.Duration
	webGateway      *WebGateway

	conns   map[net.Conn]struct{}
	connsMu sync.Mutex

	antiStallTimeout  time.Duration
	antiStallInterval time.Duration
	antiStallEnabled  bool

	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	running  bool
}

// NewServer instantiates the Valhalla CRC executive
func NewServer(addr string) *Server {
	if addr == "" {
		addr = ":52295"
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}

	webAddr := ":52296"
	if host, portStr, err := net.SplitHostPort(addr); err == nil {
		if pNum, err := strconv.Atoi(portStr); err == nil {
			if pNum == 0 {
				// Ephemeral port: keep Web Gateway on an ephemeral port (0) as well.
				// Never increment 0 to 1 (which attempts binding to privileged port 1).
				webAddr = net.JoinHostPort(host, "0")
			} else if pNum < 65535 {
				webAddr = net.JoinHostPort(host, strconv.Itoa(pNum+1))
			} else {
				webAddr = net.JoinHostPort(host, "0")
			}
		}
	} else {
		parts := strings.Split(addr, ":")
		if len(parts) == 2 {
			if pNum, err := strconv.Atoi(parts[1]); err == nil {
				if pNum == 0 {
					webAddr = fmt.Sprintf("%s:0", parts[0])
				} else if pNum < 65535 {
					webAddr = fmt.Sprintf("%s:%d", parts[0], pNum+1)
				} else {
					webAddr = fmt.Sprintf("%s:0", parts[0])
				}
			}
		}
	}

	return &Server{
		addr:              addr,
		webAddr:           webAddr,
		webAddrExplicit:   false,
		fedManager:        NewFederationManager(),
		peekTimeout:       DefaultPeekTimeout,
		conns:             make(map[net.Conn]struct{}),
		antiStallTimeout:  2 * time.Second,
		antiStallInterval: 200 * time.Millisecond,
		antiStallEnabled:  true,
		stopChan:          make(chan struct{}),
	}
}

// SetWebAddr configures a custom HTTP/WebSocket port for the HLA-4 Web Profile
func (s *Server) SetWebAddr(webAddr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !strings.Contains(webAddr, ":") {
		webAddr = ":" + webAddr
	}
	s.webAddr = webAddr
	s.webAddrExplicit = true
}

// SetPorticoAddr configures a dedicated TCP port for legacy Portico (JOS) connections, eliminating multiplexing latency
func (s *Server) SetPorticoAddr(porticoAddr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if porticoAddr != "" && !strings.Contains(porticoAddr, ":") {
		porticoAddr = ":" + porticoAddr
	}
	s.porticoAddr = porticoAddr
}

// SetNativeAddr configures a dedicated TCP port for native Valhalla (VAL4) connections, eliminating multiplexing peeking
func (s *Server) SetNativeAddr(nativeAddr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if nativeAddr != "" && !strings.Contains(nativeAddr, ":") {
		nativeAddr = ":" + nativeAddr
	}
	s.nativeAddr = nativeAddr
}

// SetPeekTimeout configures the maximum duration to wait for initial client preamble on the multiplexed port
func (s *Server) SetPeekTimeout(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if timeout > 0 {
		s.peekTimeout = timeout
	}
}

// PeekTimeout returns the configured peek timeout duration
func (s *Server) PeekTimeout() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.peekTimeout
}

// PorticoAddr returns the listening address of the dedicated Portico listener, or nil if not active
func (s *Server) PorticoAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.porticoListener != nil {
		return s.porticoListener.Addr()
	}
	return nil
}

// NativeAddr returns the listening address of the dedicated Native listener, or nil if not active
func (s *Server) NativeAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nativeListener != nil {
		return s.nativeListener.Addr()
	}
	return nil
}

// SetAntiStall configures the stall timeout and monitor interval for the Anti-Stall resilience engine
func (s *Server) SetAntiStall(timeout, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if timeout > 0 {
		s.antiStallTimeout = timeout
	}
	if interval > 0 {
		s.antiStallInterval = interval
	}
}

// EnableAntiStall enables or disables the background anti-stall watchdog
func (s *Server) EnableAntiStall(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.antiStallEnabled = enabled
}

// FederationManager returns the central federation manager
func (s *Server) FederationManager() *FederationManager {
	return s.fedManager
}

// Start begins listening on the configured TCP address and handles concurrent federate connections
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

	if s.porticoAddr != "" {
		pl, err := net.Listen("tcp", s.porticoAddr)
		if err != nil {
			_ = s.listener.Close()
			return fmt.Errorf("crc server: failed to listen on dedicated portico addr %s: %w", s.porticoAddr, err)
		}
		s.porticoListener = pl
		logCRC.Info("Valhalla Dedicated Portico listener active on %s", pl.Addr())
	}

	if s.nativeAddr != "" {
		nl, err := net.Listen("tcp", s.nativeAddr)
		if err != nil {
			_ = s.listener.Close()
			if s.porticoListener != nil {
				_ = s.porticoListener.Close()
			}
			return fmt.Errorf("crc server: failed to listen on dedicated native addr %s: %w", s.nativeAddr, err)
		}
		s.nativeListener = nl
		logCRC.Info("Valhalla Dedicated Native listener active on %s", nl.Addr())
	}

	s.webGateway = NewWebGateway(s.webAddr, s.fedManager)
	if err := s.webGateway.Start(); err != nil {
		logCRC.Error("Failed to start Web Gateway on %s: %v", s.webAddr, err)
		if !s.webAddrExplicit {
			fallbackAddr := ":0"
			if host, _, err := net.SplitHostPort(s.webAddr); err == nil && host != "" {
				fallbackAddr = net.JoinHostPort(host, "0")
			}
			s.webGateway = NewWebGateway(fallbackAddr, s.fedManager)
			if err := s.webGateway.Start(); err != nil {
				logCRC.Error("Failed to start Web Gateway on fallback %s: %v", fallbackAddr, err)
			}
		}
	}

	s.wg.Add(1)
	go s.acceptLoop()

	if s.porticoListener != nil {
		s.wg.Add(1)
		go s.acceptDedicatedPorticoLoop()
	}

	if s.nativeListener != nil {
		s.wg.Add(1)
		go s.acceptDedicatedNativeLoop()
	}

	s.wg.Add(1)
	go s.antiStallMonitorLoop()

	return nil
}

func (s *Server) antiStallMonitorLoop() {
	defer s.wg.Done()

	s.mu.Lock()
	interval := s.antiStallInterval
	s.mu.Unlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.checkStalledFederates()
		}
	}
}

func (s *Server) checkStalledFederates() {
	s.mu.Lock()
	enabled := s.antiStallEnabled
	timeout := s.antiStallTimeout
	s.mu.Unlock()

	if !enabled {
		return
	}

	for _, fed := range s.fedManager.GetAllFederations() {
		evicted, grants := fed.TimeManager().EvictStalledFederates(timeout)
		if len(evicted) > 0 {
			logCRC.Warn("Anti-Stall: Federate(s) %v stalled in federation [%s]; quarantined from LBTS calculations", evicted, fed.Name)
			dispatchTimeGrants(fed, grants)
		}
	}
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
	// Detect dialect by examining initial bytes on the unified port
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

// Stop shuts down the CRC executive gracefully
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	close(s.stopChan)
	err := s.listener.Close()

	if s.porticoListener != nil {
		_ = s.porticoListener.Close()
	}
	if s.nativeListener != nil {
		_ = s.nativeListener.Close()
	}

	if s.webGateway != nil {
		_ = s.webGateway.Stop()
	}

	// Proactively close all active connections to unblock read loops
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

func (s *Server) acceptDedicatedPorticoLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.porticoListener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
				logCRC.Warn("Warning on dedicated Portico accept: %v", err)
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
			remoteAddr := c.RemoteAddr().String()
			logCRC.Info("Dedicated Portico connection from %s", remoteAddr)
			if err := HandlePorticoConnection(c, c, s.fedManager); err != nil {
				logCRCPortico.Info("Dedicated Portico connection %s terminated: %v", remoteAddr, err)
			}
		}(conn)
	}
}

func (s *Server) acceptDedicatedNativeLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.nativeListener.Accept()
		if err != nil {
			select {
			case <-s.stopChan:
				return
			default:
				logCRC.Warn("Warning on dedicated Native accept: %v", err)
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
			remoteAddr := c.RemoteAddr().String()
			logCRC.Info("Dedicated Native connection from %s", remoteAddr)

			// Client-First: check if client sends VAL4 preamble
			br := bufio.NewReader(c)
			wrapped := &MultiplexedConn{Conn: c, Reader: br}
			peekBuf, err := br.Peek(4)
			if err == nil && bytes.Equal(peekBuf, MagicNativeVAL4) {
				_, _ = br.Discard(4)
			}
			if err := HandleNativeConnection(wrapped, wrapped, s.fedManager); err != nil {
				logCRCNative.Info("Dedicated Native connection %s terminated: %v", remoteAddr, err)
			}
		}(conn)
	}
}

// Addr returns the real listener address
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// WebGateway returns the WebGateway instance associated with the server
func (s *Server) WebGateway() *WebGateway {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.webGateway
}

// WebAddr returns the actual listening address of the Web Gateway, or nil if not running
func (s *Server) WebAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.webGateway != nil {
		return s.webGateway.Addr()
	}
	return nil
}
