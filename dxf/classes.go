package dxf

// CLASSES section — the R13+ class registry. Typed for the six canonical
// fields; the Extra list preserves any others.

// ClassEntry is one `0 CLASS` record.
type ClassEntry struct {
	CPPClassName string // 1
	AppName      string // 2
	ItemName     string // 3
	ProxyFlags   int64  // 90
	WasZombie    bool   // 280
	IsAnEntity   bool   // 281
	Extra        []Tag
}

func (w *TagWriter) writeClasses(classes []ClassEntry, ver DxfVersion) {
	w.Str(0, "SECTION")
	w.Str(2, "CLASSES")
	for _, c := range classes {
		w.Str(0, "CLASS")
		w.Str(1, c.CPPClassName)
		w.Str(2, c.AppName)
		w.Str(3, c.ItemName)
		if c.ProxyFlags != 0 {
			w.Int(90, c.ProxyFlags)
		}
		w.Bool(280, c.WasZombie)
		w.Bool(281, c.IsAnEntity)
		w.Pairs(c.Extra)
	}
	w.Str(0, "ENDSEC")
}
