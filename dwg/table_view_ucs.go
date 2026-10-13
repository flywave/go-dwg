package dwg

// VIEW (spec §19.5.7 / §20.4.7) and UCS (§19.5.6 / §20.4.6) table entries —
// named camera views and user coordinate systems. Ported from the reference
// implementation's view.rs / ucs.rs; neither corpus carries records, so the
// layouts are covered by synthetic unit tests (a wrong field order fails the
// split-stream boundary or the byte-exact expectations instead of passing).

// ViewEntry is a decoded VIEW table entry.
type ViewEntry struct {
	Header          TableEntryHeader
	ViewHeight      float64
	ViewWidth       float64
	ViewCenter      Point2D
	Target          Point3D
	ViewDirection   Point3D
	TwistAngle      float64
	LensLength      float64
	FrontClip       float64
	BackClip        float64
	ViewMode        int16
	RenderMode      uint8
	IsPaperSpace    bool
	IsAssociatedUcs bool
}

func viewBody(c *BitCursor, r2007Plus bool) (ViewEntry, error) {
	var v ViewEntry
	var err error
	if v.ViewHeight, err = c.ReadBD(); err != nil {
		return v, err
	}
	if v.ViewWidth, err = c.ReadBD(); err != nil {
		return v, err
	}
	x, err := c.ReadRD()
	if err != nil {
		return v, err
	}
	y, err := c.ReadRD()
	if err != nil {
		return v, err
	}
	v.ViewCenter = Point2D{X: x, Y: y}
	if v.Target, err = ReadBD3(c); err != nil {
		return v, err
	}
	if v.ViewDirection, err = ReadBD3(c); err != nil {
		return v, err
	}
	if v.TwistAngle, err = c.ReadBD(); err != nil {
		return v, err
	}
	if v.LensLength, err = c.ReadBD(); err != nil {
		return v, err
	}
	if v.FrontClip, err = c.ReadBD(); err != nil {
		return v, err
	}
	if v.BackClip, err = c.ReadBD(); err != nil {
		return v, err
	}
	if r2007Plus {
		// 4BITS view mode.
		var m uint8
		if m, err = c.Read4bits(); err != nil {
			return v, err
		}
		v.ViewMode = int16(m)
	} else {
		if v.ViewMode, err = c.ReadBS(); err != nil {
			return v, err
		}
	}
	if v.RenderMode, err = c.ReadRC(); err != nil {
		return v, err
	}
	if r2007Plus {
		// Lighting block, ambient CMC, and the camera-plottable flag only
		// exist on the split layout.
		if _, err = c.ReadB(); err != nil { // use default lights
			return v, err
		}
		if _, err = c.ReadRC(); err != nil { // default lighting type
			return v, err
		}
		if _, err = c.ReadBD(); err != nil { // brightness
			return v, err
		}
		if _, err = c.ReadBD(); err != nil { // contrast
			return v, err
		}
	}
	if v.IsPaperSpace, err = c.ReadB(); err != nil {
		return v, err
	}
	if v.IsAssociatedUcs, err = c.ReadB(); err != nil {
		return v, err
	}
	if v.IsAssociatedUcs {
		if _, err = ReadBD3(c); err != nil { // origin
			return v, err
		}
		if _, err = ReadBD3(c); err != nil { // x direction
			return v, err
		}
		if _, err = ReadBD3(c); err != nil { // y direction
			return v, err
		}
		if _, err = c.ReadBD(); err != nil { // elevation
			return v, err
		}
		if _, err = c.ReadBS(); err != nil { // orthographic view type
			return v, err
		}
	}
	if r2007Plus {
		if _, err = c.ReadB(); err != nil { // camera plottable
			return v, err
		}
	}
	return v, nil
}

// decodeViewInline decodes a pre-R2007 VIEW whose name is still inline.
func decodeViewInline(c *BitCursor, version Version) (ViewEntry, error) {
	header, err := readTableEntryHeader(c, version)
	if err != nil {
		return ViewEntry{}, err
	}
	v, err := viewBody(c, false)
	if err != nil {
		return ViewEntry{}, err
	}
	v.Header = header
	return v, nil
}

