package dwg

import "encoding/binary"

// HeaderVars is the decoded AcDb:Header variable table (spec §19.3). It
// holds the post-sentinel bit-stream bytes; individual variables are
// extracted lazily via accessors.
type HeaderVars struct {
	Sentinel   [16]byte
	SizeInBits uint32
	Body       []byte
	Version    Version
}

// HeaderVarsSentinel is the R2004+ AcDb:Header section sentinel.
var HeaderVarsSentinel = [16]byte{
	0xCF, 0x7B, 0x1F, 0x23, 0xFD, 0xDE, 0x38, 0xA9, 0x5F, 0x7C, 0x68, 0xB8, 0x4E, 0x6D, 0x33, 0x5F,
}

// ParseHeaderVars parses a decompressed AcDb:Header payload in lossy mode
// (tolerates inputs shorter than the 20-byte prefix).
func ParseHeaderVars(bytes []byte, version Version) HeaderVars {
	if len(bytes) < 20 {
		return HeaderVars{Version: version}
	}
	var hv HeaderVars
	copy(hv.Sentinel[:], bytes[:16])
	hv.SizeInBits = binary.LittleEndian.Uint32(bytes[16:20])
	hv.Body = append([]byte(nil), bytes[20:]...)
	hv.Version = version
	return hv
}

// HasValidSentinel reports whether the leading sentinel matches the
// expected AcDb:Header value.
func (h *HeaderVars) HasValidSentinel() bool { return h.Sentinel == HeaderVarsSentinel }

// BodyLen returns the length of the bit-stream body.
func (h *HeaderVars) BodyLen() int { return len(h.Body) }

// ReadFirstBD reads the first BD in the header bit-stream.
func (h *HeaderVars) ReadFirstBD() (float64, bool) {
	c := NewBitCursor(h.Body)
	v, err := c.ReadBD()
	return v, err == nil
}
