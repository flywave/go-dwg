package dxf

// Geometry primitives: LINE, POINT, CIRCLE, ARC, ELLIPSE, RAY, XLINE,
// SOLID, TRACE, 3DFACE, SHAPE. Group codes follow the published DXF
// reference.

// Line is a straight segment (10-30 start, 11-31 end).
type Line struct {
	Common
	Start, End Point3
}

func (*Line) EntityType() string      { return "LINE" }
func (e *Line) EntityCommon() *Common { return &e.Common }

func readLine(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Line{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 10:
			e.Start.X = t.Float()
		case 20:
			e.Start.Y = t.Float()
		case 30:
			e.Start.Z = t.Float()
		case 11:
			e.End.X = t.Float()
		case 21:
			e.End.Y = t.Float()
		case 31:
			e.End.Z = t.Float()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeLine(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Line)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbLine")
	w.Point(10, e.Start)
	w.Point(11, e.End)
	w.Pairs(e.Extra)
	return nil
}

// Point is a dimensionless marker (10-30 position, 50 X-axis angle).
type DxfPoint struct {
	Common
	Position   Point3
	XAxisAngle float64
}

func (*DxfPoint) EntityType() string      { return "POINT" }
func (e *DxfPoint) EntityCommon() *Common { return &e.Common }

func readPoint(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &DxfPoint{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 10:
			e.Position.X = t.Float()
		case 20:
			e.Position.Y = t.Float()
		case 30:
			e.Position.Z = t.Float()
		case 50:
			e.XAxisAngle = t.Float()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writePoint(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*DxfPoint)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbPoint")
	w.Point(10, e.Position)
	if e.XAxisAngle != 0 {
		w.Float(50, e.XAxisAngle)
	}
	w.Pairs(e.Extra)
	return nil
}

// Circle is a full circle (center + radius).
type Circle struct {
	Common
	Center Point3
	Radius float64
}

func (*Circle) EntityType() string      { return "CIRCLE" }
func (e *Circle) EntityCommon() *Common { return &e.Common }

func readCircle(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Circle{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 10:
			e.Center.X = t.Float()
		case 20:
			e.Center.Y = t.Float()
		case 30:
			e.Center.Z = t.Float()
		case 40:
			e.Radius = t.Float()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeCircle(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Circle)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbCircle")
	w.Point(10, e.Center)
	w.Float(40, e.Radius)
	w.Pairs(e.Extra)
	return nil
}

// Arc is a circular arc; angles are degrees CCW in OCS.
type Arc struct {
	Common
	Center     Point3
	Radius     float64
	StartAngle float64
	EndAngle   float64
}

func (*Arc) EntityType() string      { return "ARC" }
func (e *Arc) EntityCommon() *Common { return &e.Common }

func readArc(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Arc{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 10:
			e.Center.X = t.Float()
		case 20:
			e.Center.Y = t.Float()
		case 30:
			e.Center.Z = t.Float()
		case 40:
			e.Radius = t.Float()
		case 50:
			e.StartAngle = t.Float()
		case 51:
			e.EndAngle = t.Float()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeArc(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Arc)
	sub := ver.SupportsSubclassMarkers()
	w.writeCommon(&e.Common, sub, "AcDbCircle")
	w.Point(10, e.Center)
	w.Float(40, e.Radius)
	if sub {
		w.Subclass("AcDbArc")
	}
	w.Float(50, e.StartAngle)
	w.Float(51, e.EndAngle)
	w.Pairs(e.Extra)
	return nil
}

// Ellipse is the NURBS-basis ellipse; MajorAxis is the center-to-major-
// endpoint vector, Ratio the minor/major quotient, and the parameters are
// in radians.
type Ellipse struct {
	Common
	Center     Point3
	MajorAxis  Point3
	Ratio      float64
	StartParam float64
	EndParam   float64
}

func (*Ellipse) EntityType() string      { return "ELLIPSE" }
func (e *Ellipse) EntityCommon() *Common { return &e.Common }

func readEllipse(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Ellipse{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 10:
			e.Center.X = t.Float()
		case 20:
			e.Center.Y = t.Float()
		case 30:
			e.Center.Z = t.Float()
		case 11:
			e.MajorAxis.X = t.Float()
		case 21:
			e.MajorAxis.Y = t.Float()
		case 31:
			e.MajorAxis.Z = t.Float()
		case 40:
			e.Ratio = t.Float()
		case 41:
			e.StartParam = t.Float()
		case 42:
			e.EndParam = t.Float()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeEllipse(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Ellipse)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbEllipse")
	w.Point(10, e.Center)
	w.Point(11, e.MajorAxis)
	w.Float(40, e.Ratio)
	w.Float(41, e.StartParam)
	w.Float(42, e.EndParam)
	w.Pairs(e.Extra)
	return nil
}

// Ray is a half-infinite line from Point through Dir.
type Ray struct {
	Common
	Point Point3
	Dir   Point3
}

func (*Ray) EntityType() string      { return "RAY" }
func (e *Ray) EntityCommon() *Common { return &e.Common }

func readRay(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Ray{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 10:
			e.Point.X = t.Float()
		case 20:
			e.Point.Y = t.Float()
		case 30:
			e.Point.Z = t.Float()
		case 11:
			e.Dir.X = t.Float()
		case 21:
			e.Dir.Y = t.Float()
		case 31:
			e.Dir.Z = t.Float()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeRay(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Ray)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbRay")
	w.Point(10, e.Point)
	w.Point(11, e.Dir)
	w.Pairs(e.Extra)
	return nil
}

// XLine is an infinite construction line (same groups as RAY).
type XLine struct {
	Common
	Point Point3
	Dir   Point3
}

func (*XLine) EntityType() string      { return "XLINE" }
func (e *XLine) EntityCommon() *Common { return &e.Common }

func readXLine(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &XLine{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 10:
			e.Point.X = t.Float()
		case 20:
			e.Point.Y = t.Float()
		case 30:
			e.Point.Z = t.Float()
		case 11:
			e.Dir.X = t.Float()
		case 21:
			e.Dir.Y = t.Float()
		case 31:
			e.Dir.Z = t.Float()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeXLine(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*XLine)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbXline")
	w.Point(10, e.Point)
	w.Point(11, e.Dir)
	w.Pairs(e.Extra)
	return nil
}

// Solid is a filled 3- or 4-corner quadrilateral (TRACE is the wide-line
// variant with identical geometry groups).
type Solid struct {
	Common
	Corners [4]Point3
}

func (*Solid) EntityType() string      { return "SOLID" }
func (e *Solid) EntityCommon() *Common { return &e.Common }

// Trace is Solid's wide-line sibling; same wire shape.
type Trace struct {
	Common
	Corners [4]Point3
}

func (*Trace) EntityType() string      { return "TRACE" }
func (e *Trace) EntityCommon() *Common { return &e.Common }

func readSolid(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Solid{Common: st.c}
	var corners cornerSlots
	for _, t := range st.rest {
		if corners.absorb(t) {
			continue
		}
		e.Extra = append(e.Extra, t)
	}
	e.Corners = corners.p
	return e, nil
}

func readTrace(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Trace{Common: st.c}
	var corners cornerSlots
	for _, t := range st.rest {
		if corners.absorb(t) {
			continue
		}
		e.Extra = append(e.Extra, t)
	}
	e.Corners = corners.p
	return e, nil
}

func writeSolid(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Solid)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbTrace")
	for i := range e.Corners {
		w.Point(10+i, e.Corners[i])
	}
	w.Pairs(e.Extra)
	return nil
}

func writeTrace(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Trace)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbTrace")
	for i := range e.Corners {
		w.Point(10+i, e.Corners[i])
	}
	w.Pairs(e.Extra)
	return nil
}

// Face3D is a 3D face; EdgeFlags marks invisible edges (1=1st … 8=4th).
type Face3D struct {
	Common
	Corners   [4]Point3
	EdgeFlags int64
}

func (*Face3D) EntityType() string      { return "3DFACE" }
func (e *Face3D) EntityCommon() *Common { return &e.Common }

func readFace3D(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Face3D{Common: st.c}
	var corners cornerSlots
	for _, t := range st.rest {
		if corners.absorb(t) {
			continue
		}
		if t.Code == 70 {
			e.EdgeFlags = t.Int()
			continue
		}
		e.Extra = append(e.Extra, t)
	}
	e.Corners = corners.p
	return e, nil
}

func writeFace3D(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Face3D)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbFace")
	for i := range e.Corners {
		w.Point(10+i, e.Corners[i])
	}
	if e.EdgeFlags != 0 {
		w.Int(70, e.EdgeFlags)
	}
	w.Pairs(e.Extra)
	return nil
}

// Shape is an inserted .SHX glyph reference.
type Shape struct {
	Common
	Name     string
	Position Point3
	Size     float64
	Rotation float64
	RelScale float64
	Oblique  float64
}

func (*Shape) EntityType() string      { return "SHAPE" }
func (e *Shape) EntityCommon() *Common { return &e.Common }

func readShape(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Shape{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 2:
			e.Name = t.Value
		case 10:
			e.Position.X = t.Float()
		case 20:
			e.Position.Y = t.Float()
		case 30:
			e.Position.Z = t.Float()
		case 40:
			e.Size = t.Float()
		case 50:
			e.Rotation = t.Float()
		case 41:
			e.RelScale = t.Float()
		case 51:
			e.Oblique = t.Float()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeShape(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Shape)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbShape")
	w.Str(2, e.Name)
	w.Point(10, e.Position)
	w.Float(40, e.Size)
	if e.Rotation != 0 {
		w.Float(50, e.Rotation)
	}
	if e.RelScale != 0 {
		w.Float(41, e.RelScale)
	}
	if e.Oblique != 0 {
		w.Float(51, e.Oblique)
	}
	w.Pairs(e.Extra)
	return nil
}

func init() {
	registerEntity("LINE", &entityCodec{read: readLine, write: writeLine})
	registerEntity("POINT", &entityCodec{read: readPoint, write: writePoint})
	registerEntity("CIRCLE", &entityCodec{read: readCircle, write: writeCircle})
	registerEntity("ARC", &entityCodec{read: readArc, write: writeArc})
	registerEntity("ELLIPSE", &entityCodec{read: readEllipse, write: writeEllipse})
	registerEntity("RAY", &entityCodec{read: readRay, write: writeRay})
	registerEntity("XLINE", &entityCodec{read: readXLine, write: writeXLine})
	registerEntity("SOLID", &entityCodec{read: readSolid, write: writeSolid})
	registerEntity("TRACE", &entityCodec{read: readTrace, write: writeTrace})
	registerEntity("3DFACE", &entityCodec{read: readFace3D, write: writeFace3D})
	registerEntity("SHAPE", &entityCodec{read: readShape, write: writeShape})
}
