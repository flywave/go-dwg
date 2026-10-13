package dwg

// Symbol-table entry decoders (spec §19.5.x / §20.4.x). Table entries share a
// common header (name + xref bits); per-kind decoders append their trailers.

// TableEntryHeader is shared by every symbol-table entry.
type TableEntryHeader struct {
	Name            string
	IsXrefDependent bool
	XrefIndexPlus1  int16
	IsXrefResolved  bool
}

// readTableEntryHeader reads the inline name / xref header shared by every
// table entry.
func readTableEntryHeader(c *BitCursor, version Version) (TableEntryHeader, error) {
	name, err := readTV(c, version)
	if err != nil {
		return TableEntryHeader{}, err
	}
	xdep, err := c.ReadB()
	if err != nil {
		return TableEntryHeader{}, err
	}
	xrefIdx, err := c.ReadBS()
	if err != nil {
		return TableEntryHeader{}, err
	}
	xrefResolved, err := c.ReadB()
	if err != nil {
		return TableEntryHeader{}, err
	}
	return TableEntryHeader{Name: name, IsXrefDependent: xdep, XrefIndexPlus1: xrefIdx, IsXrefResolved: xrefResolved}, nil
}

// readCMCFull reads the full R2004+ CMC color form used by R2007+ VIEW/VPORT/
// LAYER: BS index, BL true-color word, RC color byte (spec §2.11).
func readCMCFull(c *BitCursor) (uint16, uint32, uint8, error) {
	index, err := c.ReadBSU()
	if err != nil {
		return 0, 0, 0, err
	}
	rgb, err := c.ReadBLU()
	if err != nil {
		return 0, 0, 0, err
	}
	cb, err := c.ReadRC()
	if err != nil {
		return 0, 0, 0, err
	}
	return index, rgb, cb, nil
}

// readCMCInline reads an R2004+ inline CMC (spec §2.11): BS index, BL RGB, RC
// color byte, plus optional color name and book name. Returns (index, rgb,
// colorByte). The index falls back to the ACI encoded in the RGB word.
func readCMCInline(c *BitCursor) (int16, uint32, uint8, error) {
	index, err := c.ReadBS()
	if err != nil {
		return 0, 0, 0, err
	}
	rgb, err := c.ReadBLU()
	if err != nil {
		return 0, 0, 0, err
	}
	cb, err := c.ReadRC()
	if err != nil {
		return 0, 0, 0, err
	}
	if cb&0x01 != 0 {
		if _, err := readTV(c, R2004); err != nil {
			return 0, 0, 0, err
		}
	}
	if cb&0x02 != 0 {
		if _, err := readTV(c, R2004); err != nil {
			return 0, 0, 0, err
		}
	}
	if index == 0 {
		index = aciFromCMC(rgb)
	}
	return index, rgb, cb, nil
}

// aciFromCMC extracts an AutoCAD color index from the BL word of a full CMC.
func aciFromCMC(rgb uint32) int16 {
	if rgb>>24 == 0xC3 {
		return int16(rgb & 0xFF)
	}
	return 0
}

// readCMCInlineVersion reads a CMC color in the release's inline form (§2.11):
// a bare BS index up to R2000, or BS + BL + RC (with optional names) from
// R2004. Returns the index.
func readCMCInlineVersion(c *BitCursor, version Version) (int16, error) {
	index, err := c.ReadBS()
	if err != nil {
		return 0, err
	}
	if !version.IsR2004Plus() {
		return index, nil
	}
	if _, err := c.ReadBLU(); err != nil {
		return 0, err
	}
	colorByte, err := c.ReadRC()
	if err != nil {
		return 0, err
	}
	if colorByte&0x01 != 0 {
		if _, err := readTV(c, version); err != nil {
			return 0, err
		}
	}
	if colorByte&0x02 != 0 {
		if _, err := readTV(c, version); err != nil {
			return 0, err
		}
	}
	return index, nil
}

// LAYER `values` word bit meanings (§20.4.54).
const (
	layerValueFrozen          int16 = 0x01
	layerValueOff             int16 = 0x02
	layerValueFrozenInNew     int16 = 0x04
	layerValueLocked          int16 = 0x08
	layerValuePlot            int16 = 0x10
	layerValueLineweightShift uint  = 5
)
