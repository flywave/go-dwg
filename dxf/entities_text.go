package dxf

import "strings"

// Text family and block references: TEXT, MTEXT, ATTRIB, ATTDEF, INSERT,
// SEQEND.

// Text justification codes for groups 72 (horizontal) / 73 (vertical).
const (
	HJustLeft int64 = iota
	HJustCenter
	HJustRight
	HJustAligned
	HJustMiddle
	HJustFit
)

const (
	VJustBaseline int64 = iota
	VJustBottom
	VJustMiddle
	VJustTop
)

// Text is a single-line text entity.
type Text struct {
	Common
	Position    Point3
	Height      float64
	Value       string
	Rotation    float64
	WidthFactor float64
	Oblique     float64
	Style       string
	GenFlags    int64
	HJustify    int64
	VJustify    int64
	AlignPoint  Point3
	HasAlign    bool
}

func (*Text) EntityType() string      { return "TEXT" }
func (e *Text) EntityCommon() *Common { return &e.Common }

// attribExtra are the ATTRIB/ATTDEF-specific fields layered on Text.
type attribExtra struct {
	tag         *string
	prompt      *string // ATTDEF only
	flags       *int64
	fieldLength *int64
}

func readText(tags []Tag) (AnyEntityData, error) {
	e := &Text{}
	if err := readTextInto(tags, e, nil); err != nil {
		return nil, err
	}
	return e, nil
}

func readAttrib(tags []Tag) (AnyEntityData, error) {
	e := &Attrib{}
	if err := readTextInto(tags, &e.Text, &attribExtra{
		tag: &e.Tag, flags: &e.Flags, fieldLength: &e.FieldLength,
	}); err != nil {
		return nil, err
	}
	return e, nil
}

func readAttdef(tags []Tag) (AnyEntityData, error) {
	e := &Attdef{}
	if err := readTextInto(tags, &e.Text, &attribExtra{
		tag: &e.Tag, prompt: &e.Prompt, flags: &e.Flags, fieldLength: &e.FieldLength,
	}); err != nil {
		return nil, err
	}
	return e, nil
}

func readTextInto(tags []Tag, e *Text, a *attribExtra) error {
	st := splitCommon(tags)
	e.Common = st.c
	for _, t := range st.rest {
		switch t.Code {
		case 1:
			e.Value = t.Value
		case 7:
			e.Style = t.Value
		case 10:
			e.Position.X = t.Float()
		case 20:
			e.Position.Y = t.Float()
		case 30:
			e.Position.Z = t.Float()
		case 40:
			e.Height = t.Float()
		case 41:
			e.WidthFactor = t.Float()
		case 50:
			e.Rotation = t.Float()
		case 51:
			e.Oblique = t.Float()
		case 71:
			e.GenFlags = t.Int()
		case 72:
			e.HJustify = t.Int()
		case 73:
			if a == nil || a.fieldLength == nil {
				e.VJustify = t.Int()
			} else {
				// ATTRIB reuses 73 for vertical justify too; 74 is the
				// (obsolete) field length. TEXT treats 73 as vjust.
				e.VJustify = t.Int()
			}
		case 11:
			e.AlignPoint.X, e.HasAlign = t.Float(), true
		case 21:
			e.AlignPoint.Y = t.Float()
		case 31:
			e.AlignPoint.Z = t.Float()
		case 2:
			if a != nil {
				*a.tag = t.Value
			} else {
				e.Extra = append(e.Extra, t)
			}
		case 3:
			if a != nil && a.prompt != nil {
				*a.prompt = t.Value
			} else {
				e.Extra = append(e.Extra, t)
			}
		case 70:
			if a != nil {
				*a.flags = t.Int()
			} else {
				e.Extra = append(e.Extra, t)
			}
		case 74:
			if a != nil && a.fieldLength != nil {
				*a.fieldLength = t.Int()
			} else {
				e.Extra = append(e.Extra, t)
			}
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return nil
}

func writeText(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Text)
	writeTextBody(w, &e.Common, e, ver, "AcDbText")
	return nil
}

func writeAttrib(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Attrib)
	writeTextBody(w, &e.Common, &e.Text, ver, "AcDbText")
	w.Str(2, e.Tag)
	w.Int(70, e.Flags)
	if e.FieldLength != 0 {
		w.Int(74, e.FieldLength)
	}
	return nil
}

