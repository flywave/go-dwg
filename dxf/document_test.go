package dxf

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewDocumentSkeleton(t *testing.T) {
	doc := NewDocument(R2018)
	var buf bytes.Buffer
	if err := doc.SaveTo(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"HEADER", "$ACADVER", "AC1032", "TABLES", "LAYER",
		"LTYPE", "Continuous", "STYLE", "Standard", "APPID", "DIMSTYLE", "BLOCK_RECORD",
		"BLOCKS", "*Model_Space", "*Paper_Space", "ENTITIES", "OBJECTS", "ENDSEC", "EOF"} {
		if !strings.Contains(out, want) {
			t.Errorf("skeleton missing %q", want)
		}
	}
	// The skeleton must round-trip cleanly.
	back, err := Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if back.Version != R2018 {
		t.Errorf("version = %v", back.Version)
	}
	if got := back.Table("LAYER"); got == nil || len(got.Entries) != 1 || got.Entries[0].Name() != "0" {
		t.Errorf("LAYER table: %+v", got)
	}
	if back.Block("*Model_Space") == nil {
		t.Error("*Model_Space missing after round trip")
	}
}

func TestNewDocumentR12Dialect(t *testing.T) {
	doc := NewDocument(R12)
	var buf bytes.Buffer
	if err := doc.SaveTo(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "AcDb") {
		t.Errorf("R12 skeleton must omit subclass markers")
	}
	if strings.Contains(out, "OBJECTS") {
		t.Errorf("R12 must not carry an OBJECTS section")
	}
	back, err := Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if back.Version != R12 {
		t.Errorf("version = %v", back.Version)
	}
}

