package dwg

import "os"

// SectionMapStatus reports how a DwgFile's section list was derived.
type SectionMapStatus struct {
	State  SectionMapState
	Reason string
}

// SectionMapState enumerates the section-list provenances.
type SectionMapState uint8

const (
	// StatusFull means the section list is authoritative.
	StatusFull SectionMapState = iota
	// StatusFallback means an R2004-family full walk failed and a stub
	// enumeration was synthesized.
	StatusFallback
	// StatusDeferred means the container could not be walked and the list
	// is a placeholder.
	StatusDeferred
)

// ParseDiagnostics summarizes a best-effort parse.
type ParseDiagnostics struct {
	SectionMapStatus SectionMapStatus
	Warnings         []string
}

// Summary is a high-level overview of a parsed DWG file.
type Summary struct {
	Version              Version
	FileSizeBytes        uint64
	SectionCount         int
	SectionMapStatus     SectionMapStatus
	HasR2004Header       bool
	HasR13Header         bool
	HasR2007CommonHeader bool
}

// DwgFile is a parsed DWG file held entirely in memory.
type DwgFile struct {
	bytes          []byte
	version        Version
	sections       []Section
	status         SectionMapStatus
	r2004          *R2004Header
	r13            *R13R15Header
	r2007Common    *CommonHeader
	r2007Container *r2007Container
}

// OpenDwg opens a DWG file at path, reads it into memory, and enumerates
// its sections.
func OpenDwg(path string) (*DwgFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DwgFromBytes(bytes)
}

// DwgFromBytes parses an already-loaded byte buffer.
func DwgFromBytes(bytes []byte) (*DwgFile, error) {
	if len(bytes) < 6 {
		return nil, newError(ErrTruncated, "file needs at least 6 bytes, got %d", len(bytes))
	}
	version, err := VersionFromMagic(bytes[0:6])
	if err != nil {
		return nil, err
	}

	switch {
	case version.IsR13R15():
		header, err := ParseR13R15Header(bytes)
		if err != nil {
			return nil, err
		}
		return &DwgFile{
			bytes:    bytes,
			version:  version,
			sections: header.IntoSections(),
			status:   SectionMapStatus{State: StatusFull},
			r13:      &header,
		}, nil
	case version.IsR2004Family():
		header, err := ParseR2004Header(bytes)
		if err != nil {
			return nil, err
		}
		sections, status := extractR2004Sections(bytes, &header)
		return &DwgFile{
			bytes:    bytes,
			version:  version,
			sections: sections,
			status:   status,
			r2004:    &header,
		}, nil
	default: // R2007
		common, err := ParseCommonHeader(bytes)
		if err != nil {
			return nil, err
		}
		container, cerr := parseR2007Container(bytes)
		if cerr == nil {
			return &DwgFile{
				bytes:          bytes,
				version:        version,
				sections:       extractR2007Sections(container),
				status:         SectionMapStatus{State: StatusFull},
				r2007Common:    &common,
				r2007Container: container,
			}, nil
		}
		sections := r2007Placeholder(common, bytes)
		return &DwgFile{
			bytes:       bytes,
			version:     version,
			sections:    sections,
			status:      SectionMapStatus{State: StatusDeferred, Reason: "R2007 container parse failed (" + cerr.Error() + "); placeholder covers the post-header byte range only"},
			r2007Common: &common,
		}, nil
	}
}

// SectionMapStatus returns how the section list was derived.
func (f *DwgFile) SectionMapStatus() SectionMapStatus { return f.status }

// Version returns the detected format version.
func (f *DwgFile) Version() Version { return f.version }

// FileSize returns the total file size in bytes.
func (f *DwgFile) FileSize() uint64 { return uint64(len(f.bytes)) }

// Sections returns all enumerated sections.
func (f *DwgFile) Sections() []Section { return f.sections }

// SectionOfKind finds the first section with a given kind.
func (f *DwgFile) SectionOfKind(kind SectionKind) *Section {
	for i := range f.sections {
		if f.sections[i].Kind == kind {
			return &f.sections[i]
		}
	}
	return nil
}

