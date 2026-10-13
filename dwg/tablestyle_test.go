package dwg

import (
	"os"
	"path/filepath"
	"testing"
)

// The ODA corpus carries one TABLESTYLE per file — an AC1018 record read
// through branch A and an AC1027 record read through branch B (25.4 full
// cell styles). Pin their decoded fields so both recovered layouts stay
// honest. Skipped when the corpus samples are absent.
func TestTableStyleCorpus(t *testing.T) {
	cases := []struct {
		file  string
		check func(t *testing.T, s *TableStyleClass)
	}{
		{"OdWriteEx XRef.dwg", func(t *testing.T, s *TableStyleClass) {
			if s.Name != "Standard" || s.IntA != 0 || s.IntB != 0 {
				t.Errorf("header = %+v", s)
			}
			if s.DoubleA != 0.06 || s.DoubleB != 0.06 {
				t.Errorf("margins = %v %v", s.DoubleA, s.DoubleB)
			}
			for i, d := range &s.Directions {
				want := []float64{0.18, 0.25, 0.18}[i]
				if d.Double != want {
					t.Errorf("direction[%d] margin = %v want %v", i, d.Double, want)
				}
			}
		}},
		{"OpenRL_PointLight.dwg", func(t *testing.T, s *TableStyleClass) {
			if s.Name != "Standard" || s.Byte != 0 || s.IntA32 != 8 || s.IntB32 != 101 {
				t.Errorf("header = %+v", s)
			}
			if s.DefaultCell.GridCount != 0 {
				t.Errorf("default grid count = %d", s.DefaultCell.GridCount)
			}
			if s.CellCount != 3 || len(s.Cells) != 3 {
				t.Fatalf("cell count = %d", s.CellCount)
			}
			for i, c := range s.Cells {
				if c.Cell.GridCount != 6 || len(c.Cell.GridLines) != 6 {
					t.Errorf("cell %d grid count = %d", i, c.Cell.GridCount)
				}
			}
		}},
	}
	for _, tc := range cases {
		path := filepath.Join("testdata", "oda_corpus", tc.file)
		if _, err := os.Stat(path); err != nil {
			t.Skipf("ODA corpus sample %s not available", tc.file)
		}
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			gf, err := DwgFromBytes(raw)
			if err != nil {
				t.Fatal(err)
			}
			ents, summary, ok, err := gf.DecodeAllEntities()
			if !ok || err != nil {
				t.Fatalf("DecodeAllEntities: ok=%v err=%v", ok, err)
			}
			if summary.Errored != 0 || summary.Unhandled != 0 {
				t.Fatalf("summary = %+v, want 0/0", summary)
			}
			for _, e := range ents {
				if e.Label != "TABLESTYLE" {
					continue
				}
				v, ok := e.Value.(*TableStyleClass)
				if !ok {
					t.Fatalf("value type %T", e.Value)
				}
				tc.check(t, v)
				return
			}
			t.Fatal("TABLESTYLE not decoded")
		})
	}
}
