package dwg

import (
	"math"
	"os"
	"testing"
)

// Decode the canonical synthetic fixtures without the Rust oracle, pinning the
// field values the differential harness also checks.
func TestDecodeSynthetic2018Entities(t *testing.T) {
	raw, err := os.ReadFile("testdata/synthetic_2018.dwg")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	gf, err := DwgFromBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	ents, summary, ok, err := gf.DecodeAllEntities()
	if !ok || err != nil {
		t.Fatalf("decode: ok=%v err=%v", ok, err)
	}
	if summary.Decoded != 5 || summary.Unhandled != 0 || summary.Errored != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if len(ents) != 5 {
		t.Fatalf("entities = %d", len(ents))
	}
	labels := []string{"LINE", "LINE", "CIRCLE", "ARC", "POINT"}
	for i, want := range labels {
		if ents[i].Label != want {
			t.Errorf("entity %d label = %s want %s", i, ents[i].Label, want)
		}
	}
	l0, ok := ents[0].Value.(*Line)
	if !ok {
		t.Fatal("entity 0 is not a Line")
	}
	if l0.Start != (Point3D{X: 0, Y: 0, Z: 0}) || l0.End != (Point3D{X: 100, Y: 50, Z: 0}) {
		t.Errorf("line0 = %+v", l0)
	}
	if !l0.Is2D || l0.Extrusion != (Vec3D{X: 0, Y: 0, Z: 1}) {
		t.Errorf("line0 flags = %+v", l0)
	}
	l1, _ := ents[1].Value.(*Line)
	if l1.Is2D || l1.Start.Z != 5 || l1.End.Z != 11 {
		t.Errorf("line1 = %+v", l1)
	}
	c, _ := ents[2].Value.(*Circle)
	if c.Center != (Point3D{X: 10, Y: 20, Z: 0}) || c.Radius != 5 {
		t.Errorf("circle = %+v", c)
	}
	a, _ := ents[3].Value.(*Arc)
	if a.Radius != 12.5 || a.Center != (Point3D{X: -4, Y: 8, Z: 0}) || math.Abs(a.EndAngle-math.Pi/2) > 1e-12 {
		t.Errorf("arc = %+v", a)
	}
	p, _ := ents[4].Value.(*Point)
	if p.Position != (Point3D{X: 1.25, Y: 2.5, Z: 3.75}) {
		t.Errorf("point = %+v", p)
	}
}
