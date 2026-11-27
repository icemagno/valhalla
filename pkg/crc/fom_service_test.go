package crc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/icemagno/valhalla/pkg/common"
)

func TestResolveFOMPath_RecursiveCanonicalFomsDir(t *testing.T) {
	tmpRoot := t.TempDir()
	fomsDir := filepath.Join(tmpRoot, "foms")

	// Create arbitrary nested user directory layouts
	paths := []string{
		filepath.Join(fomsDir, "naval", "RPR_Physical.xml"),
		filepath.Join(fomsDir, "air", "nested", "deep", "Tactical.xml"),
		filepath.Join(fomsDir, "simA", "common.xml"),
		filepath.Join(fomsDir, "simB", "common.xml"),
	}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(p, []byte("<objectModel/>"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}
	// Also create a secret file OUTSIDE ./foms to prove anti-traversal containment
	outsideFile := filepath.Join(tmpRoot, "outside.xml")
	if err := os.WriteFile(outsideFile, []byte("<objectModel/>"), 0o644); err != nil {
		t.Fatalf("write outside file failed: %v", err)
	}

	prevDir := CanonicalFOMDir
	CanonicalFOMDir = fomsDir
	defer func() { CanonicalFOMDir = prevDir }()

	// 1. Recursive lookup by filename only
	got := ResolveFOMPath("Tactical.xml")
	want := filepath.Join(fomsDir, "air", "nested", "deep", "Tactical.xml")
	if got != want {
		t.Fatalf("recursive lookup mismatch: got %q, want %q", got, want)
	}

	// 2. file:/// URL prefix stripped and resolved recursively
	gotURL := ResolveFOMPath("file:///C:/arbitrary/client/dir/RPR_Physical.xml")
	wantURL := filepath.Join(fomsDir, "naval", "RPR_Physical.xml")
	if gotURL != wantURL {
		t.Fatalf("file:// URL lookup mismatch: got %q, want %q", gotURL, wantURL)
	}

	// 3. Disambiguation via relative sub-path inside ./foms
	gotSub := ResolveFOMPath("simB/common.xml")
	wantSub := filepath.Join(fomsDir, "simB", "common.xml")
	if gotSub != wantSub {
		t.Fatalf("sub-path disambiguation mismatch: got %q, want %q", gotSub, wantSub)
	}

	// 4. Anti-traversal: files outside ./foms must NEVER be resolved
	if gotOutside := ResolveFOMPath("../outside.xml"); gotOutside != "" {
		t.Fatalf("path traversal allowed escaping ./foms: got %q", gotOutside)
	}
	if gotAbsOutside := ResolveFOMPath(outsideFile); gotAbsOutside != "" {
		t.Fatalf("absolute path outside ./foms must not be resolved: got %q", gotAbsOutside)
	}
}

func TestIngestFOMIntoFederation_InlineValidAndMalformed(t *testing.T) {
	fed := NewFederationExecution(1, "FOMTestFed")

	validXML := `<?xml version="1.0" encoding="UTF-8"?>
<objectModel>
  <objects>
    <objectClass>
      <name>HLAobjectRoot</name>
      <objectClass>
        <name>RadarTrack</name>
        <attribute><name>Position</name><dataType>HLAASCIIstring</dataType></attribute>
      </objectClass>
    </objectClass>
  </objects>
</objectModel>`

	if err := IngestFOMIntoFederation(fed, validXML); err != nil {
		t.Fatalf("valid inline FOM failed: %v", err)
	}
	cls, ok := fed.OMT().GetChildObjectClass(common.HLAobjectRootHandle, "RadarTrack")
	if !ok || cls == nil {
		t.Fatalf("expected RadarTrack class in OMT under HLAobjectRoot")
	}

	// Malformed XML must return an explicit error
	malformedXML := `<objectModel><objects><unclosed></objectModel>`
	if err := IngestFOMIntoFederation(fed, malformedXML); err == nil {
		t.Fatalf("expected error for malformed FOM XML, got nil")
	}

	// Non-existent file path must return an explicit error
	if err := IngestFOMIntoFederation(fed, "nonexistent_fom_12345.xml"); err == nil {
		t.Fatalf("expected error for non-existent FOM file, got nil")
	}
}
