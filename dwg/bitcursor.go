package dwg

import (
	"encoding/binary"
	"math"
)

// Handle is a DWG handle reference: 4-bit code + 4-bit byte-length
// counter + up to 8 bytes of value (spec §2.13).
type Handle struct {
	Code    uint8
	Counter uint8
	Value   uint64
}

// IsSoftOwner reports relation code 2 (owner does not strictly need the
// target).
func (h Handle) IsSoftOwner() bool { return h.Code == 2 }

// IsHardOwner reports relation code 3 (target cannot exist without its
// owner).
func (h Handle) IsHardOwner() bool { return h.Code == 3 }

// IsAbsolute reports absolute reference codes 2..=5.
func (h Handle) IsAbsolute() bool { return h.Code >= 2 && h.Code <= 5 }

// IsOffset reports relative offset codes 6, 8, A, C.
func (h Handle) IsOffset() bool {
	switch h.Code {
	case 0x6, 0x8, 0xA, 0xC:
		return true
	}
	return false
}

// BitCursor is a bit-level cursor over a byte slice implementing the DWG
// primitive types from ODA Open Design Specification v5.4.1 §2. Bytes
// are consumed most-significant-bit first.
type BitCursor struct {
	bytes   []byte
	bytePos int
	bitPos  uint8 // 0..7, MSB-first
}

// NewBitCursor creates a cursor positioned at the first bit of bytes.
func NewBitCursor(bytes []byte) *BitCursor {
	return &BitCursor{bytes: bytes}
}

// TotalBits is the number of bits in the underlying slice.
func (c *BitCursor) TotalBits() int { return len(c.bytes) * 8 }

// RemainingBits is the number of bits from the current position to the
// end of the slice.
func (c *BitCursor) RemainingBits() int {
	r := c.TotalBits() - (c.bytePos*8 + int(c.bitPos))
	if r < 0 {
		return 0
	}
	return r
}

// PositionBits is the current bit position (0 == start of slice).
func (c *BitCursor) PositionBits() int { return c.bytePos*8 + int(c.bitPos) }

// AlignToByte drops any bits remaining in the current byte.
func (c *BitCursor) AlignToByte() {
	if c.bitPos != 0 {
		c.bytePos++
		c.bitPos = 0
	}
}

func (c *BitCursor) need(bits int) error {
	if c.RemainingBits() < bits {
		return newError(ErrBitsExhausted, "wanted %d bits, %d remain", bits, c.RemainingBits())
	}
	return nil
}

// readBits reads n bits (n <= 64) as an MSB-first big-endian value.
func (c *BitCursor) readBits(n int) (uint64, error) {
	if err := c.need(n); err != nil {
		return 0, err
	}
	var out uint64
	taken := 0
	for taken < n {
		b := c.bytes[c.bytePos]
		avail := 8 - int(c.bitPos)
		take := n - taken
		if take > avail {
			take = avail
		}
		shiftRight := avail - take
		var mask uint8
		if take == 8 {
			mask = 0xFF
		} else {
			mask = uint8((1 << take) - 1)
		}
		chunk := uint64((b >> shiftRight) & mask)
		out = (out << take) | chunk
		c.bitPos += uint8(take)
		if c.bitPos == 8 {
			c.bytePos++
			c.bitPos = 0
		}
		taken += take
	}
	return out, nil
}

// ReadB reads a B (single bit).
func (c *BitCursor) ReadB() (bool, error) {
	v, err := c.readBits(1)
	return v != 0, err
}

// ReadBB reads a BB (2-bit code).
func (c *BitCursor) ReadBB() (uint8, error) {
	v, err := c.readBits(2)
	return uint8(v), err
}

// Read3B reads a 3B (three raw bits, 0..=7).
func (c *BitCursor) Read3B() (uint8, error) {
	v, err := c.readBits(3)
	return uint8(v), err
}