// decodeViewModern decodes an R2007+ VIEW: fields in the data stream, name
// in the object's string stream (§19.1).
func decodeViewModern(payload []byte, bodyStart int, version Version) (ViewEntry, error) {
	split, err := openTableEntry(payload, bodyStart, version)
	if err != nil {
		return ViewEntry{}, err
	}
	flag64, xrefIdx, xdep, err := readEntryFlags(split.data)
	if err != nil {
		return ViewEntry{}, err
	}
	v, err := viewBody(split.data, true)
	if err != nil {
		return ViewEntry{}, err
	}
	if err := split.finish("VIEW"); err != nil {
		return ViewEntry{}, err
	}
	name, err := split.strings.readTV()
	if err != nil {
		return ViewEntry{}, err
	}
	v.Header = TableEntryHeader{Name: name, IsXrefDependent: xdep, XrefIndexPlus1: xrefIdx, IsXrefResolved: flag64}
	return v, nil
}

// UcsEntry is a decoded UCS table entry.
type UcsEntry struct {
	Header        TableEntryHeader
	Origin        Point3D
	XAxis         Point3D
	YAxis         Point3D
	Elevation     float64
	OrthoViewType int16
	BaseUcsHandle uint64
	HasBaseUcs    bool
}

// maxUcsOrthoViewType bounds the orthographic view type (§19.5.6).
const maxUcsOrthoViewType = 6

func ucsBody(c *BitCursor) (UcsEntry, error) {
	var u UcsEntry
	var err error
	if u.Origin, err = ReadBD3(c); err != nil {
		return u, err
	}
	if u.XAxis, err = ReadBD3(c); err != nil {
		return u, err
	}
	if u.YAxis, err = ReadBD3(c); err != nil {
		return u, err
	}
	if u.Elevation, err = c.ReadBD(); err != nil {
		return u, err
	}
	if u.OrthoViewType, err = c.ReadBS(); err != nil {
		return u, err
	}
	if u.OrthoViewType < 0 || u.OrthoViewType > maxUcsOrthoViewType {
		return u, newError(ErrSectionMap, "UCS ortho_view_type %d outside 0..=%d", u.OrthoViewType, maxUcsOrthoViewType)
	}
	return u, nil
}

// decodeUcsInline decodes a pre-R2007 UCS; the base-UCS handle rides in the
// data stream.
func decodeUcsInline(c *BitCursor, version Version) (UcsEntry, error) {
	header, err := readTableEntryHeader(c, version)
	if err != nil {
		return UcsEntry{}, err
	}
	u, err := ucsBody(c)
	if err != nil {
		return UcsEntry{}, err
	}
	h, err := c.ReadHandle()
	if err != nil {
		return UcsEntry{}, err
	}
	if h.Value != 0 {
		u.BaseUcsHandle = h.Value
		u.HasBaseUcs = true
	}
	u.Header = header
	return u, nil
}

// decodeUcsModern decodes an R2007+ UCS; the base-UCS handle moved to the
// handle stream, so it is reported unset.
func decodeUcsModern(payload []byte, bodyStart int, version Version) (UcsEntry, error) {
	split, err := openTableEntry(payload, bodyStart, version)
	if err != nil {
		return UcsEntry{}, err
	}
	flag64, xrefIdx, xdep, err := readEntryFlags(split.data)
	if err != nil {
		return UcsEntry{}, err
	}
	u, err := ucsBody(split.data)
	if err != nil {
		return UcsEntry{}, err
	}
	if err := split.finish("UCS"); err != nil {
		return UcsEntry{}, err
	}
	name, err := split.strings.readTV()
	if err != nil {
		return UcsEntry{}, err
	}
	u.Header = TableEntryHeader{Name: name, IsXrefDependent: xdep, XrefIndexPlus1: xrefIdx, IsXrefResolved: flag64}
	return u, nil
}
