package dwg

import (
	"encoding/binary"
	"testing"
)

func TestR2007SystemPageSize(t *testing.T) {
	cases := map[int]int{0: 0x400, 304: 0x400, 2132: 0x1200}
	for in, want := range cases {
		if got := r2007SystemPageSize(in); got != want {
			t.Errorf("system_page_size(%d) = %#x want %#x", in, got, want)
		}
	}
}

func TestR2007Deinterleave(t *testing.T) {
	page := make([]byte, 2*r2007RSCodeword)
	for i := 0; i < r2007RSCodeword; i++ {
		page[i*2] = byte(i)
		page[i*2+1] = byte(0x80 + i)
	}
	out, err := r2007Deinterleave(page, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 1, 2, 0x80, 0x81, 0x82}
	if string(out) != string(want) {
		t.Fatalf("got %v want %v", out, want)
	}
	if _, err := r2007Deinterleave(make([]byte, 10), 1, r2007RSSystemMessage); err == nil {
		t.Fatal("expected truncation error")
	}
}

func TestR2007PageMapAccumulatesOffsets(t *testing.T) {
	var data []byte
	for _, p := range [][2]int64{{0x400, 22}, {0x200, 23}, {0x100, 3}} {
		var b [16]byte
		binary.LittleEndian.PutUint64(b[0:8], uint64(p[0]))
		binary.LittleEndian.PutUint64(b[8:16], uint64(p[1]))
		data = append(data, b[:]...)
	}
	m, err := parseR2007PageMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if m.Len() != 3 {
		t.Fatalf("len = %d", m.Len())
	}
	if o, _ := m.fileOffsetOf(22); o != r2007PageBase {
		t.Errorf("22 -> %#x", o)
	}
	if o, _ := m.fileOffsetOf(23); o != r2007PageBase+0x400 {
		t.Errorf("23 -> %#x", o)
	}
	if o, _ := m.fileOffsetOf(3); o != r2007PageBase+0x600 {
		t.Errorf("3 -> %#x", o)
	}
	if _, ok := m.fileOffsetOf(99); ok {
		t.Error("99 should be absent")
	}
}

func TestR2007PageMapFreePageResolvesByAbsID(t *testing.T) {
	var data [16]byte
	binary.LittleEndian.PutUint64(data[0:8], 0x400)
	var neg int64 = -7
	binary.LittleEndian.PutUint64(data[8:16], uint64(neg))
	m, err := parseR2007PageMap(data[:])
	if err != nil {
		t.Fatal(err)
	}
	if o, ok := m.fileOffsetOf(7); !ok || o != r2007PageBase {
		t.Fatalf("free page 7 not resolved: %v %v", o, ok)
	}
}

func TestR2007SectionMapNameLengthCountsTerminator(t *testing.T) {
	name := "AcDb:Handles"
	var data []byte
	header := make([]uint64, 8)
	header[0] = 591
	header[1] = 63488
	header[2] = 0
	header[3] = 0x3F6E0450
	header[4] = uint64(len(name)+1) * 2
	header[6] = 4
	header[7] = 1
	for _, v := range header {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], v)
		data = append(data, b[:]...)
	}
	for _, r := range name {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(r))
		data = append(data, b[:]...)
	}
	data = append(data, 0, 0)
	for _, v := range []uint64{0, 768, 14, 591, 566, 0x0A925B96, 0} {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], v)
		data = append(data, b[:]...)
	}
	m, err := parseR2007SectionMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sections) != 1 {
		t.Fatalf("sections = %d", len(m.Sections))
	}
	s := m.Sections[0]
	if s.Name != "AcDb:Handles" || s.HashCode != 0x3F6E0450 || s.Encoding != 4 {
		t.Fatalf("bad header: %+v", s)
	}
	if len(s.Pages) != 1 || s.Pages[0].ID != 14 || s.Pages[0].Compressed != 566 || s.Pages[0].Uncompressed != 591 {
		t.Fatalf("bad page: %+v", s.Pages)
	}
}

