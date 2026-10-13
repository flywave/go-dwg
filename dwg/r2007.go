package dwg

import (
	"encoding/binary"
	"unicode/utf16"
)

// R2007 (AC1021) container — ODA spec §5.1-§5.4.
//
// R2007 sits between the R13-R15 flat-locator layout and the R2004 page-map
// layout and has a container of its own: an 0x400-byte Reed-Solomon file
// header at 0x80, bare pages (no per-page headers), Reed-Solomon
// interleaving, and the §5.10 LZ variant.

const (
	r2007FileHeaderOffset   = 0x80
	r2007FileHeaderPageSize = 0x400
	r2007FileHeaderSize     = 0x110
	r2007RSCodeword         = 255
	r2007RSSystemMessage    = 239
	r2007RSDataMessage      = 251
	r2007PageBase           = 0x480
	r2007CRCBlock           = 8
	r2007PageAlign          = 0x20
	r2007MaxPages           = 1 << 20
	r2007MaxSections        = 4096
)

func alignUp(n, a int) int { return (n + a - 1) / a * a }

// r2007SystemPageSize computes the system-page size (spec §5.3.1).
func r2007SystemPageSize(dataSize int) int {
	aligned := alignUp(dataSize, r2007CRCBlock)
	page := (aligned * 2 / r2007RSSystemMessage) * r2007RSCodeword
	if aligned*2%r2007RSSystemMessage != 0 {
		page += r2007RSCodeword
	}
	if page < 0x400 {
		return 0x400
	}
	return alignUp(page, r2007PageAlign)
}

// r2007Deinterleave gathers the message bytes of blocks interleaved
// Reed-Solomon codewords out of page (§5.13.2).
func r2007Deinterleave(page []byte, blocks, message int) ([]byte, error) {
	needed := blocks * r2007RSCodeword
	if len(page) < needed {
		return nil, newError(ErrTruncated, "RS page needs %d bytes, got %d", needed, len(page))
	}
	out := make([]byte, 0, blocks*message)
	for j := 0; j < blocks; j++ {
		for i := 0; i < message; i++ {
			out = append(out, page[i*blocks+j])
		}
	}
	return out, nil
}

// r2007FileHeader is the 0x110-byte R2007 file header (§5.2), read as 34
// little-endian u64 fields.
type r2007FileHeader struct {
	HeaderSize                  uint64
	FileSize                    uint64
	PagesMapCRCCompressed       uint64
	PagesMapCorrectionFactor    uint64
	PagesMapCRCSeed             uint64
	PagesMap2Offset             uint64
	PagesMap2ID                 uint64
	PagesMapOffset              uint64
	PagesMapID                  uint64
	Header2Offset               uint64
	PagesMapSizeCompressed      uint64
	PagesMapSizeUncompressed    uint64
	PagesAmount                 uint64
	PagesMaxID                  uint64
	Unknown0x70                 uint64
	Unknown0x78                 uint64
	PagesMapCRCUncompressed     uint64
	Unknown0x88                 uint64
	Unknown0x90                 uint64
	Unknown0x98                 uint64
	SectionsAmount              uint64
	SectionsMapCRCUncompressed  uint64
	SectionsMapSizeCompressed   uint64
	SectionsMap2ID              uint64
	SectionsMapID               uint64
	SectionsMapSizeUncompressed uint64
	SectionsMapCRCCompressed    uint64
	SectionsMapCorrectionFactor uint64
	SectionsMapCRCSeed          uint64
	StreamVersion               uint64
	CRCSeed                     uint64
	CRCSeedEncoded              uint64
	RandomSeed                  uint64
	HeaderCRC64                 uint64
}

