package dwg

// STYLE table entry (spec §19.5.4 / §20.4.56) — text style (font + size
// policy).

// StyleEntry is a decoded STYLE table entry.
type StyleEntry struct {
	Header          TableEntryHeader
	Flags           uint8
	FixedHeight     float64
	WidthFactor     float64
	ObliqueAngle    float64
	Generation      uint8
	LastHeight      float64
	FontFilename    string
	BigfontFilename string
}

// IsShapeFile reports flags bit 0x01.
func (s *StyleEntry) IsShapeFile() bool { return s.Flags&0x01 != 0 }

// IsVertical reports flags bit 0x04.
func (s *StyleEntry) IsVertical() bool { return s.Flags&0x04 != 0 }

// IsXrefDependent reports flags bit 0x08.
func (s *StyleEntry) IsXrefDependent() bool { return s.Flags&0x08 != 0 }

func decodeStyleInline(c *BitCursor, version Version) (StyleEntry, error) {
	header, err := readTableEntryHeader(c, version)
	if err != nil {
		return StyleEntry{}, err
	}
	var flags uint8
	if b, err := c.ReadB(); err != nil {
		return StyleEntry{}, err
	} else if b {
		flags |= 0x01
	}
	if b, err := c.ReadB(); err != nil {
		return StyleEntry{}, err
	} else if b {
		flags |= 0x04
	}
	if header.IsXrefDependent {
		flags |= 0x08
	}
	fixedHeight, err := c.ReadBD()
	if err != nil {
		return StyleEntry{}, err
	}
	widthFactor, err := c.ReadBD()
	if err != nil {
		return StyleEntry{}, err
	}
	oblique, err := c.ReadBD()
	if err != nil {
		return StyleEntry{}, err
	}
	generation, err := c.ReadRC()
	if err != nil {
		return StyleEntry{}, err
	}
	lastHeight, err := c.ReadBD()
	if err != nil {
		return StyleEntry{}, err
	}
	font, err := readTV(c, version)
	if err != nil {
		return StyleEntry{}, err
	}
	bigfont, err := readTV(c, version)
	if err != nil {
		return StyleEntry{}, err
	}
	return StyleEntry{Header: header, Flags: flags, FixedHeight: fixedHeight, WidthFactor: widthFactor,
		ObliqueAngle: oblique, Generation: generation, LastHeight: lastHeight,
		FontFilename: font, BigfontFilename: bigfont}, nil
}

func decodeStyleModern(payload []byte, bodyStart int, version Version) (StyleEntry, error) {
	split, err := openTableEntry(payload, bodyStart, version)
	if err != nil {
		return StyleEntry{}, err
	}
	flag64, xrefIdx, xdep, err := readEntryFlags(split.data)
	if err != nil {
		return StyleEntry{}, err
	}
	isShapeFile, err := split.data.ReadB()
	if err != nil {
		return StyleEntry{}, err
	}
	isVertical, err := split.data.ReadB()
	if err != nil {
		return StyleEntry{}, err
	}
	fixedHeight, err := split.data.ReadBD()
	if err != nil {
		return StyleEntry{}, err
	}
	widthFactor, err := split.data.ReadBD()
	if err != nil {
		return StyleEntry{}, err
	}
	oblique, err := split.data.ReadBD()
	if err != nil {
		return StyleEntry{}, err
	}
	generation, err := split.data.ReadRC()
	if err != nil {
		return StyleEntry{}, err
	}
	lastHeight, err := split.data.ReadBD()
	if err != nil {
		return StyleEntry{}, err
	}
	if err := split.finish("STYLE"); err != nil {
		return StyleEntry{}, err
	}
	name, err := split.strings.readTV()
	if err != nil {
		return StyleEntry{}, err
	}
	font, err := split.strings.readTV()
	if err != nil {
		return StyleEntry{}, err
	}
	bigfont, err := split.strings.readTV()
	if err != nil {
		return StyleEntry{}, err
	}
	var flags uint8
	if isShapeFile {
		flags |= 0x01
	}
	if isVertical {
		flags |= 0x04
	}
	if xdep {
		flags |= 0x08
	}
	return StyleEntry{
		Header: TableEntryHeader{Name: name, IsXrefDependent: xdep, XrefIndexPlus1: xrefIdx, IsXrefResolved: flag64},
		Flags:  flags, FixedHeight: fixedHeight, WidthFactor: widthFactor, ObliqueAngle: oblique,
		Generation: generation, LastHeight: lastHeight, FontFilename: font, BigfontFilename: bigfont,
	}, nil
}
