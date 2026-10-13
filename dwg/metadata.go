package dwg

import (
	"encoding/binary"
	"unicode/utf16"
)

// SummaryInfo is drawing-level metadata (spec §13).
type SummaryInfo struct {
	Title            string
	Subject          string
	Author           string
	Keywords         string
	Comments         string
	LastSavedBy      string
	RevisionNumber   string
	HyperlinkBase    string
	TotalEditTimeRaw [8]byte
	CreateDateRaw    [8]byte
	ModifiedDateRaw  [8]byte
	Properties       [][2]string
}

type stringEncoding int

const (
	encANSI stringEncoding = iota
	encUnicode
)

// ParseSummaryInfo parses a decompressed AcDb:SummaryInfo payload,
// auto-detecting ANSI vs UTF-16LE.
func ParseSummaryInfo(bytes []byte) (SummaryInfo, error) {
	if si, err := parseSummaryInfoWith(bytes, encUnicode); err == nil {
		return si, nil
	}
	return parseSummaryInfoWith(bytes, encANSI)
}

func parseSummaryInfoWith(bytes []byte, enc stringEncoding) (SummaryInfo, error) {
	c := newByteCursor(bytes)
	read := func() (string, error) {
		if enc == encANSI {
			return c.readLenStr()
		}
		return c.readUnicodeLenStrInclusiveNul()
	}
	var si SummaryInfo
	var err error
	if si.Title, err = read(); err != nil {
		return SummaryInfo{}, err
	}
	if si.Subject, err = read(); err != nil {
		return SummaryInfo{}, err
	}
	if si.Author, err = read(); err != nil {
		return SummaryInfo{}, err
	}
	if si.Keywords, err = read(); err != nil {
		return SummaryInfo{}, err
	}
	if si.Comments, err = read(); err != nil {
		return SummaryInfo{}, err
	}
	if si.LastSavedBy, err = read(); err != nil {
		return SummaryInfo{}, err
	}
	if si.RevisionNumber, err = read(); err != nil {
		return SummaryInfo{}, err
	}
	if si.HyperlinkBase, err = read(); err != nil {
		return SummaryInfo{}, err
	}
	if si.TotalEditTimeRaw, err = c.readArray8(); err != nil {
		return SummaryInfo{}, err
	}
	if si.CreateDateRaw, err = c.readArray8(); err != nil {
		return SummaryInfo{}, err
	}
	if si.ModifiedDateRaw, err = c.readArray8(); err != nil {
		return SummaryInfo{}, err
	}
	propCount, err := c.readU16()
	if err != nil {
		return SummaryInfo{}, err
	}
	si.Properties = make([][2]string, 0, propCount)
	for i := 0; i < int(propCount); i++ {
		k, err := read()
		if err != nil {
			return SummaryInfo{}, err
		}
		v, err := read()
		if err != nil {
			return SummaryInfo{}, err
		}
		si.Properties = append(si.Properties, [2]string{k, v})
	}
	return si, nil
}

// AppInfo identifies the application that wrote the .dwg file (spec §16).
type AppInfo struct {
	Name       string
	ProductXML string
	Version    string
	Comment    string
	Product    string
}

// ParseAppInfo parses a decompressed AcDb:AppInfo payload.
func ParseAppInfo(bytes []byte) (AppInfo, error) {
	if len(bytes) >= 4 {
		unknown := binary.LittleEndian.Uint32(bytes[0:4])
		if unknown == 2 || unknown == 3 {
			if ai, err := parseAppInfoR21(bytes[4:]); err == nil {
				return ai, nil
			}
		}
	}
	return parseAppInfoR18(bytes)
}

func parseAppInfoR18(bytes []byte) (AppInfo, error) {
	c := newByteCursor(bytes)
	name, err := c.readLenStr()
	if err != nil {
		return AppInfo{}, err
	}
	if _, err := c.readU32(); err != nil {
		return AppInfo{}, err
	}
	if _, err := c.readLenStr(); err != nil {
		return AppInfo{}, err
	}
	productXML, err := c.readLenStr()
	if err != nil {
		return AppInfo{}, err
	}
	version, _ := c.readLenStr()
	return AppInfo{Name: name, ProductXML: productXML, Version: version}, nil
}

func parseAppInfoR21(bytes []byte) (AppInfo, error) {
	c := newByteCursor(bytes)
	name, err := c.readUnicodeLenStr()
	if err != nil {
		return AppInfo{}, err
	}
	if _, err := c.readU32(); err != nil {
		return AppInfo{}, err
	}
	if err := c.skip(16); err != nil {
		return AppInfo{}, err
	}
	version, err := c.readUnicodeLenStr()
	if err != nil {
		return AppInfo{}, err
	}
	if err := c.skip(16); err != nil {
		return AppInfo{}, err
	}
	comment, _ := c.readUnicodeLenStr()
	if err := c.skip(16); err != nil {
		return AppInfo{}, err
	}
	product, _ := c.readUnicodeLenStr()
	appVersion, _ := c.readLenStr()
	if appVersion != "" {
		version = appVersion
	}
	return AppInfo{Name: name, Version: version, Comment: comment, Product: product}, nil
}

