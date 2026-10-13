package dwg

// Shared plumbing for the non-entity object decoders — one field list
// serving both the pre-R2007 inline layout and the R2007+ split-stream
// layout (spec §19.1 + §19.4.2).

const maxObjectEEDIterations = 256

// objectStream is one non-entity object's data cursor plus the source its TV
// fields come from.
type objectStream struct {
	data            *BitCursor
	strings         *stringReader // nil on the inline layout
	dataEnd         int
	hasDataEnd      bool
	numReactors     uint32
	hasDSBinaryData bool
}

// finish verifies the data cursor consumed exactly the data stream.
func (o *objectStream) finish(what string) error {
	if !o.hasDataEnd {
		return nil
	}
	at := o.data.PositionBits()
	if at == o.dataEnd {
		return nil
	}
	return newError(ErrSectionMap, "%s data fields ended at bit %d, data stream ends at %d (delta %d)",
		what, at, o.dataEnd, at-o.dataEnd)
}

// readTV reads a TV from whichever stream holds it.
func (o *objectStream) readTV(version Version) (string, error) {
	if o.strings != nil {
		return o.strings.readTV()
	}
	return readTV(o.data, version)
}

type commonObjectPrefix struct {
	numReactors     uint32
	hasDSBinaryData bool
	objSizeBits     *uint32
}

// readCommonObjectPrefix consumes the common object data of §19.4.2 / §20.1.
func readCommonObjectPrefix(c *BitCursor, version Version) (commonObjectPrefix, error) {
	for i := 0; i < maxObjectEEDIterations; i++ {
		size, err := c.ReadBSU()
		if err != nil {
			return commonObjectPrefix{}, err
		}
		if size == 0 {
			var objSize *uint32
			if version == R14 {
				v, err := c.ReadRL()
				if err != nil {
					return commonObjectPrefix{}, err
				}
				objSize = &v
			}
			reactors, err := c.ReadBL()
			if err != nil {
				return commonObjectPrefix{}, err
			}
			if version.IsR2004Plus() {
				if _, err := c.ReadB(); err != nil {
					return commonObjectPrefix{}, err
				}
			}
			hasDS := false
			if version == R2013 || version == R2018 {
				b, err := c.ReadB()
				if err != nil {
					return commonObjectPrefix{}, err
				}
				hasDS = b
			}
			return commonObjectPrefix{numReactors: uint32(reactors), hasDSBinaryData: hasDS, objSizeBits: objSize}, nil
		}
		if _, err := c.ReadHandle(); err != nil {
			return commonObjectPrefix{}, err
		}
		for j := uint16(0); j < size; j++ {
			if _, err := c.ReadRC(); err != nil {
				return commonObjectPrefix{}, err
			}
		}
	}
	return commonObjectPrefix{}, newError(ErrSectionMap, "common object EED chain exceeded %d sub-records", maxObjectEEDIterations)
}

// openObjectStream opens the streams of one non-entity object. bodyStart is
// the bit just past the object header; inlineDataEnd is the RL object size
// (used on the inline layout).
func openObjectStream(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (*objectStream, error) {
	var strings *stringReader
	var dataEnd int
	hasDataEnd := false
	if version.IsR2007Plus() {
		if s, ok := LocateStringStream(payload, version); ok {
			r, err := newStringReader(payload, s)
			if err != nil {
				return nil, err
			}
			strings = r
			dataEnd = s.StartBit
			hasDataEnd = true
		} else {
			end, ok := DataFieldEnd(payload, version)
			if !ok {
				return nil, newError(ErrSectionMap, "object has no R2007+ data/handle stream split")
			}
			strings = emptyStringReader(payload)
			dataEnd = end
			hasDataEnd = true
		}
	} else if inlineDataEnd != nil {
		dataEnd = int(*inlineDataEnd)
		hasDataEnd = true
	}

	data := NewBitCursor(payload)
	if !seekBits(data, bodyStart) {
		return nil, newError(ErrSectionMap, "object body start past payload")
	}
	prefix, err := readCommonObjectPrefix(data, version)
	if err != nil {
		return nil, err
	}
	if !hasDataEnd && prefix.objSizeBits != nil {
		dataEnd = int(*prefix.objSizeBits)
		hasDataEnd = true
	}
	return &objectStream{
		data:            data,
		strings:         strings,
		dataEnd:         dataEnd,
		hasDataEnd:      hasDataEnd,
		numReactors:     prefix.numReactors,
		hasDSBinaryData: prefix.hasDSBinaryData,
	}, nil
}
