package dxf

// TABLES section: a table is the `0 TABLE / 2 <name>` header run plus its
// entries, closed by `0 ENDTAB`. Entries are stored as verbatim tag runs
// (everything after the `0 <TYPE>` and the optional handle/owner prefix) so
// unknown tables and unknown entry fields round-trip losslessly; typed
// accessors expose the codes consumers actually need.

// Table is one symbol table.
type Table struct {
	Name    string // "LAYER", "LTYPE", …
	Handle  uint64 // group 5 on the TABLE record (R2000+)
	Owner   uint64 // group 330
	Flags   int64  // group 70 (entry count as written)
	Extra   []Tag  // other TABLE-level tags, verbatim
	Entries []*TableEntry
}

// TableEntry is one symbol-table record. Rest preserves every tag after
// the `0 <TYPE> / 5 / 330` prefix verbatim — including the 100 subclass
// markers — so emission is byte-faithful.
type TableEntry struct {
	Type     string
	Handle   uint64
	Owner    uint64
	HasOwner bool
	Rest     []Tag
}

// Name returns the entry name (group 2; DIMSTYLE uses it too).
func (e *TableEntry) Name() string {
	for _, t := range e.Rest {
		if t.Code == 2 {
			return t.Value
		}
	}
	return ""
}

// GroupInt returns the first tag with the given code parsed as an integer.
func (e *TableEntry) GroupInt(code int) (int64, bool) {
	for _, t := range e.Rest {
		if t.Code == code {
			return t.Int(), true
		}
	}
	return 0, false
}

// GroupFloat returns the first tag with the given code parsed as a float.
func (e *TableEntry) GroupFloat(code int) (float64, bool) {
	for _, t := range e.Rest {
		if t.Code == code {
			return t.Float(), true
		}
	}
	return 0, false
}

// GroupStr returns the first tag with the given code as a string.
func (e *TableEntry) GroupStr(code int) (string, bool) {
	for _, t := range e.Rest {
		if t.Code == code {
			return t.Value, true
		}
	}
	return "", false
}

// SetGroup replaces the value of the first tag with the given code, or
// appends one.
func (e *TableEntry) SetGroup(code int, value string) {
	for i := range e.Rest {
		if e.Rest[i].Code == code {
			e.Rest[i].Value = value
			return
		}
	}
	e.Rest = append(e.Rest, Tag{Code: code, Value: value})
}

// --- LAYER convenience accessors -------------------------------------------

// LayerFlags returns group 70 (frozen/locked bit field).
func (e *TableEntry) LayerFlags() int64 {
	v, _ := e.GroupInt(70)
	return v
}

// Frozen reports the layer-frozen bit.
func (e *TableEntry) Frozen() bool { return e.LayerFlags()&1 != 0 }

// Locked reports the layer-locked bit.
func (e *TableEntry) Locked() bool { return e.LayerFlags()&4 != 0 }

// ColorACI returns group 62 (negative = layer off).
func (e *TableEntry) ColorACI() int16 {
	v, _ := e.GroupInt(62)
	return int16(v)
}

// Linetype returns group 6.
func (e *TableEntry) Linetype() string {
	v, _ := e.GroupStr(6)
	return v
}

// --- writer ----------------------------------------------------------------

func (w *TagWriter) writeTable(t *Table, ver DxfVersion) {
	sub := ver.SupportsSubclassMarkers()
	// Canonical TABLE header order: the primary name field (group 2)
	// precedes the handle/owner/subclass block — readers take the table
	// name from the base class.
	w.Str(0, "TABLE")
	w.Str(2, t.Name)
	if ver.SupportsHandles() && t.Handle != 0 {
		w.Handle(5, t.Handle)
	}
	if ver.SupportsHandles() && t.Owner != 0 {
		w.Handle(330, t.Owner)
	}
	// Preserved table-level tags (typically the 100 AcDbSymbolTable
	// marker read from a foreign file) go out here; a fresh table gets
	// the canonical marker only when none was preserved.
	hasMarker := false
	for _, x := range t.Extra {
		if x.Code == 100 {
			hasMarker = true
			break
		}
	}
	if sub && !hasMarker {
		w.Subclass("AcDbSymbolTable")
	}
	w.Pairs(t.Extra)
	w.Int(70, t.Flags)
	for _, e := range t.Entries {
		w.writeTableEntry(e, ver)
	}
	w.Str(0, "ENDTAB")
}

func (w *TagWriter) writeTableEntry(e *TableEntry, ver DxfVersion) {
	w.Str(0, e.Type)
	if ver.SupportsHandles() && e.Handle != 0 {
		w.Handle(5, e.Handle)
	}
	if ver.SupportsHandles() && e.HasOwner {
		w.Handle(330, e.Owner)
	}
	w.Pairs(e.Rest)
}
