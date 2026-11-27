// web_gateway.go - RFC 6455 WebSocket Gateway & HTTP Dashboard for Valhalla HLA-4 Web Profile
package crc

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/logger"
)

var logCRCWeb = logger.NewLogger("CRC-Web")

// WebGateway provides standard HTTP & WebSocket endpoints for the HLA-4 Web Profile (IEEE 1516-2025)
type WebGateway struct {
	addr       string
	fedManager *FederationManager
	server     *http.Server
	listener   net.Listener

	mu       sync.Mutex
	sessions map[*WebSession]struct{}
	running  bool
}

// NewWebGateway instantiates a new WebGateway
func NewWebGateway(addr string, fedManager *FederationManager) *WebGateway {
	if addr == "" {
		addr = ":52296"
	}
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	return &WebGateway{
		addr:       addr,
		fedManager: fedManager,
		sessions:   make(map[*WebSession]struct{}),
	}
}

// Start begins listening on the configured HTTP/WebSocket address
func (g *WebGateway) Start() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running {
		return fmt.Errorf("web gateway: already running")
	}

	l, err := net.Listen("tcp", g.addr)
	if err != nil {
		return fmt.Errorf("web gateway: listen error on %s: %w", g.addr, err)
	}
	g.listener = l

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", g.handleWebSocket)
	mux.HandleFunc("/api/v1/status", g.handleStatus)
	mux.HandleFunc("/", g.handleDashboard)

	g.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	g.running = true
	logCRCWeb.Info("HLA-4 Web Profile Gateway (WebSocket & Dashboard) active on %s", l.Addr())

	go func() {
		if err := g.server.Serve(l); err != nil && err != http.ErrServerClosed {
			logCRCWeb.Error("Web gateway HTTP server error: %v", err)
		}
	}()

	return nil
}

// Addr returns the real network listener address of the WebGateway
func (g *WebGateway) Addr() net.Addr {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.listener != nil {
		return g.listener.Addr()
	}
	return nil
}

// Stop gracefully shuts down the Web gateway and terminates active WebSocket client connections
// without holding g.mu during blocking socket Close() calls.
func (g *WebGateway) Stop() error {
	g.mu.Lock()
	if !g.running {
		g.mu.Unlock()
		return nil
	}
	g.running = false

	sessions := make([]*WebSession, 0, len(g.sessions))
	for s := range g.sessions {
		sessions = append(sessions, s)
	}
	g.sessions = make(map[*WebSession]struct{})
	srv := g.server
	g.mu.Unlock()

	for _, s := range sessions {
		_ = s.Close()
	}

	if srv != nil {
		_ = srv.Close()
	}
	return nil
}

// handleStatus returns current federation status as JSON
func (g *WebGateway) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	feds := g.fedManager.ListFederations()
	resp := map[string]interface{}{
		"server":      "Valhalla Central RTI Component (CRC)",
		"version":     common.Version,
		"releaseTag":  common.ReleaseTag,
		"standard":    common.StandardVersion,
		"protocols":   []string{"VAL4-Native", "Portico-JOS", "HLA4-WebProfile"},
		"federations": feds,
		"timestamp":   time.Now().Unix(),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logCRCWeb.Warn("failed to encode JSON status: %v", err)
	}
}

// handleDashboard renders a high-performance tactical monitoring dashboard
func (g *WebGateway) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(valhallaDashboardHTML)); err != nil {
		logCRCWeb.Warn("failed to write dashboard HTML: %v", err)
	}
}