func writeAttdef(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Attdef)
	writeTextBody(w, &e.Common, &e.Text, ver, "AcDbText")
	w.Str(3, e.Prompt)
	w.Str(2, e.Tag)
	w.Int(70, e.Flags)
	if e.FieldLength != 0 {
		w.Int(74, e.FieldLength)
	}
	return nil
}

func writeTextBody(w *TagWriter, c *Common, e *Text, ver DxfVersion, subclass string) {
	sub := ver.SupportsSubclassMarkers()
	w.writeCommon(c, sub, subclass)
	if e.Style != "" {
		w.Str(7, e.Style)
	}
	w.Point(10, e.Position)
	w.Float(40, e.Height)
	w.Str(1, e.Value)
	if e.WidthFactor != 0 && e.WidthFactor != 1 {
		w.Float(41, e.WidthFactor)
	}
	if e.Oblique != 0 {
		w.Float(51, e.Oblique)
	}
	if e.Rotation != 0 {
		w.Float(50, e.Rotation)
	}
	if e.GenFlags != 0 {
		w.Int(71, e.GenFlags)
	}
	if e.HJustify != 0 || e.VJustify != 0 || e.HasAlign {
		w.Int(72, e.HJustify)
		w.Point(11, e.AlignPoint)
		w.Int(73, e.VJustify)
	}
	w.Pairs(e.Extra)
}

// Attrib is an attribute value attached to an INSERT.
type Attrib struct {
	Text
	Tag         string
	Flags       int64
	FieldLength int64
}

func (*Attrib) EntityType() string      { return "ATTRIB" }
func (e *Attrib) EntityCommon() *Common { return &e.Common }

// Attdef is an attribute definition (adds the prompt).
type Attdef struct {
	Text
	Tag         string
	Prompt      string
	Flags       int64
	FieldLength int64
}

func (*Attdef) EntityType() string      { return "ATTDEF" }
func (e *Attdef) EntityCommon() *Common { return &e.Common }

// MText is multiline text.
type MText struct {
	Common
	Position          Point3
	Height            float64
	RectWidth         float64
	Attachment        int64
	FlowDir           int64
	Text              string
	Style             string
	Extrusion         Point3
	HasExtrusion      bool
	Rotation          float64
	LineSpacingStyle  int64
	LineSpacingFactor float64
	Direction         Point3
	HasDirection      bool
}

func (*MText) EntityType() string      { return "MTEXT" }
func (e *MText) EntityCommon() *Common { return &e.Common }

