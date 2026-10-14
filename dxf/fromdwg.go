package dxf

import (
	"fmt"
	"sort"

	"github.com/flywave/go-dwg/dwg"
)

// FromDwg converts an opened DWG drawing to a DXF Document targeting the
// given dialect.
//
// Coverage: every drawable entity the DWG decoder models maps to its typed
// DXF counterpart (LINE/CIRCLE/ARC/POINT/ELLIPSE/RAY/XLINE/SOLID/TRACE/
// TEXT/MTEXT/ATTRIB/ATTDEF/INSERT/LWPOLYLINE/POLYLINE family/SPLINE/HATCH/
// DIMENSION×7/3DFACE/LEADER/TOLERANCE/VIEWPORT/SHAPE), symbol tables land
// in TABLES, block definitions in BLOCKS, non-entity objects in OBJECTS.
// Records without a mapping surface as `999 skipped: KIND xN` comments —
// the omission stays visible instead of silently vanishing.
//
// Known gaps, inherent to the current DWG port: entity-level layer/color/
// linetype are handle references the decoder does not yet resolve, so all
// converted entities carry the defaults (layer "0", color ByLayer); INSERT
// names resolve only when the block was decoded; MLINE-style rich objects
// (IMAGE, MESH, MULTILEADER, 3DSOLID family, …) are skipped.
func FromDwg(f *dwg.DwgFile, version DxfVersion) (*Document, error) {
	// Seed the template handle sequence above the source drawing's handle
	// space so imported records and template records never collide.
	seed := uint64(0x10)
	if hm, ok, err := f.HandleMap(); ok && err == nil {
		for _, e := range hm.Entries {
			if e.Handle >= seed {
				seed = e.Handle + 1
			}
		}
	}
	doc := newDocumentSeeded(version, seed)

	c := &dwgConverter{doc: doc, skipped: map[string]int{}}
	if err := c.walk(f); err != nil {
		return nil, err
	}
	c.finish()
	assignEntityHandles(doc)

	// $HANDSEED sits above everything emitted.
	doc.Header.Set("$HANDSEED", padHandle(doc.nextHandle+1))

	// Emit skip diagnostics as group-999 comments so omissions stay
	// visible in the file itself.
	if len(c.skipped) > 0 {
		keys := make([]string, 0, len(c.skipped))
		for k := range c.skipped {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			doc.Comments = append(doc.Comments, fmt.Sprintf("skipped: %s x%d", k, c.skipped[k]))
		}
	}
	return doc, nil
}

// ConvertDwgFile opens path and converts it to a DXF Document.
func ConvertDwgFile(path string, version DxfVersion) (*Document, error) {
	f, err := dwg.OpenDwg(path)
	if err != nil {
		return nil, err
	}
	return FromDwg(f, version)
}

type dwgConverter struct {
	doc      *Document
	skipped  map[string]int
	entities []*Entity // model-space entities
	blocks   []*Block
	curBlock *Block
	pending  *Entity // INSERT/POLYLINE collecting children
	tableBuf map[string][]*TableEntry
	rootDict uint64 // first decoded DICTIONARY — stands in for unresolvable owners
}

// walk decodes every object and routes it to its DXF section. The
// handle-driven walk is preferred; drawings whose handle stream is
// degenerate (some synthetic/legacy containers) fall back to the
// sequential walker.
func (c *dwgConverter) walk(f *dwg.DwgFile) error {
	objs, ok, err := f.AllObjects()
	if !ok || err != nil {
		objs, ok, err = f.Objects()
		if !ok || err != nil {
			return fmt.Errorf("dxf: no decodable object stream for %s", f.Version())
		}
	}
	var classMap *dwg.ClassMap
	if cm, ok, err := f.ClassMap(); ok && err == nil {
		classMap = &cm
	}
	for i := range objs {
		raw := &objs[i]
		var d dwg.DecodedEntity
		if classMap != nil && raw.Kind.IsCustom() {
			d = dwg.DecodeFromRawWithClassMap(raw, f.Version(), classMap)
		} else {
			d = dwg.DecodeFromRaw(raw, f.Version())
		}
		c.absorb(d, raw)
	}
	return nil
}

// finish installs collected entities/blocks/tables into the document.
func (c *dwgConverter) finish() {
	doc := c.doc
	doc.Entities = append(doc.Entities, c.entities...)

	// Merge decoded table entries into the template tables, skipping
	// duplicates of the template's own seed entries by name.
	for name, entries := range c.tableBuf {
		t := doc.Table(name)
		if t == nil {
			t = &Table{Name: name, Handle: doc.AllocHandle()}
			doc.Tables = append(doc.Tables, t)
		}
		seen := map[string]bool{}
		for _, e := range t.Entries {
			seen[e.Name()] = true
		}
		for _, e := range entries {
			if seen[e.Name()] {
				continue
			}
			seen[e.Name()] = true
			t.Entries = append(t.Entries, e)
		}
		t.Flags = int64(len(t.Entries))
	}
	// Append decoded blocks that aren't the template placeholders.
	for _, b := range c.blocks {
		if doc.Block(b.Name()) == nil {
			doc.Blocks = append(doc.Blocks, b)
		}
	}
}

func (c *dwgConverter) skip(label string) { c.skipped[label]++ }

