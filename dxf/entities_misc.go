package dxf

// Miscellaneous drawables: TOLERANCE and VIEWPORT. More elaborate entities
// (MULTILEADER, IMAGE, MESH, 3DSOLID family, …) intentionally stay
// unmodeled — *UnknownEntity preserves them byte-for-byte.

// Tolerance is a feature-control-frame glyph.
type Tolerance struct {
	Common
	Value    string
	Position Point3
	Style    string
}

func (*Tolerance) EntityType() string      { return "TOLERANCE" }
func (e *Tolerance) EntityCommon() *Common { return &e.Common }

func readTolerance(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Tolerance{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 1:
			e.Value = t.Value
		case 3:
			e.Style = t.Value
		case 10:
			e.Position.X = t.Float()
		case 20:
			e.Position.Y = t.Float()
		case 30:
			e.Position.Z = t.Float()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeTolerance(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Tolerance)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbFcf")
	if e.Style != "" {
		w.Str(3, e.Style)
	}
	w.Point(10, e.Position)
	w.Str(1, e.Value)
	w.Pairs(e.Extra)
	return nil
}

// Viewport is a paper-space viewport record.
type Viewport struct {
	Common
	Name       string
	Center     Point3
	Width      float64
	Height     float64
	Status     int64
	ID         int64
	LayoutName string
}

func (*Viewport) EntityType() string      { return "VIEWPORT" }
func (e *Viewport) EntityCommon() *Common { return &e.Common }

func readViewport(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Viewport{Common: st.c}
	for _, t := range st.rest {
		switch t.Code {
		case 1:
			e.Name = t.Value
		case 10:
			e.Center.X = t.Float()
		case 20:
			e.Center.Y = t.Float()
		case 30:
			e.Center.Z = t.Float()
		case 40:
			e.Width = t.Float()
		case 41:
			e.Height = t.Float()
		case 68:
			e.Status = t.Int()
		case 69:
			e.ID = t.Int()
		case 410:
			e.LayoutName = t.Value
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeViewport(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Viewport)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbViewport")
	if e.Name != "" {
		w.Str(1, e.Name)
	}
	w.Point(10, e.Center)
	w.Float(40, e.Width)
	w.Float(41, e.Height)
	if e.Status != 0 {
		w.Int(68, e.Status)
	}
	if e.ID != 0 {
		w.Int(69, e.ID)
	}
	if e.LayoutName != "" {
		w.Str(410, e.LayoutName)
	}
	w.Pairs(e.Extra)
	return nil
}

func init() {
	registerEntity("TOLERANCE", &entityCodec{read: readTolerance, write: writeTolerance})
	registerEntity("VIEWPORT", &entityCodec{read: readViewport, write: writeViewport})
}
