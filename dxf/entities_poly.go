package dxf

// Polyline family and curves: LWPOLYLINE, POLYLINE (+VERTEX/SEQEND
// sub-entities), SPLINE, LEADER.

// LwPolylineVertex is one LWPOLYLINE vertex: position plus optional
// per-vertex widths and bulge.
type LwPolylineVertex struct {
	Point      Point2
	StartWidth float64
	EndWidth   float64
	HasWidth   bool
	Bulge      float64
	HasBulge   bool
	ID         int64
	HasID      bool
}

// LwPolyline is the lightweight 2D polyline (R13+).
type LwPolyline struct {
	Common
	Closed     bool
	Plinegen   bool
	ConstWidth float64
	Elevation  float64
	Thickness  float64 // duplicated here; group 39 handled by Common
	Vertices   []LwPolylineVertex
}

func (*LwPolyline) EntityType() string      { return "LWPOLYLINE" }
func (e *LwPolyline) EntityCommon() *Common { return &e.Common }

func readLwPolyline(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &LwPolyline{Common: st.c}
	var cur *LwPolylineVertex
	for _, t := range st.rest {
		switch t.Code {
		case 90:
			// Vertex count is advisory; the stream is authoritative.
		case 70:
			e.Closed = t.Int()&1 != 0
			e.Plinegen = t.Int()&128 != 0
		case 43:
			e.ConstWidth = t.Float()
		case 38:
			e.Elevation = t.Float()
		case 10:
			e.Vertices = append(e.Vertices, LwPolylineVertex{})
			cur = &e.Vertices[len(e.Vertices)-1]
			cur.Point.X = t.Float()
		case 20:
			if cur != nil {
				cur.Point.Y = t.Float()
			}
		case 40:
			if cur != nil {
				cur.StartWidth = t.Float()
				cur.HasWidth = true
			}
		case 41:
			if cur != nil {
				cur.EndWidth = t.Float()
				cur.HasWidth = true
			}
		case 42:
			if cur != nil {
				cur.Bulge = t.Float()
				cur.HasBulge = true
			}
		case 91:
			if cur != nil {
				cur.ID = t.Int()
				cur.HasID = true
			}
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeLwPolyline(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*LwPolyline)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbPolyline")
	var flags int64
	if e.Closed {
		flags |= 1
	}
	if e.Plinegen {
		flags |= 128
	}
	w.Int(90, int64(len(e.Vertices)))
	w.Int(70, flags)
	if e.ConstWidth != 0 {
		w.Float(43, e.ConstWidth)
	}
	if e.Elevation != 0 {
		w.Float(38, e.Elevation)
	}
	for i := range e.Vertices {
		v := &e.Vertices[i]
		w.Point2(10, v.Point)
		if v.HasWidth {
			w.Float(40, v.StartWidth)
			w.Float(41, v.EndWidth)
		}
		if v.HasBulge {
			w.Float(42, v.Bulge)
		}
		if v.HasID {
			w.Int(91, v.ID)
		}
	}
	w.Pairs(e.Extra)
	return nil
}

// PolyVertex is one VERTEX sub-entity of a legacy POLYLINE.
type PolyVertex struct {
	Common
	Position   Point3
	StartWidth float64
	EndWidth   float64
	Bulge      float64
	Flags      int64
	TangentDir float64
	ID         int64
	// FaceRecord marks a polyface-mesh face record (flags bit 128) whose
	// "position" actually packs vertex indices.
	FaceRecord bool
}

func (*PolyVertex) EntityType() string      { return "VERTEX" }
func (e *PolyVertex) EntityCommon() *Common { return &e.Common }

func readVertex(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &PolyVertex{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 10:
			e.Position.X = t.Float()
		case 20:
			e.Position.Y = t.Float()
		case 30:
			e.Position.Z = t.Float()
		case 40:
			e.StartWidth = t.Float()
		case 41:
			e.EndWidth = t.Float()
		case 42:
			e.Bulge = t.Float()
		case 70:
			e.Flags = t.Int()
			e.FaceRecord = t.Int()&128 != 0
		case 50:
			e.TangentDir = t.Float()
		case 91:
			e.ID = t.Int()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeVertex(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*PolyVertex)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbVertex")
	if e.FaceRecord {
		w.Subclass("AcDbPolyFaceMeshVertex")
	} else {
		w.Subclass("AcDb2dVertex")
	}
	w.Point(10, e.Position)
	if e.StartWidth != 0 {
		w.Float(40, e.StartWidth)
	}
	if e.EndWidth != 0 {
		w.Float(41, e.EndWidth)
	}
	if e.Bulge != 0 {
		w.Float(42, e.Bulge)
	}
	w.Int(70, e.Flags)
	if e.TangentDir != 0 {
		w.Float(50, e.TangentDir)
	}
	if e.ID != 0 {
		w.Int(91, e.ID)
	}
	w.Pairs(e.Extra)
	return nil
}

// Polyline is the legacy 2D/3D polyline with VERTEX children. The writer
// emits the POLYLINE record, every VERTEX in order, then the SEQEND.
type Polyline struct {
	Common
	// Flags is group 70: 1=closed, 2=curve-fit, 4=spline-fit,
	// 8=3D polyline, 16=3D polygon mesh, 32=polyface mesh, 64=faces.
	Flags            int64
	Elevation        float64
	Thickness        float64
	Vertices         []*Entity // VERTEX records (typed PolyVertex payloads)
	Seqend           *Entity   // SEQEND terminator (may be nil for synthesized)
	HasAttribsFollow bool
}

func (*Polyline) EntityType() string      { return "POLYLINE" }
func (e *Polyline) EntityCommon() *Common { return &e.Common }

func readPolyline(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Polyline{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 70:
			e.Flags = t.Int()
		case 30:
			e.Elevation = t.Float()
		case 39:
			e.Thickness = t.Float()
		case 66:
			e.HasAttribsFollow = t.Int() != 0
		case 10, 20:
			// Required dummy point (0,0) — consumed and dropped.
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writePolyline(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Polyline)
	sub := ver.SupportsSubclassMarkers()
	w.writeCommon(&e.Common, sub, "AcDb2dPolyline")
	w.Int(66, 1) // entities follow (VERTEX children)
	w.Point(10, Pt3(0, 0, e.Elevation))
	w.Int(70, e.Flags)
	if e.Thickness != 0 && e.Elevation == 0 {
		w.Float(39, e.Thickness)
	}
	w.Pairs(e.Extra)
	for _, v := range e.Vertices {
		if err := writeEntity(w, v, ver); err != nil {
			return err
		}
	}
	if e.Seqend != nil {
		return writeEntity(w, e.Seqend, ver)
	}
	if e.Vertices != nil {
		w.Str(0, "SEQEND")
	}
	return nil
}

// Spline is a NURBS curve.
type Spline struct {
	Common
	Degree       int64
	Closed       bool
	Periodic     bool
	Rational     bool
	Planar       bool
	Linear       bool
	KnotTol      float64
	CtrlTol      float64
	FitTol       float64
	StartTangent Point3
	EndTangent   Point3
	HasTangents  bool
	Knots        []float64
	Weights      []float64
	Controls     []Point3
	FitPoints    []Point3
}

func (*Spline) EntityType() string      { return "SPLINE" }
func (e *Spline) EntityCommon() *Common { return &e.Common }

func readSpline(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Spline{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 70:
			f := t.Int()
			e.Closed, e.Periodic, e.Rational, e.Planar, e.Linear =
				f&1 != 0, f&2 != 0, f&4 != 0, f&8 != 0, f&16 != 0
		case 71:
			e.Degree = t.Int()
		case 42:
			e.KnotTol = t.Float()
		case 43:
			e.CtrlTol = t.Float()
		case 44:
			e.FitTol = t.Float()
		case 12:
			e.StartTangent.X, e.HasTangents = t.Float(), true
		case 22:
			e.StartTangent.Y = t.Float()
		case 32:
			e.StartTangent.Z = t.Float()
		case 13:
			e.EndTangent.X, e.HasTangents = t.Float(), true
		case 23:
			e.EndTangent.Y = t.Float()
		case 33:
			e.EndTangent.Z = t.Float()
		case 40:
			e.Knots = append(e.Knots, t.Float())
		case 41:
			e.Weights = append(e.Weights, t.Float())
		case 10:
			e.Controls = append(e.Controls, Point3{X: t.Float()})
		case 20, 30:
			if n := len(e.Controls); n > 0 {
				if t.Code == 20 {
					e.Controls[n-1].Y = t.Float()
				} else {
					e.Controls[n-1].Z = t.Float()
				}
			}
		case 11:
			e.FitPoints = append(e.FitPoints, Point3{X: t.Float()})
		case 21, 31:
			if n := len(e.FitPoints); n > 0 {
				if t.Code == 21 {
					e.FitPoints[n-1].Y = t.Float()
				} else {
					e.FitPoints[n-1].Z = t.Float()
				}
			}
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeSpline(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Spline)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbSpline")
	var flags int64
	if e.Closed {
		flags |= 1
	}
	if e.Periodic {
		flags |= 2
	}
	if e.Rational {
		flags |= 4
	}
	if e.Planar {
		flags |= 8
	}
	if e.Linear {
		flags |= 16
	}
	w.Int(70, flags)
	w.Int(71, e.Degree)
	if e.KnotTol != 0 {
		w.Float(42, e.KnotTol)
	}
	if e.CtrlTol != 0 {
		w.Float(43, e.CtrlTol)
	}
	if e.FitTol != 0 {
		w.Float(44, e.FitTol)
	}
	if e.HasTangents {
		w.Point(12, e.StartTangent)
		w.Point(13, e.EndTangent)
	}
	w.Int(72, int64(len(e.Knots)))
	w.Int(73, int64(len(e.Controls)))
	w.Int(74, int64(len(e.FitPoints)))
	for _, k := range e.Knots {
		w.Float(40, k)
	}
	for _, wt := range e.Weights {
		w.Float(41, wt)
	}
	for _, p := range e.Controls {
		w.Point(10, p)
	}
	for _, p := range e.FitPoints {
		w.Point(11, p)
	}
	w.Pairs(e.Extra)
	return nil
}

// Leader is the legacy annotation leader line.
type Leader struct {
	Common
	StyleHandle    uint64 // 3 — associated annotation (dimstyle/annotation ref)
	ArrowheadFirst bool   // 71 bit 1 = arrowhead at first vertex
	PathType       int64  // 72: 0 straight, 1 spline
	CreationType   int64  // 73
	HooklineDir    int64  // 74
	Hookline       bool   // 75
	Vertices       []Point3
	TextHeight     float64
	TextWidth      float64
	Annotation     string
}

func (*Leader) EntityType() string      { return "LEADER" }
func (e *Leader) EntityCommon() *Common { return &e.Common }

func readLeader(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Leader{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 3:
			e.StyleHandle = parseHandle(t.Value)
		case 71:
			e.ArrowheadFirst = t.Int()&1 != 0
		case 72:
			e.PathType = t.Int()
		case 73:
			e.CreationType = t.Int()
		case 74:
			e.HooklineDir = t.Int()
		case 75:
			e.Hookline = t.Int() != 0
		case 10:
			e.Vertices = append(e.Vertices, Point3{X: t.Float()})
		case 20, 30:
			if n := len(e.Vertices); n > 0 {
				if t.Code == 20 {
					e.Vertices[n-1].Y = t.Float()
				} else {
					e.Vertices[n-1].Z = t.Float()
				}
			}
		case 40:
			e.TextHeight = t.Float()
		case 41:
			e.TextWidth = t.Float()
		case 1:
			e.Annotation = t.Value
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeLeader(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Leader)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbLeader")
	if e.StyleHandle != 0 {
		w.Handle(3, e.StyleHandle)
	}
	w.Int(71, boolInt(e.ArrowheadFirst))
	w.Int(72, e.PathType)
	w.Int(73, e.CreationType)
	w.Int(74, e.HooklineDir)
	w.Int(75, boolInt(e.Hookline))
	w.Int(76, int64(len(e.Vertices)))
	for _, p := range e.Vertices {
		w.Point(10, p)
	}
	if e.TextHeight != 0 {
		w.Float(40, e.TextHeight)
	}
	if e.TextWidth != 0 {
		w.Float(41, e.TextWidth)
	}
	if e.Annotation != "" {
		w.Str(1, e.Annotation)
	}
	w.Pairs(e.Extra)
	return nil
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func init() {
	registerEntity("LWPOLYLINE", &entityCodec{read: readLwPolyline, write: writeLwPolyline})
	registerEntity("POLYLINE", &entityCodec{read: readPolyline, write: writePolyline})
	registerEntity("VERTEX", &entityCodec{read: readVertex, write: writeVertex})
	registerEntity("SPLINE", &entityCodec{read: readSpline, write: writeSpline})
	registerEntity("LEADER", &entityCodec{read: readLeader, write: writeLeader})
}
