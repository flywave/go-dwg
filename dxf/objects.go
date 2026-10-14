package dxf

// OBJECTS section — non-entity, non-table records (DICTIONARY, XRECORD,
// GROUP, LAYOUT, …). Kept generic: the full tag run is preserved and typed
// views (Dictionary view below) parse on demand.

// Object is one OBJECTS-section record. Tags holds everything after the
// `0 <TYPE>` line, verbatim.
type Object struct {
	Type string
	Tags []Tag
}

// Handle returns the record's handle (group 5).
func (o *Object) Handle() uint64 {
	for _, t := range o.Tags {
		if t.Code == 5 {
			return parseHandle(t.Value)
		}
	}
	return 0
}

// Owner returns the owner handle (first group 330).
func (o *Object) Owner() uint64 {
	for _, t := range o.Tags {
		if t.Code == 330 {
			return parseHandle(t.Value)
		}
	}
	return 0
}

// GroupInt scans the record for the first tag with the given code.
func (o *Object) GroupInt(code int) (int64, bool) {
	for _, t := range o.Tags {
		if t.Code == code {
			return t.Int(), true
		}
	}
	return 0, false
}

// DictEntry is one name→handle pair of a DICTIONARY.
type DictEntry struct {
	Name   string
	Handle uint64
}

// Dictionary parses the record as an ACAD dictionary (groups 3/350 pairs).
// Returns nil when the record is not a DICTIONARY.
func (o *Object) Dictionary() []DictEntry {
	if o.Type != "DICTIONARY" {
		return nil
	}
	var out []DictEntry
	var pending string
	for _, t := range o.Tags {
		switch t.Code {
		case 3:
			pending = t.Value
		case 350:
			out = append(out, DictEntry{Name: pending, Handle: parseHandle(t.Value)})
			pending = ""
		}
	}
	return out
}

func (w *TagWriter) writeObjects(objs []*Object, ver DxfVersion) {
	if len(objs) == 0 && !ver.SupportsObjectsSection() {
		return
	}
	w.Str(0, "SECTION")
	w.Str(2, "OBJECTS")
	for _, o := range objs {
		w.Str(0, o.Type)
		w.Pairs(o.Tags)
	}
	w.Str(0, "ENDSEC")
}
