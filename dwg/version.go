package dwg

import "fmt"

// Version is a DWG file format version, identified by the 6-byte ASCII
// magic at offset 0.
type Version uint8

const (
	// R14 is AC1014 — AutoCAD R14 (1997).
	R14 Version = iota
	// R2000 is AC1015 — AutoCAD 2000 / 2000i / 2002.
	R2000
	// R2004 is AC1018 — AutoCAD 2004 / 2005 / 2006.
	R2004
	// R2007 is AC1021 — AutoCAD 2007 / 2008 / 2009.
	R2007
	// R2010 is AC1024 — AutoCAD 2010 / 2011 / 2012.
	R2010
	// R2013 is AC1027 — AutoCAD 2013 -> 2017.
	R2013
	// R2018 is AC1032 — AutoCAD 2018 onward.
	R2018
)

// versionMagic maps each version to its on-disk 6-byte magic.
var versionMagic = [7]string{
	"AC1014", "AC1015", "AC1018", "AC1021", "AC1024", "AC1027", "AC1032",
}

var versionRelease = [7]string{
	"R14", "2000", "2004", "2007", "2010", "2013", "2018",
}

var versionYear = [7]uint16{1997, 1999, 2003, 2006, 2009, 2012, 2017}

// VersionFromMagic parses the 6-byte magic. It returns ErrNotDwg when
// the first two bytes are not "AC", or ErrUnsupportedVersion when the
// numeric suffix is not one this package understands.
func VersionFromMagic(magic []byte) (Version, error) {
	if len(magic) < 6 {
		return 0, newError(ErrTruncated, "version magic needs 6 bytes, got %d", len(magic))
	}
	if magic[0] != 'A' || magic[1] != 'C' {
		return 0, newError(ErrNotDwg, "expected ASCII magic \"AC10xx\" at offset 0, got %q", magic[:6])
	}
	s := string(magic[:6])
	for i, m := range versionMagic {
		if m == s {
			return Version(i), nil
		}
	}
	return 0, newError(ErrUnsupportedVersion, "unsupported DWG version %q (known: AC1014 .. AC1032)", s)
}

// Magic returns the exact 6-byte magic string as it appears on disk.
func (v Version) Magic() string { return versionMagic[v] }

// Release returns the human-readable release label ("R14", "2000", ...).
func (v Version) Release() string { return versionRelease[v] }

// YearIntroduced returns the approximate first-ship year of the first
// AutoCAD release using this format.
func (v Version) YearIntroduced() uint16 { return versionYear[v] }

// IsR2004Plus reports R2004 and later (AC1018 onward).
func (v Version) IsR2004Plus() bool {
	return v == R2004 || v == R2007 || v == R2010 || v == R2013 || v == R2018
}

// IsR13R15 reports the R13-R15 (pre-R2004) simple header format.
func (v Version) IsR13R15() bool { return v == R14 || v == R2000 }

// IsR2004Family reports versions sharing the R2004 encrypted-header
// layout. R2007 is NOT in this set (spec §5 gives it its own layout).
func (v Version) IsR2004Family() bool {
	return v == R2004 || v == R2010 || v == R2013 || v == R2018
}

// IsR2007 reports the R2007-specific container format.
func (v Version) IsR2007() bool { return v == R2007 }

// IsR2007Plus reports the release line from AC1021 onward.
func (v Version) IsR2007Plus() bool {
	return v == R2007 || v == R2010 || v == R2013 || v == R2018
}

// UsesUTF16Text reports formats that store variable-text (TV) fields as
// UTF-16LE bit-streams rather than 8-bit bytes.
func (v Version) UsesUTF16Text() bool { return v.IsR2007Plus() }

// IsR2010Plus reports R2010+ (object-type encoding changed, spec §2.12).
func (v Version) IsR2010Plus() bool {
	return v == R2010 || v == R2013 || v == R2018
}

// HasObjectSizeField reports whether an object record carries the RL
// "object data size in bits" field between the object type and handle
// (R2000..R2007).
func (v Version) HasObjectSizeField() bool {
	return v == R2000 || v == R2004 || v == R2007
}

func (v Version) String() string {
	if int(v) >= len(versionMagic) {
		return fmt.Sprintf("Version(%d)", uint8(v))
	}
	return fmt.Sprintf("%s (%s)", v.Magic(), v.Release())
}
