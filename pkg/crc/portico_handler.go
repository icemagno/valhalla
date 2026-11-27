package crc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/icemagno/valhalla/pkg/common"
	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
	"github.com/icemagno/valhalla/pkg/crc/portico/wire/metamodel"
	fomparser "github.com/icemagno/valhalla/pkg/metamodel"
)

const (
	PorticoWelcomeMsg = "Portico Router (3.0.0 (build 0)): Valhalla CRC Sovereign Gateway"
)

// extractAttributeHandles parses a Java HashSet or ArrayList of integers from a JavaObject
func extractAttributeHandles(objAny any) []common.AttributeHandle {
	var handles []common.AttributeHandle
	if obj, ok := objAny.(*metamodel.JavaObject); ok {
		var elements []any
		if els, ok := obj.Get("elements").([]any); ok {
			elements = els
		}
		for _, el := range elements {
			switch v := el.(type) {
			case int32:
				handles = append(handles, common.AttributeHandle(v))
			case *metamodel.JavaObject:
				if val, ok := v.Get("value").(int32); ok {
					handles = append(handles, common.AttributeHandle(val))
				}
			}
		}
	}
	return handles
}

// extractAttributeMap parses a Java HashMap of (Integer -> byte[]) from a JavaObject
func extractAttributeMap(objAny any) map[int32][]byte {
	attrs := make(map[int32][]byte)
	if obj, ok := objAny.(*metamodel.JavaObject); ok {
		var entries map[any]any
		if e, ok := obj.Get("entries").(map[any]any); ok {
			entries = e
		}
		for k, v := range entries {
			var handle int32
			switch key := k.(type) {
			case int32:
				handle = key
			case *metamodel.JavaObject:
				if val, ok := key.Get("value").(int32); ok {
					handle = val
				}
			}
			if b, ok := v.([]byte); ok && handle > 0 {
				attrs[handle] = b
			}
		}
	}
	return attrs
}

// HandlePorticoConnection atende uma conexão roteada pelo multiplexador como dialeto Portico.
// A lógica é modularizada em despachantes dedicados com delegação ao domínio da federação.
func HandlePorticoConnection(conn net.Conn, reader io.Reader, fedManager *FederationManager) error {
	session := NewPorticoSession(conn)
	defer session.Close()

	// 1. Executar o Handshake Server-First exigido pelo Portico (0xBEEF / 0xFEED)
	if err := performPorticoHandshake(conn, session.writer, reader); err != nil {
		return fmt.Errorf("falha no handshake inicial do Portico: %w", err)
	}

	ctx := &porticoContext{
		session:    session,
		fedManager: fedManager,
		federateID: -1,
	}

	// 2. Loop de despacho de pacotes de controle e dados
	for {
		hdr, payload, err := session.ReadPacket(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				// Conexão encerrada normalmente pelo cliente
				if ctx.currentFed != nil && ctx.federateID > 0 {
					if _, err := ctx.currentFed.Resign(ctx.federateID); err != nil {
						logCRCPortico.Warn("failed to resign federate %d on disconnect: %v", ctx.federateID, err)
					}
				}
				return nil
			}
			return fmt.Errorf("erro lendo pacote Portico: %w", err)
		}

		if ctx.currentFed != nil && ctx.federateID > 0 {
			ctx.currentFed.TimeManager().Heartbeat(common.FederateHandle(ctx.federateID))
		}

		if handled, err := ctx.handleFederationManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleSyncPoints(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleDeclarationManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleObjectManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleTimeManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleSaveRestoreManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}
		if handled, err := ctx.handleOwnershipManagement(hdr, payload); handled {
			if err != nil {
				return err
			}
			continue
		}

		// Fallback para requisições de controle não tratadas
		if hdr.CallType == wire.CallTypeControlRequest {
			if err := session.SendControlResponseOK(hdr.RequestId, hdr.MessageType, wire.BuildSuccessResponsePayload()); err != nil {
				return err
			}
		}
	}
}