// SectionByName finds a section by exact name.
func (f *DwgFile) SectionByName(name string) *Section {
	for i := range f.sections {
		if f.sections[i].Name == name {
			return &f.sections[i]
		}
	}
	return nil
}

// R2004Header returns the parsed R2004+ header, if applicable.
func (f *DwgFile) R2004Header() *R2004Header { return f.r2004 }

// R13Header returns the parsed R13-R15 header, if applicable.
func (f *DwgFile) R13Header() *R13R15Header { return f.r13 }

// ReadSection reads the decompressed bytes of a named section. Returns
// (nil, false) when the section is not present.
func (f *DwgFile) ReadSection(name string) ([]byte, bool, error) {
	if f.version.IsR13R15() {
		return f.readSectionR13(name)
	}
	if f.r2007Container != nil {
		return f.r2007Container.readSection(f.bytes, name)
	}
	if f.r2004 == nil {
		return nil, false, nil
	}
	if f.SectionByName(name) == nil {
		return nil, false, nil
	}
	b, err := f.readSectionR2004(f.r2004, name)
	return b, true, err
}

func (f *DwgFile) readSectionR13(name string) ([]byte, bool, error) {
	if name == "AcDb:AcDbObjects" {
		return append([]byte(nil), f.bytes...), true, nil
	}
	s := f.SectionByName(name)
	if s == nil {
		return nil, false, nil
	}
	start := s.Offset
	end := start + s.Size
	if end > uint64(len(f.bytes)) {
		return nil, true, newError(ErrTruncated, "section %q needs bytes %d..%d, file is %d", name, start, end, len(f.bytes))
	}
	return append([]byte(nil), f.bytes[start:end]...), true, nil
}

func (f *DwgFile) readSectionR2004(header *R2004Header, name string) ([]byte, error) {
	pageMap, err := ParsePageMap(f.bytes, header)
	if err != nil {
		return nil, err
	}
	descs, err := ParseSectionInfo(f.bytes, header, pageMap)
	if err != nil {
		return nil, err
	}
	var desc *SectionDescription
	for i := range descs {
		if descs[i].Name == name {
			desc = &descs[i]
			break
		}
	}
	if desc == nil {
		return nil, newError(ErrSectionMap, "section %q not found", name)
	}
	return readSectionPayload(f.bytes, pageMap, desc)
}

// ObjectStream returns the AcDb:AcDbObjects bytes (or the whole file for
// R13-R15).
func (f *DwgFile) ObjectStream() ([]byte, bool, error) {
	return f.ReadSection("AcDb:AcDbObjects")
}

// SummaryInfo reads and parses AcDb:SummaryInfo.
func (f *DwgFile) SummaryInfo() (SummaryInfo, bool, error) {
	b, ok, err := f.ReadSection("AcDb:SummaryInfo")
	if !ok || err != nil {
		return SummaryInfo{}, ok, err
	}
	si, err := ParseSummaryInfo(b)
	return si, true, err
}

// AppInfo reads and parses AcDb:AppInfo.
func (f *DwgFile) AppInfo() (AppInfo, bool, error) {
	b, ok, err := f.ReadSection("AcDb:AppInfo")
	if !ok || err != nil {
		return AppInfo{}, ok, err
	}
	ai, err := ParseAppInfo(b)
	return ai, true, err
}

// Preview reads and parses AcDb:Preview.
func (f *DwgFile) Preview() (Preview, bool, error) {
	b, ok, err := f.ReadSection("AcDb:Preview")
	if !ok || err != nil {
		return Preview{}, ok, err
	}
	p, err := ParsePreview(b)
	return p, true, err
}

// FileDepList reads and parses AcDb:FileDepList.
func (f *DwgFile) FileDepList() (FileDepList, bool, error) {
	b, ok, err := f.ReadSection("AcDb:FileDepList")
	if !ok || err != nil {
		return FileDepList{}, ok, err
	}
	fdl, err := ParseFileDepList(b)
	return fdl, true, err
}

// HandleMap reads and parses AcDb:Handles.
func (f *DwgFile) HandleMap() (HandleMap, bool, error) {
	b, ok, err := f.ReadSection("AcDb:Handles")
	if !ok || err != nil {
		return HandleMap{}, ok, err
	}
	hm, err := ParseHandleMap(b)
	return hm, true, err
}

