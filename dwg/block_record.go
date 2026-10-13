package dwg

// BLOCK_HEADER (BLOCK_RECORD) table entry (spec §20.4.52) — the record for a
// block definition: name, base point, and the handles of its first/last
// entities. On R2007+ the name/xref-path/description live in the string
// stream.

// BlockRecord is a decoded BLOCK_HEADER table entry.
type BlockRecord struct {
	Header              TableEntryHeader
	IsAnonymous         bool
	HasAttribs          bool
	IsXref              bool
	XrefOverlay         bool
	IsLoadedXref        bool
	NumOwnedObjects     uint32
	HasNumOwned         bool
	BasePoint           Point3D
	XrefPath            string
	Description         string
	BlockSentinelHandle uint64
	HasSentinel         bool
}

const (
	maxBlockInsertCount  = 256
	maxBlockPreviewBytes = 1_000_000
)

func readInsertCount(c *BitCursor) error {
	for i := 0; i <= 256; i++ {
		b, err := c.ReadRC()
		if err != nil {
			return err
		}
		if b == 0 {
			return nil
		}
	}
	return nil
}

func readPreview(c *BitCursor, capBytes int) error {
	previewBytes, err := c.ReadBLU()
	if err != nil {
		return err
	}
	if int(previewBytes) > capBytes {
		return newError(ErrSectionMap, "BLOCK_HEADER preview data claims %d bytes (>%d cap)", previewBytes, capBytes)
	}
	for i := uint32(0); i < previewBytes; i++ {
		if _, err := c.ReadRC(); err != nil {
			return err
		}
	}
	return nil
}

func decodeBlockRecordInline(c *BitCursor, version Version) (BlockRecord, error) {
	header, err := readTableEntryHeader(c, version)
	if err != nil {
		return BlockRecord{}, err
	}
	var b BlockRecord
	if b.IsAnonymous, err = c.ReadB(); err != nil {
		return BlockRecord{}, err
	}
	if b.HasAttribs, err = c.ReadB(); err != nil {
		return BlockRecord{}, err
	}
	if b.IsXref, err = c.ReadB(); err != nil {
		return BlockRecord{}, err
	}
	if b.XrefOverlay, err = c.ReadB(); err != nil {
		return BlockRecord{}, err
	}
	if version != R14 {
		if b.IsLoadedXref, err = c.ReadB(); err != nil {
			return BlockRecord{}, err
		}
	}
	if version.IsR2004Plus() {
		n, err := c.ReadBL()
		if err != nil {
			return BlockRecord{}, err
		}
		b.NumOwnedObjects = uint32(n)
		b.HasNumOwned = true
	}
	if b.BasePoint, err = ReadBD3(c); err != nil {
		return BlockRecord{}, err
	}
	if b.XrefPath, err = readTV(c, version); err != nil {
		return BlockRecord{}, err
	}
	if version != R14 {
		if err := readInsertCount(c); err != nil {
			return BlockRecord{}, err
		}
		if b.Description, err = readTV(c, version); err != nil {
			return BlockRecord{}, err
		}
		if err := readPreview(c, maxBlockPreviewBytes); err != nil {
			return BlockRecord{}, err
		}
	}
	if version.IsR2007Plus() {
		if _, err := c.ReadBS(); err != nil { // insert units
			return BlockRecord{}, err
		}
		if _, err := c.ReadB(); err != nil { // explodable
			return BlockRecord{}, err
		}
		if _, err := c.ReadRC(); err != nil { // block scaling
			return BlockRecord{}, err
		}
	}
	b.Header = header
	return b, nil
}

func decodeBlockRecordModern(payload []byte, bodyStart int, version Version) (BlockRecord, error) {
	split, err := openTableEntry(payload, bodyStart, version)
	if err != nil {
		return BlockRecord{}, err
	}
	flag64, xrefIdx, xdep, err := readEntryFlags(split.data)
	if err != nil {
		return BlockRecord{}, err
	}
	var b BlockRecord
	if b.IsAnonymous, err = split.data.ReadB(); err != nil {
		return BlockRecord{}, err
	}
	if b.HasAttribs, err = split.data.ReadB(); err != nil {
		return BlockRecord{}, err
	}
	if b.IsXref, err = split.data.ReadB(); err != nil {
		return BlockRecord{}, err
	}
	if b.XrefOverlay, err = split.data.ReadB(); err != nil {
		return BlockRecord{}, err
	}
	if b.IsLoadedXref, err = split.data.ReadB(); err != nil {
		return BlockRecord{}, err
	}
	if version.IsR2004Plus() {
		n, err := split.data.ReadBL()
		if err != nil {
			return BlockRecord{}, err
		}
		b.NumOwnedObjects = uint32(n)
		b.HasNumOwned = true
	}
	if b.BasePoint, err = ReadBD3(split.data); err != nil {
		return BlockRecord{}, err
	}
	if err := readInsertCount(split.data); err != nil {
		return BlockRecord{}, err
	}
	if err := readPreview(split.data, maxBlockPreviewBytes); err != nil {
		return BlockRecord{}, err
	}
	if _, err := split.data.ReadBS(); err != nil {
		return BlockRecord{}, err
	}
	if _, err := split.data.ReadB(); err != nil {
		return BlockRecord{}, err
	}
	if _, err := split.data.ReadRC(); err != nil {
		return BlockRecord{}, err
	}
	if err := split.finish("BLOCK_HEADER"); err != nil {
		return BlockRecord{}, err
	}
	if b.Header.Name, err = split.strings.readTV(); err != nil {
		return BlockRecord{}, err
	}
	if b.XrefPath, err = split.strings.readTV(); err != nil {
		return BlockRecord{}, err
	}
	if b.Description, err = split.strings.readTV(); err != nil {
		return BlockRecord{}, err
	}
	b.Header.IsXrefDependent = xdep
	b.Header.XrefIndexPlus1 = xrefIdx
	b.Header.IsXrefResolved = flag64
	if h, ok := blockSentinelHandle(payload, version); ok {
		b.BlockSentinelHandle = h
		b.HasSentinel = true
	}
	return b, nil
}

// blockSentinelHandle returns the handle of the BLOCK sentinel entity a
// BLOCK_HEADER's handle stream names, or (0, false).
func blockSentinelHandle(payload []byte, version Version) (uint64, bool) {
	start, ok := dataSectionEnd(payload, version)
	if !ok {
		return 0, false
	}
	return blockSentinelHandleAt(payload, start)
}

func blockSentinelHandleAt(payload []byte, handleStreamStart int) (uint64, bool) {
	c := NewBitCursor(payload)
	if !seekBits(c, handleStreamStart) {
		return 0, false
	}
	sawNull := false
	for i := 0; i < 16; i++ {
		if c.RemainingBits() < 8 {
			return 0, false
		}
		h, err := c.ReadHandle()
		if err != nil {
			return 0, false
		}
		if sawNull {
			if h.IsAbsolute() {
				return h.Value, true
			}
			return 0, false
		}
		sawNull = h.Code == 5 && h.Counter == 0
	}
	return 0, false
}