// absorb routes one decoded DWG record.
func (c *dwgConverter) absorb(d dwg.DecodedEntity, raw *dwg.RawObject) {
	label := d.Label
	switch label {
	case "ERROR", "UNHANDLED":
		c.skip(label + ":" + fixedLabel(d))
		return
	case "BLOCK":
		// Block placeholder entity — open a BLOCKS record if the header
		// didn't already create one.
		if b, ok := d.Value.(*dwg.Block); ok && b != nil {
			c.openBlock(b.Name)
		}
		return
	case "ENDBLK":
		c.curBlock = nil
		return
	case "SEQEND":
		c.pending = nil
		return
	}

	// Table entries land in TABLES.
	if isTableLabel(label) {
		c.absorbTable(label, d, raw)
		return
	}
	// Non-entity objects land in OBJECTS.
	if !isEntityLabel(label) {
		c.absorbObject(label, d, raw)
		return
	}

	// Drawable entity.
	e, ok := c.convertEntity(d)
	if !ok {
		return
	}
	// Route into the open block, a pending child run, or model space.
	if c.curBlock != nil {
		c.curBlock.Entities = append(c.curBlock.Entities, e)
		return
	}
	if c.pending != nil {
		switch label {
		case "SEQEND":
		case "ATTRIB", "ATTDEF", "VERTEX", "VERTEX_2D", "VERTEX_3D",
			"VERTEX_MESH", "VERTEX_PFACE", "VERTEX_PFACE_FACE":
			attachChild(c.pending, e)
			return
		}
		c.pending = nil
	}
	c.entities = append(c.entities, e)
	if label == "INSERT" || label == "POLYLINE" ||
		label == "POLYLINE_2D" || label == "POLYLINE_3D" ||
		label == "POLYFACE_MESH" || label == "POLYGON_MESH" {
		if _, isChild := e.Data.(childOwner); isChild {
			c.pending = e
		}
	}
}

func fixedLabel(d dwg.DecodedEntity) string {
	if d.Label == "UNHANDLED" || d.Label == "ERROR" {
		return fmt.Sprintf("TYPE%d", d.TypeCode)
	}
	return d.Label
}

// isTableLabel reports whether a decoded label is a symbol-table entry.
func isTableLabel(label string) bool {
	switch label {
	case "LAYER", "APPID", "LTYPE", "STYLE", "VPORT", "VIEW", "UCS",
		"DIMSTYLE", "BLOCK_HEADER":
		return true
	}
	return false
}

// isEntityLabel reports whether a decoded label is a drawable entity.
func isEntityLabel(label string) bool {
	switch label {
	case "LINE", "POINT", "CIRCLE", "ARC", "ELLIPSE", "RAY", "XLINE",
		"SOLID", "TRACE", "TEXT", "MTEXT", "ATTRIB", "ATTDEF", "INSERT",
		"LWPOLYLINE", "POLYLINE_2D", "POLYLINE_3D", "VERTEX_2D",
		"VERTEX_3D", "VERTEX_MESH", "VERTEX_PFACE", "VERTEX_PFACE_FACE",
		"POLYFACE_MESH", "POLYGON_MESH", "SPLINE", "HATCH", "3DFACE",
		"LEADER", "TOLERANCE", "VIEWPORT", "SHAPE", "MLINE",
		"DIMENSION_ORDINATE", "DIMENSION_LINEAR", "DIMENSION_ALIGNED",
		"DIMENSION_ANG_3PT", "DIMENSION_ANG_2LN", "DIMENSION_RADIUS",
		"DIMENSION_DIAMETER", "HELIX", "IMAGE", "WIPEOUT", "MESH",
		"MULTILEADER", "OLE2FRAME", "3DSOLID", "REGION", "BODY",
		"PLANE_SURFACE", "TORUS_SURFACE", "REVOLVED_SURFACE",
		"LOFTED_SURFACE", "SWEPT_SURFACE", "CAMERA", "GEODATA",
		"SECTIONOBJECT", "ACAD_PROXY_ENTITY":
		return true
	}
	return false
}

// openBlock creates (or reuses) a BLOCKS record by name.
func (c *dwgConverter) openBlock(name string) {
	if name == "" {
		return
	}
	for _, b := range c.blocks {
		if b.Name() == name {
			c.curBlock = b
			return
		}
	}
	b := NewBlock(name, c.doc.Version)
	b.SetHandleOwner(c.doc.AllocHandle(), 0)
	c.blocks = append(c.blocks, b)
	c.curBlock = b
}

