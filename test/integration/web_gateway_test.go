// web_gateway_test.go - End-to-End Integration Test for Valhalla HLA-4 Web Profile (WebSocket & JSON)
package integration_test

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/icemagno/valhalla/pkg/crc"
)

func TestHLA4_WebProfile_11Stages(t *testing.T) {
	// 1. Start CRC Executive on dynamic ports
	server := crc.NewServer("127.0.0.1:0")
	server.SetWebAddr("127.0.0.1:0")

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start CRC server: %v", err)
	}
	defer func() { _ = server.Stop() }()

	// Wait for server to bind
	time.Sleep(100 * time.Millisecond)

	// Since WebGateway is on a separate port or configured, let's test directly with dedicated WebGateway
	fedMgr := server.FederationManager()
	webGW := crc.NewWebGateway("127.0.0.1:0", fedMgr)
	if err := webGW.Start(); err != nil {
		t.Fatalf("Failed to start WebGateway: %v", err)
	}
	defer func() { _ = webGW.Stop() }()

	wsPort := webGW.Addr().(*net.TCPAddr).Port

	time.Sleep(100 * time.Millisecond)

	// 2. Connect raw TCP and perform WebSocket handshake
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", wsPort))
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket port: %v", err)
	}
	defer conn.Close()

	wsKey := "dGhlIHNhbXBsZSBub25jZQ=="
	handshake := fmt.Sprintf("GET /ws HTTP/1.1\r\n"+
		"Host: 127.0.0.1:%d\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\n"+
		"Sec-WebSocket-Version: 13\r\n\r\n", wsPort, wsKey)

	if _, err := conn.Write([]byte(handshake)); err != nil {
		t.Fatalf("Failed to write handshake: %v", err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(statusLine, "101") {
		t.Fatalf("Unexpected handshake response: %s (err: %v)", statusLine, err)
	}

	// Read remainder of HTTP headers until \r\n\r\n
	for {
		line, err := reader.ReadString('\n')
		if err != nil || line == "\r\n" {
			break
		}
	}

	// Helper to send JSON request and receive response
	reqID := int64(0)
	callAction := func(action string, params map[string]interface{}) map[string]interface{} {
		reqID++
		req := map[string]interface{}{
			"id":     reqID,
			"action": action,
			"params": params,
		}
		data, _ := json.Marshal(req)
		if err := writeMaskedWebSocketFrame(conn, data); err != nil {
			t.Fatalf("Action %s: write failed: %v", action, err)
		}

		for {
			opcode, payload, err := readWSFrame(reader)
			if err != nil {
				t.Fatalf("Action %s: read failed: %v", action, err)
			}
			if opcode != 1 {
				continue
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(payload, &resp); err != nil {
				t.Fatalf("Action %s: invalid JSON: %v", action, err)
			}

			// If it's a response matching our request ID
			if rID, ok := resp["id"].(float64); ok && int64(rID) == reqID {
				if status, _ := resp["status"].(string); status != "OK" {
					t.Fatalf("Action %s failed with status %s: %v", action, status, resp["message"])
				}
				if res, ok := resp["result"].(map[string]interface{}); ok {
					return res
				}
				return nil
			}
		}
	}

	// -------------------------------------------------------------------------
	// Stage 1: Connect
	// -------------------------------------------------------------------------
	t.Log("[Stage 1/11] Connect action...")
	callAction("connect", nil)

	// -------------------------------------------------------------------------
	// Stage 2: Create Federation with FOM & Join
	// -------------------------------------------------------------------------
	t.Log("[Stage 2/11] Create & Join Federation...")
	callAction("createFederationExecution", map[string]interface{}{
		"federationName": "WebTestFed",
		"fomModules":     []string{"foms/ValhallaAdvanced.xml"},
	})

	joinRes := callAction("joinFederationExecution", map[string]interface{}{
		"federateName":   "WebClientAlfa",
		"federationName": "WebTestFed",
		"federateType":   "WebFederate",
	})
	fedH, ok := joinRes["federateHandle"].(float64)
	if !ok || int32(fedH) <= 0 {
		t.Fatalf("Expected valid federate handle, got %v", joinRes)
	}

	// -------------------------------------------------------------------------
	// Stage 3: Dynamic FOM Resolution (Class & Attributes)
	// -------------------------------------------------------------------------
	t.Log("[Stage 3/11] Dynamic FOM Resolution...")
	classRes := callAction("getObjectClassHandle", map[string]interface{}{"className": "TacticalEntity"})
	classH := int32(classRes["handle"].(float64))

	posRes := callAction("getAttributeHandle", map[string]interface{}{"classHandle": classH, "attributeName": "Position"})
	posH := int32(posRes["handle"].(float64))

	velRes := callAction("getAttributeHandle", map[string]interface{}{"classHandle": classH, "attributeName": "Velocity"})
	velH := int32(velRes["handle"].(float64))

	payloadRes := callAction("getAttributeHandle", map[string]interface{}{"classHandle": classH, "attributeName": "PayloadStatus"})
	payloadH := int32(payloadRes["handle"].(float64))

	callAction("publishObjectClassAttributes", map[string]interface{}{
		"classHandle":      classH,
		"attributeHandles": []int32{posH, velH, payloadH},
	})

	// -------------------------------------------------------------------------
	// Stage 4: Dynamic DDM 3D Spatial Region
	// -------------------------------------------------------------------------
	t.Log("[Stage 4/11] DDM 3D Region from FOM Dimensions...")
	dimXRes := callAction("getDimensionHandle", map[string]interface{}{"dimensionName": "DimensionX"})
	dimX := int32(dimXRes["handle"].(float64))
	dimYRes := callAction("getDimensionHandle", map[string]interface{}{"dimensionName": "DimensionY"})
	dimY := int32(dimYRes["handle"].(float64))
	dimZRes := callAction("getDimensionHandle", map[string]interface{}{"dimensionName": "DimensionZ"})
	dimZ := int32(dimZRes["handle"].(float64))

	regRes := callAction("createRegion", map[string]interface{}{"dimensions": []int32{dimX, dimY, dimZ}})
	regH := uint32(regRes["regionHandle"].(float64))

	callAction("setMetricBounds", map[string]interface{}{
		"regionHandle": regH,
		"minX":         0.0, "maxX": 1000.0,
		"minY":         0.0, "maxY": 1000.0,
		"minZ":         0.0, "maxZ": 1000.0,
	})

	callAction("subscribeObjectClassAttributesWithRegions", map[string]interface{}{
		"classHandle":      classH,
		"attributeHandles": []int32{posH, velH, payloadH},
		"regionHandle":     regH,
	})

	// -------------------------------------------------------------------------
	// Stage 5: Register Object Instance
	// -------------------------------------------------------------------------
	t.Log("[Stage 5/11] Register Object Instance...")
	regObjRes := callAction("registerObjectInstance", map[string]interface{}{
		"classHandle": classH,
		"objectName":  "Valhalla_Web_Drone_01",
	})
	objH := int32(regObjRes["objectHandle"].(float64))

	// -------------------------------------------------------------------------
	// Stage 6: 3D Spatial Attribute Updates
	// -------------------------------------------------------------------------
	t.Log("[Stage 6/11] Spatial Attribute Updates...")
	callAction("updateAttributeValuesSpatial", map[string]interface{}{
		"objectHandle": objH,
		"x":            250.0, "y": 250.0, "z": 250.0,
		"tag": "Cycle_1",
		"attributes": map[string]string{
			fmt.Sprintf("%d", posH):     "POS(x=250,y=250,z=250)",
			fmt.Sprintf("%d", velH):     "VEL(1.5)",
			fmt.Sprintf("%d", payloadH): "OK",
		},
	})

	// -------------------------------------------------------------------------
	// Stage 7: Ownership Management
	// -------------------------------------------------------------------------
	t.Log("[Stage 7/11] Attribute Ownership Management...")
	ownRes := callAction("isAttributeOwnedByFederate", map[string]interface{}{
		"objectHandle":    objH,
		"attributeHandle": posH,
	})
	if owned, _ := ownRes["isOwned"].(bool); !owned {
		t.Fatalf("Expected initial ownership to be true")
	}

	callAction("unconditionalAttributeOwnershipDivestiture", map[string]interface{}{
		"objectHandle":      objH,
		"attributeHandles": []int32{posH},
	})

	ownRes2 := callAction("isAttributeOwnedByFederate", map[string]interface{}{
		"objectHandle":    objH,
		"attributeHandle": posH,
	})
	if owned, _ := ownRes2["isOwned"].(bool); owned {
		t.Fatalf("Expected ownership to be false after divestiture")
	}

	callAction("attributeOwnershipAcquisition", map[string]interface{}{
		"objectHandle":      objH,
		"attributeHandles": []int32{posH},
		"tag":               "AcqTag",
	})

	ownRes3 := callAction("isAttributeOwnedByFederate", map[string]interface{}{
		"objectHandle":    objH,
		"attributeHandle": posH,
	})
	if owned, _ := ownRes3["isOwned"].(bool); !owned {
		t.Fatalf("Expected ownership to be true after acquisition")
	}

	// -------------------------------------------------------------------------
	// Stage 8: Delete Object Instance
	// -------------------------------------------------------------------------
	t.Log("[Stage 8/11] Delete Object Instance...")
	callAction("deleteObjectInstance", map[string]interface{}{
		"objectHandle": objH,
		"tag":          "Done",
	})

	// -------------------------------------------------------------------------
	// Stage 9: Resign
	// -------------------------------------------------------------------------
	t.Log("[Stage 9/11] Resign Federation...")
	callAction("resignFederationExecution", nil)

	// -------------------------------------------------------------------------
	// Stage 10: Destroy
	// -------------------------------------------------------------------------
	t.Log("[Stage 10/11] Destroy Federation...")
	callAction("destroyFederationExecution", map[string]interface{}{
		"federationName": "WebTestFed",
	})

	// -------------------------------------------------------------------------
	// Stage 11: Disconnect
	// -------------------------------------------------------------------------
	t.Log("[Stage 11/11] Clean Teardown...")
	conn.Close()

	t.Log(">>> ALL 11 HLA-4 WEB PROFILE STAGES PASSED 100%! <<<")
}

func writeMaskedWebSocketFrame(w io.Writer, payload []byte) error {
	n := len(payload)
	var header []byte
	if n < 126 {
		header = []byte{0x81, byte(n | 0x80)}
	} else if n <= 65535 {
		header = []byte{0x81, 126 | 0x80, byte(n >> 8), byte(n)}
	} else {
		header = make([]byte, 10)
		header[0] = 0x81
		header[1] = 127 | 0x80
		binary.BigEndian.PutUint64(header[2:], uint64(n))
	}

	mask := []byte{0x12, 0x34, 0x56, 0x78}
	header = append(header, mask...)

	maskedPayload := make([]byte, n)
	for i := 0; i < n; i++ {
		maskedPayload[i] = payload[i] ^ mask[i%4]
	}

	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(maskedPayload)
	return err
}

func readWSFrame(r io.Reader) (byte, []byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	opcode := hdr[0] & 0x0F
	masked := (hdr[1] & 0x80) != 0
	lenByte := hdr[1] & 0x7F

	var payloadLen uint64
	if lenByte < 126 {
		payloadLen = uint64(lenByte)
	} else if lenByte == 126 {
		var lBuf [2]byte
		if _, err := io.ReadFull(r, lBuf[:]); err != nil {
			return 0, nil, err
		}
		payloadLen = uint64(binary.BigEndian.Uint16(lBuf[:]))
	} else {
		var lBuf [8]byte
		if _, err := io.ReadFull(r, lBuf[:]); err != nil {
			return 0, nil, err
		}
		payloadLen = binary.BigEndian.Uint64(lBuf[:])
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(r, mask[:]); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}

	if masked {
		for i := uint64(0); i < payloadLen; i++ {
			payload[i] ^= mask[i%4]
		}
	}

	return opcode, payload, nil
}
