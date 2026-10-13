package dwg

import "encoding/binary"

// SectionPage is one row of the global Section Page Map.
type SectionPage struct {
	// Number is positive for real pages, negative for gaps.
	Number int32
	// Size is the on-disk size of this page in bytes.
	Size uint32
	// FileOffset is the computed absolute file offset of the page.
	FileOffset uint64
	// IsGap marks gap entries.
	IsGap bool
}

// SectionDescription is one named entry in the Section Info.
type SectionDescription struct {
	Size              uint64
	PageCount         uint32
	MaxDecompPageSize uint32
	Unknown           uint32
	Compressed        uint32
	SectionID         uint32
	Encrypted         uint32
	Name              string
	Pages             []SectionPageRef
}

// SectionPageRef is an entry inside a section description's page list.
type SectionPageRef struct {
	PageNumber  uint32
	DataSize    uint32
	StartOffset uint64
}

// DataPageHeader is the 32-byte Sec_Mask-encrypted header preceding every
// data section page (spec §4.6).
type DataPageHeader struct {
	PageType      uint32
	SectionNumber uint32
	DataSize      uint32
	PageSize      uint32
	StartOffset   uint32
	PageHeaderChk uint32
	DataChecksum  uint32
	Unknown       uint32
}

// DataPageType is the page-type tag for data-section pages.
const DataPageType = 0x4163043B

// ParseDataPageHeader parses the 0x20-byte data page header at fileOffset.
func ParseDataPageHeader(fileBytes []byte, fileOffset uint64) (DataPageHeader, error) {
	if fileOffset+0x20 > uint64(len(fileBytes)) {
		return DataPageHeader{}, newError(ErrTruncated, "data page header needs 0x20 bytes at %d", fileOffset)
	}
	var hdr [0x20]byte
	copy(hdr[:], fileBytes[fileOffset:fileOffset+0x20])
	mask := sectionPageMask(uint32(fileOffset))
	for i := 0; i < 0x20; i += 4 {
		v := binary.LittleEndian.Uint32(hdr[i : i+4])
		binary.LittleEndian.PutUint32(hdr[i:i+4], v^mask)
	}
	return DataPageHeader{
		PageType:      binary.LittleEndian.Uint32(hdr[0:4]),
		SectionNumber: binary.LittleEndian.Uint32(hdr[4:8]),
		DataSize:      binary.LittleEndian.Uint32(hdr[8:12]),
		PageSize:      binary.LittleEndian.Uint32(hdr[12:16]),
		StartOffset:   binary.LittleEndian.Uint32(hdr[16:20]),
		PageHeaderChk: binary.LittleEndian.Uint32(hdr[20:24]),
		DataChecksum:  binary.LittleEndian.Uint32(hdr[24:28]),
		Unknown:       binary.LittleEndian.Uint32(hdr[28:32]),
	}, nil
}

// readSectionPayload walks a section description's page list, decrypts
// each data page header, decompresses its payload if compressed, and
// assembles the full section bytes.
func readSectionPayload(fileBytes []byte, pageMap []SectionPage, desc *SectionDescription) ([]byte, error) {
	totalSize := int(desc.Size)
	result := make([]byte, totalSize)
	for _, pref := range desc.Pages {
		var page *SectionPage
		for i := range pageMap {
			if !pageMap[i].IsGap && pageMap[i].Number == int32(pref.PageNumber) {
				page = &pageMap[i]
				break
			}
		}
		if page == nil {
			return nil, newError(ErrSectionMap, "section %q references page %d not in page map", desc.Name, pref.PageNumber)
		}
		hdr, err := ParseDataPageHeader(fileBytes, page.FileOffset)
		if err != nil {
			return nil, err
		}
		if hdr.PageType != DataPageType {
			return nil, newError(ErrSectionMap, "section %q page %d has wrong page_type 0x%x", desc.Name, pref.PageNumber, hdr.PageType)
		}
		payloadStart := page.FileOffset + 0x20
		payloadEnd := payloadStart + uint64(hdr.DataSize)
		if payloadEnd > uint64(len(fileBytes)) {
			return nil, newError(ErrTruncated, "section page payload needs %d bytes at %d", hdr.DataSize, payloadStart)
		}
		payload := fileBytes[payloadStart:payloadEnd]
		var decompressed []byte
		switch desc.Compressed {
		case 1:
			decompressed = append([]byte(nil), payload...)
		case 2:
			d, err := Decompress(payload, int(hdr.PageSize))
			if err != nil {
				return nil, err
			}
			decompressed = d
		default:
			return nil, newError(ErrSectionMap, "section %q unknown compression flag %d", desc.Name, desc.Compressed)
		}
		start := int(pref.StartOffset)
		if start >= totalSize {
			continue
		}
		take := len(decompressed)
		if take > totalSize-start {
			take = totalSize - start
		}
		copy(result[start:start+take], decompressed[:take])
	}
	return result, nil
}

