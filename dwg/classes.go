package dwg

import "unicode/utf16"

// ClassDef is one custom class definition from AcDb:Classes.
type ClassDef struct {
	ClassNumber        uint16
	Version            int16
	AppName            string
	CppClassName       string
	DxfClassName       string
	WasAProxy          bool
	ItemClassID        uint16
	NumObjects         uint32
	DwgVersion         uint32
	MaintenanceVersion uint32
}

// ClassMap is a parsed custom class table.
type ClassMap struct {
	MaxClassNumber uint32
	Classes        []ClassDef
}

// FirstCustomClassNumber is the lowest object type code the class table
// can describe (spec §20.3).
const FirstCustomClassNumber = 500

const maxClasses = 4096

// ClassMapSentinel is the R2004+ AcDb:Classes section sentinel.
var ClassMapSentinel = [16]byte{
	0x8D, 0xA1, 0xC4, 0xB8, 0xC4, 0xA9, 0xF8, 0xC5, 0xC0, 0xDC, 0xF4, 0x5F, 0xE7, 0xCF, 0xB6, 0x8A,
}

// ParseClassMap parses a decompressed AcDb:Classes payload. On malformed
// structure it returns an empty map rather than failing (the class table
// is advisory).
func ParseClassMap(bytes []byte, version Version) ClassMap {
	bitStart := classBitStreamStart(version)
	if len(bytes) < bitStart+4 {
		return ClassMap{}
	}
	var declaredMax uint32
	hasDeclared := false
	c := NewBitCursor(bytes[bitStart:])
	if version.IsR2004Plus() {
		v, err := c.ReadBLU()
		if err != nil {
			return ClassMap{}
		}
		declaredMax = v
		hasDeclared = true
		if _, err := c.ReadB(); err != nil {
			return ClassMap{}
		}
	}
	sizeBytes := int(uint32(bytes[16]) | uint32(bytes[17])<<8 | uint32(bytes[18])<<16 | uint32(bytes[19])<<24)
	maxBits := sizeBytes * 8
	if avail := (len(bytes) - bitStart) * 8; avail < maxBits {
		maxBits = avail
	}

	var classes []ClassDef
	if version.IsR2007Plus() {
		expected := 0
		if int(declaredMax) > FirstCustomClassNumber {
			expected = int(declaredMax) - FirstCustomClassNumber + 1
		} else if declaredMax == FirstCustomClassNumber {
			expected = 1
		}
		if expected > maxClasses {
			expected = maxClasses
		}
		classes = readSplitStreamClasses(c, expected)
	} else {
		classes = readInlineClasses(c, version, maxBits)
	}

	consecutive := true
	for i, d := range classes {
		if int(d.ClassNumber) != FirstCustomClassNumber+i {
			consecutive = false
			break
		}
	}
	maxClassNumber := declaredMax
	if !hasDeclared {
		if len(classes) == 0 {
			maxClassNumber = 0
		} else {
			maxClassNumber = uint32(FirstCustomClassNumber + len(classes) - 1)
		}
	}
	cm := ClassMap{MaxClassNumber: maxClassNumber}
	if consecutive {
		cm.Classes = classes
	}
	return cm
}

// ByTypeCode looks up a class by its type code.
func (m *ClassMap) ByTypeCode(typeCode uint16) *ClassDef {
	for i := range m.Classes {
		if m.Classes[i].ClassNumber == typeCode {
			return &m.Classes[i]
		}
	}
	return nil
}

func classBitStreamStart(version Version) int {
	switch version {
	case R14, R2000, R2004:
		return 20
	case R2007:
		return 24
	default:
		return 28
	}
}

func readInlineClasses(c *BitCursor, version Version, maxBits int) []ClassDef {
	var classes []ClassDef
	for c.PositionBits() < maxBits && c.RemainingBits() >= 64 && len(classes) < maxClasses {
		def, ok := readInlineClassRecord(c, version)
		if !ok {
			break
		}
		if c.PositionBits() > maxBits {
			break
		}
		classes = append(classes, def)
	}
	return classes
}

