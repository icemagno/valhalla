package crc

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/logger"
	fomparser "github.com/icemagno/valhalla/pkg/metamodel"
)

var logFOM = logger.NewLogger("FOM-Service")

// ResolveFOMPath searches for a FOM file on disk across common repository and distribution directories.
func ResolveFOMPath(rawPath string) string {
	cleanPath := strings.TrimPrefix(rawPath, "file:///")
	cleanPath = strings.TrimPrefix(cleanPath, "file://")
	cleanPath = strings.TrimPrefix(cleanPath, "file:")

	normPath := strings.ReplaceAll(cleanPath, "\\", "/")

	// 1. Direct path check
	if _, err := os.Stat(normPath); err == nil {
		return normPath
	}

	base := filepath.Base(normPath)

	// 2. Standard search locations
	candidates := []string{
		filepath.Join("foms", base),
		filepath.Join("bin", "foms", base),
		filepath.Join("..", "foms", base),
		filepath.Join("..", "..", "foms", base),
		filepath.Join("examples", "java-federate", "foms", base),
		filepath.Join("..", "..", "examples", "java-federate", "foms", base),
		filepath.Join("dist", "bin", "foms", base),
		filepath.Join("..", "..", "dist", "bin", "foms", base),
		filepath.Join("examples", "java-federate", "foms", "siso-rpr-v2.0", base),
		filepath.Join("test", "resources", "rpr-fom-v2.0", base),
		filepath.Join("resources", "rpr-fom-v2.0", base),
		filepath.Join("..", "test", "resources", "rpr-fom-v2.0", base),
		filepath.Join("..", "..", "test", "resources", "rpr-fom-v2.0", base),
	}

	for _, target := range candidates {
		if _, err := os.Stat(target); err == nil {
			return target
		}
	}

	return ""
}

// IngestFOMIntoFederation parses a FOM module (file path, URL, or raw XML string) into the federation's OMT registry.
func IngestFOMIntoFederation(fed *FederationExecution, fomRef string) error {
	if fed == nil || strings.TrimSpace(fomRef) == "" {
		return nil
	}

	trimmed := strings.TrimSpace(fomRef)

	// Check if fomRef is inline XML
	if strings.Contains(trimmed, "<objectModel") {
		if err := fomparser.ParseFOMXML(fed.OMT(), []byte(trimmed)); err != nil {
			logFOM.Warn("Failed to parse inline FOM XML module: %v", err)
			return err
		}
		logFOM.Info("Ingested inline FOM XML module (%d bytes) for federation [%s]", len(trimmed), fed.Name)
		return nil
	}

	// File path resolution
	resolved := ResolveFOMPath(trimmed)
	if resolved == "" {
		logFOM.Warn("Could not resolve FOM file '%s' on disk for federation [%s]", trimmed, fed.Name)
		return os.ErrNotExist
	}

	if err := fomparser.ParseFOMFile(fed.OMT(), resolved); err != nil {
		logFOM.Warn("Failed to parse FOM file '%s': %v", resolved, err)
		return err
	}

	logFOM.Info("Ingested FOM file '%s' (from '%s') for federation [%s]", resolved, trimmed, fed.Name)
	return nil
}

// IngestCanonicalDefaults loads default FOMs for recognized standard federation executions.
func IngestCanonicalDefaults(fed *FederationExecution) {
	if fed == nil {
		return
	}

	if fed.Name == "ExampleFederation" {
		for _, mod := range []string{"RestaurantProcesses.xml", "RestaurantFood.xml", "RestaurantDrinks.xml", "RestaurantSoup.xml"} {
			_ = IngestFOMIntoFederation(fed, mod)
		}
	} else if fed.Name == "ValhallaAdvancedFed" || fed.Name == "ValhallaHomologFederation" {
		_ = IngestFOMIntoFederation(fed, "ValhallaAdvanced.xml")
	}
}

// EnsureClassAndAttrInOMT ensures that a dynamically requested class/attribute exists in the OMT.
func EnsureClassAndAttrInOMT(omt *core.OMTRegistry, className string, attrName string) (int32, int32, error) {
	// Look up class
	cls, ok := omt.GetObjectClassByName(className)
	if !ok {
		return 0, 0, core.ErrClassNotFound
	}

	// Look up attribute
	attr, err := omt.ResolveAttributeByName(cls.Handle, attrName)
	if err != nil {
		return int32(cls.Handle), 0, err
	}

	return int32(cls.Handle), int32(attr.Handle), nil
}
