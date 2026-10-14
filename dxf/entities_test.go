package dxf

import (
	"strings"
	"testing"
)

// roundTripEntity writes an entity at the given dialect and parses it back,
// returning the reconstructed payload.
func roundTripEntity(t *testing.T, e *Entity, ver DxfVersion) *Entity {
	t.Helper()
	var sb strings.Builder
	w := NewTagWriter(&sb)
	if err := writeEntity(w, e, ver); err != nil {
		t.Fatalf("write %s: %v", e.Type, err)
	}
	tags, err := ParseTags(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("reparse %s: %v", e.Type, err)
	}
	if len(tags) == 0 || tags[0].Code != 0 {
		t.Fatalf("entity stream must start with a 0-code: %+v", tags)
	}
	return parseEntity(tags[0].Value, tags[1:])
}

func TestRoundTripLine(t *testing.T) {
	l := &Line{Start: Pt3(1, 2, 3), End: Pt3(4, 5, 6)}
	l.Handle = 0x83
	l.Layer = "Walls"
	l.Color, l.HasColor = 1, true
	e := roundTripEntity(t, &Entity{Type: "LINE", Data: l}, R2018)
	got, ok := e.Data.(*Line)
	if !ok {
		t.Fatalf("payload type %T", e.Data)
	}
	if got.Start != l.Start || got.End != l.End {
		t.Errorf("geometry drifted: %+v", got)
	}
	if got.Handle != 0x83 || got.Layer != "Walls" || got.Color != 1 {
		t.Errorf("common drifted: %+v", got.Common)
	}
}

func TestRoundTripLineR12OmitsSubclass(t *testing.T) {
	l := &Line{Start: Pt2(0, 0), End: Pt2(1, 1)}
	var sb strings.Builder
	w := NewTagWriter(&sb)
	if err := writeEntity(w, &Entity{Type: "LINE", Data: l}, R12); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	if strings.Contains(out, "AcDb") {
		t.Errorf("R12 must not carry subclass markers: %q", out)
	}
	e := parseEntity("LINE", mustTags(t, out))
	got, ok := e.Data.(*Line)
	if !ok || got.End != l.End {
		t.Errorf("R12 round trip failed: %+v", e.Data)
	}
}

func TestRoundTripCircleArcPoint(t *testing.T) {
	c := &Circle{Center: Pt3(5, 5, 0), Radius: 2.5}
	got := roundTripEntity(t, &Entity{Type: "CIRCLE", Data: c}, R2018).Data.(*Circle)
	if got.Center != c.Center || got.Radius != c.Radius {
		t.Errorf("circle: %+v", got)
	}

	a := &Arc{Center: Pt2(0, 0), Radius: 1, StartAngle: 10, EndAngle: 95.5}
	got2 := roundTripEntity(t, &Entity{Type: "ARC", Data: a}, R2018).Data.(*Arc)
	if got2.StartAngle != 10 || got2.EndAngle != 95.5 {
		t.Errorf("arc: %+v", got2)
	}

	p := &DxfPoint{Position: Pt3(1, 1, 0), XAxisAngle: 45}
	got3 := roundTripEntity(t, &Entity{Type: "POINT", Data: p}, R2018).Data.(*DxfPoint)
	if got3.XAxisAngle != 45 {
		t.Errorf("point: %+v", got3)
	}
}

func TestRoundTripTextFamily(t *testing.T) {
	tx := &Text{Height: 2.5, Value: "Hello, DXF", Rotation: 30, Style: "Standard"}
	got := roundTripEntity(t, &Entity{Type: "TEXT", Data: tx}, R2018).Data.(*Text)
	if got.Value != tx.Value || got.Height != 2.5 || got.Style != "Standard" || got.Rotation != 30 {
		t.Errorf("text: %+v", got)
	}

	at := &Attrib{Tag: "TAG1", Flags: 1, FieldLength: 3}
	at.Text.Value = "v1"
	got2 := roundTripEntity(t, &Entity{Type: "ATTRIB", Data: at}, R2018).Data.(*Attrib)
	if got2.Tag != "TAG1" || got2.Value != "v1" || got2.Flags != 1 || got2.FieldLength != 3 {
		t.Errorf("attrib: %+v", got2)
	}

	ad := &Attdef{Tag: "T2", Prompt: "Enter value"}
	ad.Text.Value = "default"
	got3 := roundTripEntity(t, &Entity{Type: "ATTDEF", Data: ad}, R2018).Data.(*Attdef)
	if got3.Prompt != "Enter value" || got3.Tag != "T2" || got3.Value != "default" {
		t.Errorf("attdef: %+v", got3)
	}
}