// ClassMap reads and parses AcDb:Classes.
func (f *DwgFile) ClassMap() (ClassMap, bool, error) {
	b, ok, err := f.ReadSection("AcDb:Classes")
	if !ok || err != nil {
		return ClassMap{}, ok, err
	}
	return ParseClassMap(b, f.version), true, nil
}

// HeaderVars reads and parses AcDb:Header.
func (f *DwgFile) HeaderVars() (HeaderVars, bool, error) {
	b, ok, err := f.ReadSection("AcDb:Header")
	if !ok || err != nil {
		return HeaderVars{}, ok, err
	}
	return ParseHeaderVars(b, f.version), true, nil
}

// Objects walks the AcDb:AcDbObjects stream sequentially.
func (f *DwgFile) Objects() ([]RawObject, bool, error) {
	b, ok, err := f.ObjectStream()
	if !ok || err != nil {
		return nil, ok, err
	}
	objs, err := NewObjectWalker(b, f.version).CollectAll()
	return objs, true, err
}

// AllObjects walks every object via the handle map.
func (f *DwgFile) AllObjects() ([]RawObject, bool, error) {
	hmap, ok, err := f.HandleMap()
	if !ok || err != nil {
		return nil, ok, err
	}
	b, ok, err := f.ObjectStream()
	if !ok || err != nil {
		return nil, ok, err
	}
	objs, err := WithHandleMap(b, f.version, &hmap).CollectAll()
	return objs, true, err
}

// AllObjectsLossy walks every object via the handle map, returning the
// walk summary alongside.
func (f *DwgFile) AllObjectsLossy() ([]RawObject, ObjectWalkSummary, bool, error) {
	hmap, ok, err := f.HandleMap()
	if !ok || err != nil {
		return nil, ObjectWalkSummary{}, ok, err
	}
	b, ok, err := f.ObjectStream()
	if !ok || err != nil {
		return nil, ObjectWalkSummary{}, ok, err
	}
	objs, summary := WithHandleMap(b, f.version, &hmap).CollectAllLossy()
	return objs, summary, true, nil
}

// RawBytes returns the raw file bytes.
func (f *DwgFile) RawBytes() []byte { return f.bytes }

// Summary returns a high-level overview of the file.
func (f *DwgFile) Summary() Summary {
	return Summary{
		Version:              f.version,
		FileSizeBytes:        uint64(len(f.bytes)),
		SectionCount:         len(f.sections),
		SectionMapStatus:     f.status,
		HasR2004Header:       f.r2004 != nil,
		HasR13Header:         f.r13 != nil,
		HasR2007CommonHeader: f.r2007Common != nil,
	}
}

func extractR2004Sections(bytes []byte, header *R2004Header) ([]Section, SectionMapStatus) {
	full, err := extractR2004SectionsFull(bytes, header)
	if err == nil && len(full) > 0 {
		return full, SectionMapStatus{State: StatusFull}
	}
	stub, serr := extractR2004SectionsStub(bytes, header)
	if serr != nil {
		return nil, SectionMapStatus{State: StatusFallback, Reason: "stub enumeration failed: " + serr.Error()}
	}
	reason := "R2004 full section-map walk returned zero entries; fell back to stub enumeration"
	if err != nil {
		reason = "R2004 full section-map walk failed (" + err.Error() + "); fell back to stub enumeration"
	}
	return stub, SectionMapStatus{State: StatusFallback, Reason: reason}
}

func extractR2004SectionsFull(bytes []byte, header *R2004Header) ([]Section, error) {
	pageMap, err := ParsePageMap(bytes, header)
	if err != nil {
		return nil, err
	}
	descs, err := ParseSectionInfo(bytes, header, pageMap)
	if err != nil {
		return nil, err
	}
	pageOffset := make(map[int32]uint64, len(pageMap))
	for _, p := range pageMap {
		if !p.IsGap {
			pageOffset[p.Number] = p.FileOffset
		}
	}
	var out []Section
	for i := range descs {
		d := &descs[i]
		if d.Name == "" {
			continue
		}
		var firstOffset uint64
		if len(d.Pages) > 0 {
			firstOffset = pageOffset[int32(d.Pages[0].PageNumber)]
		}
		out = append(out, Section{
			Name:       d.Name,
			Kind:       SectionKindFromR2004Name(d.Name),
			Offset:     firstOffset,
			Size:       d.Size,
			Compressed: d.Compressed == 2,
			Encrypted:  d.Encrypted == 1,
		})
	}
	return out, nil
}

