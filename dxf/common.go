package dxf

// Common entity preamble (DXF "common entity data"): the handle/owner pair,
// the 100 AcDbEntity subclass block, and the optional group codes every
// drawable entity may carry. Typed entities embed this struct; the reader
// routes these codes out of the entity's tag run before type-specific
// parsing, and the writer emits them ahead of the type-specific block.

// Lineweight sentinels for group 370.
const (
	LineweightByLayer int16 = -1
	LineweightByBlock int16 = -2
	LineweightDefault int16 = -3
)

// Common is the shared entity preamble. Zero-value fields mean "absent":
// writers only emit codes that were read or explicitly set. Layer is the
// exception — it always emits (defaulting to "0"), matching what every DXF
// writer produces.
type Common struct {
	// Handle is the object handle (group 5). 0 = absent (readers assign).
	Handle uint64
	// Owner is the owner handle (group 330). HasOwner gates emission.
	Owner    uint64
	HasOwner bool

	// Layer name (group 8).
	Layer string
	// Color is the ACI (group 62): 0=ByBlock, 1-255 indexed, 256=ByLayer.
	Color    int16
	HasColor bool
	// TrueColor is the 24-bit RGB override (group 420). 0 = absent.
	TrueColor uint32
	// ColorName is the color-book name (group 430). Empty = absent.
	ColorName string
	// Linetype name (group 6). Empty = ByLayer.
	Linetype string
	// Lineweight (group 370): -3 default, -1 ByLayer, -2 ByBlock, else
	// the weight in 1/100 mm.
	Lineweight    int16
	HasLineweight bool
	// LTypeScale is the linetype scale (group 48). 0 = absent.
	LTypeScale float64
	// Invisible mirrors group 60 (1 = invisible).
	Invisible bool
	// Thickness (group 39).
	Thickness float64
	// PaperSpace is group 67 (1 = paper space).
	PaperSpace bool
	// MaterialHandle is group 347. 0 = absent.
	MaterialHandle uint64
	// PlotStyleHandle is group 390. 0 = absent.
	PlotStyleHandle uint64

	// Extrusion is the object-space extrusion vector (groups 210-230).
	Extrusion    Point3
	HasExtrusion bool

	// Extra preserves any unrecognized codes from the type block so round
	// trips stay lossless. Emitted after the typed fields.
	Extra []Tag

	// pre preserves verbatim tags that preceded the 100 AcDbEntity marker
	// (typically 102 {ACAD_REACTORS …} control groups). Re-emitted between
	// the owner handle and the subclass marker.
	pre []Tag
}

// commonState is the extracted common preamble of one entity tag run.
type commonState struct {
	c      Common
	subcls bool  // saw a 100 AcDbEntity marker
	rest   []Tag // type-specific tags after the AcDbEntity block
}

// splitCommon peels the handle/owner/subclass block and the AcDbEntity
// common groups off an entity's tag run, returning the remaining
// type-specific tags. For runs without subclass markers (R12 dialects) the
// common codes are extracted wherever they appear and everything else is
// returned in rest.
func splitCommon(tags []Tag) commonState {
	var st commonState
	// Phase 1: tags before the first 100 marker.
	i := 0
	for ; i < len(tags); i++ {
		t := tags[i]
		if t.Code == 100 {
			break
		}
		switch t.Code {
		case 5:
			st.c.Handle = parseHandle(t.Value)
		case 330:
			st.c.Owner = parseHandle(t.Value)
			st.c.HasOwner = true
		case 102:
			st.pre102(&t)
			if t.Value != "}" {
				for i+1 < len(tags) {
					i++
					nxt := tags[i]
					st.pre102(&nxt)
					if nxt.Code == 102 && nxt.Value == "}" {
						break
					}
				}
			}
		default:
			st.c.pre = append(st.c.pre, t)
		}
	}
	// Phase 2: 100 AcDbEntity followed by common groups until the next 100.
	if i < len(tags) && tags[i].Code == 100 {
		if tags[i].Value == "AcDbEntity" {
			st.subcls = true
			i++
			for ; i < len(tags); i++ {
				t := tags[i]
				if t.Code == 100 {
					break
				}
				if !absorbCommonGroup(&st.c, t) {
					st.c.Extra = append(st.c.Extra, t)
				}
			}
		}
	}
	// Phase 3: the tail. With a marker recognized, the type marker itself
	// is dropped (the writer re-emits it); without one — a foreign or R12
	// run — everything routes through the common extractor so the typed
	// reader sees only genuine type data.
	if st.subcls {
		if i < len(tags) {
			st.rest = append(st.rest, tags[i+1:]...)
		}
	} else {
		for _, t := range st.c.pre {
			if !absorbCommonGroup(&st.c, t) {
				st.rest = append(st.rest, t)
			}
		}
		st.c.pre = nil
		st.rest = append(st.rest, tags[i:]...)
	}
	return st
}