func TestMTextChunking(t *testing.T) {
	long := strings.Repeat("A", 600)
	m := &MText{Height: 2.5, RectWidth: 10, Text: long}
	var sb strings.Builder
	w := NewTagWriter(&sb)
	if err := writeEntity(w, &Entity{Type: "MTEXT", Data: m}, R2018); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(sb.String(), "\n  3\n"); got != 2 {
		t.Errorf("expected 2 chunk-3 groups for 600 runes, got %d", got)
	}
	got := roundTripEntity(t, &Entity{Type: "MTEXT", Data: m}, R2018).Data.(*MText)
	if got.Text != long {
		t.Errorf("mtext content drifted: %d runes", len([]rune(got.Text)))
	}
}

func TestRoundTripLwPolyline(t *testing.T) {
	p := &LwPolyline{Closed: true, ConstWidth: 0.5}
	p.Vertices = []LwPolylineVertex{
		{Point: Point2{0, 0}, Bulge: 0.5, HasBulge: true},
		{Point: Point2{10, 0}, HasWidth: true, StartWidth: 0.2, EndWidth: 0.4},
		{Point: Point2{10, 10}, HasID: true, ID: 7},
	}
	got := roundTripEntity(t, &Entity{Type: "LWPOLYLINE", Data: p}, R2018).Data.(*LwPolyline)
	if !got.Closed || got.ConstWidth != 0.5 || len(got.Vertices) != 3 {
		t.Fatalf("lwpolyline: %+v", got)
	}
	if got.Vertices[0].Bulge != 0.5 || got.Vertices[1].StartWidth != 0.2 ||
		got.Vertices[1].EndWidth != 0.4 || got.Vertices[2].ID != 7 {
		t.Errorf("vertex payloads: %+v", got.Vertices)
	}
}

func TestPolylineWithVertices(t *testing.T) {
	p := &Polyline{Flags: 1}
	for _, pt := range []Point3{{0, 0, 0}, {1, 1, 0}, {2, 0, 0}} {
		v := &PolyVertex{Position: pt}
		p.Vertices = append(p.Vertices, &Entity{Type: "VERTEX", Data: v})
	}
	p.Seqend = &Entity{Type: "SEQEND", Data: &SeqEnd{}}
	var sb strings.Builder
	w := NewTagWriter(&sb)
	if err := writeEntity(w, &Entity{Type: "POLYLINE", Data: p}, R2018); err != nil {
		t.Fatal(err)
	}
	// The written stream must carry POLYLINE + 3 VERTEX + SEQEND as
	// separate 0-records.
	tags := mustTags(t, sb.String())
	var kinds []string
	for _, tg := range tags {
		if tg.Code == 0 {
			kinds = append(kinds, tg.Value)
		}
	}
	want := []string{"POLYLINE", "VERTEX", "VERTEX", "VERTEX", "SEQEND"}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v", kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("kinds[%d] = %s, want %s", i, kinds[i], want[i])
		}
	}
	// And the parser must re-attach the children.
	e := parseEntityList(tags)
	if len(e) != 1 || e[0].Type != "POLYLINE" {
		t.Fatalf("parseEntityList = %+v", e)
	}
	got := e[0].Data.(*Polyline)
	if len(got.Vertices) != 3 || got.Seqend == nil {
		t.Errorf("children not attached: %d vertices, seqend %v", len(got.Vertices), got.Seqend)
	}
}

func TestInsertWithAttribs(t *testing.T) {
	i := &Insert{Name: "MYBLOCK", Position: Pt2(1, 2), Rotation: 90, HasAttribs: true}
	at := &Attrib{Tag: "N", Flags: 0}
	at.Text.Value = "42"
	i.Attribs = append(i.Attribs, &Entity{Type: "ATTRIB", Data: at})
	i.Seqend = &Entity{Type: "SEQEND", Data: &SeqEnd{}}
	var sb strings.Builder
	w := NewTagWriter(&sb)
	if err := writeEntity(w, &Entity{Type: "INSERT", Data: i}, R2018); err != nil {
		t.Fatal(err)
	}
	tags := mustTags(t, sb.String())
	var kinds []string
	for _, tg := range tags {
		if tg.Code == 0 {
			kinds = append(kinds, tg.Value)
		}
	}
	if strings.Join(kinds, ",") != "INSERT,ATTRIB,SEQEND" {
		t.Fatalf("kinds = %v", kinds)
	}
	e := parseEntityList(tags)
	if len(e) != 1 || e[0].Type != "INSERT" {
		t.Fatalf("parseEntityList = %+v", e)
	}
	got := e[0].Data.(*Insert)
	if got.Name != "MYBLOCK" || got.Rotation != 90 || len(got.Attribs) != 1 || got.Seqend == nil {
		t.Errorf("insert: %+v", got)
	}
}

