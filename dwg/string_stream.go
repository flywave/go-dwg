package dwg

import "unicode/utf16"

// R2007+ per-object string stream (spec §19.1 split-stream layout).
//
// Up to R2004 an object's TV fields are inline. From R2007 onward the
// characters move into a separate string stream between the object's data
// stream and its handle stream. The bit at which the data fields end is the
// first bit of that string stream, or — when there are no strings — one bit
// before the handle stream (the "strings present" trailer flag).

// StringStream is the bit range occupied by one object's string stream.
type StringStream struct {
	StartBit int
	EndBit   int
}

// LenBits returns the length of the string data in bits.
func (s StringStream) LenBits() int {
	if s.EndBit < s.StartBit {
		return 0
	}
	return s.EndBit - s.StartBit
}

// LocateStringStream locates the R2007+ string stream inside an object
// payload per §19.1.
func LocateStringStream(payload []byte, version Version) (StringStream, bool) {
	trailerEnd, ok := dataSectionEnd(payload, version)
	if !ok || trailerEnd < 17 {
		return StringStream{}, false
	}
	if b, ok := readBitAt(payload, trailerEnd-1); !ok || !b {
		return StringStream{}, false
	}
	lo, ok := readRSAt(payload, trailerEnd-17)
	if !ok {
		return StringStream{}, false
	}
	var size, endBit int
	if lo&0x8000 != 0 {
		hiAt := trailerEnd - 33
		if hiAt < 0 {
			return StringStream{}, false
		}
		hi, ok := readRSAt(payload, hiAt)
		if !ok {
			return StringStream{}, false
		}
		size = int(lo&0x7FFF) | int(hi)<<15
		endBit = hiAt
	} else {
		size = int(lo)
		endBit = trailerEnd - 17
	}
	if endBit < size {
		return StringStream{}, false
	}
	return StringStream{StartBit: endBit - size, EndBit: endBit}, true
}

// DataFieldEnd returns the bit offset at which an object's data fields must
// end.
func DataFieldEnd(payload []byte, version Version) (int, bool) {
	if s, ok := LocateStringStream(payload, version); ok {
		return s.StartBit, true
	}
	end, ok := dataSectionEnd(payload, version)
	if !ok {
		return 0, false
	}
	if end == 0 {
		return 0, false
	}
	return end - 1, true
}

// dataSectionEnd returns the bit at which the object's data + string area
// ends and its handle stream begins.
func dataSectionEnd(payload []byte, version Version) (int, bool) {
	if version == R2007 {
		bits, ok := readR2007ObjectDataSize(payload)
		if !ok {
			return 0, false
		}
		total := len(payload) * 8
		if int(bits) > total {
			return 0, false
		}
		return int(bits), true
	}
	if !version.IsR2010Plus() {
		return 0, false
	}
	handleBits, ok := readHandleStreamSize(payload)
	if !ok {
		return 0, false
	}
	total := len(payload) * 8
	if handleBits > total {
		return 0, false
	}
	return total - handleBits, true
}

func readR2007ObjectDataSize(payload []byte) (uint32, bool) {
	c := NewBitCursor(payload)
	if _, err := c.ReadBSU(); err != nil {
		return 0, false
	}
	v, err := c.ReadRL()
	if err != nil {
		return 0, false
	}
	return v, true
}

func readHandleStreamSize(payload []byte) (int, bool) {
	var value uint64
	var shift uint
	for i := 0; i < 10; i++ {
		if i >= len(payload) {
			return 0, false
		}
		b := uint64(payload[i])
		value |= (b & 0x7F) << shift
		if b&0x80 == 0 {
			return int(value), true
		}
		shift += 7
		if shift >= 64 {
			return 0, false
		}
	}
	return 0, false
}

func readBitAt(bytes []byte, bit int) (bool, bool) {
	idx := bit / 8
	if bit < 0 || idx >= len(bytes) {
		return false, false
	}
	return (bytes[idx]>>(7-uint(bit%8)))&1 != 0, true
}

func readRSAt(bytes []byte, bit int) (uint16, bool) {
	c := NewBitCursor(bytes)
	if !seekBits(c, bit) {
		return 0, false
	}
	v, err := c.ReadRS()
	if err != nil {
		return 0, false
	}
	return uint16(v), true
}

// seekBits advances a cursor to absolute bit position.
func seekBits(c *BitCursor, bit int) bool {
	for c.PositionBits() < bit {
		if _, err := c.ReadB(); err != nil {
			return false
		}
	}
	return true
}

// stringReader is a sequential reader over one object's string stream
// (§19.1). Each readTV consumes the next length-prefixed UTF-16LE string.
// Once the stream is exhausted it yields empty strings rather than erroring,
// because a field table may list more TV slots than the object wrote.
type stringReader struct {
	cursor *BitCursor
	endBit int
}

// newStringReader opens a reader positioned at the start of stream.
func newStringReader(payload []byte, stream StringStream) (*stringReader, error) {
	c := NewBitCursor(payload)
	if !seekBits(c, stream.StartBit) {
		return nil, newError(ErrSectionMap, "string stream start bit past payload")
	}
	return &stringReader{cursor: c, endBit: stream.EndBit}, nil
}

// emptyStringReader models an object that carries no string stream.
func emptyStringReader(payload []byte) *stringReader {
	return &stringReader{cursor: NewBitCursor(payload), endBit: 0}
}

func (r *stringReader) isExhausted() bool {
	return r.cursor.PositionBits() >= r.endBit
}

func (r *stringReader) readTV() (string, error) {
	if r.isExhausted() {
		return "", nil
	}
	length, err := r.cursor.ReadBSU()
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	if r.cursor.PositionBits()+int(length)*16 > r.endBit {
		return "", newError(ErrSectionMap, "string stream TV of %d units overruns the stream", length)
	}
	units := make([]uint16, 0, length)
	for i := 0; i < int(length); i++ {
		lo, err := r.cursor.ReadRC()
		if err != nil {
			return "", err
		}
		hi, err := r.cursor.ReadRC()
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