func (st *commonState) pre102(t *Tag) {
	st.c.pre = append(st.c.pre, *t)
}

// absorbCommonGroup tries to file one tag into the common block. It
// reports whether it took the tag; callers route the leftovers (to Extra
// in the AcDbEntity block, to type data in R12-style runs).
func absorbCommonGroup(c *Common, t Tag) bool {
	switch t.Code {
	case 8:
		c.Layer = t.Value
	case 6:
		c.Linetype = t.Value
	case 62:
		c.Color = int16(t.Int())
		c.HasColor = true
	case 420:
		c.TrueColor = uint32(t.Int())
	case 430:
		c.ColorName = t.Value
	case 48:
		c.LTypeScale = t.Float()
	case 60:
		c.Invisible = t.Int() != 0
	case 39:
		c.Thickness = t.Float()
	case 370:
		c.Lineweight = int16(t.Int())
		c.HasLineweight = true
	case 67:
		c.PaperSpace = t.Int() != 0
	case 347:
		c.MaterialHandle = parseHandle(t.Value)
	case 390:
		c.PlotStyleHandle = parseHandle(t.Value)
	case 210:
		c.Extrusion.X = t.Float()
		c.HasExtrusion = true
	case 220:
		c.Extrusion.Y = t.Float()
		c.HasExtrusion = true
	case 230:
		c.Extrusion.Z = t.Float()
		c.HasExtrusion = true
	default:
		return false
	}
	return true
}

// writeCommon emits the shared preamble for a typed entity. subclasses
// gates the 100 markers; extraSubclass is the type marker (e.g. AcDbLine).
func (w *TagWriter) writeCommon(c *Common, subclasses bool, extraSubclass string) {
	if c.Handle != 0 {
		w.Handle(5, c.Handle)
	}
	if c.HasOwner {
		w.Handle(330, c.Owner)
	}
	w.Pairs(c.pre)
	if subclasses {
		w.Subclass("AcDbEntity")
		if c.PaperSpace {
			w.Bool(67, true)
		}
	}
	if c.Layer == "" {
		w.Str(8, "0")
	} else {
		w.Str(8, c.Layer)
	}
	if c.HasColor {
		w.Int(62, int64(c.Color))
	}
	if c.TrueColor != 0 {
		w.Int(420, int64(c.TrueColor))
	}
	if c.ColorName != "" {
		w.Str(430, c.ColorName)
	}
	if c.Linetype != "" {
		w.Str(6, c.Linetype)
	}
	if c.HasLineweight {
		w.Int(370, int64(c.Lineweight))
	}
	if c.LTypeScale != 0 {
		w.Float(48, c.LTypeScale)
	}
	if c.Invisible {
		w.Int(60, 1)
	}
	if c.MaterialHandle != 0 {
		w.Handle(347, c.MaterialHandle)
	}
	if c.PlotStyleHandle != 0 {
		w.Handle(390, c.PlotStyleHandle)
	}
	if c.Thickness != 0 {
		w.Float(39, c.Thickness)
	}
	if c.HasExtrusion {
		w.Point(210, c.Extrusion)
	}
	if subclasses && extraSubclass != "" && extraSubclass != "AcDbEntity" {
		w.Subclass(extraSubclass)
	}
}

// parseHandle parses a hex handle string with a decimal fallback.
func parseHandle(s string) uint64 {
	if s == "" {
		return 0
	}
	return parseUintHex(s)
}
