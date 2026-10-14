package dxf

// Document emission — canonical section order HEADER, CLASSES, TABLES,
// BLOCKS, ENTITIES, OBJECTS, THUMBNAILIMAGE, then any preserved foreign
// sections, then EOF.

func (d *Document) writeHeader(w *TagWriter) {
	d.refreshHandseed()
	w.Str(0, "SECTION")
	w.Str(2, "HEADER")
	for _, v := range d.Header {
		w.Str(9, v.Name)
		w.Pairs(v.Values)
	}
	w.Str(0, "ENDSEC")
}

// refreshHandseed keeps $HANDSEED above every allocated handle when the
// document owns a live allocator (synthesized documents). Parsed documents
// have nextHandle == 0 and keep their stored value.
func (d *Document) refreshHandseed() {
	if d.nextHandle == 0 {
		return
	}
	need := d.nextHandle + 1
	if v, ok := d.Header.Find("$HANDSEED"); ok {
		if len(v.Values) > 0 && parseHandle(v.Values[0].Value) < need {
			v.Values[0].Value = padHandle(need)
		}
		return
	}
	d.Header = append(d.Header, HeaderVar{Name: "$HANDSEED",
		Values: []Tag{{Code: 5, Value: padHandle(need)}}})
}

func (d *Document) writeTo(w *TagWriter) error {
	d.writeHeader(w)
	if len(d.Classes) > 0 {
		w.writeClasses(d.Classes, d.Version)
	}
	w.Str(0, "SECTION")
	w.Str(2, "TABLES")
	for _, t := range d.Tables {
		w.writeTable(t, d.Version)
	}
	w.Str(0, "ENDSEC")
	d.writeBlocksSection(w)
	d.writeEntitiesSection(w)
	w.writeObjects(d.Objects, d.Version)
	if len(d.Thumbnail) > 0 {
		w.Str(0, "SECTION")
		w.Str(2, "THUMBNAILIMAGE")
		w.Pairs(d.Thumbnail)
		w.Str(0, "ENDSEC")
	}
	for _, s := range d.OtherSections {
		w.Str(0, "SECTION")
		w.Str(2, s.Name)
		w.Pairs(s.Tags)
		w.Str(0, "ENDSEC")
	}
	for _, c := range d.Comments {
		w.Pair(999, c)
	}
	w.Str(0, "EOF")
	return nil
}

func (d *Document) writeBlocksSection(w *TagWriter) {
	w.Str(0, "SECTION")
	w.Str(2, "BLOCKS")
	for _, b := range d.Blocks {
		w.writeBlock(b, d.Version)
	}
	w.Str(0, "ENDSEC")
}

func (d *Document) writeEntitiesSection(w *TagWriter) error {
	w.Str(0, "SECTION")
	w.Str(2, "ENTITIES")
	for _, e := range d.Entities {
		if err := writeEntity(w, e, d.Version); err != nil {
			continue // skip malformed payloads
		}
	}
	w.Str(0, "ENDSEC")
	return nil
}

// symTableRest builds a table entry's Rest list with the two canonical
// subclass markers prefixed for R13+ dialects (omitted under R12).
func symTableRest(sub bool, marker string, pairs []Tag) []Tag {
	out := make([]Tag, 0, len(pairs)+2)
	if sub {
		out = append(out, Tag{Code: 100, Value: "AcDbSymbolTableRecord"})
		if marker != "" {
			out = append(out, Tag{Code: 100, Value: marker})
		}
	}
	return append(out, pairs...)
}

// NewDocument builds a minimal, standards-conformant drawing skeleton for
// the target dialect: HEADER with the standard variable set, TABLES with
// the canonical table roster, and the *Model_Space/*Paper_Space block
// placeholders. Handles are allocated sequentially from 0x10.
func NewDocument(version DxfVersion) *Document {
	return newDocumentSeeded(version, 0x10)
}

