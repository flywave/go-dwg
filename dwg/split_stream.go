package dwg

// Shared plumbing for R2007+ split-stream symbol-table decoders (spec §19.1 +
// §20.4.x). The data-stream fields must end exactly where the string stream
// begins; finish() turns any other landing spot into an error.

// splitStream is a table entry's data cursor plus its string-stream reader.
type splitStream struct {
	data        *BitCursor
	strings     *stringReader
	stringStart int
}

// finish verifies the data cursor consumed exactly the data stream.
func (s *splitStream) finish(what string) error {
	at := s.data.PositionBits()
	if at == s.stringStart {
		return nil
	}
	return newError(ErrSectionMap, "%s data fields ended at bit %d, string stream starts at %d (delta %d)",
		what, at, s.stringStart, at-s.stringStart)
}

// openTableEntry opens the split streams of an R2007+ symbol-table entry.
func openTableEntry(payload []byte, bodyStart int, version Version) (*splitStream, error) {
	stream, ok := LocateStringStream(payload, version)
	if !ok {
		return nil, newError(ErrSectionMap, "object has no R2007+ string stream")
	}
	data := NewBitCursor(payload)
	if !seekBits(data, bodyStart) {
		return nil, newError(ErrSectionMap, "table entry body start past payload")
	}
	if err := skipObjectPrefix(data, version); err != nil {
		return nil, err
	}
	reader, err := newStringReader(payload, stream)
	if err != nil {
		return nil, err
	}
	return &splitStream{data: data, strings: reader, stringStart: stream.StartBit}, nil
}

// skipObjectPrefix skips the common object prefix of a table entry: EED
// chain, xdictionary flag (R2004+), binary-data flag (R2013+) plus the one
// RC that follows a set binary-data flag.
func skipObjectPrefix(c *BitCursor, version Version) error {
	for i := 0; i < maxObjectEEDIterations; i++ {
		size, err := c.ReadBSU()
		if err != nil {
			return err
		}
		if size == 0 {
			break
		}
		if _, err := c.ReadHandle(); err != nil {
			return err
		}
		for j := uint16(0); j < size; j++ {
			if _, err := c.ReadRC(); err != nil {
				return err
			}
		}
	}
	if version.IsR2004Plus() {
		if _, err := c.ReadB(); err != nil {
			return err
		}
	}
	if version == R2013 || version == R2018 {
		hasBinary, err := c.ReadB()
		if err != nil {
			return err
		}
		if hasBinary {
			if _, err := c.ReadRC(); err != nil {
				return err
			}
		}
	}
	return nil
}

// readEntryFlags reads the three non-string fields every table entry shares
// once its name has moved to the string stream. Observed order: B, B, BS.
func readEntryFlags(c *BitCursor) (flag64 bool, xrefIndexPlus1 int16, isXrefDependent bool, err error) {
	flag64, err = c.ReadB()
	if err != nil {
		return
	}
	isXrefDependent, err = c.ReadB()
	if err != nil {
		return
	}
	xrefIndexPlus1, err = c.ReadBS()
	return
}