func parseR2007FileHeader(bytes []byte, limits DecompressLimits) (r2007FileHeader, error) {
	end := r2007FileHeaderOffset + r2007FileHeaderPageSize
	if len(bytes) < end {
		return r2007FileHeader{}, newError(ErrTruncated, "R2007 file header needs %d bytes, got %d", r2007FileHeaderPageSize, len(bytes))
	}
	decoded, err := r2007Deinterleave(bytes[r2007FileHeaderOffset:end], 3, r2007RSSystemMessage)
	if err != nil {
		return r2007FileHeader{}, err
	}
	if len(decoded) < 0x20 {
		return r2007FileHeader{}, newError(ErrSectionMap, "R2007 file header: RS block shorter than its 0x20-byte prefix")
	}
	comprLen := int32(binary.LittleEndian.Uint32(decoded[0x18:0x1C]))
	var plain []byte
	if comprLen < 0 {
		stored := int(-comprLen)
		if 0x20+stored > len(decoded) {
			return r2007FileHeader{}, newError(ErrSectionMap, "R2007 file header: stored payload runs past the decoded block")
		}
		plain = append([]byte(nil), decoded[0x20:0x20+stored]...)
	} else {
		if 0x20+int(comprLen) > len(decoded) {
			return r2007FileHeader{}, newError(ErrSectionMap, "R2007 file header: ComprLen runs past the RS block")
		}
		compressed := decoded[0x20 : 0x20+int(comprLen)]
		p, err := DecompressR2007(compressed, r2007FileHeaderSize, limits)
		if err != nil {
			return r2007FileHeader{}, err
		}
		plain = p
	}
	if len(plain) < r2007FileHeaderSize {
		return r2007FileHeader{}, newError(ErrSectionMap, "R2007 file header decompressed to %d bytes, expected %d", len(plain), r2007FileHeaderSize)
	}
	return r2007FileHeaderFromPlain(plain), nil
}

func r2007FileHeaderFromPlain(p []byte) r2007FileHeader {
	q := func(i int) uint64 { return binary.LittleEndian.Uint64(p[i*8 : i*8+8]) }
	return r2007FileHeader{
		HeaderSize:                  q(0),
		FileSize:                    q(1),
		PagesMapCRCCompressed:       q(2),
		PagesMapCorrectionFactor:    q(3),
		PagesMapCRCSeed:             q(4),
		PagesMap2Offset:             q(5),
		PagesMap2ID:                 q(6),
		PagesMapOffset:              q(7),
		PagesMapID:                  q(8),
		Header2Offset:               q(9),
		PagesMapSizeCompressed:      q(10),
		PagesMapSizeUncompressed:    q(11),
		PagesAmount:                 q(12),
		PagesMaxID:                  q(13),
		Unknown0x70:                 q(14),
		Unknown0x78:                 q(15),
		PagesMapCRCUncompressed:     q(16),
		Unknown0x88:                 q(17),
		Unknown0x90:                 q(18),
		Unknown0x98:                 q(19),
		SectionsAmount:              q(20),
		SectionsMapCRCUncompressed:  q(21),
		SectionsMapSizeCompressed:   q(22),
		SectionsMap2ID:              q(23),
		SectionsMapID:               q(24),
		SectionsMapSizeUncompressed: q(25),
		SectionsMapCRCCompressed:    q(26),
		SectionsMapCorrectionFactor: q(27),
		SectionsMapCRCSeed:          q(28),
		StreamVersion:               q(29),
		CRCSeed:                     q(30),
		CRCSeedEncoded:              q(31),
		RandomSeed:                  q(32),
		HeaderCRC64:                 q(33),
	}
}