// Preview is a thumbnail image embedded in the .dwg (BMP/WMF/PNG).
type Preview struct {
	OverallSize uint32
	BMP         []byte
	WMF         []byte
	HeaderData  []byte
}

var previewStartSentinel = [16]byte{
	0x1F, 0x25, 0x6D, 0x07, 0xD4, 0x36, 0x28, 0x28, 0x9D, 0x57, 0xCA, 0x3F, 0x9D, 0x44, 0x10, 0x2B,
}

var previewEndSentinel = [16]byte{
	0xE0, 0xDA, 0x92, 0xF8, 0x2B, 0xC9, 0xD7, 0xD7, 0x62, 0xA8, 0x35, 0xC0, 0x62, 0xBB, 0xEF, 0xD4,
}

// ParsePreview parses a decompressed AcDb:Preview payload.
func ParsePreview(bytes []byte) (Preview, error) {
	if len(bytes) < 20 || string(bytes[:16]) != string(previewStartSentinel[:]) {
		return Preview{}, newError(ErrSectionMap, "preview: start sentinel missing")
	}
	c := newByteCursor(bytes[16:])
	overallSize, _ := c.readU32()
	imagesPresent, _ := c.readU8()
	var headerRange, bmpRange, wmfRange, pngRange *[2]int
	for i := 0; i < int(imagesPresent); i++ {
		code, err := c.readU8()
		if err != nil {
			break
		}
		start, err1 := c.readU32()
		size, err2 := c.readU32()
		if err1 != nil || err2 != nil {
			break
		}
		r := [2]int{int(start), int(size)}
		switch code {
		case 1:
			headerRange = &r
		case 2:
			bmpRange = &r
		case 3:
			wmfRange = &r
		case 6:
			pngRange = &r
		}
	}
	extract := func(r *[2]int) []byte {
		if r == nil || r[0] == 0 || r[1] == 0 {
			return nil
		}
		end := r[0] + r[1]
		if end > len(bytes) {
			return nil
		}
		return append([]byte(nil), bytes[r[0]:end]...)
	}
	bmp := extract(bmpRange)
	wmf := extract(wmfRange)
	header := extract(headerRange)
	if bmp == nil {
		bmp = extract(pngRange)
	}
	if bmp == nil && wmf == nil {
		if b := carveImageMagic(bytes); b != nil {
			bmp = b
		}
	}
	return Preview{OverallSize: overallSize, BMP: bmp, WMF: wmf, HeaderData: header}, nil
}

