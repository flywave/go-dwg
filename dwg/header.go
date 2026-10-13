package dwg

import "encoding/binary"

// CommonHeader is the parsed 128-byte prefix all DWG files share.
type CommonHeader struct {
	Version     Version
	MaintVer    uint8
	ImageSeeker uint32
	Codepage    uint16
}

// ParseCommonHeader parses the first 0x15 bytes both families agree on.
func ParseCommonHeader(bytes []byte) (CommonHeader, error) {
	if len(bytes) < 0x15 {
		return CommonHeader{}, newError(ErrTruncated, "common header needs 0x15 bytes, got %d", len(bytes))
	}
	v, err := VersionFromMagic(bytes[0:6])
	if err != nil {
		return CommonHeader{}, err
	}
	return CommonHeader{
		Version:     v,
		MaintVer:    bytes[0x0B],
		ImageSeeker: binary.LittleEndian.Uint32(bytes[0x0D:0x11]),
		Codepage:    binary.LittleEndian.Uint16(bytes[0x13:0x15]),
	}, nil
}

// LocatorRecord is an R13-R15 section-locator record — 9 bytes on disk.
type LocatorRecord struct {
	Number uint8
	Seeker uint32
	Size   uint32
}

// LocatorRecordSize is the on-disk size of one locator record.
const LocatorRecordSize = 9

// ParseLocatorRecord parses one 9-byte section locator record.
func ParseLocatorRecord(bytes []byte) (LocatorRecord, error) {
	if len(bytes) < LocatorRecordSize {
		return LocatorRecord{}, newError(ErrSectionLocator, "locator record needs %d bytes, got %d", LocatorRecordSize, len(bytes))
	}
	return LocatorRecord{
		Number: bytes[0],
		Seeker: binary.LittleEndian.Uint32(bytes[1:5]),
		Size:   binary.LittleEndian.Uint32(bytes[5:9]),
	}, nil
}

// R13R15Header is a fully-parsed R13-R15 header.
type R13R15Header struct {
	Common       CommonHeader
	LocatorCount uint32
	Locators     []LocatorRecord
}

// ParseR13R15Header parses the R13-R15 header (spec §3.2.6): at offset
// 0x15 a u32 count, then that many 9-byte locator records.
func ParseR13R15Header(bytes []byte) (R13R15Header, error) {
	common, err := ParseCommonHeader(bytes)
	if err != nil {
		return R13R15Header{}, err
	}
	if len(bytes) < 0x19 {
		return R13R15Header{}, newError(ErrTruncated, "R13-R15 header needs 0x19 bytes, got %d", len(bytes))
	}
	count := binary.LittleEndian.Uint32(bytes[0x15:0x19])
	needed := 0x19 + int(count)*LocatorRecordSize
	if len(bytes) < needed {
		return R13R15Header{}, newError(ErrTruncated, "R13-R15 locator table needs %d bytes, got %d", needed, len(bytes))
	}
	locators := make([]LocatorRecord, 0, count)
	for i := 0; i < int(count); i++ {
		start := 0x19 + i*LocatorRecordSize
		rec, err := ParseLocatorRecord(bytes[start : start+LocatorRecordSize])
		if err != nil {
			return R13R15Header{}, err
		}
		locators = append(locators, rec)
	}
	return R13R15Header{Common: common, LocatorCount: count, Locators: locators}, nil
}

// IntoSections converts locator records into the generic Section list.
func (h R13R15Header) IntoSections() []Section {
	out := make([]Section, 0, len(h.Locators))
	for _, r := range h.Locators {
		var name string
		switch r.Number {
		case 0:
			name = "AcDb:Header"
		case 1:
			name = "AcDb:Classes"
		case 2:
			name = "AcDb:Handles"
		case 3:
			name = "R13C3_UNKNOWN"
		case 4:
			name = "AcDb:Template"
		default:
			name = "RECORD_" + string(rune('0'+r.Number))
		}
		out = append(out, Section{
			Name:   name,
			Kind:   SectionKindFromR13Record(r.Number),
			Offset: uint64(r.Seeker),
			Size:   uint64(r.Size),
		})
	}
	return out
}

