package dxf

// HATCH — boundary paths, pattern definition and seed points. The grammar
// (groups 2/70/71/91-99 plus the per-edge 72-typed sub-blocks) is strictly
// ordered, so the reader is a sequential scanner; any deviation degrades
// the whole entity to *UnknownEntity via the codec error path.

// Boundary path type flags for group 92.
const (
	HatchPathExternal  int64 = 1
	HatchPathPolyline  int64 = 2
	HatchPathDerived   int64 = 4
	HatchPathText      int64 = 8
	HatchPathOutermost int64 = 16
)

// HatchEdgeType codes for group 72 within a boundary path.
const (
	HatchEdgeLine    int64 = 1
	HatchEdgeArc     int64 = 2
	HatchEdgeEllipse int64 = 3
	HatchEdgeSpline  int64 = 4
)

// HatchPolyPath is a polyline boundary path (flag bit 2 of group 92).
type HatchPolyPath struct {
	HasBulge bool
	Closed   bool
	Vertices []LwPolylineVertex
}

// HatchEdge is one boundary edge (line / arc / ellipse arc / spline).
type HatchEdge struct {
	Type int64
	// Line
	Start, End Point3
	// Arc / Ellipse
	Center     Point3
	Radius     float64 // arc
	MajorAxis  Point3  // ellipse
	MinorRatio float64 // ellipse
	StartAngle float64
	EndAngle   float64
	CCW        bool
	// Spline
	Degree     int64
	Rational   bool
	Periodic   bool
	Knots      []float64
	Controls   []Point3
	Tangents   [2]Point3
	HasTangent [2]bool
}

// HatchLoop is one boundary loop: either a polyline path or an edge list.
type HatchLoop struct {
	PathFlags int64
	Poly      *HatchPolyPath
	Edges     []HatchEdge
}

// HatchPatternLine is one row of the pattern-space hatch definition.
type HatchPatternLine struct {
	Angle     float64 // 53
	BasePoint Point2  // 43/44
	Offset    Point2  // 45/46
	Dashes    []float64
}

// Hatch models the HATCH entity.
type Hatch struct {
	Common
	PatternName    string
	SolidFill      bool
	Associative    bool
	Loops          []HatchLoop
	IslandStyle    int64
	PatternType    int64
	PatternAngle   float64
	PatternScale   float64
	PatternSpacing float64
	PatternDouble  bool
	PatternLines   []HatchPatternLine
	SeedPoints     []Point2
}

func (*Hatch) EntityType() string      { return "HATCH" }
func (e *Hatch) EntityCommon() *Common { return &e.Common }

// tagStream is a sequential tag cursor with typed expectations.
type tagStream struct {
	tags []Tag
	i    int
}

func (s *tagStream) next() (Tag, bool) {
	if s.i >= len(s.tags) {
		return Tag{}, false
	}
	t := s.tags[s.i]
	s.i++
	return t, true
}

func (s *tagStream) peek() (Tag, bool) {
	if s.i >= len(s.tags) {
		return Tag{}, false
	}
	return s.tags[s.i], true
}

func (s *tagStream) expect(code int) (Tag, error) {
	t, ok := s.next()
	if !ok || t.Code != code {
		return Tag{}, &parseError{"dxf: expected group code " + itoa(int64(code)) + " in HATCH stream"}
	}
	return t, nil
}

func (s *tagStream) point(xCode int) (Point3, error) {
	tx, err := s.expect(xCode)
	if err != nil {
		return Point3{}, err
	}
	ty, err := s.expect(xCode + 10)
	if err != nil {
		return Point3{}, err
	}
	tz, err := s.expect(xCode + 20)
	if err != nil {
		return Point3{}, err
	}
	return Point3{X: tx.Float(), Y: ty.Float(), Z: tz.Float()}, nil
}

func (s *tagStream) done() bool { return s.i >= len(s.tags) }

