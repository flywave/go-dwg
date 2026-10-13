package dwg

// VPORT table entry (spec §19.5.8 / §20.4.64) — a model-space viewport
// preset: its screen rectangle, camera and snap/grid state.

// VportEntry is a decoded VPORT table entry.
type VportEntry struct {
	Header        TableEntryHeader
	ViewHeight    float64
	AspectRatio   float64
	ViewCenter    PlotPoint2D
	ViewTarget    PlotPoint3D
	ViewDirection PlotPoint3D
	ViewTwist     float64
	LensLength    float64
	FrontClip     float64
	BackClip      float64
	ViewMode      int16
	RenderMode    uint8
	LowerLeft     PlotPoint2D
	UpperRight    PlotPoint2D
	UcsAtOrigin   bool
	UcsPerVport   bool
	SnapBase      PlotPoint2D
	SnapSpacing   PlotPoint2D
	GridSpacing   PlotPoint2D
	SnapRotation  float64
}

// decodeVportInline decodes a pre-R2007 VPORT whose name is still inline.
func decodeVportInline(c *BitCursor, version Version) (VportEntry, error) {
	header, err := readTableEntryHeader(c, version)
	if err != nil {
		return VportEntry{}, err
	}
	return vportBody(c, header, version, false)
}

// decodeVportModern decodes an R2007+ VPORT: its fields live in the data
// stream and its name in the object's string stream (§19.1).
func decodeVportModern(payload []byte, bodyStart int, version Version) (VportEntry, error) {
	split, err := openTableEntry(payload, bodyStart, version)
	if err != nil {
		return VportEntry{}, err
	}
	flag64, xrefIdx, xdep, err := readEntryFlags(split.data)
	if err != nil {
		return VportEntry{}, err
	}
	v, err := vportBody(split.data, TableEntryHeader{}, version, true)
	if err != nil {
		return VportEntry{}, err
	}
	if err := split.finish("VPORT"); err != nil {
		return VportEntry{}, err
	}
	name, err := split.strings.readTV()
	if err != nil {
		return VportEntry{}, err
	}
	v.Header = TableEntryHeader{Name: name, IsXrefDependent: xdep, XrefIndexPlus1: xrefIdx, IsXrefResolved: flag64}
	return v, nil
}

// vportBody reads the VPORT field table shared by the inline and R2007+
// forms. The split-stream form adds the R2007+ lighting block after the
// render mode and the two trailing grid words.
func vportBody(c *BitCursor, header TableEntryHeader, version Version, modern bool) (VportEntry, error) {
	var v VportEntry
	v.Header = header
	var err error
	if v.ViewHeight, err = c.ReadBD(); err != nil {
		return v, err
	}
	if v.AspectRatio, err = c.ReadBD(); err != nil {
		return v, err
	}
	if v.ViewCenter, err = readRD2(c); err != nil {
		return v, err
	}
	if v.ViewTarget, err = readBD3(c); err != nil {
		return v, err
	}
	if v.ViewDirection, err = readBD3(c); err != nil {
		return v, err
	}
	if v.ViewTwist, err = c.ReadBD(); err != nil {
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
	vm, err := c.Read4bits()
	if err != nil {
		return v, err
	}
	v.ViewMode = int16(vm)
	// §20.4.64 gates Render Mode on R2000+; R14 skips straight to the
	// screen rectangle.
	if version != R14 {
		if v.RenderMode, err = c.ReadRC(); err != nil {
			return v, err
		}
	}
	if modern {
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
		if _, _, _, err = readCMCFull(c); err != nil { // ambient colour
			return v, err
		}
	}
	if v.LowerLeft, err = readRD2(c); err != nil {
		return v, err
	}
	if v.UpperRight, err = readRD2(c); err != nil {
		return v, err
	}
	if _, err = c.ReadB(); err != nil { // UCSFOLLOW
		return v, err
	}
	if _, err = c.ReadBS(); err != nil { // circle zoom percent
		return v, err
	}
	if _, err = c.ReadB(); err != nil { // fast zoom
		return v, err
	}
	ucsIcon, err := c.ReadBB()
	if err != nil {
		return v, err
	}
	v.UcsAtOrigin = ucsIcon&0x02 != 0
	if _, err = c.ReadB(); err != nil { // grid on
		return v, err
	}
	if v.GridSpacing, err = readRD2(c); err != nil {
		return v, err
	}
	if _, err = c.ReadB(); err != nil { // snap on
		return v, err
	}
	if _, err = c.ReadB(); err != nil { // snap style
		return v, err
	}
	if _, err = c.ReadBS(); err != nil { // snap isopair
		return v, err
	}
	if v.SnapRotation, err = c.ReadBD(); err != nil {
		return v, err
	}
	if v.SnapBase, err = readRD2(c); err != nil {
		return v, err
	}
	if v.SnapSpacing, err = readRD2(c); err != nil {
		return v, err
	}
	// §20.4.64 gates the per-viewport UCS block on R2000+.
	if version != R14 {
		if _, err = c.ReadB(); err != nil { // unknown
			return v, err
		}
		if v.UcsPerVport, err = c.ReadB(); err != nil {
			return v, err
		}
		if _, err = readBD3(c); err != nil { // ucs origin
			return v, err
		}
		if _, err = readBD3(c); err != nil { // ucs x-direction
			return v, err
		}
		if _, err = readBD3(c); err != nil { // ucs y-direction
			return v, err
		}
		if _, err = c.ReadBD(); err != nil { // ucs elevation
			return v, err
		}
		if _, err = c.ReadBS(); err != nil { // ucs orthographic view type
			return v, err
		}
	}
	if modern {
		if _, err = c.ReadBS(); err != nil { // grid flags
			return v, err
		}
		if _, err = c.ReadBS(); err != nil { // grid major
			return v, err
		}
	}
	return v, nil
}