func TestRoundTripSpline(t *testing.T) {
	s := &Spline{Degree: 3, Closed: true, Rational: true, Planar: true}
	s.Knots = []float64{0, 0, 0, 0, 1, 2, 3, 3, 3, 3}
	s.Controls = []Point3{{0, 0, 0}, {1, 2, 0}, {3, 2, 0}, {4, 0, 0}, {5, 1, 0}, {6, 0, 0}}
	s.Weights = []float64{1, 1, 1, 1, 1, 1}
	got := roundTripEntity(t, &Entity{Type: "SPLINE", Data: s}, R2018).Data.(*Spline)
	if got.Degree != 3 || !got.Closed || !got.Rational || len(got.Knots) != 10 || len(got.Controls) != 6 {
		t.Errorf("spline: %+v", got)
	}
	if got.Controls[2] != (Point3{3, 2, 0}) {
		t.Errorf("control[2] = %+v", got.Controls[2])
	}
}

func TestRoundTripHatchPolylinePath(t *testing.T) {
	h := &Hatch{PatternName: "SOLID", SolidFill: true, IslandStyle: 1, PatternType: 1}
	h.Loops = append(h.Loops, HatchLoop{PathFlags: HatchPathPolyline | HatchPathExternal,
		Poly: &HatchPolyPath{HasBulge: true, Closed: true, Vertices: []LwPolylineVertex{
			{Point: Point2{0, 0}, Bulge: 0.25, HasBulge: true},
			{Point: Point2{10, 0}},
			{Point: Point2{10, 10}},
			{Point: Point2{0, 10}},
		}}})
	h.SeedPoints = []Point2{{5, 5}}
	got := roundTripEntity(t, &Entity{Type: "HATCH", Data: h}, R2018).Data.(*Hatch)
	if got.PatternName != "SOLID" || !got.SolidFill || len(got.Loops) != 1 || len(got.SeedPoints) != 1 {
		t.Fatalf("hatch: %+v", got)
	}
	if got.Loops[0].Poly == nil || len(got.Loops[0].Poly.Vertices) != 4 {
		t.Fatalf("poly path: %+v", got.Loops[0])
	}
	if got.Loops[0].Poly.Vertices[0].Bulge != 0.25 {
		t.Errorf("bulge: %+v", got.Loops[0].Poly.Vertices[0])
	}
}

func TestRoundTripHatchEdgePaths(t *testing.T) {
	h := &Hatch{PatternName: "ANSI31", PatternType: 1, PatternAngle: 0, PatternScale: 1}
	line := HatchEdge{Type: HatchEdgeLine, Start: Pt2(0, 0), End: Pt2(1, 0)}
	arc := HatchEdge{Type: HatchEdgeArc, Center: Pt2(1, 1), Radius: 2, StartAngle: 0, EndAngle: 90, CCW: true}
	h.Loops = append(h.Loops, HatchLoop{PathFlags: HatchPathOutermost, Edges: []HatchEdge{line, arc}})
	h.PatternLines = append(h.PatternLines, HatchPatternLine{
		Angle: 45, BasePoint: Point2{0, 0}, Offset: Point2{0.125, 0.125}, Dashes: []float64{0.25}})
	got := roundTripEntity(t, &Entity{Type: "HATCH", Data: h}, R2018).Data.(*Hatch)
	if len(got.Loops) != 1 || len(got.Loops[0].Edges) != 2 {
		t.Fatalf("edges: %+v", got)
	}
	if got.Loops[0].Edges[1].Radius != 2 || !got.Loops[0].Edges[1].CCW {
		t.Errorf("arc edge: %+v", got.Loops[0].Edges[1])
	}
	if len(got.PatternLines) != 1 || len(got.PatternLines[0].Dashes) != 1 {
		t.Errorf("pattern lines: %+v", got.PatternLines)
	}
}