func TestR2007SectionMapDropsTrailingUnnamed(t *testing.T) {
	data := make([]byte, 0x40)
	m, err := parseR2007SectionMap(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sections) != 0 {
		t.Fatalf("expected empty, got %d", len(m.Sections))
	}
}

// HEADER_PREFIX is the first 12 bytes of line_2007.dwg's RS-decoded file
// header payload, decompressed by the spec's rules (see r21_lz docs).
var r2007HeaderPrefix = []byte{0x20, 0x00, 0x00, 0x02, 0x00, 0x70, 0x70, 0x02, 0x01, 0x03, 0x58, 0x00}

func TestR2007HeaderPrefixDecodesToDocumentedFields(t *testing.T) {
	out, err := DecompressR2007(r2007HeaderPrefix, 16, DefaultDecompressLimits())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 16 {
		t.Fatalf("len = %d", len(out))
	}
	if got := binary.LittleEndian.Uint64(out[0:8]); got != 0x70 {
		t.Errorf("header size = %#x want 0x70", got)
	}
	if got := binary.LittleEndian.Uint64(out[8:16]); got != 66304 {
		t.Errorf("file size = %d want 66304", got)
	}
}

func TestR2007EmitPermutedBlocks(t *testing.T) {
	seq := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(i)
		}
		return b
	}
	var two []byte
	if err := emitPermuted(seq(2), 0, 2, &two); err != nil {
		t.Fatal(err)
	}
	if string(two) != string([]byte{1, 0}) {
		t.Fatalf("2-block = %v", two)
	}
	var four, eight []byte
	_ = emitPermuted(seq(8), 0, 4, &four)
	_ = emitPermuted(seq(8), 0, 8, &eight)
	if string(four) != string([]byte{0, 1, 2, 3}) || string(eight) != string(seq(8)) {
		t.Fatalf("straight blocks wrong: %v %v", four, eight)
	}
	var sixteen []byte
	_ = emitPermuted(seq(16), 0, 16, &sixteen)
	want16 := []byte{8, 9, 10, 11, 12, 13, 14, 15, 0, 1, 2, 3, 4, 5, 6, 7}
	if string(sixteen) != string(want16) {
		t.Fatalf("16-block = %v", sixteen)
	}
	var eleven []byte
	_ = emitPermuted(seq(11), 0, 11, &eleven)
	if string(eleven) != string([]byte{10, 9, 1, 2, 3, 4, 5, 6, 7, 8, 0}) {
		t.Fatalf("11-block = %v", eleven)
	}
	var thirtytwo []byte
	_ = emitPermuted(seq(32), 0, 32, &thirtytwo)
	var want32 []byte
	for i := 24; i < 32; i++ {
		want32 = append(want32, byte(i))
	}
	for i := 16; i < 24; i++ {
		want32 = append(want32, byte(i))
	}
	for i := 8; i < 16; i++ {
		want32 = append(want32, byte(i))
	}
	for i := 0; i < 8; i++ {
		want32 = append(want32, byte(i))
	}
	if string(thirtytwo) != string(want32) {
		t.Fatalf("32-block = %v", thirtytwo)
	}
}

func TestR2007TruncatedAndCap(t *testing.T) {
	if _, err := DecompressR2007([]byte{0x20, 0x00, 0x00, 0x07}, 64, DefaultDecompressLimits()); err == nil {
		t.Fatal("expected truncated error")
	}
	limits := DefaultDecompressLimits()
	limits.MaxOutputBytes = 8
	if _, err := DecompressR2007(r2007HeaderPrefix, 16, limits); err == nil {
		t.Fatal("expected output-limit error")
	}
	if out, err := DecompressR2007(nil, 0, DefaultDecompressLimits()); err != nil || len(out) != 0 {
		t.Fatalf("empty input: %v %v", out, err)
	}
}