// Read4bits reads a 4BITS field (four raw bits, MSB first, §2.1) — the
// VIEWMODE word of VIEW and VPORT.
func (c *BitCursor) Read4bits() (uint8, error) {
	v, err := c.readBits(4)
	return uint8(v), err
}

// ReadBS reads a BS (bitshort): 00=16-bit LE, 01=8-bit, 10=0, 11=256.
func (c *BitCursor) ReadBS() (int16, error) {
	tag, err := c.ReadBB()
	if err != nil {
		return 0, err
	}
	switch tag {
	case 0b00:
		lsb, err := c.readBits(8)
		if err != nil {
			return 0, err
		}
		msb, err := c.readBits(8)
		if err != nil {
			return 0, err
		}
		return int16(uint16(msb)<<8 | uint16(lsb)), nil
	case 0b01:
		v, err := c.readBits(8)
		return int16(v), err
	case 0b10:
		return 0, nil
	default:
		return 256, nil
	}
}

// ReadBSU reads a BS and reinterprets the bits as unsigned.
func (c *BitCursor) ReadBSU() (uint16, error) {
	v, err := c.ReadBS()
	return uint16(v), err
}

// ReadBL reads a BL (bitlong): 00=32-bit LE, 01=8-bit, 10=0, 11=reserved.
func (c *BitCursor) ReadBL() (int32, error) {
	tag, err := c.ReadBB()
	if err != nil {
		return 0, err
	}
	switch tag {
	case 0b00:
		var v uint32
		for i := 0; i < 4; i++ {
			b, err := c.readBits(8)
			if err != nil {
				return 0, err
			}
			v |= uint32(b) << (8 * i)
		}
		return int32(v), nil
	case 0b01:
		v, err := c.readBits(8)
		return int32(v), err
	case 0b10:
		return 0, nil
	default:
		return 0, newError(ErrReservedBitPattern, "reserved BL pattern 11")
	}
}

// ReadBLU reads a BL and reinterprets the bits as unsigned.
func (c *BitCursor) ReadBLU() (uint32, error) {
	v, err := c.ReadBL()
	return uint32(v), err
}

// ReadBLL reads a BLL (bitlonglong): a 3B byte count then that many
// little-endian bytes.
func (c *BitCursor) ReadBLL() (uint64, error) {
	n, err := c.Read3B()
	if err != nil {
		return 0, err
	}
	var out uint64
	for i := 0; i < int(n); i++ {
		b, err := c.readBits(8)
		if err != nil {
			return 0, err
		}
		out |= b << (8 * i)
	}
	return out, nil
}

// ReadBD reads a BD (bitdouble): 00=raw f64, 01=1.0, 10=0.0, 11=reserved.
func (c *BitCursor) ReadBD() (float64, error) {
	tag, err := c.ReadBB()
	if err != nil {
		return 0, err
	}
	switch tag {
	case 0b00:
		return c.ReadRD()
	case 0b01:
		return 1.0, nil
	case 0b10:
		return 0.0, nil
	default:
		return 0, newError(ErrReservedBitPattern, "reserved BD pattern 11")
	}
}

// ReadDD reads a DD (bitdouble with default): the 2-bit prefix keeps
// default or patches 4, 6, or all 8 of its bytes.
func (c *BitCursor) ReadDD(def float64) (float64, error) {
	tag, err := c.ReadBB()
	if err != nil {
		return 0, err
	}
	switch tag {
	case 0b00:
		return def, nil
	case 0b01:
		bs := leBytes(def)
		for i := 0; i < 4; i++ {
			b, err := c.readBits(8)
			if err != nil {
				return 0, err
			}
			bs[i] = byte(b)
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(bs)), nil
	case 0b10:
		bs := leBytes(def)
		b4, err := c.readBits(8)
		if err != nil {
			return 0, err
		}
		b5, err := c.readBits(8)
		if err != nil {
			return 0, err
		}
		bs[4] = byte(b4)
		bs[5] = byte(b5)
		for i := 0; i < 4; i++ {
			b, err := c.readBits(8)
			if err != nil {
				return 0, err
			}
			bs[i] = byte(b)
		}
		return math.Float64frombits(binary.LittleEndian.Uint64(bs)), nil
	default:
		return c.ReadRD()
	}
}

