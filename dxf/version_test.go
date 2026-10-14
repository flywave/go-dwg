package dxf

import "testing"

func TestAcadVerMapping(t *testing.T) {
	cases := map[DxfVersion]string{
		R12: "AC1009", R14: "AC1014", R2000: "AC1015", R2004: "AC1018",
		R2007: "AC1021", R2010: "AC1024", R2013: "AC1027", R2018: "AC1032",
	}
	for v, want := range cases {
		if got := v.AcadVer(); got != want {
			t.Errorf("%s.AcadVer() = %q, want %q", v, got, want)
		}
	}
}

func TestParseDxfVersion(t *testing.T) {
	for _, c := range []struct {
		in   string
		want DxfVersion
		ok   bool
	}{
		{"R12", R12, true},
		{"r12", R12, true},
		{"AC1009", R12, true},
		{"ac1032", R2018, true},
		{" R2013 ", R2013, true},
		{"R13", R14, true}, // R13 maps onto the R14 dialect
		{"AC1012", R14, true},
		{"bogus", 0, false},
		{"", 0, false},
	} {
		got, ok := ParseDxfVersion(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ParseDxfVersion(%q) = %v,%v; want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestVersionFeatureGates(t *testing.T) {
	if R12.SupportsSubclassMarkers() {
		t.Error("R12 must not support subclass markers")
	}
	if R12.SupportsHandles() {
		t.Error("R12 must not support handles")
	}
	if !R14.SupportsSubclassMarkers() || !R14.SupportsHandles() {
		t.Error("R14 must support subclass markers and handles")
	}
	if R14.SupportsObjectsSection() {
		t.Error("OBJECTS is R2000+")
	}
	if !R2000.SupportsObjectsSection() {
		t.Error("R2000 must support OBJECTS")
	}
	if DxfVersionFromAcadVer("AC1032") != R2018 {
		t.Error("AC1032 → R2018")
	}
	// Unknown magics fall forward to the widest dialect.
	if DxfVersionFromAcadVer("AC9999") != DefaultVersion {
		t.Error("unknown magic should default")
	}
}
