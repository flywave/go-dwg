package dwg

// MTEXT entity (§19.4.44 / §20.4.46) — multiline text. The reference's
// inline decoder stops two bits short of the R2004+ record because it omits
// the R2004+ `background_flags` block; this decoder reads it (a clear flag
// is the 2-bit `BL` zero form, which closes the AC1018 record exactly).

// maxMTextColumnHeights caps the R2018+ per-column height list (§20.4.46 BD 46).
const maxMTextColumnHeights = 4096

// MText is a decoded MTEXT entity.
type MText struct {
	InsertionPoint    PlotPoint3D
	Extrusion         PlotPoint3D
	XAxisDirection    PlotPoint3D
	RectWidth         float64
	RectHeight        float64
	NominalTextHeight float64
	AttachmentPoint   int16
	DrawingDirection  int16
	ExtentsHeight     float64
	ExtentsWidth      float64
	Text              string
	LinespaceStyle    int16
	LinespaceFactor   float64
	BackgroundFlags   int32
	ColumnType        int16
	ColumnWidth       float64
	ColumnGutter      float64
	ColumnHeights     []float64
}

// decodeMText decodes an MTEXT body. On R2007+ the text comes from the
// record's string stream; before that it is inline.
func decodeMText(c *BitCursor, version Version, strings *stringReader) (MText, error) {
	var m MText
	var err error
	if m.InsertionPoint, err = readBD3(c); err != nil {
		return m, err
	}
	if m.Extrusion, err = readBD3(c); err != nil {
		return m, err
	}
	if m.XAxisDirection, err = readBD3(c); err != nil {
		return m, err
	}
	if m.RectWidth, err = c.ReadBD(); err != nil {
		return m, err
	}
	if version.IsR2007Plus() {
		if m.RectHeight, err = c.ReadBD(); err != nil {
			return m, err
		}
	}
	if m.NominalTextHeight, err = c.ReadBD(); err != nil {
		return m, err
	}
	if m.AttachmentPoint, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.DrawingDirection, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.ExtentsHeight, err = c.ReadBD(); err != nil {
		return m, err
	}
	if m.ExtentsWidth, err = c.ReadBD(); err != nil {
		return m, err
	}
	if version.IsR2007Plus() {
		if m.Text, err = strings.readTV(); err != nil {
			return m, err
		}
	} else {
		if m.Text, err = readTV(c, version); err != nil {
			return m, err
		}
	}
	if m.LinespaceStyle, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.LinespaceFactor, err = c.ReadBD(); err != nil {
		return m, err
	}
	if _, err = c.ReadB(); err != nil { // unknown
		return m, err
	}
	if version.IsR2004Plus() {
		if m.BackgroundFlags, err = c.ReadBL(); err != nil {
			return m, err
		}
		frameBit := int32(0)
		if version == R2018 {
			frameBit = 0x10
		}
		if m.BackgroundFlags&(0x01|frameBit) != 0 {
			if _, err = c.ReadBD(); err != nil { // background scale
				return m, err
			}
			if _, err = c.ReadBSU(); err != nil { // background color index
				return m, err
			}
			if _, err = c.ReadBLU(); err != nil { // background color rgb
				return m, err
			}
			if _, err = c.ReadRC(); err != nil { // background color byte
				return m, err
			}
			if _, err = c.ReadBL(); err != nil { // background transparency
				return m, err
			}
		}
	}
	if version == R2018 {
		isNotAnnotative, err := c.ReadB()
		if err != nil {
			return m, err
		}
		if isNotAnnotative {
			if _, err = c.ReadBS(); err != nil { // version
				return m, err
			}
			if _, err = c.ReadB(); err != nil { // default flag
				return m, err
			}
			if _, err = c.ReadBL(); err != nil { // attachment point
				return m, err
			}
			if _, err = readBD3(c); err != nil { // x-axis dir
				return m, err
			}
			if _, err = readBD3(c); err != nil { // insertion point
				return m, err
			}
			if _, err = c.ReadBD(); err != nil { // rect width
				return m, err
			}
			if _, err = c.ReadBD(); err != nil { // rect height
				return m, err
			}
			if _, err = c.ReadBD(); err != nil { // extents width
				return m, err
			}
			if _, err = c.ReadBD(); err != nil { // extents height
				return m, err
			}
			if m.ColumnType, err = c.ReadBS(); err != nil {
				return m, err
			}
			if m.ColumnType != 0 {
				heightCount, err := c.ReadBLU()
				if err != nil {
					return m, err
				}
				if m.ColumnWidth, err = c.ReadBD(); err != nil {
					return m, err
				}
				if m.ColumnGutter, err = c.ReadBD(); err != nil {
					return m, err
				}
				autoHeight, err := c.ReadB()
				if err != nil {
					return m, err
				}
				if _, err = c.ReadB(); err != nil { // flow reversed
					return m, err
				}
				if !autoHeight && m.ColumnType == 2 {
					if int(heightCount) > maxMTextColumnHeights || int(heightCount) > c.RemainingBits() {
						return m, newError(ErrSectionMap, "MTEXT column height count %d exceeds cap (%d) or remaining bits (%d)",
							heightCount, maxMTextColumnHeights, c.RemainingBits())
					}
					m.ColumnHeights = make([]float64, 0, heightCount)
					for i := uint32(0); i < heightCount; i++ {
						h, err := c.ReadBD()
						if err != nil {
							return m, err
						}
						m.ColumnHeights = append(m.ColumnHeights, h)
					}
				}
			}
		}
	}
	return m, nil
}
