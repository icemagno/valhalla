package crc

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/icemagno/valhalla/pkg/crc/core"
	"github.com/icemagno/valhalla/pkg/logger"
	fomparser "github.com/icemagno/valhalla/pkg/metamodel"
)

var logFOM = logger.NewLogger("FOM-Service")

// CanonicalFOMDir is the official, canonical directory (relative to the CRC execution directory)
// where all FOM XML modules are recursively discovered.
var CanonicalFOMDir = "foms"

// ResolveFOMPath searches for a FOM file strictly inside CanonicalFOMDir (./foms) using
// deterministic recursive directory traversal, regardless of how subdirectories are organized.
func ResolveFOMPath(rawPath string) string {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return ""
	}

	cleanPath := strings.TrimPrefix(trimmed, "file:///")
	cleanPath = strings.TrimPrefix(cleanPath, "file://")
	cleanPath = strings.TrimPrefix(cleanPath, "file:")
	normPath := filepath.ToSlash(strings.ReplaceAll(cleanPath, "\\", "/"))

	rootDir := CanonicalFOMDir
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return ""
	}
	if info, err := os.Stat(absRoot); err != nil || !info.IsDir() {
		return ""
	}

	// 1. Direct relative sub-path match inside ./foms (supports disambiguating e.g. "simA/FOM.xml" or "foms/simA/FOM.xml")
	relCandidate := strings.TrimPrefix(normPath, "./")
	relCandidate = strings.TrimPrefix(relCandidate, "foms/")
	if relCandidate != "" && !filepath.IsAbs(relCandidate) {
		joined := filepath.Clean(filepath.Join(absRoot, filepath.FromSlash(relCandidate)))
		if rel, err := filepath.Rel(absRoot, joined); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if info, err := os.Stat(joined); err == nil && !info.IsDir() {
				return joined
			}
		}
	}

	// 2. Deterministic recursive search inside ./foms by filename (filepath.WalkDir visits in lexical order)
	base := filepath.Base(normPath)
	if base == "" || base == "." || base == "/" || base == ".." {
		return ""
	}

	var found string
	_ = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == base {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	return found
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
