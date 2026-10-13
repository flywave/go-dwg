package dwg

import (
	"bytes"
	"testing"
)

func TestMagicSequenceMatchesSpec(t *testing.T) {
	m := magicSequence()
	expected := [MagicLen]byte{
		0x29, 0x23, 0xBE, 0x84, 0xE1, 0x6C, 0xD6, 0xAE, 0x52, 0x90, 0x49, 0xF1, 0xF1, 0xBB,
		0xE9, 0xEB, 0xB3, 0xA6, 0xDB, 0x3C, 0x87, 0x0C, 0x3E, 0x99, 0x24, 0x5E, 0x0D, 0x1C,
		0x06, 0xB7, 0x47, 0xDE, 0xB3, 0x12, 0x4D, 0xC8, 0x43, 0xBB, 0x8B, 0xA6, 0x1F, 0x03,
		0x5A, 0x7D, 0x09, 0x38, 0x25, 0x1F, 0x5D, 0xD4, 0xCB, 0xFC, 0x96, 0xF5, 0x45, 0x3B,
		0x13, 0x0D, 0x89, 0x0A, 0x1C, 0xDB, 0xAE, 0x32, 0x20, 0x9A, 0x50, 0xEE, 0x40, 0x78,
		0x36, 0xFD, 0x12, 0x49, 0x32, 0xF6, 0x9E, 0x7D, 0x49, 0xDC, 0xAD, 0x4F, 0x14, 0xF2,
		0x44, 0x40, 0x66, 0xD0, 0x6B, 0xC4, 0x30, 0xB7, 0x32, 0x3B, 0xA1, 0x22, 0xF6, 0x22,
		0x91, 0x9D, 0xE1, 0x8B, 0x1F, 0xDA, 0xB0, 0xCA, 0x99, 0x02,
	}
	if m != expected {
		t.Fatalf("magic sequence mismatch")
	}
}

func TestVersionRoundTrip(t *testing.T) {
	for v := R14; v <= R2018; v++ {
		got, err := VersionFromMagic([]byte(v.Magic()))
		if err != nil || got != v {
			t.Fatalf("roundtrip %s: got %v err %v", v, got, err)
		}
	}
	if _, err := VersionFromMagic([]byte("XX1015")); err == nil {
		t.Fatal("expected NotDwg")
	}
	if _, err := VersionFromMagic([]byte("AC1009")); err == nil {
		t.Fatal("expected UnsupportedVersion")
	}
}

func TestCRC8MatchesReflectedPolyA001(t *testing.T) {
	msg := []byte("123456789")
	var bitCRC uint16
	for _, b := range msg {
		bitCRC ^= uint16(b)
		for i := 0; i < 8; i++ {
			if bitCRC&1 != 0 {
				bitCRC = (bitCRC >> 1) ^ 0xA001
			} else {
				bitCRC >>= 1
			}
		}
	}
	if got := CRC8(0, msg); got != bitCRC {
		t.Fatalf("crc8 = %#x want %#x", got, bitCRC)
	}
}

func TestCRC32IEEEVector(t *testing.T) {
	if got := CRC32(0, []byte("123456789")); got != 0xCBF43926 {
		t.Fatalf("crc32 = %#x want 0xCBF43926", got)
	}
}

func TestSectionPageChecksumEmptyIsSeed(t *testing.T) {
	if got := SectionPageChecksum(0, nil); got != 0 {
		t.Fatalf("got %#x", got)
	}
	if got := SectionPageChecksum(0xDEADBEEF, nil); got != 0xDEADBEEF {
		t.Fatalf("got %#x", got)
	}
}

// packBits matches the Rust test helper: MSB-first packing of
// (value, bitCount) pairs padded to a byte boundary.
func packBits(fields [][2]uint64) []byte {
	var acc uint64
	var nbits uint
	words := [][2]uint64{fields[0]}
	_ = words
	// Use a big buffer of bits.
	var bits []uint64
	for _, f := range fields {
		for i := int(f[1]) - 1; i >= 0; i-- {
			bits = append(bits, (f[0]>>uint(i))&1)
		}
	}
	nbits = uint(len(bits))
	_ = acc
	pad := (8 - (nbits % 8)) % 8
	for i := uint(0); i < pad; i++ {
		bits = append(bits, 0)
	}
	out := make([]byte, len(bits)/8)
	for i, b := range bits {
		if b != 0 {
			out[i/8] |= 1 << (7 - uint(i%8))
		}
	}
	return out
}

func TestSpecBSExample(t *testing.T) {
	stream := packBits([][2]uint64{
		{0b00, 2}, {0b00000001, 8}, {0b00000001, 8},
		{0b10, 2}, {0b11, 2}, {0b01, 2}, {0b00001111, 8}, {0b10, 2},
	})
	c := NewBitCursor(stream)
	want := []int16{257, 0, 256, 15, 0}
	for _, w := range want {
		got, err := c.ReadBS()
		if err != nil || got != w {
			t.Fatalf("ReadBS = %d err %v want %d", got, err, w)
		}
	}
}

func TestSpecBLExample(t *testing.T) {
	stream := packBits([][2]uint64{
		{0b00, 2}, {0x01, 8}, {0x01, 8}, {0x00, 8}, {0x00, 8},
		{0b10, 2}, {0b01, 2}, {0b00001111, 8}, {0b10, 2},
	})
	c := NewBitCursor(stream)
	want := []int32{257, 0, 15, 0}
	for _, w := range want {
		got, err := c.ReadBL()
		if err != nil || got != w {
			t.Fatalf("ReadBL = %d err %v want %d", got, err, w)
		}
	}
}

func TestSpecBDAndDD(t *testing.T) {
	stream := packBits([][2]uint64{{0b01, 2}, {0b10, 2}, {0b00, 2},
		{0x00, 8}, {0x00, 8}, {0x00, 8}, {0x00, 8}, {0x00, 8}, {0x00, 8}, {0x00, 8}, {0x40, 8}})
	c := NewBitCursor(stream)
	if v, _ := c.ReadBD(); v != 1.0 {
		t.Fatalf("bd1 %v", v)
	}
	if v, _ := c.ReadBD(); v != 0.0 {
		t.Fatalf("bd2 %v", v)
	}
	if v, _ := c.ReadBD(); v != 2.0 {
		t.Fatalf("bd3 %v", v)
	}
}

func TestLZ77LiteralOnly(t *testing.T) {
	stream := []byte{0x02, 'H', 'e', 'l', 'l', 'o', 0x11}
	out, err := Decompress(stream, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "Hello" {
		t.Fatalf("got %q", out)
	}
}

func TestLZ77RLEWrap(t *testing.T) {
	stream := []byte{
		0x04, 'A', 'B', 'C', 'D', 'E', 'F', 'G',
		0x22, 0x0A, 0x00,
		'X', 'Y',
		0x11,
	}
	out, err := Decompress(stream, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "ABCDEFGEFGEXY" {
		t.Fatalf("got %q", out)
	}
}

func TestLZ77Opcode40Family(t *testing.T) {
	stream := []byte{
		0x04, 'A', 'B', 'C', 'D', 'E', 'F', 'G',
		0x41, 0x01, 'Z', 0x11,
	}
	out, err := Decompress(stream, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, []byte("ABCDEFGCDEZ")) {
		t.Fatalf("got %q", out)
	}
}

func TestLZ77Truncated(t *testing.T) {
	_, err := Decompress([]byte{0x04, 'A', 'B'}, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}