func carveImageMagic(bytes []byte) []byte {
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	start := -1
	for i := 16; i < len(bytes)-len(pngMagic); i++ {
		if string(bytes[i:i+len(pngMagic)]) == string(pngMagic) {
			start = i
			break
		}
		if bytes[i] == 'B' && i+1 < len(bytes) && bytes[i+1] == 'M' && len(bytes)-i > 54 {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	end := len(bytes)
	for i := len(bytes) - 16; i >= start; i-- {
		if string(bytes[i:i+16]) == string(previewEndSentinel[:]) {
			end = i
			break
		}
	}
	if end < start {
		end = start
	}
	return append([]byte(nil), bytes[start:end]...)
}

// FileDependency is a single entry in the file-dependency list.
type FileDependency struct {
	FullFilename    string
	FoundPath       string
	FingerprintGUID string
	VersionGUID     string
	FeatureIndex    int32
	Timestamp       int32
	Filesize        int32
	AffectsGraphics bool
	ReferenceCount  int32
}

// FileDepList is an aggregated file dependency list (spec §17).
type FileDepList struct {
	Features []string
	Files    []FileDependency
}

// ParseFileDepList parses a decompressed AcDb:FileDepList payload.
func ParseFileDepList(bytes []byte) (FileDepList, error) {
	c := newByteCursor(bytes)
	featureCount, err := c.readU32()
	if err != nil {
		return FileDepList{}, err
	}
	features := make([]string, 0, featureCount)
	for i := 0; i < int(featureCount); i++ {
		s, err := c.readLenStr32()
		if err != nil {
			return FileDepList{}, err
		}
		features = append(features, s)
	}
	fileCount, err := c.readU32()
	if err != nil {
		return FileDepList{}, err
	}
	files := make([]FileDependency, 0, fileCount)
	for i := 0; i < int(fileCount); i++ {
		var fd FileDependency
		if fd.FullFilename, err = c.readLenStr32(); err != nil {
			return FileDepList{}, err
		}
		if fd.FoundPath, err = c.readLenStr32(); err != nil {
			return FileDepList{}, err
		}
		if fd.FingerprintGUID, err = c.readLenStr32(); err != nil {
			return FileDepList{}, err
		}
		if fd.VersionGUID, err = c.readLenStr32(); err != nil {
			return FileDepList{}, err
		}
		fi, err := c.readU32()
		if err != nil {
			return FileDepList{}, err
		}
		ts, err := c.readU32()
		if err != nil {
			return FileDepList{}, err
		}
		fs, err := c.readU32()
		if err != nil {
			return FileDepList{}, err
		}
		ag, err := c.readU16()
		if err != nil {
			return FileDepList{}, err
		}
		rc, err := c.readU32()
		if err != nil {
			return FileDepList{}, err
		}
		fd.FeatureIndex = int32(fi)
		fd.Timestamp = int32(ts)
		fd.Filesize = int32(fs)
		fd.AffectsGraphics = ag != 0
		fd.ReferenceCount = int32(rc)
		files = append(files, fd)
	}
	return FileDepList{Features: features, Files: files}, nil
}

// byteCursor is a byte-aligned cursor for the fixed-format metadata sections.
type byteCursor struct {
	bytes []byte
	pos   int
}

func newByteCursor(b []byte) *byteCursor { return &byteCursor{bytes: b} }

func (c *byteCursor) need(n int) error {
	if c.pos+n > len(c.bytes) {
		return newError(ErrTruncated, "need %d bytes at offset %d, have %d", n, c.pos, len(c.bytes))
	}
	return nil
}

func (c *byteCursor) skip(n int) error {
	if err := c.need(n); err != nil {
		return err
	}
	c.pos += n
	return nil
}

func (c *byteCursor) readU8() (uint8, error) {
	if err := c.need(1); err != nil {
		return 0, err
	}
	b := c.bytes[c.pos]
	c.pos++
	return b, nil
}

func (c *byteCursor) readU16() (uint16, error) {
	if err := c.need(2); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint16(c.bytes[c.pos : c.pos+2])
	c.pos += 2
	return v, nil
}

func (c *byteCursor) readU32() (uint32, error) {
	if err := c.need(4); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint32(c.bytes[c.pos : c.pos+4])
	c.pos += 4
	return v, nil
}

func (c *byteCursor) readArray8() ([8]byte, error) {
	var out [8]byte
	if err := c.need(8); err != nil {
		return out, err
	}
	copy(out[:], c.bytes[c.pos:c.pos+8])
	c.pos += 8
	return out, nil
}

func (c *byteCursor) readLenStr() (string, error) {
	length, err := c.readU16()
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	if err := c.need(int(length)); err != nil {
		return "", err
	}
	end := c.pos + int(length)
	textEnd := end
	if c.bytes[end-1] == 0 {
		textEnd = end - 1
	}
	s := string(c.bytes[c.pos:textEnd])
	c.pos = end
	return s, nil
}

func (c *byteCursor) readUnicodeLenStr() (string, error) {
	charCount, err := c.readU16()
	if err != nil {
		return "", err
	}
	if charCount == 0 {
		return "", nil
	}
	byteCount := int(charCount) * 2
	if err := c.need(byteCount + 2); err != nil {
		return "", err
	}
	units := make([]uint16, charCount)
	for i := 0; i < int(charCount); i++ {
		units[i] = binary.LittleEndian.Uint16(c.bytes[c.pos+i*2 : c.pos+i*2+2])
	}
	c.pos += byteCount + 2
	for len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	return string(utf16.Decode(units)), nil
}

func (c *byteCursor) readUnicodeLenStrInclusiveNul() (string, error) {
	charCount, err := c.readU16()
	if err != nil {
		return "", err
	}
	if charCount == 0 {
		return "", nil
	}
	byteCount := int(charCount) * 2
	if err := c.need(byteCount); err != nil {
		return "", err
	}
	units := make([]uint16, charCount)
	for i := 0; i < int(charCount); i++ {
		units[i] = binary.LittleEndian.Uint16(c.bytes[c.pos+i*2 : c.pos+i*2+2])
	}
	c.pos += byteCount
	for len(units) > 0 && units[len(units)-1] == 0 {
		units = units[:len(units)-1]
	}
	return string(utf16.Decode(units)), nil
}

func (c *byteCursor) readLenStr32() (string, error) {
	length, err := c.readU32()
	if err != nil {
		return "", err
	}
	if length == 0 || length == 0xFFFFFFFF {
		return "", nil
	}
	if err := c.need(int(length)); err != nil {
		return "", err
	}
	s := string(c.bytes[c.pos : c.pos+int(length)])
	c.pos += int(length)
	return s, nil
}