// absorbTable maps a decoded symbol-table entry into TABLES.
func (c *dwgConverter) absorbTable(label string, d dwg.DecodedEntity, raw *dwg.RawObject) {
	if c.tableBuf == nil {
		c.tableBuf = map[string][]*TableEntry{}
	}
	mk := func(dxfType, name string) *TableEntry {
		e := &TableEntry{Type: dxfType, Handle: raw.Handle.Value}
		e.Rest = []Tag{
			{Code: 100, Value: "AcDbSymbolTableRecord"},
			{Code: 100, Value: subclassForTable(dxfType)},
			{Code: 2, Value: name},
			{Code: 70, Value: "0"},
		}
		return e
	}
	switch label {
	case "LAYER":
		l, ok := d.Value.(*dwg.LayerEntry)
		if !ok || l == nil {
			c.skip("LAYER")
			return
		}
		e := mk("LAYER", l.Header.Name)
		flags := int64(0)
		if l.IsFrozen() {
			flags |= 1
		}
		if l.IsFrozenInNewViewports() {
			flags |= 2
		}
		if l.IsLocked() {
			flags |= 4
		}
		if l.Header.IsXrefDependent {
			flags |= 16
		}
		e.SetGroup(70, itoa(flags))
		e.SetGroup(62, itoa(int64(l.ColorIndex)))
		if l.Lineweight != 0 {
			e.Rest = append(e.Rest, Tag{Code: 370, Value: itoa(int64(l.Lineweight))})
		}
		c.tableBuf["LAYER"] = append(c.tableBuf["LAYER"], e)
	case "APPID":
		a, ok := d.Value.(*dwg.AppID)
		if !ok || a == nil {
			c.skip("APPID")
			return
		}
		c.tableBuf["APPID"] = append(c.tableBuf["APPID"], mk("APPID", a.Header.Name))
	case "LTYPE":
		l, ok := d.Value.(*dwg.LtypeEntry)
		if !ok || l == nil {
			c.skip("LTYPE")
			return
		}
		e := mk("LTYPE", l.Header.Name)
		e.SetGroup(3, l.Description)
		e.SetGroup(40, FormatFloat(l.PatternLength))
		e.SetGroup(72, "65")
		e.SetGroup(73, itoa(int64(len(l.Dashes))))
		for _, dash := range l.Dashes {
			e.Rest = append(e.Rest, Tag{Code: 49, Value: FormatFloat(dash.Length)})
		}
		c.tableBuf["LTYPE"] = append(c.tableBuf["LTYPE"], e)
	case "STYLE":
		s, ok := d.Value.(*dwg.StyleEntry)
		if !ok || s == nil {
			c.skip("STYLE")
			return
		}
		e := mk("STYLE", s.Header.Name)
		e.SetGroup(40, FormatFloat(s.FixedHeight))
		e.SetGroup(41, FormatFloat(s.WidthFactor))
		e.SetGroup(50, FormatFloat(s.ObliqueAngle))
		e.SetGroup(71, itoa(int64(s.Generation)))
		e.SetGroup(42, FormatFloat(s.LastHeight))
		e.SetGroup(3, s.FontFilename)
		c.tableBuf["STYLE"] = append(c.tableBuf["STYLE"], e)
	case "VPORT":
		v, ok := d.Value.(*dwg.VportEntry)
		if !ok || v == nil {
			c.skip("VPORT")
			return
		}
		c.tableBuf["VPORT"] = append(c.tableBuf["VPORT"], mk("VPORT", v.Header.Name))
	case "VIEW":
		v, ok := d.Value.(*dwg.ViewEntry)
		if !ok || v == nil {
			c.skip("VIEW")
			return
		}
		c.tableBuf["VIEW"] = append(c.tableBuf["VIEW"], mk("VIEW", v.Header.Name))
	case "UCS":
		u, ok := d.Value.(*dwg.UcsEntry)
		if !ok || u == nil {
			c.skip("UCS")
			return
		}
		c.tableBuf["UCS"] = append(c.tableBuf["UCS"], mk("UCS", u.Header.Name))
	case "DIMSTYLE":
		s, ok := d.Value.(*dwg.DimStyleEntry)
		if !ok || s == nil {
			c.skip("DIMSTYLE")
			return
		}
		e := mk("DIMSTYLE", s.Header.Name)
		e.SetGroup(40, FormatFloat(s.Dimscale))
		c.tableBuf["DIMSTYLE"] = append(c.tableBuf["DIMSTYLE"], e)
	case "BLOCK_HEADER":
		b, ok := d.Value.(*dwg.BlockRecord)
		if !ok || b == nil {
			c.skip("BLOCK_HEADER")
			return
		}
		// Register the block and append a BLOCK_RECORD entry.
		c.openBlock(b.Header.Name)
		e := &TableEntry{Type: "BLOCK_RECORD", Handle: raw.Handle.Value}
		e.Rest = []Tag{
			{Code: 100, Value: "AcDbSymbolTableRecord"},
			{Code: 100, Value: "AcDbBlockTableRecord"},
			{Code: 2, Value: b.Header.Name},
			{Code: 70, Value: "0"},
		}
		c.tableBuf["BLOCK_RECORD"] = append(c.tableBuf["BLOCK_RECORD"], e)
		// Mirror the base point into the block definition.
		if blk := c.findBlock(b.Header.Name); blk != nil {
			blk.SetBasePoint(p3(b.BasePoint))
		}
	}
}

func (c *dwgConverter) findBlock(name string) *Block {
	for _, b := range c.blocks {
		if b.Name() == name {
			return b
		}
	}
	return nil
}

func subclassForTable(dxfType string) string {
	switch dxfType {
	case "LAYER":
		return "AcDbLayerTableRecord"
	case "LTYPE":
		return "AcDbLinetypeTableRecord"
	case "STYLE":
		return "AcDbTextStyleTableRecord"
	case "APPID":
		return "AcDbRegAppTableRecord"
	case "VPORT":
		return "AcDbViewportTableRecord"
	case "VIEW":
		return "AcDbViewTableRecord"
	case "UCS":
		return "AcDbUCSTableRecord"
	case "DIMSTYLE":
		return "AcDbDimStyleTableRecord"
	case "BLOCK_RECORD":
		return "AcDbBlockTableRecord"
	}
	return "AcDbSymbolTableRecord"
}