// r2007LoadSystemPage loads one system page (§5.3): RS de-interleave, then
// decompress if the container says the payload is compressed.
func r2007LoadSystemPage(bytes []byte, offset, compressed, uncompressed, factor int, limits DecompressLimits) ([]byte, error) {
	stored := compressed
	if uncompressed < stored {
		stored = uncompressed
	}
	if factor == 0 || stored == 0 {
		return nil, newError(ErrSectionMap, "R2007 system page: zero repeat factor or zero stored size")
	}
	blocks := (factor * alignUp(stored, r2007CRCBlock)) / r2007RSSystemMessage
	if (factor*alignUp(stored, r2007CRCBlock))%r2007RSSystemMessage != 0 {
		blocks++
	}
	span := blocks * r2007RSCodeword
	if offset < 0 || offset+span > len(bytes) {
		return nil, newError(ErrTruncated, "R2007 system page needs %d bytes at %d", span, offset)
	}
	decoded, err := r2007Deinterleave(bytes[offset:offset+span], blocks, r2007RSSystemMessage)
	if err != nil {
		return nil, err
	}
	if stored > len(decoded) {
		return nil, newError(ErrSectionMap, "R2007 system page: decoded block shorter than the stored size")
	}
	payload := decoded[:stored]
	if compressed < uncompressed {
		return DecompressR2007(payload, uncompressed, limits)
	}
	n := uncompressed
	if n > len(payload) {
		n = len(payload)
	}
	return append([]byte(nil), payload[:n]...), nil
}

// r2007PageEntry is one entry of the R2007 page map (§5.2).
type r2007PageEntry struct {
	ID     int64
	Size   int64
	Offset uint64
}

// r2007PageMap maps every page id to its on-disk offset.
type r2007PageMap struct {
	Entries []r2007PageEntry
}

func parseR2007PageMap(data []byte) (r2007PageMap, error) {
	var entries []r2007PageEntry
	var offset uint64
	pos := 0
	for pos+16 <= len(data) {
		size := int64(binary.LittleEndian.Uint64(data[pos : pos+8]))
		id := int64(binary.LittleEndian.Uint64(data[pos+8 : pos+16]))
		pos += 16
		if len(entries) >= r2007MaxPages {
			return r2007PageMap{}, newError(ErrSectionMap, "R2007 page map declares more than %d pages", r2007MaxPages)
		}
		entries = append(entries, r2007PageEntry{ID: id, Size: size, Offset: offset})
		if size < 0 {
			offset += uint64(-size)
		} else {
			offset += uint64(size)
		}
	}
	return r2007PageMap{Entries: entries}, nil
}

func (m *r2007PageMap) fileOffsetOf(id int64) (int, bool) {
	abs := id
	if abs < 0 {
		abs = -abs
	}
	for _, e := range m.Entries {
		ea := e.ID
		if ea < 0 {
			ea = -ea
		}
		if ea == abs {
			return r2007PageBase + int(e.Offset), true
		}
	}
	return 0, false
}

func (m *r2007PageMap) Len() int { return len(m.Entries) }

// r2007SectionPage is one page of a section (§5.2).
type r2007SectionPage struct {
	DataOffset   uint64
	Size         uint64
	ID           int64
	Uncompressed uint64
	Compressed   uint64
	Checksum     uint64
	CRC          uint64
}

// r2007SectionDescriptor is one section descriptor (§5.2).
type r2007SectionDescriptor struct {
	Name       string
	DataSize   uint64
	MaxSize    uint64
	Encryption uint64
	HashCode   uint64
	Encoding   uint64
	Pages      []r2007SectionPage
}

// r2007SectionMap holds every named section (§5.2).
type r2007SectionMap struct {
	Sections []r2007SectionDescriptor
}

