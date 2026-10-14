package dxf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dwgCorpus collects the synthetic fixtures plus the ODA samples when
// present. The conversion suite runs against whatever is available.
func dwgCorpus(t *testing.T) []string {
	t.Helper()
	patterns := []string{
		"../dwg/testdata/synthetic_*.dwg",
		"../dwg/testdata/oda_corpus/*.dwg",
	}
	var files []string
	for _, p := range patterns {
		m, err := filepath.Glob(p)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skip("no DWG corpus available")
	}
	return files
}

func TestConvertDwgCorpus(t *testing.T) {
	for _, path := range dwgCorpus(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			doc, err := ConvertDwgFile(path, R2018)
			if err != nil {
				t.Fatalf("convert: %v", err)
			}
			if doc.Version != R2018 {
				t.Errorf("version = %v", doc.Version)
			}
			// The skeleton always carries the layer roster.
			lt := doc.Table("LAYER")
			if lt == nil || len(lt.Entries) == 0 {
				t.Fatal("no LAYER table")
			}
			// Every decoded layer must appear exactly once.
			seen := map[string]int{}
			for _, e := range lt.Entries {
				seen[e.Name()]++
			}
			for name, n := range seen {
				if n > 1 {
					t.Errorf("layer %q duplicated %d times", name, n)
				}
			}
			// The produced document must parse back cleanly.
			var buf strings.Builder
			if err := doc.SaveTo(w2(&buf)); err != nil {
				t.Fatalf("save: %v", err)
			}
			back, err := Parse(strings.NewReader(buf.String()))
			if err != nil {
				t.Fatalf("reparse: %v", err)
			}
			if back.Version != R2018 {
				t.Errorf("reparse version = %v", back.Version)
			}
			if len(back.Entities) != len(doc.Entities) {
				t.Errorf("entity count drifted: %d → %d", len(doc.Entities), len(back.Entities))
			}
		})
	}
}

func TestConvertDwgEntitiesPresent(t *testing.T) {
	// The synthetic fixtures carry LINE/CIRCLE/ARC/POINT records across
	// every release; require at least one converted drawable.
	found := 0
	for _, path := range dwgCorpus(t) {
		doc, err := ConvertDwgFile(path, R2018)
		if err != nil {
			continue
		}
		for _, e := range doc.Entities {
			if e.Type != "COMMENT" {
				found++
			}
		}
	}
	if found == 0 {
		t.Error("no drawable entities converted from the corpus")
	}
}

func TestConvertDwgDeterministic(t *testing.T) {
	paths := dwgCorpus(t)
	if len(paths) == 0 {
		t.Skip("no corpus")
	}
	path := paths[0]
	d1, err := ConvertDwgFile(path, R2018)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := ConvertDwgFile(path, R2018)
	if err != nil {
		t.Fatal(err)
	}
	var b1, b2 strings.Builder
	if err := d1.SaveTo(w2(&b1)); err != nil {
		t.Fatal(err)
	}
	if err := d2.SaveTo(w2(&b2)); err != nil {
		t.Fatal(err)
	}
	if b1.String() != b2.String() {
		t.Error("conversion is not deterministic")
	}
}

func TestConvertDwgFileMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := ConvertDwgFile(filepath.Join(dir, "nope.dwg"), R2018); err == nil {
		t.Error("expected error for missing file")
	}
}

// buf2Writer/w2 are tiny aliases so the helpers above share one writer type.
type buf2Writer struct{ *strings.Builder }

func (w *buf2Writer) Write(p []byte) (int, error) { return w.Builder.Write(p) }

func w2(b *strings.Builder) *buf2Writer { return &buf2Writer{b} }

func TestSkipCommentsEmitted(t *testing.T) {
	// The point-light sample carries 3DSOLID records the converter skips;
	// the omissions must surface as group-999 comments.
	path := filepath.Join("..", "dwg", "testdata", "oda_corpus", "OpenRL_PointLight.dwg")
	if _, err := os.Stat(path); err != nil {
		t.Skip("ODA corpus not available")
	}
	doc, err := ConvertDwgFile(path, R2018)
	if err != nil {
		t.Fatal(err)
	}
	var saw3DSolid bool
	for _, c := range doc.Comments {
		if strings.Contains(c, "skipped: 3DSOLID") {
			saw3DSolid = true
		}
	}
	if !saw3DSolid {
		t.Errorf("expected a 3DSOLID skip comment, got %v", doc.Comments)
	}
	// And it must round-trip through the writer.
	var buf strings.Builder
	if err := doc.SaveTo(w2(&buf)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "skipped: 3DSOLID") {
		t.Error("skip comment lost in output")
	}
}