// newDocumentSeeded builds the skeleton with the handle sequence starting
// just above seed — callers merging foreign content (the DWG converter)
// pick a seed above every imported handle so the handle space stays
// collision-free.
func newDocumentSeeded(version DxfVersion, seed uint64) *Document {
	d := &Document{Version: version, nextHandle: seed}
	doc := &d.nextHandle
	alloc := func() uint64 {
		*doc++
		return *doc
	}

	// HEADER.
	d.Header = append(d.Header,
		HeaderVar{Name: "$ACADVER", Values: []Tag{{Code: 1, Value: version.AcadVer()}}},
		HeaderVar{Name: "$INSBASE", Values: point3Tags(Pt3(0, 0, 0))},
		HeaderVar{Name: "$EXTMIN", Values: point3Tags(Pt3(1e20, 1e20, 1e20))},
		HeaderVar{Name: "$EXTMAX", Values: point3Tags(Pt3(-1e20, -1e20, -1e20))},
		HeaderVar{Name: "$LIMMIN", Values: point3Tags(Pt2(0, 0))},
		HeaderVar{Name: "$LIMMAX", Values: point3Tags(Pt2(12, 9))},
		HeaderVar{Name: "$ORTHOMODE", Values: []Tag{{Code: 70, Value: "0"}}},
		HeaderVar{Name: "$REGENMODE", Values: []Tag{{Code: 70, Value: "1"}}},
		HeaderVar{Name: "$FILLMODE", Values: []Tag{{Code: 70, Value: "1"}}},
		HeaderVar{Name: "$QTEXTMODE", Values: []Tag{{Code: 70, Value: "0"}}},
		HeaderVar{Name: "$MIRRTEXT", Values: []Tag{{Code: 70, Value: "0"}}},
		HeaderVar{Name: "$LTSCALE", Values: []Tag{{Code: 40, Value: "1"}}},
		HeaderVar{Name: "$ATTREQ", Values: []Tag{{Code: 70, Value: "1"}}},
		HeaderVar{Name: "$ATTDIA", Values: []Tag{{Code: 70, Value: "0"}}},
		HeaderVar{Name: "$AUNITS", Values: []Tag{{Code: 70, Value: "0"}}},
		HeaderVar{Name: "$ANGDIR", Values: []Tag{{Code: 70, Value: "0"}}},
		HeaderVar{Name: "$ANGBASE", Values: []Tag{{Code: 40, Value: "0"}}},
		HeaderVar{Name: "$INSUNITS", Values: []Tag{{Code: 70, Value: "0"}}},
		HeaderVar{Name: "$CECOLOR", Values: []Tag{{Code: 62, Value: "256"}}},
		HeaderVar{Name: "$CELTYPE", Values: []Tag{{Code: 6, Value: "ByLayer"}}},
	)

	// TABLES.
	sub := version.SupportsSubclassMarkers()
	vport := &Table{Name: "VPORT", Flags: 1, Handle: alloc()}
	vport.Entries = append(vport.Entries, activeVport(alloc(), sub))

	ltype := &Table{Name: "LTYPE", Flags: 3, Handle: alloc()}
	ltype.Entries = append(ltype.Entries,
		simpleEntry("LTYPE", "ByBlock", alloc(), "Ltype by block", sub),
		simpleEntry("LTYPE", "ByLayer", alloc(), "Ltype by layer", sub),
		simpleEntry("LTYPE", "Continuous", alloc(), "Solid line", sub),
	)

	layer := &Table{Name: "LAYER", Flags: 1, Handle: alloc()}
	l0 := &TableEntry{Type: "LAYER", Handle: alloc()}
	l0.Rest = symTableRest(sub, "AcDbLayerTableRecord", []Tag{
		{Code: 2, Value: "0"},
		{Code: 70, Value: "0"},
		{Code: 62, Value: "7"},
		{Code: 6, Value: "Continuous"},
	})
	layer.Entries = append(layer.Entries, l0)

	style := &Table{Name: "STYLE", Flags: 1, Handle: alloc()}
	st := &TableEntry{Type: "STYLE", Handle: alloc()}
	st.Rest = symTableRest(sub, "AcDbTextStyleTableRecord", []Tag{
		{Code: 2, Value: "Standard"},
		{Code: 70, Value: "0"},
		{Code: 40, Value: "0"},
		{Code: 41, Value: "1"},
		{Code: 50, Value: "0"},
		{Code: 71, Value: "0"},
		{Code: 42, Value: "2.5"},
		{Code: 3, Value: "txt"},
		{Code: 4, Value: ""},
	})
	style.Entries = append(style.Entries, st)

	appid := &Table{Name: "APPID", Flags: 1, Handle: alloc()}
	ap := &TableEntry{Type: "APPID", Handle: alloc()}
	ap.Rest = symTableRest(sub, "AcDbRegAppTableRecord", []Tag{
		{Code: 2, Value: "ACAD"},
		{Code: 70, Value: "0"},
	})
	appid.Entries = append(appid.Entries, ap)

	dimstyle := &Table{Name: "DIMSTYLE", Flags: 1, Handle: alloc()}
	ds := &TableEntry{Type: "DIMSTYLE", Handle: alloc()}
	ds.Rest = symTableRest(sub, "AcDbDimStyleTableRecord", []Tag{
		{Code: 2, Value: "Standard"},
		{Code: 70, Value: "0"},
		{Code: 3, Value: ""},
		{Code: 4, Value: ""},
		{Code: 5, Value: ""},
		{Code: 6, Value: ""},
		{Code: 7, Value: ""},
		{Code: 40, Value: "1"},
		{Code: 41, Value: "2.5"},
		{Code: 42, Value: "0.625"},
		{Code: 43, Value: "3.75"},
		{Code: 44, Value: "1.25"},
		{Code: 45, Value: "0"},
		{Code: 46, Value: "0"},
		{Code: 47, Value: "0"},
		{Code: 48, Value: "0"},
		{Code: 140, Value: "2.5"},
		{Code: 141, Value: "2.5"},
		{Code: 142, Value: "0"},
		{Code: 143, Value: "25.4"},
		{Code: 144, Value: "1"},
		{Code: 145, Value: "0"},
		{Code: 146, Value: "1"},
		{Code: 147, Value: "0.625"},
		{Code: 71, Value: "0"},
		{Code: 72, Value: "0"},
		{Code: 73, Value: "0"},
		{Code: 74, Value: "0"},
		{Code: 75, Value: "0"},
		{Code: 76, Value: "0"},
		{Code: 77, Value: "0"},
		{Code: 78, Value: "0"},
	})
	dimstyle.Entries = append(dimstyle.Entries, ds)

	d.Tables = append(d.Tables, vport, ltype, layer, style, appid, dimstyle)

	if version.SupportsObjectsSection() {
		view := &Table{Name: "VIEW", Flags: 0, Handle: alloc()}
		ucs := &Table{Name: "UCS", Flags: 0, Handle: alloc()}
		brec := &Table{Name: "BLOCK_RECORD", Flags: 2, Handle: alloc()}
		for _, name := range []string{"*Model_Space", "*Paper_Space"} {
			e := &TableEntry{Type: "BLOCK_RECORD", Handle: alloc()}
			e.Rest = symTableRest(sub, "AcDbBlockTableRecord", []Tag{
				{Code: 2, Value: name},
				{Code: 70, Value: "0"},
			})
			brec.Entries = append(brec.Entries, e)
		}
		d.Tables = append(d.Tables, view, ucs, brec)

		mk := func(name string) *Block {
			b := NewBlock(name, version)
			b.SetHandleOwner(alloc(), alloc())
			b.SetEndHandle(alloc())
			b.setEndblk(version)
			return b
		}
		d.Blocks = append(d.Blocks,
			mk("*Model_Space"),
			mk("*Paper_Space"),
		)
	} else {
		mk := func(name string) *Block {
			b := NewBlock(name, version)
			b.setEndblk(version)
			return b
		}
		d.Blocks = append(d.Blocks,
			mk("*Model_Space"),
			mk("*Paper_Space"),
		)
	}

	// $HANDSEED points at the next unallocated handle.
	d.Header = append(d.Header, HeaderVar{Name: "$HANDSEED", Values: []Tag{{Code: 5, Value: padHandle(d.nextHandle + 1)}}})
	return d
}