func TestDocumentWriteStability(t *testing.T) {
	doc := NewDocument(R2004)
	line := &Line{Start: Pt2(0, 0), End: Pt2(1, 1)}
	line.Handle = doc.AllocHandle()
	doc.Entities = append(doc.Entities, &Entity{Type: "LINE", Data: line})

	var first, second bytes.Buffer
	if err := doc.SaveTo(&first); err != nil {
		t.Fatal(err)
	}
	back, err := Parse(bytes.NewReader(first.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if err := back.SaveTo(&second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Errorf("write→parse→write is not byte-stable")
	}
}

func TestForeignContentPreserved(t *testing.T) {
	src := strings.Join([]string{
		"  0", "SECTION", "  2", "HEADER",
		"  9", "$ACADVER", "  1", "AC1024",
		"  9", "$EXOTIC", " 40", "1.25", " 70", "3",
		"  0", "ENDSEC",
		"  0", "SECTION", "  2", "ENTITIES",
		"  0", "LINE", "  5", "2B", "  8", "0",
		" 10", "0", " 20", "0", " 30", "0",
		" 11", "9", " 21", "9", " 31", "0",
		"  0", "MYSTERYWIDGET", "  5", "2C", " 90", "11", " 10", "5", " 20", "6",
		"  0", "ENDSEC",
		"  0", "SECTION", "  2", "ACADSECRETZONE",
		" 40", "42",
		"  0", "ENDSEC",
		"  0", "EOF",
	}, "\n")

	doc, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != R2010 {
		t.Fatalf("version = %v", doc.Version)
	}
	// Unknown header var preserved with its raw value pairs.
	hv, ok := doc.Header.Find("$EXOTIC")
	if !ok || len(hv.Values) != 2 || hv.Values[0].Float() != 1.25 || hv.Values[1].Int() != 3 {
		t.Fatalf("$EXOTIC lost: %+v", doc.Header)
	}
	// Unknown entity preserved.
	var foundUnknown bool
	for _, e := range doc.Entities {
		if e.Type == "MYSTERYWIDGET" {
			foundUnknown = true
			u := e.Data.(*UnknownEntity)
			if len(u.Tags) != 4 {
				t.Errorf("unknown entity tags: %+v", u.Tags)
			}
		}
	}
	if !foundUnknown {
		t.Error("MYSTERYWIDGET dropped")
	}
	// Unknown section preserved.
	if len(doc.OtherSections) != 1 || doc.OtherSections[0].Name != "ACADSECRETZONE" {
		t.Errorf("other sections: %+v", doc.OtherSections)
	}
	// Round trip keeps everything.
	var buf bytes.Buffer
	if err := doc.SaveTo(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"MYSTERYWIDGET", "ACADSECRETZONE", "$EXOTIC"} {
		if !strings.Contains(out, want) {
			t.Errorf("round trip lost %q", want)
		}
	}
}

func TestParseEntitiesFromText(t *testing.T) {
	src := strings.Join([]string{
		"  0", "SECTION", "  2", "ENTITIES",
		"  0", "CIRCLE", "  8", "Eyes", " 62", "3",
		" 10", "1", " 20", "2", " 30", "0", " 40", "3.5",
		"  0", "LWPOLYLINE", "  8", "0", " 90", "3", " 70", "1",
		" 10", "0", " 20", "0", " 42", "0.5",
		" 10", "4", " 20", "0",
		" 10", "4", " 20", "4",
		"  0", "ENDSEC", "  0", "EOF",
	}, "\n")
	doc, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Entities) != 2 {
		t.Fatalf("entities = %d", len(doc.Entities))
	}
	c, ok := doc.Entities[0].Data.(*Circle)
	if !ok || c.Radius != 3.5 || c.Layer != "Eyes" || c.Color != 3 {
		t.Errorf("circle: %+v", doc.Entities[0].Data)
	}
	p, ok := doc.Entities[1].Data.(*LwPolyline)
	if !ok || !p.Closed || len(p.Vertices) != 3 || p.Vertices[0].Bulge != 0.5 {
		t.Errorf("lwpolyline: %+v", doc.Entities[1].Data)
	}
}

func TestHeaderAccessors(t *testing.T) {
	doc := NewDocument(R2018)
	if _, ok := doc.Header.Find("$ACADVER"); !ok {
		t.Fatal("$ACADVER missing")
	}
	doc.Header.SetPoint("$EXTMIN", Pt3(0, 0, 0))
	p, ok := doc.Header.Point("$EXTMIN")
	if !ok || p != (Point3{}) {
		t.Errorf("$EXTMIN = %+v", p)
	}
	doc.Header.SetInt("$INSUNITS", 6)
	if n, _ := doc.Header.Int("$INSUNITS"); n != 6 {
		t.Errorf("$INSUNITS = %d", n)
	}
	doc.Header.Remove("$INSUNITS")
	if _, ok := doc.Header.Int("$INSUNITS"); ok {
		t.Error("$INSUNITS not removed")
	}
}

func TestAllocHandle(t *testing.T) {
	doc := NewDocument(R2018)
	base := doc.nextHandle
	if doc.AllocHandle() != base+1 || doc.AllocHandle() != base+2 {
		t.Error("AllocHandle not sequential")
	}
}

func TestBinaryRoundTrip(t *testing.T) {
	doc := NewDocument(R2018)
	line := &Line{Start: Pt2(0.5, 0.25), End: Pt2(3, 4)}
	line.Handle = 0x2A
	doc.Entities = append(doc.Entities, &Entity{Type: "LINE", Data: line})
	circ := &Circle{Center: Pt2(1, 1), Radius: 2}
	doc.Entities = append(doc.Entities, &Entity{Type: "CIRCLE", Data: circ})

	var bin bytes.Buffer
	if err := doc.writeBinary(&bin); err != nil {
		t.Fatal(err)
	}
	if !isBinaryDxf(bin.Bytes()) {
		t.Fatal("binary sentinel missing")
	}
	back, err := Parse(bytes.NewReader(bin.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if back.Version != R2018 {
		t.Errorf("binary version = %v", back.Version)
	}
	if len(back.Entities) != 2 {
		t.Fatalf("binary entities = %d", len(back.Entities))
	}
	got, ok := back.Entities[0].Data.(*Line)
	if !ok || got.Start.X != 0.5 || got.End.X != 3 {
		t.Errorf("binary line: %+v", back.Entities[0].Data)
	}
	got2, ok := back.Entities[1].Data.(*Circle)
	if !ok || got2.Radius != 2 {
		t.Errorf("binary circle: %+v", back.Entities[1].Data)
	}
}
