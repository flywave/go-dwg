package dwg

// ACAD_VISUALSTYLE object (custom class, no ODA §20.4 prescription —
// measured). R14-R2007 use a flag-less 30-field list; R2010+ use a
// (value, flag) property list behind a three-field head. This port implements
// both; the R2013+ tail is 30 positionally-named (value, flag) pairs.

// VisualStyleColor is one CMC color slot of a visual style.
type VisualStyleColor struct {
	Index     uint16
	RGB       uint32
	ColorByte uint8
}

// VisualStyle is a decoded ACAD_VISUALSTYLE record.
type VisualStyle struct {
	Description              string
	InternalStyleType        int16
	FormatVersion            int16
	IsInternalUseOnly        bool
	FaceLightingModel        int16
	FaceLightingQuality      int16
	FaceColorMode            int16
	FaceModifier             int16
	FaceOpacity              float64
	FaceSpecular             float64
	FaceMonoColor            VisualStyleColor
	EdgeModel                int16
	EdgeStyle                int16
	EdgeIntersectionColor    VisualStyleColor
	EdgeObscuredColor        VisualStyleColor
	EdgeObscuredLinetype     int16
	EdgeIntersectionLinetype int16
	EdgeCreaseAngle          float64
	EdgeModifier             int16
	EdgeColor                VisualStyleColor
	EdgeOpacity              float64
	EdgeWidth                int16
	EdgeOverhang             int16
	EdgeJitter               int16
	EdgeSilhouetteColor      VisualStyleColor
	EdgeSilhouetteWidth      int16
	EdgeHaloGap              int16
	EdgeIsolineCount         int16
	EdgeHidePrecision        bool
	EdgeStyleApply           int16
	DisplayBrightness        float64
	DisplayShadowType        int16
	PropertyFlags            []int16
	ExtendedCount            int
	TrailingStrings          []string
}

type vsTailKind uint8

const (
	vsShort vsTailKind = iota
	vsDouble
	vsBool
	vsColor
)

// vsR2013Tail is the 30 R2013+ tail property widths, measured.
var vsR2013Tail = [30]vsTailKind{
	vsBool, vsBool, vsBool, vsBool, vsBool, vsBool, vsBool, vsBool, vsBool,
	vsShort, vsShort, vsDouble, vsShort, vsColor, vsShort, vsShort, vsColor,
	vsBool, vsShort, vsShort, vsShort, vsBool, vsShort, vsColor, vsBool, vsBool,
	vsShort, vsBool, vsDouble, vsDouble,
}

type vsReader struct {
	c     *BitCursor
	flags []int16
}

func (r *vsReader) flag() (int16, error) {
	v, err := r.c.ReadBS()
	if err == nil {
		r.flags = append(r.flags, v)
	}
	return v, err
}

func (r *vsReader) short() (int16, error) {
	v, err := r.c.ReadBS()
	if err != nil {
		return 0, err
	}
	if _, err := r.flag(); err != nil {
		return 0, err
	}
	return v, nil
}

func (r *vsReader) double() (float64, error) {
	v, err := r.c.ReadBD()
	if err != nil {
		return 0, err
	}
	if _, err := r.flag(); err != nil {
		return 0, err
	}
	return v, nil
}

func (r *vsReader) boolean() (bool, error) {
	v, err := r.c.ReadB()
	if err != nil {
		return false, err
	}
	if _, err := r.flag(); err != nil {
		return false, err
	}
	return v, nil
}

func (r *vsReader) color() (VisualStyleColor, error) {
	idx, rgb, cb, err := readCMCFull(r.c)
	if err != nil {
		return VisualStyleColor{}, err
	}
	if _, err := r.flag(); err != nil {
		return VisualStyleColor{}, err
	}
	return VisualStyleColor{Index: idx, RGB: rgb, ColorByte: cb}, nil
}

func (r *vsReader) property(kind vsTailKind) error {
	switch kind {
	case vsShort:
		_, err := r.c.ReadBS()
		if err != nil {
			return err
		}
	case vsDouble:
		if _, err := r.c.ReadBD(); err != nil {
			return err
		}
	case vsBool:
		if _, err := r.c.ReadB(); err != nil {
			return err
		}
	case vsColor:
		if _, _, _, err := readCMCFull(r.c); err != nil {
			return err
		}
	}
	_, err := r.flag()
	return err
}

func readVSColorLegacy(c *BitCursor, version Version) (VisualStyleColor, error) {
	if !version.IsR2004Plus() {
		idx, err := c.ReadBSU()
		if err != nil {
			return VisualStyleColor{}, err
		}
		return VisualStyleColor{Index: idx}, nil
	}
	idx, rgb, cb, err := readCMCFull(c)
	if err != nil {
		return VisualStyleColor{}, err
	}
	return VisualStyleColor{Index: idx, RGB: rgb, ColorByte: cb}, nil
}

func drainTrailingStrings(o *objectStream) []string {
	var out []string
	if o.strings == nil {
		return out
	}
	for !o.strings.isExhausted() {
		s, err := o.strings.readTV()
		if err != nil {
			break
		}
		out = append(out, s)
	}
	return out
}

