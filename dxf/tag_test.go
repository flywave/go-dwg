package dxf

import (
	"strings"
	"testing"
)

func TestParseTagsBasic(t *testing.T) {
	src := "  0\nSECTION\n  2\nHEADER\n  9\n$ACADVER\n  1\nAC1032\n  0\nENDSEC\n  0\nEOF\n"
	tags, err := ParseTags(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []Tag{
		{0, "SECTION"}, {2, "HEADER"}, {9, "$ACADVER"}, {1, "AC1032"},
		{0, "ENDSEC"}, {0, "EOF"},
	}
	if len(tags) != len(want) {
		t.Fatalf("got %d tags, want %d", len(tags), len(want))
	}
	for i := range want {
		if tags[i] != want[i] {
			t.Errorf("tag[%d] = %+v, want %+v", i, tags[i], want[i])
		}
	}
}

func TestParseTagsCRLFAndBOM(t *testing.T) {
	src := "\ufeff  0\r\nSECTION\r\n  2\r\nA\r\n"
	tags, err := ParseTags(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 || tags[0].Value != "SECTION" || tags[1].Value != "A" {
		t.Fatalf("CRLF/BOM parse failed: %+v", tags)
	}
}

func TestParseTagsErrors(t *testing.T) {
	if _, err := ParseTags(strings.NewReader("xyz\n1\n")); err == nil {
		t.Error("non-integer code line must error")
	}
	if _, err := ParseTags(strings.NewReader("  0\n")); err == nil {
		t.Error("truncated pair must error")
	}
}

func TestTagAccessors(t *testing.T) {
	if v := (Tag{70, " 42 "}).Int(); v != 42 {
		t.Errorf("Int() = %d", v)
	}
	if v := (Tag{40, "2.5"}).Float(); v != 2.5 {
		t.Errorf("Float() = %v", v)
	}
	if v := (Tag{62, "-3"}).Int(); v != -3 {
		t.Errorf("negative Int() = %d", v)
	}
}

func TestFormatFloatRoundTrips(t *testing.T) {
	for _, v := range []float64{0, 1, -1, 0.5, 3.14159265358979, 1e20, -1e20, 1e-7, 123456789.123456} {
		parsed := Tag{40, FormatFloat(v)}.Float()
		if parsed != v {
			t.Errorf("FormatFloat(%v) = %q reparsed %v", v, FormatFloat(v), parsed)
		}
	}
}

func TestTagWriterPairFormat(t *testing.T) {
	var sb strings.Builder
	w := NewTagWriter(&sb)
	w.Pair(0, "EOF")
	if sb.String() != "  0\nEOF\n" {
		t.Errorf("pair format = %q", sb.String())
	}
}

func TestTagWriterHandleHex(t *testing.T) {
	var sb strings.Builder
	w := NewTagWriter(&sb)
	w.Handle(5, 0x1A3F)
	if !strings.Contains(sb.String(), "1A3F") {
		t.Errorf("handle not uppercase hex: %q", sb.String())
	}
}

func TestTagWriterPoint(t *testing.T) {
	var sb strings.Builder
	w := NewTagWriter(&sb)
	w.Point(10, Pt3(1, 2, 3))
	for _, code := range []string{" 10\n", " 20\n", " 30\n"} {
		if !strings.Contains(sb.String(), code) {
			t.Errorf("missing %s in %q", code, sb.String())
		}
	}
}

func TestSanitizeValue(t *testing.T) {
	if got := sanitizeValue("a\r\nb\nc\rd"); got != "a b c d" {
		t.Errorf("sanitizeValue = %q", got)
	}
}

func TestParseUintHex(t *testing.T) {
	if v := parseUintHex("1A3F"); v != 0x1A3F {
		t.Errorf("hex parse = %x", v)
	}
	if v := parseUintHex("83"); v != 0x83 {
		t.Errorf("hex 83 = %x", v)
	}
	if v := parseUintHex(""); v != 0 {
		t.Errorf("empty handle = %x", v)
	}
}