// absorbObject maps a non-entity, non-table record into OBJECTS.
func (c *dwgConverter) absorbObject(label string, d dwg.DecodedEntity, raw *dwg.RawObject) {
	if !c.doc.Version.SupportsObjectsSection() {
		return
	}
	// The DWG port does not surface owner handles. Ownerless OBJECTS
	// records are pruned by strict readers, so every record claims the
	// named-object root dictionary (the first DICTIONARY in stream order)
	// as its owner; the root itself stays owned by 0.
	owner := c.rootDict
	if label == "DICTIONARY" {
		if c.rootDict == 0 {
			c.rootDict = raw.Handle.Value
		}
		if raw.Handle.Value == c.rootDict {
			owner = 0
		}
	}
	handleTag := func() []Tag {
		var out []Tag
		if raw.Handle.Value != 0 {
			out = append(out, Tag{Code: 5, Value: padHandle(raw.Handle.Value)})
		}
		// Owner is always explicit: 0 for the named-object root, the root
		// dictionary for everything else — readers prune ownerless records.
		out = append(out, Tag{Code: 330, Value: padHandle(owner)})
		return out
	}
	switch label {
	case "DICTIONARY":
		dict, _ := d.Value.(*dwg.Dictionary)
		o := &Object{Type: "DICTIONARY"}
		o.Tags = append(o.Tags, handleTag()...)
		o.Tags = append(o.Tags, Tag{Code: 100, Value: "AcDbDictionary"})
		if dict != nil {
			if dict.HardOwner {
				o.Tags = append(o.Tags, Tag{Code: 281, Value: "1"})
			} else {
				o.Tags = append(o.Tags, Tag{Code: 281, Value: "0"})
			}
			for _, k := range dict.Keys {
				o.Tags = append(o.Tags, Tag{Code: 3, Value: k})
			}
			// Value handles live in the object's owner-reference list,
			// which the port does not surface; say so explicitly.
			if len(dict.Keys) > 0 {
				o.Tags = append(o.Tags, Tag{Code: 999, Value: "dictionary value handles not resolvable from the DWG port"})
			}
		}
		c.doc.Objects = append(c.doc.Objects, o)
	case "XRECORD":
		o := &Object{Type: "XRECORD"}
		o.Tags = append(o.Tags, handleTag()...)
		o.Tags = append(o.Tags, Tag{Code: 100, Value: "AcDbXrecord"})
		o.Tags = append(o.Tags, Tag{Code: 999, Value: "XRECORD data pairs not surfaced"})
		c.doc.Objects = append(c.doc.Objects, o)
	case "PLACEHOLDER":
		o := &Object{Type: "ACDBPLACEHOLDER"}
		o.Tags = append(o.Tags, handleTag()...)
		o.Tags = append(o.Tags, Tag{Code: 100, Value: "AcDbPlaceHolder"})
		c.doc.Objects = append(c.doc.Objects, o)
	case "LAYOUT":
		lay, _ := d.Value.(*dwg.AcadLayout)
		o := &Object{Type: "LAYOUT"}
		o.Tags = append(o.Tags, handleTag()...)
		// Canonical subclass chain: AcDbPlotSettings then AcDbLayout —
		// readers index the name from the second block.
		o.Tags = append(o.Tags, Tag{Code: 100, Value: "AcDbPlotSettings"})
		o.Tags = append(o.Tags, Tag{Code: 100, Value: "AcDbLayout"})
		name := "Layout"
		if lay != nil && lay.LayoutName != "" {
			name = lay.LayoutName
		}
		o.Tags = append(o.Tags, Tag{Code: 1, Value: name})
		o.Tags = append(o.Tags, Tag{Code: 999, Value: "LAYOUT body pass-through"})
		c.doc.Objects = append(c.doc.Objects, o)
	case "MLINESTYLE":
		m, _ := d.Value.(*dwg.AcadMlinestyle)
		o := &Object{Type: "ACAD_MLINESTYLE"}
		o.Tags = append(o.Tags, handleTag()...)
		o.Tags = append(o.Tags, Tag{Code: 100, Value: "AcDbMlineStyle"})
		if m != nil {
			o.Tags = append(o.Tags, Tag{Code: 2, Value: m.Name})
		}
		c.doc.Objects = append(c.doc.Objects, o)
	default:
		// CONTROL owners and custom classes belong to TABLES/internals —
		// dropped silently rather than misfiled into OBJECTS.
	}
}