// performPorticoHandshake executa a saudação binária oficial do Portico (0xBEEF -> 0xFEED)
func performPorticoHandshake(conn net.Conn, writer io.Writer, reader io.Reader) error {
	var prefix [8]byte
	binary.BigEndian.PutUint32(prefix[0:4], 0xBEEF)
	binary.BigEndian.PutUint32(prefix[4:8], uint32(len(PorticoWelcomeMsg)))

	if _, err := writer.Write(prefix[:]); err != nil {
		return err
	}
	if _, err := writer.Write([]byte(PorticoWelcomeMsg)); err != nil {
		return err
	}

	var srvReady [4]byte
	binary.BigEndian.PutUint32(srvReady[:], 0xFEED)
	if _, err := writer.Write(srvReady[:]); err != nil {
		return err
	}

	if flusher, ok := writer.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			return err
		}
	}

	var clientReady int32
	if err := binary.Read(reader, binary.BigEndian, &clientReady); err != nil {
		return fmt.Errorf("erro lendo 0xFEED do cliente: %w", err)
	}
	if clientReady != 0xFEED {
		return fmt.Errorf("cliente enviou ready code invalido: 0x%X (esperado 0xFEED)", clientReady)
	}

	return nil
}

// resolveFOMPath searches for a FOM file on disk strictly as ./foms/<ARQUIVO>.xml or bin/foms/<ARQUIVO>.xml
func resolveFOMPath(rawPath string) string {
	cleanPath := strings.TrimPrefix(rawPath, "file:///")
	cleanPath = strings.TrimPrefix(cleanPath, "file://")
	cleanPath = strings.TrimPrefix(cleanPath, "file:")

	normPath := strings.ReplaceAll(cleanPath, "\\", "/")
	base := filepath.Base(normPath)

	candidates := []string{
		filepath.Join("foms", base),
		filepath.Join("bin", "foms", base),
	}
	for _, target := range candidates {
		if _, err := os.Stat(target); err == nil {
			return target
		}
	}
	return ""
}

// ingestPorticoFOMModules extracts and parses FOM XML modules from the Portico CreateFederation payload
func ingestPorticoFOMModules(omt *core.OMTRegistry, modulesAny any) {
	if omt == nil || modulesAny == nil {
		return
	}
	var elements []any
	if listObj, ok := modulesAny.(*metamodel.JavaObject); ok {
		if els, ok := listObj.Get("elements").([]any); ok {
			elements = els
		}
	} else if els, ok := modulesAny.([]any); ok {
		elements = els
	}

	for _, el := range elements {
		switch v := el.(type) {
		case []byte:
			if strings.Contains(string(v), "<objectModel") {
				if err := fomparser.ParseFOMXML(omt, v); err != nil {
					logCRCPortico.Warn("Warning: failed to parse byte FOM module: %v", err)
				} else {
					logCRCPortico.Info("Ingested in-memory FOM XML module (%d bytes)", len(v))
				}
			}
		case string:
			if strings.Contains(v, "<objectModel") {
				if err := fomparser.ParseFOMXML(omt, []byte(v)); err != nil {
					logCRCPortico.Warn("Warning: failed to parse string FOM module: %v", err)
				} else {
					logCRCPortico.Info("Ingested string FOM XML module (%d bytes)", len(v))
				}
			} else {
				resolved := resolveFOMPath(v)
				if resolved != "" {
					if err := fomparser.ParseFOMFile(omt, resolved); err != nil {
						logCRCPortico.Warn("Warning: failed to parse FOM file '%s': %v", resolved, err)
					} else {
						logCRCPortico.Info("Ingested FOM file '%s' (from '%s')", resolved, v)
					}
				} else {
					logCRCPortico.Warn("Warning: could not resolve FOM path '%s'", v)
				}
			}
		case *metamodel.JavaObject:
			// If JavaObject is java.net.URL, extract "file" or "path"
			if v.Class != nil && v.Class.Name == "java.net.URL" {
				if fileStr, ok := v.Get("file").(string); ok && fileStr != "" {
					ingestPorticoFOMModules(omt, []any{fileStr})
					continue
				}
				if pathStr, ok := v.Get("path").(string); ok && pathStr != "" {
					ingestPorticoFOMModules(omt, []any{pathStr})
					continue
				}
			}
			// Generic fallback: check known fields
			for _, f := range []string{"file", "path", "content", "data", "raw", "xml", "url"} {
				if val := v.Get(f); val != nil {
					ingestPorticoFOMModules(omt, []any{val})
					break
				}
			}
		}
	}
}
