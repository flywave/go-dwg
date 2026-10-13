package dwg

import "encoding/binary"

// HandleEntry is one entry in the parsed handle map — one DWG object.
type HandleEntry struct {
	// Handle is the absolute handle value (monotonically increasing).
	Handle uint64
	// Offset is a byte offset into the AcDb:AcDbObjects decompressed stream.
	Offset uint64
}

// HandleMap is the full handle->offset index for a drawing.
type HandleMap struct {
	Entries []HandleEntry
}

// ParseHandleMap parses a decompressed AcDb:Handles payload.
func ParseHandleMap(bytes []byte) (HandleMap, error) {
	var entries []HandleEntry
	pos := 0
	for pos < len(bytes) {
		if pos+2 > len(bytes) {
			break
		}
		sectionSize := int(binary.BigEndian.Uint16(bytes[pos : pos+2]))
		pos += 2
		if sectionSize == 0 {
			break
		}
		if pos+sectionSize > len(bytes) {
			return HandleMap{}, newError(ErrSectionMap, "AcDb:Handles section at byte %d claims %d bytes, %d remain", pos-2, sectionSize, len(bytes)-pos)
		}
		payloadEnd := pos + sectionSize - 2
		payload := bytes[pos:payloadEnd]
		pos += sectionSize

		var lastHandle uint64
		var lastOffset int64
		cur := NewBitCursor(payload)
		for cur.RemainingBits() >= 8 {
			hDelta, err := readUnsignedMC(cur)
			if err != nil {
				break
			}
			oDelta, err := readSignedMC(cur)
			if err != nil {
				break
			}
			lastHandle += hDelta
			lastOffset += oDelta
			if len(entries) >= MaxHandleEntries {
				return HandleMap{}, newError(ErrSectionMap, "AcDb:Handles parse exceeded MAX_HANDLE_ENTRIES (%d)", MaxHandleEntries)
			}
			entries = append(entries, HandleEntry{Handle: lastHandle, Offset: uint64(lastOffset)})
		}
	}
	return HandleMap{Entries: entries}, nil
}

// OffsetOf looks up an object's offset by handle.
func (m *HandleMap) OffsetOf(handle uint64) (uint64, bool) {
	for _, e := range m.Entries {
		if e.Handle == handle {
			return e.Offset, true
		}
	}
	return 0, false
}

// Len returns the number of entries.
func (m *HandleMap) Len() int { return len(m.Entries) }

// IsEmpty reports whether the map has no entries.
func (m *HandleMap) IsEmpty() bool { return len(m.Entries) == 0 }

// readUnsignedMC reads an unsigned modular char (every byte contributes
// 7 magnitude bits; 0x80 is the only flag).
func readUnsignedMC(r *BitCursor) (uint64, error) {
	var value uint64
	var shift uint
	for {
		b, err := r.ReadRC()
		if err != nil {
			return 0, err
		}
		value |= uint64(b&0x7F) << shift
		shift += 7
		if b&0x80 == 0 {
			return value, nil
		}
		if shift >= 64 {
			return value, nil
		}
	}
}

// readSignedMC reads a signed modular char (0x40 on the terminator
// indicates negation).
func readSignedMC(r *BitCursor) (int64, error) {
	var value uint64
	var shift uint
	negate := false
	for {
		b, err := r.ReadRC()
		if err != nil {
			return 0, err
		}
		cont := b&0x80 != 0
		var data uint64
		if cont {
			data = uint64(b & 0x7F)
		} else {
			data = uint64(b & 0x3F)
		}
		value |= data << shift
		if cont {
			shift += 7
		} else {
			shift += 6
		}
		if !cont {
			negate = b&0x40 != 0
			break
		}
		if shift >= 64 {
			break
		}
	}
	return signedModularCharValue(value, negate)
}