func point3Tags(p Point3) []Tag {
	return []Tag{
		{Code: 10, Value: FormatFloat(p.X)},
		{Code: 20, Value: FormatFloat(p.Y)},
		{Code: 30, Value: FormatFloat(p.Z)},
	}
}

func activeVport(h uint64, sub bool) *TableEntry {
	e := &TableEntry{Type: "VPORT", Handle: h}
	e.Rest = symTableRest(sub, "AcDbViewportTableRecord", []Tag{
		{Code: 2, Value: "*Active"},
		{Code: 70, Value: "0"},
		{Code: 10, Value: "0"}, {Code: 20, Value: "0"}, {Code: 30, Value: "0"},
		{Code: 11, Value: "1"}, {Code: 21, Value: "1"}, {Code: 31, Value: "0"},
		{Code: 12, Value: "0"}, {Code: 22, Value: "0"},
		{Code: 13, Value: "0"}, {Code: 23, Value: "0"},
		{Code: 14, Value: "10"}, {Code: 24, Value: "10"},
		{Code: 15, Value: "10"}, {Code: 25, Value: "10"},
		{Code: 16, Value: "0"}, {Code: 26, Value: "0"}, {Code: 36, Value: "1"},
		{Code: 17, Value: "0"}, {Code: 27, Value: "0"}, {Code: 37, Value: "0"},
		{Code: 41, Value: "1"},
		{Code: 42, Value: "1"},
		{Code: 43, Value: "0"},
		{Code: 44, Value: "0"},
		{Code: 50, Value: "0"},
		{Code: 51, Value: "0"},
		{Code: 71, Value: "0"},
		{Code: 72, Value: "100"},
		{Code: 73, Value: "1"},
		{Code: 74, Value: "3"},
		{Code: 75, Value: "0"},
		{Code: 76, Value: "1"},
		{Code: 77, Value: "0"},
		{Code: 78, Value: "0"},
		{Code: 281, Value: "0"},
		{Code: 65, Value: "1"},
	})
	return e
}

// simpleEntry builds an LTYPE entry with a dash pattern of the given shape.
func simpleEntry(kind, name string, h uint64, desc string, sub bool) *TableEntry {
	e := &TableEntry{Type: kind, Handle: h}
	e.Rest = symTableRest(sub, "AcDbLinetypeTableRecord", []Tag{
		{Code: 2, Value: name},
		{Code: 70, Value: "0"},
		{Code: 3, Value: desc},
		{Code: 72, Value: "65"},
		{Code: 73, Value: "0"},
		{Code: 40, Value: "0"},
	})
	return e
}
