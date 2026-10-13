package dwg

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// The ODA AC1027 corpus carries one of each shape-history primitive; pin
// their decoded fields so the recovered chain stays honest. Skipped when the
// corpus sample is absent.
func TestShPrimitiveCorpus(t *testing.T) {
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

	type want struct {
		label  string
		shape  []float64
		color  uint32
		matrix [16]float64
	}
	wants := map[string]want{
		"SH_SPHERE": {
			shape: []float64{41.58550159076675},
			color: 0xc3000001,
			matrix: [16]float64{
				1, 0, 0, 202.8749733738452,
				0, 1, 0, 343.1695608826469,
				0, 0, 1, 42.68024807424331,
				0, 0, 0, 1,
			},
		},
		"SH_CYLINDER": {shape: []float64{111.8122199526575, 43.41136496477853, 43.41136496477853, 43.41136496477853}, color: 0xc3000003},
		"SH_CONE":     {shape: []float64{148.81482598453786, 47.3475237660236, 47.3475237660236, 0}, color: 0xc3000006},
		"SH_WEDGE":    {shape: []float64{102.48179587822952, 88.0025148877719, 94.91972659627017}, color: 0xc2266bd9},
	}
	seen := map[string]bool{}
	for _, e := range ents {
		w, ok := wants[e.Label]
		if !ok || seen[e.Label] {
			continue
		}
		p, ok := e.Value.(*ShPrimitive)
		if !ok {
			t.Fatalf("%s: value type %T", e.Label, e.Value)
		}
		seen[e.Label] = true
		if p.GroupCode != -1 || p.EvalFields != [2]int32{31, 8} || p.EvalType != -9999 ||
			p.Variant.Kind != 0 || p.TailCode != 1 || p.NodeFields != [3]int32{31, 8, 0} {
			t.Errorf("%s: eval/node header = %+v", e.Label, p)
		}
		if p.Color.RGB != w.color {
			t.Errorf("%s: color rgb = %#x want %#x", e.Label, p.Color.RGB, w.color)
		}
		for i, v := range w.shape {
			if math.Float64bits(p.Shape[i]) != math.Float64bits(v) {
				t.Errorf("%s: shape[%d] = %v want %v", e.Label, i, p.Shape[i], v)
			}
		}
		if e.Label == "SH_SPHERE" {
			for i, v := range w.matrix {
				if math.Float64bits(p.Matrix[i]) != math.Float64bits(v) {
					t.Errorf("%s: matrix[%d] = %v want %v", e.Label, i, p.Matrix[i], v)
				}
			}
		}
	}
	for label := range wants {
		if !seen[label] {
			t.Errorf("%s: not decoded", label)
		}
	}
}