// convertEntity maps one decoded drawable to its DXF payload.
func (c *dwgConverter) convertEntity(d dwg.DecodedEntity) (*Entity, bool) {
	e := func(name string, data AnyEntityData) (*Entity, bool) {
		return &Entity{Type: name, Data: data}, true
	}

	switch d.Label {
	case "LINE":
		if v, ok := d.Value.(*dwg.Line); ok && v != nil {
			return e("LINE", &Line{Start: p3(v.Start), End: p3(v.End),
				Common: extruded(v.Extrusion, v.Thickness)})
		}
	case "POINT":
		if v, ok := d.Value.(*dwg.Point); ok && v != nil {
			return e("POINT", &DxfPoint{Position: p3(v.Position),
				XAxisAngle: radToDeg(v.XAxisAngle),
				Common:     extruded(v.Extrusion, v.Thickness)})
		}
	case "CIRCLE":
		if v, ok := d.Value.(*dwg.Circle); ok && v != nil {
			return e("CIRCLE", &Circle{Center: p3(v.Center), Radius: v.Radius,
				Common: extruded(v.Extrusion, v.Thickness)})
		}
	case "ARC":
		if v, ok := d.Value.(*dwg.Arc); ok && v != nil {
			return e("ARC", &Arc{Center: p3(v.Center), Radius: v.Radius,
				StartAngle: radToDeg(v.StartAngle), EndAngle: radToDeg(v.EndAngle),
				Common: extruded(v.Extrusion, v.Thickness)})
		}
	case "ELLIPSE":
		if v, ok := d.Value.(*dwg.Ellipse); ok && v != nil {
			return e("ELLIPSE", &Ellipse{Center: p3pp(v.Center), MajorAxis: p3pp(v.MajorAxis),
				Ratio: v.AxisRatio, StartParam: v.StartParam, EndParam: v.EndParam,
				Common: extrudedFromPlot(v.Extrusion, 0)})
		}
	case "RAY":
		if v, ok := d.Value.(*dwg.Ray); ok && v != nil {
			return e("RAY", &Ray{Point: p3(v.Start), Dir: p3(v.Direction)})
		}
	case "XLINE":
		if v, ok := d.Value.(*dwg.XLine); ok && v != nil {
			return e("XLINE", &XLine{Point: p3(v.Point), Dir: p3(v.Direction)})
		}
	case "SOLID", "TRACE":
		if v, ok := d.Value.(*dwg.Solid); ok && v != nil {
			cmn := extruded(dwg.Vec3D(v.Extrusion), v.Thickness)
			corners := [4]Point3{}
			for i, c2 := range v.Corners {
				corners[i] = Pt3(c2.X, c2.Y, v.Elevation)
			}
			if d.Label == "TRACE" {
				return e("TRACE", &Trace{Common: cmn, Corners: corners})
			}
			return e("SOLID", &Solid{Common: cmn, Corners: corners})
		}
	case "TEXT":
		if v, ok := d.Value.(*dwg.Text); ok && v != nil {
			t := &Text{Height: v.Height, Value: v.Text,
				Rotation: radToDeg(v.RotationAngle), WidthFactor: v.WidthFactor,
				Oblique: radToDeg(v.ObliqueAngle), GenFlags: int64(v.Generation),
				HJustify: int64(v.HAlign), VJustify: int64(v.VAlign)}
			t.Position = Pt3(v.InsertionPoint.X, v.InsertionPoint.Y, v.Elevation)
			if v.AlignmentPoint != nil {
				t.AlignPoint = Pt3(v.AlignmentPoint.X, v.AlignmentPoint.Y, v.Elevation)
				t.HasAlign = true
			}
			t.Common = extruded(v.Extrusion, v.Thickness)
			return e("TEXT", t)
		}
	case "MTEXT":
		if v, ok := d.Value.(*dwg.MText); ok && v != nil {
			m := &MText{Position: p3pp(v.InsertionPoint), Height: v.NominalTextHeight,
				RectWidth: v.RectWidth, Attachment: int64(v.AttachmentPoint),
				FlowDir: int64(v.DrawingDirection), Text: v.Text,
				LineSpacingStyle: int64(v.LinespaceStyle), LineSpacingFactor: v.LinespaceFactor}
			m.Extrusion = p3pp(v.Extrusion)
			m.HasExtrusion = v.Extrusion != (dwg.PlotPoint3D{})
			m.Direction = p3pp(v.XAxisDirection)
			m.HasDirection = v.XAxisDirection != (dwg.PlotPoint3D{})
			return e("MTEXT", m)
		}
	case "ATTRIB":
		if v, ok := d.Value.(*dwg.Attrib); ok && v != nil {
			return e("ATTRIB", attribFromText(v.Text, v.Tag, int64(v.Flags), int64(v.FieldLength), ""))
		}
	case "ATTDEF":
		if v, ok := d.Value.(*dwg.AttDef); ok && v != nil {
			return e("ATTDEF", attribFromText(v.Text, v.Tag, int64(v.Flags), int64(v.FieldLength), v.Prompt))
		}
	case "INSERT":
		if v, ok := d.Value.(*dwg.Insert); ok && v != nil {
			i := &Insert{Position: p3(v.InsertionPoint), Scale: p3(v.Scale),
				Rotation: radToDeg(v.Rotation), HasAttribs: v.HasAttribs}
			i.Common = extruded(v.Extrusion, 0)
			return e("INSERT", i)
		}
	case "LWPOLYLINE":
		if v, ok := d.Value.(*dwg.LwPolyline); ok && v != nil {
			return e("LWPOLYLINE", lwpolylineFromDwg(v))
		}
	case "SPLINE":
		if v, ok := d.Value.(*dwg.Spline); ok && v != nil {
			return e("SPLINE", splineFromDwg(v))
		}
	case "3DFACE":
		if v, ok := d.Value.(*dwg.ThreeDFace); ok && v != nil {
			f := &Face3D{EdgeFlags: int64(v.InvisibleEdges)}
			for i, c2 := range v.Corners {
				f.Corners[i] = p3(c2)
			}
			return e("3DFACE", f)
		}
	case "LEADER":
		if v, ok := d.Value.(*dwg.Leader); ok && v != nil {
			l := &Leader{PathType: int64(v.PathType), CreationType: int64(v.AnnotType)}
			for _, p := range v.Points {
				l.Vertices = append(l.Vertices, p3(p))
			}
			return e("LEADER", l)
		}
	case "TOLERANCE":
		if v, ok := d.Value.(*dwg.Tolerance); ok && v != nil {
			return e("TOLERANCE", &Tolerance{Value: v.TextString, Position: p3(v.InsertionPoint)})
		}
	case "VIEWPORT":
		if v, ok := d.Value.(*dwg.ViewportEntity); ok && v != nil {
			return e("VIEWPORT", &Viewport{Center: p3(v.Center), Width: v.Width, Height: v.Height})
		}
	case "SHAPE":
		// The DWG decoder does not expose SHAPE; fall through to skip.
	case "HATCH":
		if v, ok := d.Value.(*dwg.Hatch); ok && v != nil {
			return e("HATCH", hatchFromDwg(v))
		}
	case "POLYLINE_2D", "POLYLINE_3D", "POLYFACE_MESH", "POLYGON_MESH":
		if v, ok := d.Value.(*dwg.Polyline); ok && v != nil {
			p := &Polyline{}
			flags := int64(v.Flag)
			switch d.Label {
			case "POLYLINE_3D":
				flags |= 8
			case "POLYFACE_MESH":
				flags |= 64
			case "POLYGON_MESH":
				flags |= 16
			}
			p.Flags = flags
			p.Elevation = v.Elevation
			p.Thickness = v.Thickness
			p.Common = extruded(v.Extrusion, 0)
			return e("POLYLINE", p)
		}
		if v, ok := d.Value.(*dwg.Polyline3d); ok && v != nil {
			return e("POLYLINE", &Polyline{Flags: 8})
		}
	case "VERTEX_2D":
		if v, ok := d.Value.(*dwg.Vertex); ok && v != nil {
			return e("VERTEX", vertexFromDwg(v))
		}
	case "VERTEX_3D", "VERTEX_MESH", "VERTEX_PFACE":
		if v, ok := d.Value.(*dwg.VertexPoint); ok && v != nil {
			vx := &PolyVertex{Flags: int64(v.Flag), Position: p3(v.Location)}
			if d.Label == "VERTEX_PFACE" {
				vx.Flags |= 128
			}
			return e("VERTEX", vx)
		}
	case "VERTEX_PFACE_FACE":
		if v, ok := d.Value.(*dwg.VertexPfaceFace); ok && v != nil {
			vx := &PolyVertex{Flags: 128, FaceRecord: true}
			vx.Position = Pt3(float64(v.VertexIndices[0]), float64(v.VertexIndices[1]), float64(v.VertexIndices[2]))
			vx.ID = int64(v.VertexIndices[3])
			return e("VERTEX", pfaceFaceVertex(v.VertexIndices))
		}
	case "DIMENSION_ORDINATE", "DIMENSION_LINEAR", "DIMENSION_ALIGNED",
		"DIMENSION_ANG_3PT", "DIMENSION_ANG_2LN", "DIMENSION_RADIUS",
		"DIMENSION_DIAMETER":
		if v, ok := d.Value.(*dwg.Dimension); ok && v != nil {
			return e("DIMENSION", dimensionFromDwg(v))
		}
	}
	c.skip(d.Label)
	return nil, false
}

