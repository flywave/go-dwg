package dwg

import "testing"

// A truncated/zero AC1021 file must open with the Deferred placeholder
// rather than panicking, mirroring the reference reader's fallback when its
// §5.1-§5.4 container parse fails.
func TestR2007ContainerFailureFallsBackToDeferred(t *testing.T) {
	bytes := make([]byte, 0x200)
	copy(bytes, []byte("AC1021"))
	gf, err := DwgFromBytes(bytes)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if gf.Version() != R2007 {
		t.Fatalf("version = %v", gf.Version())
	}
	if gf.SectionMapStatus().State != StatusDeferred {
		t.Fatalf("status = %v want Deferred", gf.SectionMapStatus())
	}
	found := false
	for _, s := range gf.Sections() {
		if s.Name == "_R2007_UNREADABLE" {
			found = true
		}
	}
	if !found {
		t.Fatal("placeholder section missing")
	}
}
