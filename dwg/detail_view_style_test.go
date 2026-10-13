package dwg

import (
	"os"
	"path/filepath"
	"testing"
)

// The ODA AC1027 corpus carries one ACDBDETAILVIEWSTYLE record; pin its
// decoded fields so the recovered field order stays honest. Skipped when
// the corpus sample is absent.
func TestDetailViewStyleCorpus(t *testing.T) {
	path := filepath.Join("testdata", "oda_corpus", "OpenRL_PointLight.dwg")
	if _, err := os.Stat(path); err != nil {
		t.Skip("ODA corpus sample not available")
	}
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
	if summary.Errored != 0 {
		t.Fatalf("errored=%d, want 0", summary.Errored)
	}
	var found *AcadDetailViewStyle
	for _, e := range ents {
		if e.Label != "DETAILVIEWSTYLE" {
			continue
		}
		v, ok := e.Value.(*AcadDetailViewStyle)
		if !ok {
			t.Fatalf("value type %T", e.Value)
		}
		found = v
	}
	if found == nil {
		t.Fatal("DETAILVIEWSTYLE not decoded")
	}
	s := found
	if s.UnknownHead != 0 || s.Name != "Metric50" || !s.UnknownFlag || s.UnknownA != 0 {
		t.Errorf("header = %+v", s)
	}
	if s.UnknownB != 3 || s.ByteA != 1 || s.ByteB != 0 {
		t.Errorf("bytes: B=%d A=%d", s.UnknownB, s.ByteA)
	}
	if s.IntA != 25 || s.IntB != 0 || s.IntC != 1 || s.IntD != 25 || s.IntE != 25 {
		t.Errorf("ints = %d %d %d %d %d", s.IntA, s.IntB, s.IntC, s.IntD, s.IntE)
	}
	if s.DoubleA != 5 || s.DoubleB != 0 || s.DoubleC != 5 || s.DoubleD != 5 || s.DoubleE != 15 {
		t.Errorf("doubles = %v %v %v %v %v", s.DoubleA, s.DoubleB, s.DoubleC, s.DoubleD, s.DoubleE)
	}
	colors := [][3]uint32{{s.ColorA.RGB, 0xc0000000, 0}, {s.ColorB.RGB, 0xc0000000, 1},
		{s.ColorC.RGB, 0xc0000000, 2}, {s.ColorD.RGB, 0xc0000000, 3},
		{s.ColorE.RGB, 0xc0000000, 4}, {s.ColorF.RGB, 0xc0000000, 5}}
	for i, c := range colors {
		if c[0] != c[1] {
			t.Errorf("color[%d] rgb = %#x want %#x", i, c[0], c[1])
		}
	}
	if s.StringA != "" || s.StringB != "%<\\AcVar ViewDetailId>% (%<\\AcVar ViewScale \\f \"%sn\">%)" {
		t.Errorf("strings = %q %q", s.StringA, s.StringB)
	}
}