func readInlineClassRecord(c *BitCursor, version Version) (ClassDef, bool) {
	classNumber, err := c.ReadBSU()
	if err != nil {
		return ClassDef{}, false
	}
	versionFlag, err := c.ReadBS()
	if err != nil {
		return ClassDef{}, false
	}
	appName, err := readTV(c, version)
	if err != nil {
		return ClassDef{}, false
	}
	cppName, err := readTV(c, version)
	if err != nil {
		return ClassDef{}, false
	}
	dxfName, err := readTV(c, version)
	if err != nil {
		return ClassDef{}, false
	}
	wasProxy, err := c.ReadB()
	if err != nil {
		return ClassDef{}, false
	}
	itemClassID, err := c.ReadBSU()
	if err != nil {
		return ClassDef{}, false
	}
	var tail recordTail
	if version.IsR2004Plus() {
		tail, err = readR2004RecordTail(c)
		if err != nil {
			return ClassDef{}, false
		}
	}
	if appName == "" && cppName == "" && dxfName == "" {
		return ClassDef{}, false
	}
	return ClassDef{
		ClassNumber:        classNumber,
		Version:            versionFlag,
		AppName:            appName,
		CppClassName:       cppName,
		DxfClassName:       dxfName,
		WasAProxy:          wasProxy,
		ItemClassID:        itemClassID,
		NumObjects:         tail.numObjects,
		DwgVersion:         tail.dwgVersion,
		MaintenanceVersion: tail.maintenanceVersion,
	}, true
}

func readSplitStreamClasses(c *BitCursor, expected int) []ClassDef {
	classes := make([]ClassDef, 0, min(expected, 64))
	for i := 0; i < expected; i++ {
		def, ok := readSplitStreamClassRecord(c)
		if !ok {
			return nil
		}
		classes = append(classes, def)
	}
	for i := range classes {
		app, err1 := readTV(c, R2018)
		cpp, err2 := readTV(c, R2018)
		dxf, err3 := readTV(c, R2018)
		if err1 != nil || err2 != nil || err3 != nil {
			return nil
		}
		classes[i].AppName = app
		classes[i].CppClassName = cpp
		classes[i].DxfClassName = dxf
	}
	for i := range classes {
		if classes[i].DxfClassName == "" {
			return nil
		}
	}
	return classes
}

func readSplitStreamClassRecord(c *BitCursor) (ClassDef, bool) {
	classNumber, err := c.ReadBSU()
	if err != nil {
		return ClassDef{}, false
	}
	versionFlag, err := c.ReadBS()
	if err != nil {
		return ClassDef{}, false
	}
	wasProxy, err := c.ReadB()
	if err != nil {
		return ClassDef{}, false
	}
	itemClassID, err := c.ReadBSU()
	if err != nil {
		return ClassDef{}, false
	}
	tail, err := readR2004RecordTail(c)
	if err != nil {
		return ClassDef{}, false
	}
	return ClassDef{
		ClassNumber:        classNumber,
		Version:            versionFlag,
		WasAProxy:          wasProxy,
		ItemClassID:        itemClassID,
		NumObjects:         tail.numObjects,
		DwgVersion:         tail.dwgVersion,
		MaintenanceVersion: tail.maintenanceVersion,
	}, true
}

type recordTail struct {
	numObjects         uint32
	dwgVersion         uint32
	maintenanceVersion uint32
}

func readR2004RecordTail(c *BitCursor) (recordTail, error) {
	numObjects, err := c.ReadBLU()
	if err != nil {
		return recordTail{}, err
	}
	dwgVersion, err := c.ReadBLU()
	if err != nil {
		return recordTail{}, err
	}
	maintenanceVersion, err := c.ReadBLU()
	if err != nil {
		return recordTail{}, err
	}
	if _, err := c.ReadBL(); err != nil {
		return recordTail{}, err
	}
	if _, err := c.ReadBL(); err != nil {
		return recordTail{}, err
	}
	return recordTail{numObjects: numObjects, dwgVersion: dwgVersion, maintenanceVersion: maintenanceVersion}, nil
}

// readTV reads a TV (variable text) field: a BS length followed by that
// many UTF-16LE code units (R2007+) or raw bytes. A trailing NUL unit is
// stripped.
func readTV(c *BitCursor, version Version) (string, error) {
	length, err := c.ReadBSU()
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	if version.IsR2007Plus() {
		units := make([]uint16, 0, length)
		for i := 0; i < int(length); i++ {
			lo, err := c.ReadRC()
			if err != nil {
				return "", err
			}
			hi, err := c.ReadRC()
			if err != nil {
				return "", err
			}
			units = append(units, uint16(hi)<<8|uint16(lo))
		}
		if len(units) > 0 && units[len(units)-1] == 0 {
			units = units[:len(units)-1]
		}
		return string(utf16.Decode(units)), nil
	}
	bs := make([]byte, 0, length)
	for i := 0; i < int(length); i++ {
		b, err := c.ReadRC()
		if err != nil {
			return "", err
		}
		bs = append(bs, b)
	}
	if len(bs) > 0 && bs[len(bs)-1] == 0 {
		bs = bs[:len(bs)-1]
	}
	return string(bs), nil
}