func pfaceFaceVertex(idx [4]int16) *PolyVertex {
	vx := &PolyVertex{Flags: 128, FaceRecord: true}
	// Face records pack 1-based vertex indices into the vertex "point";
	// DXF carries them in groups 71-74, which the writer derives from the
	// Extra tags below.
	for i, v := range idx {
		if v == 0 {
			continue
		}
		vx.Extra = append(vx.Extra, Tag{Code: 71 + i, Value: itoa(int64(v))})
	}
	return vx
}

func attribFromText(t dwg.Text, tag string, flags, fieldLength int64, prompt string) AnyEntityData {
	base := &Text{Height: t.Height, Value: t.Text,
		Rotation: radToDeg(t.RotationAngle), WidthFactor: t.WidthFactor,
		Oblique: radToDeg(t.ObliqueAngle), GenFlags: int64(t.Generation),
		HJustify: int64(t.HAlign), VJustify: int64(t.VAlign)}
	base.Position = Pt3(t.InsertionPoint.X, t.InsertionPoint.Y, t.Elevation)
	if t.AlignmentPoint != nil {
		base.AlignPoint = Pt3(t.AlignmentPoint.X, t.AlignmentPoint.Y, t.Elevation)
		base.HasAlign = true
	}
	base.Common = extruded(t.Extrusion, t.Thickness)
	if prompt != "" {
		return &Attdef{Text: *base, Tag: tag, Prompt: prompt, Flags: flags, FieldLength: fieldLength}
	}
	return &Attrib{Text: *base, Tag: tag, Flags: flags, FieldLength: fieldLength}
}

func lwpolylineFromDwg(v *dwg.LwPolyline) *LwPolyline {
	p := &LwPolyline{Closed: v.Closed}
	if v.ConstantWidth != nil {
		p.ConstWidth = *v.ConstantWidth
	}
	if v.Elevation != nil {
		p.Elevation = *v.Elevation
	}
	if v.Thickness != nil {
		p.Common.Thickness = *v.Thickness
	}
	if v.Extrusion != nil {
		p.Common.Extrusion = Point3{X: v.Extrusion.X, Y: v.Extrusion.Y, Z: v.Extrusion.Z}
		p.Common.HasExtrusion = true
	}
	for i, pt := range v.Vertices {
		vx := LwPolylineVertex{Point: Point2{X: pt.X, Y: pt.Y}}
		if i < len(v.Bulges) {
			vx.Bulge, vx.HasBulge = v.Bulges[i], true
		}
		if i < len(v.Widths) {
			vx.StartWidth, vx.EndWidth, vx.HasWidth = v.Widths[i][0], v.Widths[i][1], true
		}
		if i < len(v.VertexIDs) {
			vx.ID, vx.HasID = int64(v.VertexIDs[i]), true
		}
		p.Vertices = append(p.Vertices, vx)
	}
	return p
}