// R2004Header is the parsed 0x6C-byte decrypted header payload at file
// offset 0x80 plus the plaintext prefix.
type R2004Header struct {
	Common     CommonHeader
	SecFlags   uint32
	SummaryAdr uint32
	VBAAddr    uint32

	FileID             [12]byte
	LastSectionPageID  uint32
	LastSectionPageEnd uint64
	SecondHeaderAddr   uint64
	GapAmount          uint32
	SectionPageAmount  uint32
	SectionPageMapID   uint32
	SectionPageMapAddr uint64
	SectionMapID       uint32
	SectionPageArrSize uint32
	GapArraySize       uint32
	CRC32Stored        uint32
}

// ParseR2004Header parses the first 0xEC bytes of an R2004+ DWG file.
func ParseR2004Header(bytes []byte) (R2004Header, error) {
	if len(bytes) < 0xEC {
		return R2004Header{}, newError(ErrTruncated, "R2004+ header needs 0xEC bytes, got %d", len(bytes))
	}
	common, err := ParseCommonHeader(bytes)
	if err != nil {
		return R2004Header{}, err
	}
	if !common.Version.IsR2004Family() {
		return R2004Header{}, newError(ErrR2004Decrypt, "expected R2004-family version, got %s", common.Version)
	}
	var block [MagicLen]byte
	copy(block[:], bytes[0x80:0x80+MagicLen])
	xorInPlace(block[:])

	if string(block[:11]) != "AcFssFcAJMB" {
		return R2004Header{}, newError(ErrR2004Decrypt, "decrypted file ID mismatch: got %q", block[:12])
	}
	h := R2004Header{
		Common:             common,
		SecFlags:           binary.LittleEndian.Uint32(bytes[0x18:0x1C]),
		SummaryAdr:         binary.LittleEndian.Uint32(bytes[0x20:0x24]),
		VBAAddr:            binary.LittleEndian.Uint32(bytes[0x24:0x28]),
		LastSectionPageID:  binary.LittleEndian.Uint32(block[0x28:0x2C]),
		LastSectionPageEnd: binary.LittleEndian.Uint64(block[0x2C:0x34]),
		SecondHeaderAddr:   binary.LittleEndian.Uint64(block[0x34:0x3C]),
		GapAmount:          binary.LittleEndian.Uint32(block[0x3C:0x40]),
		SectionPageAmount:  binary.LittleEndian.Uint32(block[0x40:0x44]),
		SectionPageMapID:   binary.LittleEndian.Uint32(block[0x50:0x54]),
		SectionPageMapAddr: binary.LittleEndian.Uint64(block[0x54:0x5C]),
		SectionMapID:       binary.LittleEndian.Uint32(block[0x5C:0x60]),
		SectionPageArrSize: binary.LittleEndian.Uint32(block[0x60:0x64]),
		GapArraySize:       binary.LittleEndian.Uint32(block[0x64:0x68]),
		CRC32Stored:        binary.LittleEndian.Uint32(block[0x68:0x6C]),
	}
	copy(h.FileID[:], block[0x00:0x0C])
	return h, nil
}

// ValidateR2004HeaderCRC computes the CRC-32 over the decrypted header
// block (stored CRC bytes zeroed) and returns (expected, actual).
func ValidateR2004HeaderCRC(bytes []byte) (uint32, uint32, error) {
	if len(bytes) < 0xEC {
		return 0, 0, newError(ErrTruncated, "R2004+ header needs 0xEC bytes, got %d", len(bytes))
	}
	var block [MagicLen]byte
	copy(block[:], bytes[0x80:0x80+MagicLen])
	xorInPlace(block[:])
	expected := binary.LittleEndian.Uint32(block[0x68:0x6C])
	for i := 0x68; i < 0x6C; i++ {
		block[i] = 0
	}
	actual := CRC32(0, block[:])
	return expected, actual, nil
}
