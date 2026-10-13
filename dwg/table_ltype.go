package dwg

// LTYPE table entry (spec §19.5.3 / §20.4.58) — linetype definition.

// LtypeDash is one entry in an LTYPE dash/shape pattern.
type LtypeDash struct {
	Length      float64
	ShapeFlag   int16
	XOffset     float64
	YOffset     float64
	Scale       float64
	Rotation    float64
	ShapeNumber int16
}

// HasText reports SHAPE_FLAG bit 0x02.
func (d *LtypeDash) HasText() bool { return d.ShapeFlag&0x02 != 0 }

// HasShape reports SHAPE_FLAG bit 0x04.
func (d *LtypeDash) HasShape() bool { return d.ShapeFlag&0x04 != 0 }

// LtypeEntry is a decoded LTYPE table entry.
type LtypeEntry struct {
	Header        TableEntryHeader
	Flags         uint8
	UsedCount     int16
	Description   string
	PatternLength float64
	Alignment     uint8
	Dashes        []LtypeDash
}

// ltypeTextAreaBytes is the §20.4.58 `X 9` text area on R2004 and earlier.
const ltypeTextAreaBytes = 256

// maxLtypeDashes caps the dash count per spec §20.4.58.
const maxLtypeDashes = 256

func decodeDashes(c *BitCursor, numDashes int) ([]LtypeDash, bool, error) {
	dashes := make([]LtypeDash, 0, numDashes)
	hasTextArea := false
	for i := 0; i < numDashes; i++ {
		length, err := c.ReadBD()
		if err != nil {
			return nil, false, err
		}
		shapeNumber, err := c.ReadBS()
		if err != nil {
			return nil, false, err
		}
		xOffset, err := c.ReadRD()
		if err != nil {
			return nil, false, err
		}
		yOffset, err := c.ReadRD()
		if err != nil {
			return nil, false, err
		}
		scale, err := c.ReadBD()
		if err != nil {
			return nil, false, err
		}
		rotation, err := c.ReadBD()
		if err != nil {
			return nil, false, err
		}
		shapeFlag, err := c.ReadBS()
		if err != nil {
			return nil, false, err
		}
		if shapeFlag&0x02 != 0 {
			hasTextArea = true
		}
		dashes = append(dashes, LtypeDash{
			Length: length, ShapeFlag: shapeFlag, XOffset: xOffset, YOffset: yOffset,
			Scale: scale, Rotation: rotation, ShapeNumber: shapeNumber,
		})
	}
	return dashes, hasTextArea, nil
}

func decodeLtypeInline(c *BitCursor, version Version) (LtypeEntry, error) {
	header, err := readTableEntryHeader(c, version)
	if err != nil {
		return LtypeEntry{}, err
	}
	description, err := readTV(c, version)
	if err != nil {
		return LtypeEntry{}, err
	}
	patternLength, err := c.ReadBD()
	if err != nil {
		return LtypeEntry{}, err
	}
	alignment, err := c.ReadRC()
	if err != nil {
		return LtypeEntry{}, err
	}
	numDashes, err := c.ReadRC()
	if err != nil {
		return LtypeEntry{}, err
	}
	if int(numDashes) > maxLtypeDashes {
		return LtypeEntry{}, newError(ErrSectionMap, "LTYPE num_dashes %d exceeds spec cap of %d", numDashes, maxLtypeDashes)
	}
	dashes, _, err := decodeDashes(c, int(numDashes))
	if err != nil {
		return LtypeEntry{}, err
	}
	if !version.IsR2007Plus() {
		for i := 0; i < ltypeTextAreaBytes; i++ {
			if _, err := c.ReadRC(); err != nil {
				return LtypeEntry{}, err
			}
		}
	}
	return LtypeEntry{
		Header: header, Description: description, PatternLength: patternLength,
		Alignment: alignment, Dashes: dashes,
	}, nil
}

func decodeLtypeModern(payload []byte, bodyStart int, version Version) (LtypeEntry, error) {
	split, err := openTableEntry(payload, bodyStart, version)
	if err != nil {
		return LtypeEntry{}, err
	}
	flag64, xrefIdx, xdep, err := readEntryFlags(split.data)
	if err != nil {
		return LtypeEntry{}, err
	}
	patternLength, err := split.data.ReadBD()
	if err != nil {
		return LtypeEntry{}, err
	}
	alignment, err := split.data.ReadRC()
	if err != nil {
		return LtypeEntry{}, err
	}
	numDashes, err := split.data.ReadRC()
	if err != nil {
		return LtypeEntry{}, err
	}
	if int(numDashes) > maxLtypeDashes {
		return LtypeEntry{}, newError(ErrSectionMap, "LTYPE num_dashes %d exceeds spec cap of %d", numDashes, maxLtypeDashes)
	}
	dashes, hasTextArea, err := decodeDashes(split.data, int(numDashes))
	if err != nil {
		return LtypeEntry{}, err
	}
	if hasTextArea {
		for i := 0; i < 512; i++ {
			if _, err := split.data.ReadRC(); err != nil {
				return LtypeEntry{}, err
			}
		}
	}
	if err := split.finish("LTYPE"); err != nil {
		return LtypeEntry{}, err
	}
	name, err := split.strings.readTV()
	if err != nil {
		return LtypeEntry{}, err
	}
	description, err := split.strings.readTV()
	if err != nil {
		return LtypeEntry{}, err
	}
	var flags uint8
	if flag64 {
		flags = 64
	}
	return LtypeEntry{
		Header: TableEntryHeader{Name: name, IsXrefDependent: xdep, XrefIndexPlus1: xrefIdx, IsXrefResolved: flag64},
		Flags:  flags, Description: description, PatternLength: patternLength,
		Alignment: alignment, Dashes: dashes,
	}, nil
}