func decodeVisualStyleLegacy(payload []byte, bodyStart int, inlineEnd *uint32, version Version) (VisualStyle, error) {
	o, err := openObjectStream(payload, bodyStart, inlineEnd, version)
	if err != nil {
		return VisualStyle{}, err
	}
	var vs VisualStyle
	if vs.Description, err = o.readTV(version); err != nil {
		return VisualStyle{}, err
	}
	c := o.data
	rd := func(dst *int16) error { v, err := c.ReadBS(); *dst = v; return err }
	if err := rd(&vs.InternalStyleType); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.FaceLightingModel); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.FaceLightingQuality); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.FaceColorMode); err != nil {
		return VisualStyle{}, err
	}
	if vs.FaceOpacity, err = c.ReadBD(); err != nil {
		return VisualStyle{}, err
	}
	if vs.FaceSpecular, err = c.ReadBD(); err != nil {
		return VisualStyle{}, err
	}
	if vs.FaceMonoColor, err = readVSColorLegacy(c, version); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.FaceModifier); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeModel); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeStyle); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeIntersectionColor, err = readVSColorLegacy(c, version); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeObscuredColor, err = readVSColorLegacy(c, version); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeObscuredLinetype); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeCreaseAngle, err = c.ReadBD(); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeModifier); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeColor, err = readVSColorLegacy(c, version); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeOpacity, err = c.ReadBD(); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeWidth); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeOverhang); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeJitter); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeSilhouetteColor, err = readVSColorLegacy(c, version); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeSilhouetteWidth); err != nil {
		return VisualStyle{}, err
	}
	if _, err := c.ReadRC(); err != nil { // constant 13-bit run
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeHaloGap); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeHidePrecision, err = c.ReadB(); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeIsolineCount); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeIntersectionLinetype); err != nil {
		return VisualStyle{}, err
	}
	if err := rd(&vs.EdgeStyleApply); err != nil {
		return VisualStyle{}, err
	}
	bl, err := c.ReadBL()
	if err != nil {
		return VisualStyle{}, err
	}
	vs.DisplayBrightness = float64(bl)
	if err := rd(&vs.DisplayShadowType); err != nil {
		return VisualStyle{}, err
	}
	if version == R2007 {
		var dummy int16
		if err := rd(&dummy); err != nil {
			return VisualStyle{}, err
		}
	}
	if vs.IsInternalUseOnly, err = c.ReadB(); err != nil {
		return VisualStyle{}, err
	}
	if err := o.finish("VISUALSTYLE"); err != nil {
		return VisualStyle{}, err
	}
	vs.TrailingStrings = drainTrailingStrings(o)
	return vs, nil
}

func decodeVisualStylePaired(payload []byte, bodyStart int, inlineEnd *uint32, version Version) (VisualStyle, error) {
	o, err := openObjectStream(payload, bodyStart, inlineEnd, version)
	if err != nil {
		return VisualStyle{}, err
	}
	var vs VisualStyle
	if vs.Description, err = o.readTV(version); err != nil {
		return VisualStyle{}, err
	}
	if vs.InternalStyleType, err = o.data.ReadBS(); err != nil {
		return VisualStyle{}, err
	}
	if vs.FormatVersion, err = o.data.ReadBS(); err != nil {
		return VisualStyle{}, err
	}
	if vs.IsInternalUseOnly, err = o.data.ReadB(); err != nil {
		return VisualStyle{}, err
	}
	r := &vsReader{c: o.data, flags: make([]int16, 0, 58)}
	if vs.FaceLightingModel, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.FaceLightingQuality, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.FaceColorMode, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.FaceModifier, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.FaceOpacity, err = r.double(); err != nil {
		return VisualStyle{}, err
	}
	if vs.FaceSpecular, err = r.double(); err != nil {
		return VisualStyle{}, err
	}
	if vs.FaceMonoColor, err = r.color(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeModel, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeStyle, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeIntersectionColor, err = r.color(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeObscuredColor, err = r.color(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeObscuredLinetype, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeIntersectionLinetype, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeCreaseAngle, err = r.double(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeModifier, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeColor, err = r.color(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeOpacity, err = r.double(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeWidth, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeOverhang, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeJitter, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeSilhouetteColor, err = r.color(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeSilhouetteWidth, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeHaloGap, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeIsolineCount, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeHidePrecision, err = r.boolean(); err != nil {
		return VisualStyle{}, err
	}
	if vs.EdgeStyleApply, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if vs.DisplayBrightness, err = r.double(); err != nil {
		return VisualStyle{}, err
	}
	if vs.DisplayShadowType, err = r.short(); err != nil {
		return VisualStyle{}, err
	}
	if version == R2013 || version == R2018 {
		for _, kind := range vsR2013Tail {
			if err := r.property(kind); err != nil {
				return VisualStyle{}, err
			}
			vs.ExtendedCount++
		}
	}
	vs.PropertyFlags = r.flags
	if err := o.finish("VISUALSTYLE"); err != nil {
		return VisualStyle{}, err
	}
	vs.TrailingStrings = drainTrailingStrings(o)
	return vs, nil
}

func decodeVisualStyleObject(payload []byte, bodyStart int, inlineEnd *uint32, version Version) (VisualStyle, error) {
	if version.IsR2010Plus() {
		return decodeVisualStylePaired(payload, bodyStart, inlineEnd, version)
	}
	return decodeVisualStyleLegacy(payload, bodyStart, inlineEnd, version)
}
