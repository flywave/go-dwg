package dwg

import "fmt"

// Section is a named section in a DWG file (either version family).
type Section struct {
	// Name is the canonical name — the record slot label for R13-R15,
	// or the on-disk string for R2004+.
	Name string
	// Kind classifies the section by role.
	Kind SectionKind
	// Offset is the absolute byte offset where the section payload begins.
	Offset uint64
	// Size is the payload size in bytes (compressed size for compressed
	// sections).
	Size uint64
	// Compressed reports R2004+ LZ77 compression.
	Compressed bool
	// Encrypted reports R2004+ Sec_Mask XOR encryption.
	Encrypted bool

	// DecodeAttempted / DecodeSucceeded / DecompressedBytes are post-hoc
	// diagnostics populated after a read_section call. They default to
	// "not attempted".
	DecodeAttempted   bool
	DecodeSucceeded   bool
	DecompressedBytes *int
}

// MarkDecodeSuccess records a successful decode with its output size.
func (s *Section) MarkDecodeSuccess(decompressedBytes int) {
	s.DecodeAttempted = true
	s.DecodeSucceeded = true
	n := decompressedBytes
	s.DecompressedBytes = &n
}

// MarkDecodeFailure records a failed decode attempt.
func (s *Section) MarkDecodeFailure() {
	s.DecodeAttempted = true
	s.DecodeSucceeded = false
}

// CompressionRatio is decompressed/on-disk size, or (0, false) when
// unknown or the on-disk size is zero.
func (s *Section) CompressionRatio() (float64, bool) {
	if s.DecompressedBytes == nil || s.Size == 0 {
		return 0, false
	}
	return float64(*s.DecompressedBytes) / float64(s.Size), true
}

// SectionKind classifies a section by role.
type SectionKind uint8

const (
	KindHeader SectionKind = iota
	KindAuxHeader
	KindClasses
	KindHandles
	KindTemplate
	KindObjFreeSpace
	KindObjects
	KindRevHistory
	KindSummaryInfo
	KindPreview
	KindAppInfo
	KindAppInfoHistory
	KindFileDepList
	KindSecurity
	KindVbaProject
	KindSignature
	KindAcDsData
	KindSystemSection
	KindR13MeasurementPtr
	KindUnknown
)

// SectionKindFromR13Record maps an R13-R15 record number to its kind.
func SectionKindFromR13Record(n uint8) SectionKind {
	switch n {
	case 0:
		return KindHeader
	case 1:
		return KindClasses
	case 2:
		return KindHandles
	case 4:
		return KindR13MeasurementPtr
	default:
		return KindUnknown
	}
}

// SectionKindFromR2004Name maps an R2004+ on-disk section name to its kind.
func SectionKindFromR2004Name(name string) SectionKind {
	switch name {
	case "AcDb:Header":
		return KindHeader
	case "AcDb:AuxHeader":
		return KindAuxHeader
	case "AcDb:Classes":
		return KindClasses
	case "AcDb:Handles":
		return KindHandles
	case "AcDb:Template":
		return KindTemplate
	case "AcDb:ObjFreeSpace":
		return KindObjFreeSpace
	case "AcDb:AcDbObjects":
		return KindObjects
	case "AcDb:RevHistory":
		return KindRevHistory
	case "AcDb:SummaryInfo":
		return KindSummaryInfo
	case "AcDb:Preview":
		return KindPreview
	case "AcDb:AppInfo":
		return KindAppInfo
	case "AcDb:AppInfoHistory":
		return KindAppInfoHistory
	case "AcDb:FileDepList":
		return KindFileDepList
	case "AcDb:Security":
		return KindSecurity
	case "AcDb:VBAProject":
		return KindVbaProject
	case "AcDb:Signature":
		return KindSignature
	}
	if len(name) >= 8 && name[:8] == "AcDb:AcD" {
		return KindAcDsData
	}
	return KindUnknown
}

// ShortLabel returns a stable one-word label used in CLI output.
func (k SectionKind) ShortLabel() string {
	switch k {
	case KindHeader:
		return "header"
	case KindAuxHeader:
		return "auxheader"
	case KindClasses:
		return "classes"
	case KindHandles:
		return "handles"
	case KindTemplate:
		return "template"
	case KindObjFreeSpace:
		return "objfree"
	case KindObjects:
		return "objects"
	case KindRevHistory:
		return "revhist"
	case KindSummaryInfo:
		return "summary"
	case KindPreview:
		return "preview"
	case KindAppInfo:
		return "appinfo"
	case KindAppInfoHistory:
		return "appinfohist"
	case KindFileDepList:
		return "filedep"
	case KindSecurity:
		return "security"
	case KindVbaProject:
		return "vba"
	case KindSignature:
		return "signature"
	case KindAcDsData:
		return "acds"
	case KindSystemSection:
		return "system"
	case KindR13MeasurementPtr:
		return "measurement"
	default:
		return "unknown"
	}
}

func (k SectionKind) String() string { return k.ShortLabel() }

// SectionDiagnostics is a printable diagnostic view over a Section.
type SectionDiagnostics struct {
	Name              string
	Kind              SectionKind
	Compressed        bool
	Encrypted         bool
	OnDiskBytes       uint64
	DecodeAttempted   bool
	DecodeSucceeded   bool
	DecompressedBytes *int
	CompressionRatio  float64
}

// Diagnostics renders the section's post-hoc diagnostic fields.
func (s *Section) Diagnostics() SectionDiagnostics {
	d := SectionDiagnostics{
		Name:              s.Name,
		Kind:              s.Kind,
		Compressed:        s.Compressed,
		Encrypted:         s.Encrypted,
		OnDiskBytes:       s.Size,
		DecodeAttempted:   s.DecodeAttempted,
		DecodeSucceeded:   s.DecodeSucceeded,
		DecompressedBytes: s.DecompressedBytes,
	}
	if r, ok := s.CompressionRatio(); ok {
		d.CompressionRatio = r
	}
	return d
}

func (d SectionDiagnostics) String() string {
	s := fmt.Sprintf("%-28s kind=%-12s on_disk=%10d bytes", d.Name, d.Kind.ShortLabel(), d.OnDiskBytes)
	if d.Compressed {
		s += " [compressed]"
	}
	if d.Encrypted {
		s += " [encrypted]"
	}
	if !d.DecodeAttempted {
		s += " [decode: not attempted]"
	} else if d.DecodeSucceeded {
		if d.DecompressedBytes != nil {
			s += fmt.Sprintf(" [decoded: %d bytes, ratio %.2fx]", *d.DecompressedBytes, d.CompressionRatio)
		} else {
			s += " [decoded: ok]"
		}
	} else {
		s += " [decode: failed]"
	}
	return s
}