// systemPageHeader is the 20-byte plaintext header of a system page.
type systemPageHeader struct {
	pageType   uint32
	decompSize uint32
	compSize   uint32
	compType   uint32
	checksum   uint32
}

const (
	sysPageMapType    = 0x41630E3B
	sysSectionMapType = 0x4163003B
)

func parseSystemPageHeader(fileBytes []byte, fileOffset uint64) (systemPageHeader, error) {
	if fileOffset+0x14 > uint64(len(fileBytes)) {
		return systemPageHeader{}, newError(ErrTruncated, "system page header needs 0x14 bytes at %d", fileOffset)
	}
	hdr := fileBytes[fileOffset : fileOffset+0x14]
	return systemPageHeader{
		pageType:   binary.LittleEndian.Uint32(hdr[0:4]),
		decompSize: binary.LittleEndian.Uint32(hdr[4:8]),
		compSize:   binary.LittleEndian.Uint32(hdr[8:12]),
		compType:   binary.LittleEndian.Uint32(hdr[12:16]),
		checksum:   binary.LittleEndian.Uint32(hdr[16:20]),
	}, nil
}

// ParsePageMap decompresses and parses the global Section Page Map.
func ParsePageMap(fileBytes []byte, header *R2004Header) ([]SectionPage, error) {
	pageOffset := header.SectionPageMapAddr + 0x100
	sysHdr, err := parseSystemPageHeader(fileBytes, pageOffset)
	if err != nil {
		return nil, err
	}
	if sysHdr.pageType != sysPageMapType {
		return nil, newError(ErrSectionMap, "page map type tag 0x%x != expected 0x%x", sysHdr.pageType, sysPageMapType)
	}
	if sysHdr.compType != 2 {
		return nil, newError(ErrSectionMap, "page map comp_type %d != 2", sysHdr.compType)
	}
	payloadStart := pageOffset + 0x14
	payloadEnd := payloadStart + uint64(sysHdr.compSize)
	if payloadEnd > uint64(len(fileBytes)) {
		return nil, newError(ErrTruncated, "page map payload needs %d bytes at %d", sysHdr.compSize, payloadStart)
	}
	decompressed, err := Decompress(fileBytes[payloadStart:payloadEnd], int(sysHdr.decompSize))
	if err != nil {
		return nil, err
	}
	return decodePageMapEntries(decompressed)
}

func decodePageMapEntries(data []byte) ([]SectionPage, error) {
	var out []SectionPage
	i := 0
	fileOffset := uint64(0x100)
	for i+8 <= len(data) {
		number := int32(binary.LittleEndian.Uint32(data[i : i+4]))
		size := binary.LittleEndian.Uint32(data[i+4 : i+8])
		i += 8
		isGap := number < 0
		if isGap {
			if i+16 > len(data) {
				return nil, newError(ErrSectionMap, "page map gap record truncated")
			}
			i += 16
		}
		out = append(out, SectionPage{Number: number, Size: size, FileOffset: fileOffset, IsGap: isGap})
		fileOffset += uint64(size)
	}
	return out, nil
}