func parseR2007SectionMap(data []byte) (r2007SectionMap, error) {
	var sections []r2007SectionDescriptor
	pos := 0
	for pos+0x40 <= len(data) {
		q := func(i int) uint64 { return binary.LittleEndian.Uint64(data[pos+i*8 : pos+i*8+8]) }
		dataSize := q(0)
		maxSize := q(1)
		encryption := q(2)
		hashCode := q(3)
		nameLen := int(q(4))
		encoding := q(6)
		numPages := int(q(7))
		pos += 0x40
		var name string
		if nameLen > 0 {
			if pos+nameLen > len(data) {
				return r2007SectionMap{}, newError(ErrSectionMap, "R2007 section map: section name runs past the end of the map")
			}
			raw := data[pos : pos+nameLen]
			units := make([]uint16, 0, len(raw)/2)
			for i := 0; i+1 < len(raw); i += 2 {
				units = append(units, binary.LittleEndian.Uint16(raw[i:i+2]))
			}
			name = string(utf16.Decode(units))
			for len(name) > 0 && name[len(name)-1] == 0 {
				name = name[:len(name)-1]
			}
			pos += nameLen
		}
		if numPages > r2007MaxPages {
			return r2007SectionMap{}, newError(ErrSectionMap, "R2007 section %q declares %d pages, above the cap", name, numPages)
		}
		pages := make([]r2007SectionPage, 0, numPages)
		for p := 0; p < numPages; p++ {
			if pos+56 > len(data) {
				return r2007SectionMap{}, newError(ErrSectionMap, "R2007 section map: page entry runs past the end of the map")
			}
			raw := data[pos : pos+56]
			qq := func(i int) uint64 { return binary.LittleEndian.Uint64(raw[i*8 : i*8+8]) }
			pages = append(pages, r2007SectionPage{
				DataOffset:   qq(0),
				Size:         qq(1),
				ID:           int64(binary.LittleEndian.Uint64(raw[16:24])),
				Uncompressed: qq(3),
				Compressed:   qq(4),
				Checksum:     qq(5),
				CRC:          qq(6),
			})
			pos += 56
		}
		if name == "" {
			continue
		}
		if len(sections) >= r2007MaxSections {
			return r2007SectionMap{}, newError(ErrSectionMap, "R2007 section map declares more than %d sections", r2007MaxSections)
		}
		sections = append(sections, r2007SectionDescriptor{
			Name:       name,
			DataSize:   dataSize,
			MaxSize:    maxSize,
			Encryption: encryption,
			HashCode:   hashCode,
			Encoding:   encoding,
			Pages:      pages,
		})
	}
	return r2007SectionMap{Sections: sections}, nil
}

func (m *r2007SectionMap) byName(name string) *r2007SectionDescriptor {
	for i := range m.Sections {
		if m.Sections[i].Name == name {
			return &m.Sections[i]
		}
	}
	return nil
}

// r2007Container is the parsed R2007 container.
type r2007Container struct {
	header     r2007FileHeader
	pageMap    r2007PageMap
	sectionMap r2007SectionMap
}

// parseR2007Container walks the whole R2007 container: file header → page
// map → section map.
func parseR2007Container(bytes []byte) (*r2007Container, error) {
	limits := DefaultDecompressLimits()
	header, err := parseR2007FileHeader(bytes, limits)
	if err != nil {
		return nil, err
	}
	pageMapOffset := r2007PageBase + int(header.PagesMapOffset)
	pageBytes, err := r2007LoadSystemPage(bytes, pageMapOffset,
		int(header.PagesMapSizeCompressed), int(header.PagesMapSizeUncompressed),
		int(header.PagesMapCorrectionFactor), limits)
	if err != nil {
		return nil, err
	}
	pageMap, err := parseR2007PageMap(pageBytes)
	if err != nil {
		return nil, err
	}
	sectionMapOffset, ok := pageMap.fileOffsetOf(int64(header.SectionsMapID))
	if !ok {
		return nil, newError(ErrSectionMap, "R2007 SectionsMapId %d is not present in the page map", header.SectionsMapID)
	}
	sectionBytes, err := r2007LoadSystemPage(bytes, sectionMapOffset,
		int(header.SectionsMapSizeCompressed), int(header.SectionsMapSizeUncompressed),
		int(header.SectionsMapCorrectionFactor), limits)
	if err != nil {
		return nil, err
	}
	sectionMap, err := parseR2007SectionMap(sectionBytes)
	if err != nil {
		return nil, err
	}
	return &r2007Container{header: header, pageMap: pageMap, sectionMap: sectionMap}, nil
}

