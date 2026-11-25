// web_gateway.go - RFC 6455 WebSocket Gateway & HTTP Dashboard for Valhalla HLA-4 Web Profile
package crc

import (
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
	"github.com/icemagno/valhalla/pkg/crc/core"
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

// Stop gracefully shuts down the Web gateway and terminates active WebSocket client connections
func (g *WebGateway) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return nil
	}
	g.running = false

	for s := range g.sessions {
		_ = s.Close()
	}
	g.sessions = make(map[*WebSession]struct{})

	if g.server != nil {
		_ = g.server.Close()
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
	_ = json.NewEncoder(w).Encode(resp)
}

// handleDashboard renders a high-performance tactical monitoring dashboard
func (g *WebGateway) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(valhallaDashboardHTML))
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
			_, _ = currentFed.Resign(federateID)
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
		} else if opcode == 9 { // Ping
			_ = writeWebSocketTextFrame(conn, []byte{})
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
			_ = session.SendJSON(map[string]interface{}{
				"id":     0,
				"status": "ERROR",
				"error":  "Invalid JSON payload: " + err.Error(),
			})
			continue
		}

		g.processWebAction(session, req.ID, req.Action, req.Params, &currentFed, &federateID)
	}
}

// processWebAction dispatches actions to federation managers and returns standard JSON responses
func (g *WebGateway) processWebAction(
	session *WebSession,
	reqID int64,
	action string,
	params map[string]interface{},
	currentFed **FederationExecution,
	federateID *int32,
) {
	sendSuccess := func(result map[string]interface{}) {
		_ = session.SendJSON(map[string]interface{}{
			"id":     reqID,
			"status": "OK",
			"result": result,
		})
	}

	sendError := func(errCode string, msg string) {
		_ = session.SendJSON(map[string]interface{}{
			"id":      reqID,
			"status":  "ERROR",
			"code":    errCode,
			"message": msg,
		})
	}

	switch action {
	case "connect":
		sendSuccess(map[string]interface{}{
			"server":  "Valhalla CRC",
			"version": "IEEE 1516-2025 (HLA-4 Web Profile)",
		})

	case "createFederationExecution":
		fedName := getString(params, "federationName")
		if fedName == "" {
			sendError("InvalidLocalSettingsDesignator", "Missing federationName")
			return
		}
		fomModules := getStringSlice(params, "fomModules")
		if len(fomModules) == 0 {
			fomSingle := getString(params, "fomModule")
			if fomSingle != "" {
				fomModules = []string{fomSingle}
			}
		}

		var err error
		if len(fomModules) > 0 {
			_, err = g.fedManager.CreateFederationWithFOMs(fedName, fomModules)
		} else {
			_, err = g.fedManager.CreateFederation(fedName)
		}

		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				sendError("FederationExecutionAlreadyExists", err.Error())
			} else {
				sendError("RTIinternalError", err.Error())
			}
			return
		}
		sendSuccess(map[string]interface{}{"federationName": fedName})

	case "destroyFederationExecution":
		fedName := getString(params, "federationName")
		err := g.fedManager.DestroyFederation(fedName)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				sendError("FederationExecutionDoesNotExist", err.Error())
			} else {
				sendError("FederatesCurrentlyJoined", err.Error())
			}
			return
		}
		sendSuccess(map[string]interface{}{"federationName": fedName})

	case "joinFederationExecution":
		fedName := getString(params, "federationName")
		fedNameParam := getString(params, "federateName")
		fedType := getString(params, "federateType")
		if fedType == "" {
			fedType = "WebFederate"
		}

		fedExec, exists := g.fedManager.GetFederation(fedName)
		if !exists {
			sendError("FederationExecutionDoesNotExist", "Federation does not exist: "+fedName)
			return
		}

		fed, err := fedExec.Join(fedNameParam, fedType, ProtocolWeb, session)
		if err != nil {
			sendError("FederateNameAlreadyInUse", err.Error())
			return
		}

		*currentFed = fedExec
		*federateID = fed.ID
		session.SetFederateID(fed.ID)

		sendSuccess(map[string]interface{}{
			"federateHandle": fed.ID,
			"federationName": fedName,
		})

	case "resignFederationExecution":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		_, _ = (*currentFed).Resign(*federateID)
		*currentFed = nil
		*federateID = 0
		session.SetFederateID(0)
		sendSuccess(nil)

	case "getObjectClassHandle":
		if *currentFed == nil {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		className := getString(params, "className")
		cls, ok := (*currentFed).OMT().GetObjectClassByName(className)
		if !ok {
			sendError("NameNotFound", "class not found: "+className)
			return
		}
		sendSuccess(map[string]interface{}{"handle": int32(cls.Handle)})

	case "getAttributeHandle":
		if *currentFed == nil {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		classH := getInt32(params, "classHandle")
		attrName := getString(params, "attributeName")
		attr, err := (*currentFed).OMT().ResolveAttributeByName(common.ObjectClassHandle(classH), attrName)
		if err != nil {
			sendError("NameNotFound", err.Error())
			return
		}
		sendSuccess(map[string]interface{}{"handle": int32(attr.Handle)})

	case "getDimensionHandle":
		if *currentFed == nil {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		dimName := getString(params, "dimensionName")
		dim, ok := (*currentFed).OMT().GetDimensionByName(dimName)
		if !ok {
			sendError("NameNotFound", "dimension not found: "+dimName)
			return
		}
		sendSuccess(map[string]interface{}{"handle": int32(dim.Handle)})

	case "publishObjectClassAttributes":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		classH := getInt32(params, "classHandle")
		attrs := getInt32Slice(params, "attributeHandles")
		var attrList []common.AttributeHandle
		for _, a := range attrs {
			attrList = append(attrList, common.AttributeHandle(a))
		}
		err := (*currentFed).Router().PublishObjectClass(common.FederateHandle(*federateID), common.ObjectClassHandle(classH), attrList)
		if err != nil {
			sendError("ObjectClassNotDefined", err.Error())
			return
		}
		sendSuccess(nil)

	case "subscribeObjectClassAttributes":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		classH := getInt32(params, "classHandle")
		attrs := getInt32Slice(params, "attributeHandles")
		var attrList []common.AttributeHandle
		for _, a := range attrs {
			attrList = append(attrList, common.AttributeHandle(a))
		}
		err := (*currentFed).Router().SubscribeObjectClass(common.FederateHandle(*federateID), common.ObjectClassHandle(classH), attrList, true)
		if err != nil {
			sendError("ObjectClassNotDefined", err.Error())
			return
		}
		sendSuccess(nil)

	case "createRegion":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		regH := (*currentFed).CreateRegion()
		sendSuccess(map[string]interface{}{"regionHandle": regH})

	case "setMetricBounds":
		if *currentFed == nil {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		regH := uint32(getInt32(params, "regionHandle"))
		minX := getFloat64(params, "minX")
		maxX := getFloat64(params, "maxX")
		minY := getFloat64(params, "minY")
		maxY := getFloat64(params, "maxY")
		minZ := getFloat64(params, "minZ")
		maxZ := getFloat64(params, "maxZ")

		err := (*currentFed).SetRegionMetricBounds(regH, minX, maxX, minY, maxY, minZ, maxZ)
		if err != nil {
			sendError("InvalidRegion", err.Error())
			return
		}
		sendSuccess(nil)

	case "subscribeObjectClassAttributesWithRegions":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		classH := getInt32(params, "classHandle")
		attrs := getInt32Slice(params, "attributeHandles")
		regH := uint32(getInt32(params, "regionHandle"))

		region, exists := (*currentFed).GetRegion(regH)
		if !exists {
			sendError("InvalidRegion", "region handle not found")
			return
		}

		var attrList []common.AttributeHandle
		for _, a := range attrs {
			attrList = append(attrList, common.AttributeHandle(a))
		}
		err := (*currentFed).Router().SubscribeObjectClassWithRegion(common.FederateHandle(*federateID), common.ObjectClassHandle(classH), attrList, region)
		if err != nil {
			sendError("InvalidRegion", err.Error())
			return
		}
		sendSuccess(nil)

	case "registerObjectInstance":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		classH := getInt32(params, "classHandle")
		objName := getString(params, "objectName")

		obj, err := (*currentFed).RegisterObjectInstance(classH, objName, *federateID)
		if err != nil {
			sendError("ObjectClassNotDefined", err.Error())
			return
		}

		discPayload, _ := common.DiscoverObjectInstancePayload{
			ObjectHandle: common.ObjectInstanceHandle(obj.Handle),
			ClassHandle:  common.ObjectClassHandle(classH),
			ObjectName:   obj.Name,
		}.MarshalBinary()

		for _, f := range (*currentFed).GetAllFederates() {
			if f.ID != *federateID && f.Session != nil {
				_ = f.Session.SendNotification(common.NativeMsgDiscoverObject, discPayload)
			}
		}

		sendSuccess(map[string]interface{}{"objectHandle": obj.Handle})

	case "deleteObjectInstance":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		objH := getInt32(params, "objectHandle")
		tag := getString(params, "tag")

		_, err := (*currentFed).DeleteObjectInstance(objH, *federateID, []byte(tag))
		if err != nil {
			sendError("ObjectInstanceNotKnown", err.Error())
			return
		}

		remPayload, _ := common.RemoveObjectInstancePayload{
			ObjectHandle: common.ObjectInstanceHandle(objH),
			Tag:          []byte(tag),
		}.MarshalBinary()

		for _, f := range (*currentFed).GetAllFederates() {
			if f.ID != *federateID && f.Session != nil {
				_ = f.Session.SendNotification(common.NativeMsgRemoveObjectInstance, remPayload)
			}
		}
		sendSuccess(nil)

	case "updateAttributeValues":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		objH := getInt32(params, "objectHandle")
		tag := getString(params, "tag")
		rawAttrs := getStringMap(params, "attributes")

		attrsMap := make(map[int32][]byte)
		for k, v := range rawAttrs {
			if id, err := strconv.Atoi(k); err == nil {
				attrsMap[int32(id)] = []byte(v)
			}
		}

		obj, exists := (*currentFed).GetObjectInstance(objH)
		if !exists {
			sendError("ObjectInstanceNotKnown", "Object not found")
			return
		}

		attrsPayloadMap := make(map[common.AttributeHandle][]byte)
		for k, v := range rawAttrs {
			if id, err := strconv.Atoi(k); err == nil {
				attrsPayloadMap[common.AttributeHandle(id)] = []byte(v)
			}
		}

		_ = (*currentFed).UpdateObjectAttributes(objH, attrsMap, [32]byte{})

		uavPayload, _ := common.UpdateAttributesPayload{
			ObjectHandle: common.ObjectInstanceHandle(objH),
			Attributes:   attrsPayloadMap,
			Tag:          []byte(tag),
		}.MarshalBinary()

		targetSubs := make(map[common.FederateHandle]bool)
		for attrH := range attrsMap {
			subs := (*currentFed).Router().GetAttributeSubscribers(common.ObjectClassHandle(obj.ClassHandle), common.AttributeHandle(attrH))
			for _, sub := range subs {
				if sub != common.FederateHandle(*federateID) {
					targetSubs[sub] = true
				}
			}
		}

		for sub := range targetSubs {
			if f, exists := (*currentFed).GetFederate(int32(sub)); exists && f.Session != nil {
				_ = f.Session.SendNotification(common.NativeMsgReflectAttributes, uavPayload)
			}
		}
		sendSuccess(nil)

	case "updateAttributeValuesSpatial":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		objH := getInt32(params, "objectHandle")
		x := getFloat64(params, "x")
		y := getFloat64(params, "y")
		z := getFloat64(params, "z")
		tag := getString(params, "tag")
		rawAttrs := getStringMap(params, "attributes")

		attrsMap := make(map[int32][]byte)
		attrsPayloadMap := make(map[common.AttributeHandle][]byte)
		for k, v := range rawAttrs {
			if id, err := strconv.Atoi(k); err == nil {
				attrsMap[int32(id)] = []byte(v)
				attrsPayloadMap[common.AttributeHandle(id)] = []byte(v)
			}
		}

		obj, exists := (*currentFed).GetObjectInstance(objH)
		if !exists {
			sendError("ObjectInstanceNotKnown", "Object not found")
			return
		}

		_ = (*currentFed).UpdateObjectAttributes(objH, attrsMap, [32]byte{})

		uavPayload, _ := common.UpdateAttributesPayload{
			ObjectHandle: common.ObjectInstanceHandle(objH),
			Attributes:   attrsPayloadMap,
			Tag:          []byte(tag),
		}.MarshalBinary()

		morton := core.EncodeECEF(x, y, z)
		targetSubs := make(map[common.FederateHandle]bool)
		for attrH := range attrsMap {
			subs := (*currentFed).Router().GetAttributeSubscribersInRegion(common.ObjectClassHandle(obj.ClassHandle), common.AttributeHandle(attrH), morton)
			for _, sub := range subs {
				if sub != common.FederateHandle(*federateID) {
					targetSubs[sub] = true
				}
			}
		}

		for sub := range targetSubs {
			if f, exists := (*currentFed).GetFederate(int32(sub)); exists && f.Session != nil {
				_ = f.Session.SendNotification(common.NativeMsgReflectAttributes, uavPayload)
			}
		}
		sendSuccess(nil)

	case "isAttributeOwnedByFederate":
		if *currentFed == nil {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		objH := getInt32(params, "objectHandle")
		attrH := getInt32(params, "attributeHandle")

		owner, owned := (*currentFed).OwnershipManager().QueryAttributeOwnership(common.ObjectInstanceHandle(objH), common.AttributeHandle(attrH))
		sendSuccess(map[string]interface{}{
			"isOwned": owned,
			"owner":   int32(owner),
		})

	case "unconditionalAttributeOwnershipDivestiture":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		objH := getInt32(params, "objectHandle")
		attrs := getInt32Slice(params, "attributeHandles")

		var attrList []common.AttributeHandle
		for _, a := range attrs {
			attrList = append(attrList, common.AttributeHandle(a))
		}

		err := (*currentFed).OwnershipManager().UnconditionalDivestiture(common.ObjectInstanceHandle(objH), attrList, common.FederateHandle(*federateID))
		if err != nil {
			sendError("AttributeNotOwned", err.Error())
			return
		}
		sendSuccess(nil)

	case "attributeOwnershipAcquisition":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		objH := getInt32(params, "objectHandle")
		attrs := getInt32Slice(params, "attributeHandles")

		var attrList []common.AttributeHandle
		for _, a := range attrs {
			attrList = append(attrList, common.AttributeHandle(a))
		}

		err := (*currentFed).OwnershipManager().AcquireAttributeOwnership(common.ObjectInstanceHandle(objH), attrList, common.FederateHandle(*federateID))
		if err != nil {
			sendError("FederateOwnsAttributes", err.Error())
			return
		}
		sendSuccess(nil)

	case "timeAdvanceRequest":
		if *currentFed == nil || *federateID <= 0 {
			sendError("FederateNotExecutionMember", "Not joined to federation")
			return
		}
		reqTime := getFloat64(params, "time")
		grants, err := (*currentFed).TimeManager().TimeAdvanceRequest(common.FederateHandle(*federateID), reqTime)
		if err != nil {
			sendError("InvalidLogicalTime", err.Error())
			return
		}
		for _, gFed := range grants {
			if int32(gFed.Federate) == *federateID {
				grantPayload, _ := common.TimeFloat64Payload{Time: gFed.Time}.MarshalBinary()
				_ = session.SendNotification(common.NativeMsgTimeAdvanceGrant, grantPayload)
			}
		}
		sendSuccess(nil)

	default:
		sendError("UnsupportedAction", "Action not recognized: "+action)
	}
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

// Built-in Lightweight Tactical Radar Dashboard HTML
const valhallaDashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Valhalla RTI - HLA-4 Tactical Web Dashboard</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { background: #0a0e14; color: #d3c6aa; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, monospace; display: flex; flex-direction: column; height: 100vh; }
  header { background: #131920; border-bottom: 2px solid #2d3b4d; padding: 12px 24px; display: flex; justify-content: space-between; align-items: center; }
  .logo { font-size: 1.25rem; font-weight: bold; color: #7fbbb3; display: flex; align-items: center; gap: 8px; }
  .badge { background: #3a516b; color: #e5e9f0; padding: 4px 8px; border-radius: 4px; font-size: 0.75rem; }
  .main { display: flex; flex: 1; overflow: hidden; }
  .sidebar { width: 340px; background: #0f141c; border-right: 1px solid #1f2a37; padding: 16px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; }
  .card { background: #141c26; border: 1px solid #233142; border-radius: 6px; padding: 12px; }
  .card h3 { font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.05em; color: #a7c080; margin-bottom: 8px; }
  .stat-row { display: flex; justify-content: space-between; font-size: 0.85rem; margin-bottom: 4px; }
  .stat-val { color: #83c092; font-weight: bold; }
  .radar-container { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; position: relative; background: radial-gradient(circle at center, #0d1520 0%, #080c12 100%); }
  canvas { border-radius: 50%; box-shadow: 0 0 40px rgba(127, 187, 179, 0.15); background: #090e15; }
  .log-panel { height: 160px; background: #0b0f14; border-top: 1px solid #1f2a37; padding: 8px 16px; overflow-y: auto; font-size: 0.75rem; }
  .log-line { margin-bottom: 2px; }
  .log-info { color: #7fbbb3; }
  .log-rx { color: #a7c080; }
  .log-warn { color: #dbbc7f; }
</style>
</head>
<body>
<header>
  <div class="logo">
    <span>PROJECT VALHALLA</span>
    <span class="badge">IEEE 1516-2025 (HLA-4)</span>
    <span class="badge" style="background:#2d4a3e; color:#a7c080;">WEB PROFILE ACTIVE</span>
  </div>
  <div style="font-size:0.8rem; color:#859289;" id="connStatus">Connecting to WebSocket...</div>
</header>
<div class="main">
  <div class="sidebar">
    <div class="card">
      <h3>Active Federation</h3>
      <div class="stat-row"><span>Status:</span><span class="stat-val" id="fedStatus">Listening</span></div>
      <div class="stat-row"><span>Federation:</span><span class="stat-val" id="fedName">-</span></div>
      <div class="stat-row"><span>My Handle:</span><span class="stat-val" id="fedHandle">-</span></div>
    </div>
    <div class="card">
      <h3>Spatial Entities (DDM 3D)</h3>
      <div class="stat-row"><span>Tracked Objects:</span><span class="stat-val" id="objCount">0</span></div>
      <div class="stat-row"><span>Reflections Rx:</span><span class="stat-val" id="reflectCount">0</span></div>
    </div>
    <div class="card" style="flex:1;">
      <h3>Entities List</h3>
      <div id="entityList" style="font-size:0.8rem; display:flex; flex-direction:column; gap:4px;">No entities active</div>
    </div>
  </div>
  <div class="radar-container">
    <canvas id="radar" width="460" height="460"></canvas>
  </div>
</div>
<div class="log-panel" id="logPanel"></div>

<script>
  const logEl = document.getElementById('logPanel');
  function addLog(text, cls = 'log-info') {
    const el = document.createElement('div');
    el.className = 'log-line ' + cls;
    el.textContent = '[' + new Date().toLocaleTimeString() + '] ' + text;
    logEl.appendChild(el);
    logEl.scrollTop = logEl.scrollHeight;
  }

  const canvas = document.getElementById('radar');
  const ctx = canvas.getContext('2d');
  const entities = {};
  let sweepAngle = 0;
  let totalReflects = 0;

  function drawRadar() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    const cx = canvas.width / 2;
    const cy = canvas.height / 2;
    const r = cx - 10;

    // Rings
    ctx.strokeStyle = '#1b2b3a';
    ctx.lineWidth = 1.5;
    for (let i = 1; i <= 4; i++) {
      ctx.beginPath();
      ctx.arc(cx, cy, (r / 4) * i, 0, Math.PI * 2);
      ctx.stroke();
    }

    // Crosshairs
    ctx.beginPath();
    ctx.moveTo(cx, cy - r); ctx.lineTo(cx, cy + r);
    ctx.moveTo(cx - r, cy); ctx.lineTo(cx + r, cy);
    ctx.stroke();

    // Sweep Line
    sweepAngle = (sweepAngle + 0.02) % (Math.PI * 2);
    ctx.strokeStyle = 'rgba(127, 187, 179, 0.4)';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(cx, cy);
    ctx.lineTo(cx + Math.cos(sweepAngle) * r, cy + Math.sin(sweepAngle) * r);
    ctx.stroke();

    // Draw Entities
    for (const id in entities) {
      const e = entities[id];
      // Normalize [0..1000] to canvas coords
      const ex = cx + ((e.x - 500) / 500) * (r * 0.85);
      const ey = cy + ((e.y - 500) / 500) * (r * 0.85);

      ctx.fillStyle = '#a7c080';
      ctx.beginPath();
      ctx.arc(ex, ey, 5, 0, Math.PI * 2);
      ctx.fill();

      ctx.fillStyle = '#d3c6aa';
      ctx.font = '10px monospace';
      ctx.fillText(e.name || ('Entity #' + id), ex + 8, ey + 3);
    }

    requestAnimationFrame(drawRadar);
  }
  requestAnimationFrame(drawRadar);

  // Connect WebSocket
  const wsUrl = (location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/ws';
  const ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    document.getElementById('connStatus').textContent = 'WebSocket Online';
    document.getElementById('connStatus').style.color = '#a7c080';
    addLog('Connected to Valhalla CRC WebSocket endpoint: ' + wsUrl);

    // Auto-discover status
    ws.send(JSON.stringify({ id: 1, action: "connect", params: {} }));
  };

  ws.onmessage = (evt) => {
    const msg = JSON.parse(evt.data);
    if (msg.callback === 'discoverObjectInstance') {
      const p = msg.params;
      entities[p.object] = { x: 500, y: 500, name: p.name, class: p.class };
      addLog('DISCOVER: Object #' + p.object + ' (' + p.name + ') class=' + p.class, 'log-rx');
      updateEntityList();
    } else if (msg.callback === 'reflectAttributeValues') {
      totalReflects++;
      document.getElementById('reflectCount').textContent = totalReflects;
      const p = msg.params;
      if (!entities[p.object]) {
        entities[p.object] = { x: 500, y: 500, name: 'Track #' + p.object };
      }
      const pos = p.attributes['1001'] || Object.values(p.attributes)[0] || '';
      const match = pos.match(/x=([0-9.]+)[^y]+y=([0-9.]+)/) || pos.match(/([0-9.]+)[,\s]+([0-9.]+)/);
      if (match) {
        entities[p.object].x = parseFloat(match[1]);
        entities[p.object].y = parseFloat(match[2]);
      }
      addLog('REFLECT: Object #' + p.object + ' attrs=' + JSON.stringify(p.attributes), 'log-rx');
      updateEntityList();
    } else if (msg.callback === 'removeObjectInstance') {
      delete entities[msg.params.object];
      addLog('REMOVE: Object #' + msg.params.object, 'log-warn');
      updateEntityList();
    }
  };

  function updateEntityList() {
    const el = document.getElementById('entityList');
    const keys = Object.keys(entities);
    document.getElementById('objCount').textContent = keys.length;
    if (keys.length === 0) {
      el.innerHTML = '<span style="color:#859289">No entities active</span>';
      return;
    }
    el.innerHTML = keys.map(k => '<div style="color:#7fbbb3;">* [' + k + '] ' + entities[k].name + '</div>').join('');
  }
</script>
</body>
</html>
`