func splineFromDwg(v *dwg.Spline) *Spline {
	s := &Spline{Degree: int64(v.Degree)}
	if v.Control != nil {
		s.Rational = v.Control.Rational
		s.Closed = v.Control.Closed
		s.Periodic = v.Control.Periodic
		s.KnotTol = v.Control.KnotTolerance
		s.CtrlTol = v.Control.ControlTolerance
		s.Knots = append(s.Knots, v.Control.Knots...)
		s.Weights = append(s.Weights, v.Control.Weights...)
		for _, p := range v.Control.ControlPoints {
			s.Controls = append(s.Controls, Point3{X: p.X, Y: p.Y, Z: p.Z})
		}
	}
	if v.Fit != nil {
		s.FitTol = v.Fit.Tolerance
		s.StartTangent = Point3{X: v.Fit.BeginTangent.X, Y: v.Fit.BeginTangent.Y, Z: v.Fit.BeginTangent.Z}
		s.EndTangent = Point3{X: v.Fit.EndTangent.X, Y: v.Fit.EndTangent.Y, Z: v.Fit.EndTangent.Z}
		s.HasTangents = v.Fit.BeginTangent != (dwg.Vec3D{}) || v.Fit.EndTangent != (dwg.Vec3D{})
		for _, p := range v.Fit.FitPoints {
			s.FitPoints = append(s.FitPoints, Point3{X: p.X, Y: p.Y, Z: p.Z})
		}
	}
	return s
}

func vertexFromDwg(v *dwg.Vertex) *PolyVertex {
	vx := &PolyVertex{
		Flags:      int64(v.Flag),
		Position:   p3(v.Location),
		StartWidth: v.StartWidth,
		EndWidth:   v.EndWidth,
		Bulge:      v.Bulge,
		ID:         int64(v.VertexID),
	}
	if v.TangentDirection != nil {
		vx.TangentDir = radToDeg(*v.TangentDirection)
	}
	return vx
}

func dimensionFromDwg(v *dwg.Dimension) *Dimension {
	dim := &Dimension{Text: v.Common.UserText}
	dim.DefPoint = p3(v.DefPoint10)
	dim.TextMidpoint = Pt3(v.Common.TextMidpoint.X, v.Common.TextMidpoint.Y, v.Common.Elevation)
	switch v.Kind {
	case "DIMENSION_ORDINATE":
		dim.Subtype, dim.Flags = DimTypeOrdinate, DimTypeOrdinate
		dim.DefPoint1, dim.HasPoint1 = p3(v.FeatureLocation13), true
		dim.DefPoint2, dim.HasPoint2 = p3(v.LeaderEndpoint14), true
	case "DIMENSION_LINEAR":
		dim.Subtype, dim.Flags = DimTypeLinear, DimTypeLinear
		dim.DefPoint1, dim.HasPoint1 = p3(v.DefPoint13), true
		dim.DefPoint2, dim.HasPoint2 = p3(v.DefPoint14), true
		dim.Angle = radToDeg(v.DimRotation)
		dim.ObliqueAngle = radToDeg(v.ExtensionLineRotation)
	case "DIMENSION_ALIGNED":
		dim.Subtype, dim.Flags = DimTypeAligned, DimTypeAligned
		dim.DefPoint1, dim.HasPoint1 = p3(v.DefPoint13), true
		dim.DefPoint2, dim.HasPoint2 = p3(v.DefPoint14), true
		dim.Angle = radToDeg(v.DimRotation)
	case "DIMENSION_ANG_3PT":
		dim.Subtype, dim.Flags = DimTypeAngular3P, DimTypeAngular3P
		dim.DefPoint1, dim.HasPoint1 = p3(v.DefPoint13), true
		dim.DefPoint2, dim.HasPoint2 = p3(v.DefPoint14), true
		dim.DefPoint3, dim.HasPoint3 = p3(v.DefPoint15), true
		dim.DefPoint4, dim.HasPoint4 = Pt3(v.DefPoint16.X, v.DefPoint16.Y, v.Common.Elevation), true
	case "DIMENSION_ANG_2LN":
		dim.Subtype, dim.Flags = DimTypeAngular, DimTypeAngular
		dim.DefPoint1, dim.HasPoint1 = p3(v.DefPoint13), true
		dim.DefPoint2, dim.HasPoint2 = p3(v.DefPoint14), true
		dim.DefPoint3, dim.HasPoint3 = p3(v.DefPoint15), true
		dim.DefPoint4, dim.HasPoint4 = Pt3(v.DefPoint16.X, v.DefPoint16.Y, v.Common.Elevation), true
	case "DIMENSION_RADIUS":
		dim.Subtype, dim.Flags = DimTypeRadius, DimTypeRadius
		dim.LeaderLength = v.LeaderLength
		dim.DefPoint3, dim.HasPoint3 = p3(v.DefPoint15), true
	case "DIMENSION_DIAMETER":
		dim.Subtype, dim.Flags = DimTypeDiameter, DimTypeDiameter
		dim.LeaderLength = v.LeaderLength
		dim.DefPoint3, dim.HasPoint3 = p3(v.DefPoint15), true
	}
	dim.AttachmentPt = int64(v.Common.Attachment)
	dim.LineSpacing = int64(v.Common.LineSpacingStyle)
	dim.LineSpacingFactor = v.Common.LineSpacingFactor
	dim.ActualText = v.Common.ActualMeasurement
	if v.Common.UserText == "" {
		dim.Text = ""
	}
	return dim
}