// readSection assembles a named section's decompressed bytes (§5.4).
func (c *r2007Container) readSection(fileBytes []byte, name string) ([]byte, bool, error) {
	desc := c.sectionMap.byName(name)
	if desc == nil {
		return nil, false, nil
	}
	data, err := r2007ReadSection(fileBytes, &c.pageMap, desc, DefaultDecompressLimits())
	return data, true, err
}

func r2007ReadSection(bytes []byte, pageMap *r2007PageMap, desc *r2007SectionDescriptor, limits DecompressLimits) ([]byte, error) {
	if desc.Encryption != 0 && desc.Encryption != 2 {
		return nil, newError(ErrUnsupported, "R2007 section %q declares encryption %d (password-protected files are not supported)", desc.Name, desc.Encryption)
	}
	total := int(desc.DataSize)
	if total > limits.MaxOutputBytes {
		return nil, newError(ErrLZ77OutputLimitExceeded, "section size exceeds limit %d", limits.MaxOutputBytes)
	}
	out := make([]byte, 0, total)
	for i := range desc.Pages {
		page := &desc.Pages[i]
		want := int(page.DataOffset)
		if len(out) < want {
			if want > limits.MaxOutputBytes {
				return nil, newError(ErrLZ77OutputLimitExceeded, "section gap exceeds limit %d", limits.MaxOutputBytes)
			}
			for len(out) < want {
				out = append(out, 0)
			}
		}
		base, ok := pageMap.fileOffsetOf(page.ID)
		if !ok {
			return nil, newError(ErrSectionMap, "R2007 section %q references page id %d which the page map does not list", desc.Name, page.ID)
		}
		compressed := int(page.Compressed)
		uncompressed := int(page.Uncompressed)
		var stored []byte
		if desc.Encoding == 4 {
			blocks := compressed / r2007RSDataMessage
			if compressed%r2007RSDataMessage != 0 {
				blocks++
			}
			if blocks < 1 {
				blocks = 1
			}
			span := blocks * r2007RSCodeword
			if base+span > len(bytes) {
				return nil, newError(ErrTruncated, "R2007 data page needs %d bytes at %d", span, base)
			}
			decoded, err := r2007Deinterleave(bytes[base:base+span], blocks, r2007RSDataMessage)
			if err != nil {
				return nil, err
			}
			if compressed > len(decoded) {
				return nil, newError(ErrSectionMap, "R2007 section %q: page %d shorter than its compressed size", desc.Name, page.ID)
			}
			stored = append([]byte(nil), decoded[:compressed]...)
		} else {
			if base+compressed > len(bytes) {
				return nil, newError(ErrTruncated, "R2007 data page needs %d bytes at %d", compressed, base)
			}
			stored = append([]byte(nil), bytes[base:base+compressed]...)
		}
		var plain []byte
		if compressed < uncompressed {
			p, err := DecompressR2007(stored, uncompressed, limits)
			if err != nil {
				return nil, err
			}
			plain = p
		} else {
			plain = stored
		}
		take := uncompressed
		if take > len(plain) {
			take = len(plain)
		}
		out = append(out, plain[:take]...)
	}
	if len(out) > total {
		out = out[:total]
	}
	return out, nil
}

// extractR2007Sections converts a parsed container's section map into the
// generic Section list (§5.2).
func extractR2007Sections(container *r2007Container) []Section {
	out := make([]Section, 0, len(container.sectionMap.Sections))
	for i := range container.sectionMap.Sections {
		d := &container.sectionMap.Sections[i]
		var offset uint64
		if len(d.Pages) > 0 {
			if o, ok := container.pageMap.fileOffsetOf(d.Pages[0].ID); ok {
				offset = uint64(o)
			}
		}
		compressed := false
		for _, p := range d.Pages {
			if p.Compressed < p.Uncompressed {
				compressed = true
				break
			}
		}
		out = append(out, Section{
			Name:       d.Name,
			Kind:       SectionKindFromR2004Name(d.Name),
			Offset:     offset,
			Size:       d.DataSize,
			Compressed: compressed,
			Encrypted:  d.Encryption != 0,
		})
	}
	return out
}