func leBytes(f float64) []byte {
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, math.Float64bits(f))
	return bs
}

// ReadRC reads an RC (raw 8-bit char).
func (c *BitCursor) ReadRC() (uint8, error) {
	v, err := c.readBits(8)
	return uint8(v), err
}

// ReadRS reads an RS (raw little-endian 16-bit short).
func (c *BitCursor) ReadRS() (int16, error) {
	lsb, err := c.readBits(8)
	if err != nil {
		return 0, err
	}
	msb, err := c.readBits(8)
	if err != nil {
		return 0, err
	}
	return int16(uint16(msb)<<8 | uint16(lsb)), nil
}

// ReadRL reads an RL (raw little-endian 32-bit long).
func (c *BitCursor) ReadRL() (uint32, error) {
	var v uint32
	for i := 0; i < 4; i++ {
		b, err := c.readBits(8)
		if err != nil {
			return 0, err
		}
		v |= uint32(b) << (8 * i)
	}
	return v, nil
}

// ReadRD reads an RD (raw little-endian IEEE-754 double).
func (c *BitCursor) ReadRD() (float64, error) {
	var bs [8]byte
	for i := range bs {
		b, err := c.readBits(8)
		if err != nil {
			return 0, err
		}
		bs[i] = byte(b)
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(bs[:])), nil
}

// ReadMC reads an MC (signed modular char): 7-bit groups with a
// continuation high bit and the sign in the final group.
func (c *BitCursor) ReadMC() (int64, error) {
	var value uint64
	var shift uint
	negate := false
	for {
		b, err := c.ReadRC()
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

// ReadMS reads an MS (unsigned modular short): 15-bit groups with a
// continuation high bit.
func (c *BitCursor) ReadMS() (uint64, error) {
	var value uint64
	var shift uint
	for {
		lo, err := c.ReadRC()
		if err != nil {
			return 0, err
		}
		hi, err := c.ReadRC()
		if err != nil {
			return 0, err
		}
		module := uint64(hi)<<8 | uint64(lo)
		cont := module&0x8000 != 0
		data := module & 0x7FFF
		value |= data << shift
		shift += 15
		if !cont || shift >= 64 {
			break
		}
	}
	return value, nil
}

// ReadHandle reads an H (handle reference): code/counter byte then
// counter big-endian value bytes.
func (c *BitCursor) ReadHandle() (Handle, error) {
	code, err := c.readBits(4)
	if err != nil {
		return Handle{}, err
	}
	counter, err := c.readBits(4)
	if err != nil {
		return Handle{}, err
	}
	if counter > 8 {
		return Handle{}, newError(ErrHandleCounterTooLarge, "handle counter %d exceeds 8 bytes (code %d)", counter, code)
	}
	var value uint64
	for i := 0; i < int(counter); i++ {
		b, err := c.ReadRC()
		if err != nil {
			return Handle{}, err
		}
		value = (value << 8) | uint64(b)
	}
	return Handle{Code: uint8(code), Counter: uint8(counter), Value: value}, nil
}

// signedModularCharValue converts an unsigned modular-char magnitude and
// negation flag into a signed value.
func signedModularCharValue(value uint64, negate bool) (int64, error) {
	if negate {
		if value == 1<<63 {
			return math.MinInt64, nil
		}
		if value > math.MaxInt64 {
			return 0, newError(ErrUnsupported, "signed modular char value %d out of i64 range", value)
		}
		return -int64(value), nil
	}
	if value > math.MaxInt64 {
		return 0, newError(ErrUnsupported, "signed modular char value %d out of i64 range", value)
	}
	return int64(value), nil
}
