// Package dxf reads and writes AutoCAD DXF drawings — the ASCII (and binary)
// companion format to DWG. DXF is a published, open specification: a file is a
// flat stream of (group code, value) pairs organized into named sections
// (HEADER, CLASSES, TABLES, BLOCKS, ENTITIES, OBJECTS).
//
// The package is symmetric: Document parses a tag stream into typed entities,
// tables and header variables, and writes the same model back out. Content
// this package does not model is preserved verbatim (entity Extra tags,
// UnknownEntity / generic TableEntry / generic Object), so a read→write round
// trip never silently drops data.
package dxf

// DxfVersion targets a DXF dialect. The $ACADVER magic gates which group
// codes are legal — most importantly the `100 AcDb*` subclass markers,
// introduced at R13 and rejected by R12 readers.
type DxfVersion uint8

const (
	// R12 (AC1009) — last pre-subclass release; no `100 AcDb*` markers,
	// no OBJECTS/CLASSES sections, no handles (group 5) in practice.
	R12 DxfVersion = iota
	// R14 (AC1014) — first R13+ marker dialect in wide use.
	R14
	// R2000 (AC1015) — handles, CLASSES/OBJECTS, extension dictionaries.
	R2000
	// R2004 (AC1018).
	R2004
	// R2007 (AC1021) — UTF-8 text encoding.
	R2007
	// R2010 (AC1024).
	R2010
	// R2013 (AC1027).
	R2013
	// R2018 (AC1032) — current default; widest reader acceptance.
	R2018
)

// DefaultVersion is the dialect NewDocument and FromDwg target when the
// caller does not choose one.
const DefaultVersion = R2018

var dxfVersionAcadver = [...]string{
	R12:   "AC1009",
	R14:   "AC1014",
	R2000: "AC1015",
	R2004: "AC1018",
	R2007: "AC1021",
	R2010: "AC1024",
	R2013: "AC1027",
	R2018: "AC1032",
}

var dxfVersionLabel = [...]string{
	R12:   "R12",
	R14:   "R14",
	R2000: "R2000",
	R2004: "R2004",
	R2007: "R2007",
	R2010: "R2010",
	R2013: "R2013",
	R2018: "R2018",
}

// AcadVer returns the $ACADVER header string canonical for this target.
func (v DxfVersion) AcadVer() string {
	if int(v) >= len(dxfVersionAcadver) {
		return ""
	}
	return dxfVersionAcadver[v]
}

// String returns the short release label ("R12", "R2000", ...).
func (v DxfVersion) String() string {
	if int(v) >= len(dxfVersionLabel) {
		return "DXF?"
	}
	return dxfVersionLabel[v]
}

// SupportsSubclassMarkers reports whether this dialect understands
// `100 AcDb*` subclass markers. R12 predates them and rejects files
// that contain them.
func (v DxfVersion) SupportsSubclassMarkers() bool { return v != R12 }

// SupportsHandles reports whether entities/table entries carry group-5
// handles and owner references (group 330). R12 predates both.
func (v DxfVersion) SupportsHandles() bool { return v >= R14 }

// SupportsObjectsSection reports whether the OBJECTS section exists in this
// dialect (an R2000+ concept per the DXF reference).
func (v DxfVersion) SupportsObjectsSection() bool { return v >= R2000 }

// ParseDxfVersion resolves a case-insensitive release label ("R12", "R2000",
// ...) or $ACADVER magic ("AC1009", ...) to a DxfVersion.
func ParseDxfVersion(s string) (DxfVersion, bool) {
	switch trimUpper(s) {
	case "R12", "AC1009":
		return R12, true
	case "R13", "AC1012":
		// AC1012 files are read as R14: the R13/R14 dialects share the
		// same group-code surface and no writer targets R13 directly.
		return R14, true
	case "R14", "AC1014":
		return R14, true
	case "R2000", "AC1015":
		return R2000, true
	case "R2004", "AC1018":
		return R2004, true
	case "R2007", "AC1021":
		return R2007, true
	case "R2010", "AC1024":
		return R2010, true
	case "R2013", "AC1027":
		return R2013, true
	case "R2018", "AC1032":
		return R2018, true
	}
	return 0, false
}

// DxfVersionFromAcadVer resolves a $ACADVER magic string. Unknown magics
// fall back to the widest modern dialect, mirroring what readers do in
// practice.
func DxfVersionFromAcadVer(acadver string) DxfVersion {
	if v, ok := ParseDxfVersion(acadver); ok {
		return v
	}
	return DefaultVersion
}

func trimUpper(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' {
			continue
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}