func readMText(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &MText{Common: st.c}
	var chunks []string
	for _, t := range st.rest {
		switch t.Code {
		case 10:
			e.Position.X = t.Float()
		case 20:
			e.Position.Y = t.Float()
		case 30:
			e.Position.Z = t.Float()
		case 40:
			e.Height = t.Float()
		case 41:
			e.RectWidth = t.Float()
		case 71:
			e.Attachment = t.Int()
		case 72:
			e.FlowDir = t.Int()
		case 3:
			chunks = append(chunks, t.Value)
		case 1:
			chunks = append(chunks, t.Value)
		case 7:
			e.Style = t.Value
		case 210:
			e.Extrusion.X, e.HasExtrusion = t.Float(), true
		case 220:
			e.Extrusion.Y = t.Float()
		case 230:
			e.Extrusion.Z = t.Float()
		case 11:
			e.Direction.X, e.HasDirection = t.Float(), true
		case 21:
			e.Direction.Y = t.Float()
		case 31:
			e.Direction.Z = t.Float()
		case 43:
			e.Rotation = t.Float()
		case 73:
			e.LineSpacingStyle = t.Int()
		case 44:
			e.LineSpacingFactor = t.Float()
		case 50:
			// Pre-R2000 rotation; 43 takes precedence when both appear.
			if e.Rotation == 0 {
				e.Rotation = t.Float()
			}
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	e.Text = strings.Join(chunks, "")
	return e, nil
}

// maxMTextChunk is the 250-character DXF chunk limit for group 3/1 chains.
const maxMTextChunk = 250

func writeMText(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*MText)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbMText")
	w.Point(10, e.Position)
	w.Float(40, e.Height)
	w.Float(41, e.RectWidth)
	w.Int(71, e.Attachment)
	if e.FlowDir != 0 {
		w.Int(72, e.FlowDir)
	}
	if e.HasDirection {
		w.Point(11, e.Direction)
	}
	if e.Style != "" {
		w.Str(7, e.Style)
	}
	if e.HasExtrusion {
		w.Point(210, e.Extrusion)
	}
	if e.Rotation != 0 {
		w.Float(43, e.Rotation)
	}
	if e.LineSpacingStyle != 0 {
		w.Int(73, e.LineSpacingStyle)
	}
	if e.LineSpacingFactor != 0 {
		w.Float(44, e.LineSpacingFactor)
	}
	// Split the text into ≤250-rune group-3 chunks plus the final group-1
	// chunk, mirroring what AutoCAD writes.
	runes := []rune(e.Text)
	for len(runes) > maxMTextChunk {
		w.Str(3, string(runes[:maxMTextChunk]))
		runes = runes[maxMTextChunk:]
	}
	w.Str(1, string(runes))
	w.Pairs(e.Extra)
	return nil
}

// Insert is a block reference. When the source declared attributes-follow,
// the attached ATTRIB entities ride along in Attribs and the writer emits
// them before the SEQEND.
type Insert struct {
	Common
	Name       string
	Position   Point3
	Scale      Point3
	Rotation   float64
	ColCount   int64
	RowCount   int64
	ColSpacing float64
	RowSpacing float64
	HasAttribs bool
	Attribs    []*Entity
	Seqend     *Entity
}

func (*Insert) EntityType() string      { return "INSERT" }
func (e *Insert) EntityCommon() *Common { return &e.Common }

func readInsert(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Insert{Common: st.c, Scale: Point3{X: 1, Y: 1, Z: 1}}
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
		case 41:
			e.Scale.X = t.Float()
		case 42:
			e.Scale.Y = t.Float()
		case 43:
			e.Scale.Z = t.Float()
		case 50:
			e.Rotation = t.Float()
		case 70:
			e.ColCount = t.Int()
		case 71:
			e.RowCount = t.Int()
		case 44:
			e.ColSpacing = t.Float()
		case 45:
			e.RowSpacing = t.Float()
		case 66:
			e.HasAttribs = t.Int() != 0
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeInsert(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Insert)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "AcDbBlockReference")
	w.Str(2, e.Name)
	if e.HasAttribs {
		w.Int(66, 1)
	}
	w.Point(10, e.Position)
	w.Float(41, e.Scale.X)
	w.Float(42, e.Scale.Y)
	w.Float(43, e.Scale.Z)
	if e.Rotation != 0 {
		w.Float(50, e.Rotation)
	}
	if e.ColCount != 0 {
		w.Int(70, e.ColCount)
	}
	if e.RowCount != 0 {
		w.Int(71, e.RowCount)
	}
	if e.ColSpacing != 0 {
		w.Float(44, e.ColSpacing)
	}
	if e.RowSpacing != 0 {
		w.Float(45, e.RowSpacing)
	}
	w.Pairs(e.Extra)
	for _, a := range e.Attribs {
		if err := writeEntity(w, a, ver); err != nil {
			return err
		}
	}
	if e.Seqend != nil {
		return writeEntity(w, e.Seqend, ver)
	}
	if e.HasAttribs {
		w.Str(0, "SEQEND")
	}
	return nil
}

// SeqEnd terminates an ATTRIB/POLYLINE-VERTEX run; it carries only the
// common preamble.
type SeqEnd struct {
	Common
}

func (*SeqEnd) EntityType() string      { return "SEQEND" }
func (e *SeqEnd) EntityCommon() *Common { return &e.Common }

func readSeqEnd(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	return &SeqEnd{Common: st.c}, nil
}

func writeSeqEnd(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*SeqEnd)
	w.writeCommon(&e.Common, ver.SupportsSubclassMarkers(), "")
	w.Pairs(e.Extra)
	return nil
}

func init() {
	registerEntity("TEXT", &entityCodec{read: readText, write: writeText})
	registerEntity("ATTRIB", &entityCodec{read: readAttrib, write: writeAttrib})
	registerEntity("ATTDEF", &entityCodec{read: readAttdef, write: writeAttdef})
	registerEntity("MTEXT", &entityCodec{read: readMText, write: writeMText})
	registerEntity("INSERT", &entityCodec{read: readInsert, write: writeInsert})
	registerEntity("SEQEND", &entityCodec{read: readSeqEnd, write: writeSeqEnd})
}