// ParseSectionInfo locates the Section Map page, decompresses its body,
// and emits the named section list.
func ParseSectionInfo(fileBytes []byte, header *R2004Header, pageMap []SectionPage) ([]SectionDescription, error) {
	var smPage *SectionPage
	for i := range pageMap {
		if pageMap[i].Number == int32(header.SectionMapID) {
			smPage = &pageMap[i]
			break
		}
	}
	if smPage == nil {
		return nil, newError(ErrSectionMap, "section_map_id %d not found in %d page map entries", header.SectionMapID, len(pageMap))
	}
	sysHdr, err := parseSystemPageHeader(fileBytes, smPage.FileOffset)
	if err != nil {
		return nil, err
	}
	if sysHdr.pageType != sysSectionMapType {
		return nil, newError(ErrSectionMap, "section map type 0x%x != expected 0x%x", sysHdr.pageType, sysSectionMapType)
	}
	if sysHdr.compType != 2 {
		return nil, newError(ErrSectionMap, "section map comp_type %d != 2", sysHdr.compType)
	}
	payloadStart := smPage.FileOffset + 0x14
	payloadEnd := payloadStart + uint64(sysHdr.compSize)
	if payloadEnd > uint64(len(fileBytes)) {
		return nil, newError(ErrTruncated, "section map payload needs %d bytes at %d", sysHdr.compSize, payloadStart)
	}
	decompressed, err := Decompress(fileBytes[payloadStart:payloadEnd], int(sysHdr.decompSize))
	if err != nil {
		return nil, err
	}
	return decodeSectionDescriptions(decompressed)
}

func decodeSectionDescriptions(data []byte) ([]SectionDescription, error) {
	if len(data) < 0x14 {
		return nil, newError(ErrSectionMap, "section info shorter than 0x14-byte header: %d bytes", len(data))
	}
	numDescriptions := binary.LittleEndian.Uint32(data[0x00:0x04])
	cursor := 0x14
	out := make([]SectionDescription, 0, numDescriptions)
	for d := uint32(0); d < numDescriptions; d++ {
		if cursor+0x60 > len(data) {
			return nil, newError(ErrSectionMap, "section description #%d at offset 0x%x overruns buffer", len(out), cursor)
		}
		size := binary.LittleEndian.Uint64(data[cursor : cursor+8])
		pageCount := binary.LittleEndian.Uint32(data[cursor+8 : cursor+12])
		maxDecomp := binary.LittleEndian.Uint32(data[cursor+12 : cursor+16])
		unknown := binary.LittleEndian.Uint32(data[cursor+16 : cursor+20])
		compressed := binary.LittleEndian.Uint32(data[cursor+20 : cursor+24])
		sectionID := binary.LittleEndian.Uint32(data[cursor+24 : cursor+28])
		encrypted := binary.LittleEndian.Uint32(data[cursor+28 : cursor+32])
		nameBytes := data[cursor+32 : cursor+96]
		nameEnd := len(nameBytes)
		for i, b := range nameBytes {
			if b == 0 {
				nameEnd = i
				break
			}
		}
		name := string(nameBytes[:nameEnd])
		cursor += 0x60

		pages := make([]SectionPageRef, 0, pageCount)
		for p := uint32(0); p < pageCount; p++ {
			if cursor+16 > len(data) {
				return nil, newError(ErrSectionMap, "page ref %d of section %q at 0x%x overruns buffer", p, name, cursor)
			}
			pages = append(pages, SectionPageRef{
				PageNumber:  binary.LittleEndian.Uint32(data[cursor : cursor+4]),
				DataSize:    binary.LittleEndian.Uint32(data[cursor+4 : cursor+8]),
				StartOffset: binary.LittleEndian.Uint64(data[cursor+8 : cursor+16]),
			})
			cursor += 16
		}
		out = append(out, SectionDescription{
			Size:              size,
			PageCount:         pageCount,
			MaxDecompPageSize: maxDecomp,
			Unknown:           unknown,
			Compressed:        compressed,
			SectionID:         sectionID,
			Encrypted:         encrypted,
			Name:              name,
			Pages:             pages,
		})
	}
	return out, nil
}