func extractR2004SectionsStub(bytes []byte, header *R2004Header) ([]Section, error) {
	var out []Section
	pageMapOffset := header.SectionPageMapAddr + 0x100
	out = append(out, Section{Name: "SectionPageMap", Kind: KindSystemSection, Offset: pageMapOffset, Compressed: true})
	if header.SummaryAdr != 0 {
		out = append(out, Section{Name: "AcDb:SummaryInfo", Kind: KindSummaryInfo, Offset: uint64(header.SummaryAdr) + 0x20})
	}
	if header.VBAAddr != 0 {
		out = append(out, Section{Name: "AcDb:VBAProject", Kind: KindVbaProject, Offset: uint64(header.VBAAddr) + 0x20})
	}
	if header.Common.ImageSeeker != 0 && uint64(header.Common.ImageSeeker) < uint64(len(bytes)) {
		out = append(out, Section{Name: "AcDb:Preview", Kind: KindPreview, Offset: uint64(header.Common.ImageSeeker) + 0x20})
	}
	if header.SecondHeaderAddr != 0 && header.SecondHeaderAddr < uint64(len(bytes)) {
		out = append(out, Section{Name: "SecondHeader", Kind: KindSystemSection, Offset: header.SecondHeaderAddr})
	}
	if len(bytes) >= 0x120 {
		if s, ok := sniffFirstPageAt0x100(bytes); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func sniffFirstPageAt0x100(bytes []byte) (Section, bool) {
	const hdrOff = 0x100
	if len(bytes) < hdrOff+0x20 {
		return Section{}, false
	}
	var hdr [0x20]byte
	copy(hdr[:], bytes[hdrOff:hdrOff+0x20])
	mask := sectionPageMask(hdrOff)
	for i := 0; i < 0x20; i += 4 {
		v := uint32(hdr[i]) | uint32(hdr[i+1])<<8 | uint32(hdr[i+2])<<16 | uint32(hdr[i+3])<<24
		v ^= mask
		hdr[i] = byte(v)
		hdr[i+1] = byte(v >> 8)
		hdr[i+2] = byte(v >> 16)
		hdr[i+3] = byte(v >> 24)
	}
	pageType := uint32(hdr[0]) | uint32(hdr[1])<<8 | uint32(hdr[2])<<16 | uint32(hdr[3])<<24
	switch pageType {
	case 0x41630E3B, 0x4163003B, 0x4163043B:
	default:
		return Section{}, false
	}
	decompSize := uint32(hdr[4]) | uint32(hdr[5])<<8 | uint32(hdr[6])<<16 | uint32(hdr[7])<<24
	compSize := uint32(hdr[8]) | uint32(hdr[9])<<8 | uint32(hdr[10])<<16 | uint32(hdr[11])<<24
	kind := KindSystemSection
	var name string
	switch pageType {
	case 0x41630E3B:
		name = "SystemSection(PageMap)"
	case 0x4163003B:
		name = "SystemSection(SectionMap)"
	default:
		name = "DataPage(decomp=" + u32str(decompSize) + ")"
		kind = KindUnknown
	}
	return Section{Name: name, Kind: kind, Offset: hdrOff + 0x20, Size: uint64(compSize), Compressed: true, Encrypted: true}, true
}

func u32str(v uint32) string {
	if v == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func r2007Placeholder(common CommonHeader, bytes []byte) []Section {
	var out []Section
	if common.ImageSeeker != 0 && uint64(common.ImageSeeker) < uint64(len(bytes)) {
		out = append(out, Section{Name: "AcDb:Preview", Kind: KindPreview, Offset: uint64(common.ImageSeeker) + 0x20})
	}
	out = append(out, Section{Name: "_R2007_UNREADABLE", Kind: KindUnknown, Offset: 0x80, Size: uint64(len(bytes)) - 0x80, Compressed: true})
	return out
}