func readHatch(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Hatch{Common: st.c}
	s := &tagStream{tags: st.rest}
	// Header until the loop count.
	for {
		t, ok := s.next()
		if !ok {
			return nil, errHatchTruncated
		}
		switch t.Code {
		case 2:
			e.PatternName = t.Value
		case 70:
			e.SolidFill = t.Int()&1 != 0
		case 71:
			e.Associative = t.Int() != 0
		case 91:
			n := t.Int()
			for j := int64(0); j < n; j++ {
				loop, err := readHatchLoop(s)
				if err != nil {
					return nil, err
				}
				e.Loops = append(e.Loops, loop)
			}
			goto style
		default:
			e.Extra = append(e.Extra, t)
		}
	}
style:
	for !s.done() {
		t, _ := s.next()
		switch t.Code {
		case 75:
			e.IslandStyle = t.Int()
		case 76:
			e.PatternType = t.Int()
		case 52:
			e.PatternAngle = t.Float()
		case 41:
			e.PatternScale = t.Float()
		case 77:
			e.PatternSpacing = t.Float()
		case 78:
			e.PatternDouble = t.Int() != 0
		case 79:
			n := t.Int()
			for j := int64(0); j < n; j++ {
				line, err := readHatchPatternLine(s)
				if err != nil {
					return nil, err
				}
				e.PatternLines = append(e.PatternLines, line)
			}
		case 98:
			n := t.Int()
			for j := int64(0); j < n; j++ {
				tx, err := s.expect(10)
				if err != nil {
					return nil, err
				}
				ty, err := s.expect(20)
				if err != nil {
					return nil, err
				}
				e.SeedPoints = append(e.SeedPoints, Point2{X: tx.Float(), Y: ty.Float()})
			}
		default:
			// Gradient data (450-463, 470-471), boundary association
			// handles (330), pattern-fill metadata — preserved verbatim.
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func readHatchLoop(s *tagStream) (HatchLoop, error) {
	t92, err := s.expect(92)
	if err != nil {
		return HatchLoop{}, err
	}
	loop := HatchLoop{PathFlags: t92.Int()}
	if loop.PathFlags&HatchPathPolyline != 0 {
		var poly HatchPolyPath
		closed, err := s.expect(72)
		if err != nil {
			return HatchLoop{}, err
		}
		poly.HasBulge = closed.Int() != 0
		isClosed, err := s.expect(73)
		if err != nil {
			return HatchLoop{}, err
		}
		poly.Closed = isClosed.Int() != 0
		nTag, err := s.expect(93)
		if err != nil {
			return HatchLoop{}, err
		}
		for j := int64(0); j < nTag.Int(); j++ {
			var v LwPolylineVertex
			tx, err := s.expect(10)
			if err != nil {
				return HatchLoop{}, err
			}
			ty, err := s.expect(20)
			if err != nil {
				return HatchLoop{}, err
			}
			v.Point = Point2{X: tx.Float(), Y: ty.Float()}
			if poly.HasBulge {
				tb, err := s.expect(42)
				if err != nil {
					return HatchLoop{}, err
				}
				v.Bulge, v.HasBulge = tb.Float(), true
			}
			poly.Vertices = append(poly.Vertices, v)
		}
		loop.Poly = &poly
		return loop, nil
	}
	nTag, err := s.expect(93)
	if err != nil {
		return HatchLoop{}, err
	}
	for j := int64(0); j < nTag.Int(); j++ {
		edge, err := readHatchEdge(s)
		if err != nil {
			return HatchLoop{}, err
		}
		loop.Edges = append(loop.Edges, edge)
	}
	return loop, nil
}

func readHatchEdge(s *tagStream) (HatchEdge, error) {
	t72, err := s.expect(72)
	if err != nil {
		return HatchEdge{}, err
	}
	var e HatchEdge
	e.Type = t72.Int()
	switch e.Type {
	case HatchEdgeLine:
		if e.Start, err = s.point(10); err != nil {
			return HatchEdge{}, err
		}
		if e.End, err = s.point(11); err != nil {
			return HatchEdge{}, err
		}
	case HatchEdgeArc:
		if e.Center, err = s.point(10); err != nil {
			return HatchEdge{}, err
		}
		if e.Radius, err = s.floatAt(40); err != nil {
			return HatchEdge{}, err
		}
		if e.StartAngle, err = s.floatAt(50); err != nil {
			return HatchEdge{}, err
		}
		if e.EndAngle, err = s.floatAt(51); err != nil {
			return HatchEdge{}, err
		}
		ccw, err := s.expect(73)
		if err != nil {
			return HatchEdge{}, err
		}
		e.CCW = ccw.Int() != 0
	case HatchEdgeEllipse:
		if e.Center, err = s.point(10); err != nil {
			return HatchEdge{}, err
		}
		if e.MajorAxis, err = s.point(11); err != nil {
			return HatchEdge{}, err
		}
		if e.MinorRatio, err = s.floatAt(40); err != nil {
			return HatchEdge{}, err
		}
		if e.StartAngle, err = s.floatAt(50); err != nil {
			return HatchEdge{}, err
		}
		if e.EndAngle, err = s.floatAt(51); err != nil {
			return HatchEdge{}, err
		}
		ccw, err := s.expect(73)
		if err != nil {
			return HatchEdge{}, err
		}
		e.CCW = ccw.Int() != 0
	case HatchEdgeSpline:
		deg, err := s.expect(40)
		if err != nil {
			return HatchEdge{}, err
		}
		e.Degree = deg.Int()
		rat, err := s.expect(41)
		if err != nil {
			return HatchEdge{}, err
		}
		e.Rational = rat.Int() != 0
		per, err := s.expect(42)
		if err != nil {
			return HatchEdge{}, err
		}
		e.Periodic = per.Int() != 0
		nk, err := s.expect(73)
		if err != nil {
			return HatchEdge{}, err
		}
		nc, err := s.expect(74)
		if err != nil {
			return HatchEdge{}, err
		}
		for j := int64(0); j < nk.Int(); j++ {
			t, err := s.expect(40)
			if err != nil {
				return HatchEdge{}, err
			}
			e.Knots = append(e.Knots, t.Float())
		}
		for j := int64(0); j < nc.Int(); j++ {
			p, err := s.point(10)
			if err != nil {
				return HatchEdge{}, err
			}
			e.Controls = append(e.Controls, p)
		}
		if e.Tangents[0], e.HasTangent[0], err = s.tangentMaybe(12); err != nil {
			return HatchEdge{}, err
		}
		if e.Tangents[1], e.HasTangent[1], err = s.tangentMaybe(13); err != nil {
			return HatchEdge{}, err
		}
	default:
		return HatchEdge{}, errHatchGrammar
	}
	return e, nil
}

func (s *tagStream) tangentMaybe(xCode int) (Point3, bool, error) {
	if t, ok := s.peek(); !ok || t.Code != xCode {
		return Point3{}, false, nil
	}
	p, err := s.point(xCode)
	return p, true, err
}

func (s *tagStream) floatAt(code int) (float64, error) {
	t, err := s.expect(code)
	if err != nil {
		return 0, err
	}
	return t.Float(), nil
}

func readHatchPatternLine(s *tagStream) (HatchPatternLine, error) {
	var line HatchPatternLine
	t53, err := s.expect(53)
	if err != nil {
		return line, err
	}
	line.Angle = t53.Float()
	t43, err := s.expect(43)
	if err != nil {
		return line, err
	}
	t44, err := s.expect(44)
	if err != nil {
		return line, err
	}
	line.BasePoint = Point2{X: t43.Float(), Y: t44.Float()}
	t45, err := s.expect(45)
	if err != nil {
		return line, err
	}
	t46, err := s.expect(46)
	if err != nil {
		return line, err
	}
	line.Offset = Point2{X: t45.Float(), Y: t46.Float()}
	t79, err := s.expect(79)
	if err != nil {
		return line, err
	}
	for d := int64(0); d < t79.Int(); d++ {
		t49, err := s.expect(49)
		if err != nil {
			return line, err
		}
		line.Dashes = append(line.Dashes, t49.Float())
	}
	return line, nil
}

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

var (
	errHatchTruncated = &parseError{"dxf: HATCH stream ended inside boundary data"}
	errHatchGrammar   = &parseError{"dxf: HATCH boundary grammar violation"}
)

func writeHatch(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Hatch)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbHatch")
	w.Str(2, e.PatternName)
	var solid int64
	if e.SolidFill {
		solid = 1
	}
	w.Int(70, solid)
	w.Bool(71, e.Associative)
	w.Int(91, int64(len(e.Loops)))
	for _, loop := range e.Loops {
		w.Int(92, loop.PathFlags)
		if loop.Poly != nil {
			w.Bool(72, loop.Poly.HasBulge)
			w.Bool(73, loop.Poly.Closed)
			w.Int(93, int64(len(loop.Poly.Vertices)))
			for _, v := range loop.Poly.Vertices {
				w.Point2(10, v.Point)
				if loop.Poly.HasBulge {
					w.Float(42, v.Bulge)
				}
			}
		} else {
			w.Int(93, int64(len(loop.Edges)))
			for i := range loop.Edges {
				writeHatchEdge(w, &loop.Edges[i])
			}
		}
	}
	w.Int(75, e.IslandStyle)
	w.Int(76, e.PatternType)
	w.Float(52, e.PatternAngle)
	w.Float(41, e.PatternScale)
	if e.PatternSpacing != 0 {
		w.Float(77, e.PatternSpacing)
	}
	if e.PatternDouble {
		w.Int(78, 1)
	}
	w.Int(79, int64(len(e.PatternLines)))
	for _, line := range e.PatternLines {
		w.Float(53, line.Angle)
		w.Float(43, line.BasePoint.X)
		w.Float(44, line.BasePoint.Y)
		w.Float(45, line.Offset.X)
		w.Float(46, line.Offset.Y)
		w.Int(79, int64(len(line.Dashes)))
		for _, d := range line.Dashes {
			w.Float(49, d)
		}
	}
	w.Int(98, int64(len(e.SeedPoints)))
	for _, p := range e.SeedPoints {
		w.Point2(10, p)
	}
	w.Pairs(e.Extra)
	return nil
}

func writeHatchEdge(w *TagWriter, e *HatchEdge) {
	w.Int(72, e.Type)
	switch e.Type {
	case HatchEdgeLine:
		w.Point(10, e.Start)
		w.Point(11, e.End)
	case HatchEdgeArc:
		w.Point(10, e.Center)
		w.Float(40, e.Radius)
		w.Float(50, e.StartAngle)
		w.Float(51, e.EndAngle)
		w.Bool(73, e.CCW)
	case HatchEdgeEllipse:
		w.Point(10, e.Center)
		w.Point(11, e.MajorAxis)
		w.Float(40, e.MinorRatio)
		w.Float(50, e.StartAngle)
		w.Float(51, e.EndAngle)
		w.Bool(73, e.CCW)
	case HatchEdgeSpline:
		w.Float(40, float64(e.Degree))
		w.Bool(41, e.Rational)
		w.Bool(42, e.Periodic)
		w.Int(73, int64(len(e.Knots)))
		w.Int(74, int64(len(e.Controls)))
		for _, k := range e.Knots {
			w.Float(40, k)
		}
		for _, p := range e.Controls {
			w.Point(10, p)
		}
		if e.HasTangent[0] {
			w.Point(12, e.Tangents[0])
		}
		if e.HasTangent[1] {
			w.Point(13, e.Tangents[1])
		}
	}
}

func init() {
	registerEntity("HATCH", &entityCodec{read: readHatch, write: writeHatch})
}