func hatchFromDwg(v *dwg.Hatch) *Hatch {
	h := &Hatch{
		PatternName:   v.PatternName,
		SolidFill:     v.SolidFill,
		Associative:   v.Associative,
		IslandStyle:   int64(v.PatternStyle),
		PatternType:   int64(v.PatternType),
		PatternAngle:  v.PatternAngle,
		PatternScale:  v.PatternScale,
		PatternDouble: v.PatternDouble,
	}
	el := v.Elevation
	for _, p := range v.Paths {
		loop := HatchLoop{PathFlags: int64(p.Flags)}
		if poly, ok := p.Segments.(*dwg.HatchPolylineSegments); ok {
			hp := &HatchPolyPath{HasBulge: poly.HasBulge, Closed: poly.IsClosed}
			for _, vt := range poly.Vertices {
				vx := LwPolylineVertex{Point: Point2{X: vt.Point.X, Y: vt.Point.Y}}
				if vt.Bulge != nil {
					vx.Bulge, vx.HasBulge = *vt.Bulge, true
				}
				hp.Vertices = append(hp.Vertices, vx)
			}
			loop.Poly = hp
		} else if seg, ok := p.Segments.(*dwg.HatchEdgeSegments); ok {
			for _, edge := range seg.Edges {
				switch e := edge.(type) {
				case *dwg.HatchEdgeLine:
					loop.Edges = append(loop.Edges, HatchEdge{Type: HatchEdgeLine,
						Start: Pt3(e.Start.X, e.Start.Y, el), End: Pt3(e.End.X, e.End.Y, el)})
				case *dwg.HatchEdgeArc:
					loop.Edges = append(loop.Edges, HatchEdge{Type: HatchEdgeArc,
						Center: Pt3(e.Center.X, e.Center.Y, el), Radius: e.Radius,
						StartAngle: e.StartAngle, EndAngle: e.EndAngle, CCW: e.CounterClockwise})
				case *dwg.HatchEdgeEllipse:
					loop.Edges = append(loop.Edges, HatchEdge{Type: HatchEdgeEllipse,
						Center:    Pt3(e.Center.X, e.Center.Y, el),
						MajorAxis: Pt3(e.Endpoint.X, e.Endpoint.Y, 0), MinorRatio: e.AxisRatio,
						StartAngle: e.StartAngle, EndAngle: e.EndAngle, CCW: e.CounterClockwise})
				case *dwg.HatchEdgeSpline:
					se := HatchEdge{Type: HatchEdgeSpline,
						Degree: int64(e.Degree), Rational: e.IsRational, Periodic: e.IsPeriodic,
						Knots: append([]float64(nil), e.Knots...)}
					for _, p := range e.ControlPoints {
						se.Controls = append(se.Controls, Pt3(p.X, p.Y, el))
					}
					if e.StartTangent != (dwg.PlotPoint2D{}) {
						se.Tangents[0] = Pt3(e.StartTangent.X, e.StartTangent.Y, 0)
						se.HasTangent[0] = true
					}
					if e.EndTangent != (dwg.PlotPoint2D{}) {
						se.Tangents[1] = Pt3(e.EndTangent.X, e.EndTangent.Y, 0)
						se.HasTangent[1] = true
					}
					loop.Edges = append(loop.Edges, se)
				}
			}
		}
		h.Loops = append(h.Loops, loop)
	}
	for _, pl := range v.PatternLines {
		h.PatternLines = append(h.PatternLines, HatchPatternLine{
			Angle:     pl.Angle,
			BasePoint: Point2{X: pl.Origin.X, Y: pl.Origin.Y},
			Offset:    Point2{X: pl.Offset.X, Y: pl.Offset.Y},
			Dashes:    append([]float64(nil), pl.Dashes...),
		})
	}
	for _, sp := range v.SeedPoints {
		h.SeedPoints = append(h.SeedPoints, Point2{X: sp.X, Y: sp.Y})
	}
	return h
}

// extrudedFromPlot builds a Common block from a PlotPoint3D extrusion.
func extrudedFromPlot(ex dwg.PlotPoint3D, thickness float64) Common {
	c := Common{Thickness: thickness}
	if ex != (dwg.PlotPoint3D{}) {
		c.Extrusion = Point3{X: ex.X, Y: ex.Y, Z: ex.Z}
		c.HasExtrusion = true
	}
	return c
}

// extruded builds a Common block from a DWG extrusion + thickness pair.
func extruded(ex dwg.Vec3D, thickness float64) Common {
	c := Common{Thickness: thickness}
	if ex != (dwg.Vec3D{}) {
		c.Extrusion = Point3{X: ex.X, Y: ex.Y, Z: ex.Z}
		c.HasExtrusion = true
	}
	return c
}

func radToDeg(rad float64) float64 {
	return rad * 180.0 / 3.14159265358979323846
}

// p3 converts a DWG 3D point.
func p3(p dwg.Point3D) Point3 { return Point3{X: p.X, Y: p.Y, Z: p.Z} }

// p3pp converts a PlotPoint3D.
func p3pp(p dwg.PlotPoint3D) Point3 { return Point3{X: p.X, Y: p.Y, Z: p.Z} }

// p2 converts a PlotPoint2D at the given elevation.
func p2z(p dwg.PlotPoint2D, z float64) Point3 { return Point3{X: p.X, Y: p.Y, Z: z} }

// assignEntityHandles gives every converted entity a handle from the
// document sequence — DXF readers key cross-references by handle.
func assignEntityHandles(doc *Document) {
	var walk func(list []*Entity)
	walk = func(list []*Entity) {
		for _, e := range list {
			if c := e.AccessCommon(); c != nil && c.Handle == 0 {
				c.Handle = doc.AllocHandle()
			}
			switch data := e.Data.(type) {
			case *Insert:
				walk(data.Attribs)
			case *Polyline:
				walk(data.Vertices)
			}
		}
	}
	walk(doc.Entities)
	for _, b := range doc.Blocks {
		walk(b.Entities)
	}
}