// handleWebSocket performs RFC 6455 handshake and dispatches incoming HLA requests
func (g *WebGateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
		http.Error(w, "Expected WebSocket Upgrade", http.StatusBadRequest)
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "Missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	acceptKey := computeWebSocketAccept(key)
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"

	if _, err := bufrw.WriteString(response); err != nil {
		_ = conn.Close()
		return
	}
	if err := bufrw.Flush(); err != nil {
		_ = conn.Close()
		return
	}

	session := NewWebSession(conn)

	g.mu.Lock()
	g.sessions[session] = struct{}{}
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		delete(g.sessions, session)
		g.mu.Unlock()
		_ = session.Close()
	}()

	logCRCWeb.Info("New HLA-4 WebSocket client connected from %s", conn.RemoteAddr())

	var currentFed *FederationExecution
	var federateID int32

	defer func() {
		if currentFed != nil && federateID > 0 {
			if _, err := currentFed.Resign(federateID); err != nil {
				logCRCWeb.Warn("failed to resign web federate %d on disconnect: %v", federateID, err)
			}
			logCRCWeb.Info("Web federate %d resigned upon WebSocket disconnect", federateID)
		}
	}()

	for {
		opcode, payload, err := readWebSocketFrame(bufrw)
		if err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "closed") {
				logCRCWeb.Debug("WebSocket client read error: %v", err)
			}
			break
		}

		if opcode == 8 { // Close frame
			break
		} else if opcode == 9 { // Ping -> respond with RFC 6455 Pong (0x8A) under session.mu
			if err := session.SendPong(payload); err != nil {
				logCRCWeb.Warn("failed to write WebSocket pong frame: %v", err)
			}
			continue
		} else if opcode != 1 { // Not text frame
			continue
		}

		var req struct {
			ID     int64                  `json:"id"`
			Action string                 `json:"action"`
			Params map[string]interface{} `json:"params"`
		}

		if err := json.Unmarshal(payload, &req); err != nil {
			if sendErr := session.SendJSON(map[string]interface{}{
				"id":     0,
				"status": "ERROR",
				"error":  "Invalid JSON payload: " + err.Error(),
			}); sendErr != nil {
				logCRCWeb.Warn("failed to send JSON error response: %v", sendErr)
			}
			continue
		}

		g.processWebAction(session, req.ID, req.Action, req.Params, &currentFed, &federateID)
	}
}

// processWebAction dispatches actions to modular service dispatchers
func (g *WebGateway) processWebAction(
	session *WebSession,
	reqID int64,
	action string,
	params map[string]interface{},
	currentFed **FederationExecution,
	federateID *int32,
) {
	ctx := &webContext{
		gateway:    g,
		session:    session,
		currentFed: currentFed,
		federateID: federateID,
		reqID:      reqID,
	}

	if ctx.handleFederationManagement(action, params) {
		return
	}
	if ctx.handleMetadataManagement(action, params) {
		return
	}
	if ctx.handleDeclarationManagement(action, params) {
		return
	}
	if ctx.handleObjectManagement(action, params) {
		return
	}
	if ctx.handleDDMManagement(action, params) {
		return
	}
	if ctx.handleOwnershipManagement(action, params) {
		return
	}
	if ctx.handleTimeManagement(action, params) {
		return
	}

	ctx.sendError("UnsupportedAction", "Action not recognized: "+action)
}

// Parameter extraction helpers
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt32(m map[string]interface{}, key string) int32 {
	if v, ok := m[key]; ok {
		switch num := v.(type) {
		case float64:
			return int32(num)
		case int:
			return int32(num)
		case int32:
			return num
		case string:
			if parsed, err := strconv.Atoi(num); err == nil {
				return int32(parsed)
			}
		}
	}
	return 0
}

func getFloat64(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch num := v.(type) {
		case float64:
			return num
		case int:
			return float64(num)
		case int32:
			return float64(num)
		case string:
			if parsed, err := strconv.ParseFloat(num, 64); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func getInt32Slice(m map[string]interface{}, key string) []int32 {
	var result []int32
	if v, ok := m[key]; ok {
		if list, ok := v.([]interface{}); ok {
			for _, item := range list {
				switch num := item.(type) {
				case float64:
					result = append(result, int32(num))
				case int:
					result = append(result, int32(num))
				case int32:
					result = append(result, num)
				}
			}
		}
	}
	return result
}

func getStringSlice(m map[string]interface{}, key string) []string {
	var result []string
	if v, ok := m[key]; ok {
		if list, ok := v.([]interface{}); ok {
			for _, item := range list {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
		}
	}
	return result
}

func getStringMap(m map[string]interface{}, key string) map[string]string {
	result := make(map[string]string)
	if v, ok := m[key]; ok {
		if obj, ok := v.(map[string]interface{}); ok {
			for k, val := range obj {
				result[k] = fmt.Sprintf("%v", val)
			}
		}
	}
	return result
}

// Built-in Lightweight Tactical Radar Dashboard HTML (loaded via go:embed)
//go:embed web_dashboard.html
var valhallaDashboardHTML string