func TestRoundTripDimension(t *testing.T) {
	d := &Dimension{Subtype: DimTypeLinear, Flags: DimTypeLinear | DimFlagUserText,
		Block: "*D1", Text: "12.5", Angle: 15, ObliqueAngle: 30}
	d.DefPoint = Pt2(0, 0)
	d.DefPoint1, d.HasPoint1 = Pt2(0, 0), true
	d.DefPoint2, d.HasPoint2 = Pt2(12.5, 0), true
	got := roundTripEntity(t, &Entity{Type: "DIMENSION", Data: d}, R2018).Data.(*Dimension)
	if got.Subtype != DimTypeLinear || got.Block != "*D1" || got.Text != "12.5" ||
		got.Angle != 15 || got.ObliqueAngle != 30 {
		t.Errorf("dimension: %+v", got)
	}
	if !got.HasPoint1 || !got.HasPoint2 || got.DefPoint2.X != 12.5 {
		t.Errorf("dimension points: %+v", got)
	}
}

func TestRoundTripMisc(t *testing.T) {
	s := &Solid{}
	s.Corners = [4]Point3{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}}
	got := roundTripEntity(t, &Entity{Type: "SOLID", Data: s}, R2018).Data.(*Solid)
	if got.Corners[3] != (Point3{1, 1, 0}) {
		t.Errorf("solid: %+v", got)
	}

	f := &Face3D{EdgeFlags: 1}
	f.Corners[1] = Pt2(5, 0)
	got2 := roundTripEntity(t, &Entity{Type: "3DFACE", Data: f}, R2018).Data.(*Face3D)
	if got2.EdgeFlags != 1 || got2.Corners[1].X != 5 {
		t.Errorf("3dface: %+v", got2)
	}

	tol := &Tolerance{Value: "%%v0.1"}
	tol.Position = Pt2(1, 1)
	got3 := roundTripEntity(t, &Entity{Type: "TOLERANCE", Data: tol}, R2018).Data.(*Tolerance)
	if got3.Value != "%%v0.1" {
		t.Errorf("tolerance: %+v", got3)
	}

	el := &Ellipse{Center: Pt2(1, 1), MajorAxis: Pt2(2, 0), Ratio: 0.5, StartParam: 0, EndParam: 6.28}
	got4 := roundTripEntity(t, &Entity{Type: "ELLIPSE", Data: el}, R2018).Data.(*Ellipse)
	if got4.Ratio != 0.5 || got4.MajorAxis.X != 2 {
		t.Errorf("ellipse: %+v", got4)
	}
}

func TestUnknownEntityPreservedVerbatim(t *testing.T) {
	src := "  0\nMYSTERY\n  5\n1F\n100\nAcDbWhatever\n 90\n7\n 10\n1.5\n 20\n2.5\n"
	tags := mustTags(t, src)
	e := parseEntity("MYSTERY", tags[1:])
	if _, ok := e.Data.(*UnknownEntity); !ok {
		t.Fatalf("expected UnknownEntity, got %T", e.Data)
	}
	var sb strings.Builder
	w := NewTagWriter(&sb)
	if err := writeEntity(w, e, R2018); err != nil {
		t.Fatal(err)
	}
	if sb.String() != src {
		t.Errorf("unknown not verbatim:\n got %q\nwant %q", sb.String(), src)
	}
}

func TestExtraTagsPreserved(t *testing.T) {
	// A LINE with an extension-dictionary owner code the model doesn't know.
	src := "  5\n2A\n330\n1A\n100\nAcDbEntity\n  8\nL1\n420\n255\n100\nAcDbLine\n 10\n1\n 20\n2\n 30\n3\n" +
		" 11\n4\n 21\n5\n 31\n6\n102\n{ACAD_REACTORS\n330\n9\n102\n}\n"
	tags := mustTags(t, src)
	e := parseEntity("LINE", tags)
	got, ok := e.Data.(*Line)
	if !ok {
		t.Fatalf("type %T", e.Data)
	}
	if got.Handle != 0x2A || got.Owner != 0x1A || got.Layer != "L1" {
		t.Errorf("common: %+v", got.Common)
	}
	if got.Start.X != 1 || got.End.Z != 6 {
		t.Errorf("geometry: %+v", got)
	}
	if got.TrueColor != 255 {
		t.Errorf("true color: %+v", got.Common)
	}
	// The reactors control group must survive round trip.
	var sb strings.Builder
	w := NewTagWriter(&sb)
	if err := writeEntity(w, e, R2018); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "{ACAD_REACTORS") {
		t.Errorf("reactors group lost: %q", sb.String())
	}
}

func mustTags(t *testing.T, src string) []Tag {
	t.Helper()
	tags, err := ParseTags(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return tags
}
