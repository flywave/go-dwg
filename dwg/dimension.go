package dwg

// DIMENSION entity family (§19.4.10-§19.4.15): seven fixed type codes
// (0x14 ordinate … 0x1A diameter) sharing one preamble and finishing with
// per-subtype suffixes. On R2007+ the user_text TV lives in the string
// stream; every field after it stays on the data stream only when the TV
// is read from there too.

// DimensionCommon is the shared dimension preamble (§19.4.10.1).
type DimensionCommon struct {
	VersionFlag       uint8
	Extrusion         Vec3D
	TextMidpoint      Point2D
	Elevation         float64
	Flags             uint8
	UserText          string
	TextRotation      float64
	HorizDir          float64
	InsScale          Point3D
	InsRotation       float64
	Attachment        int16
	LineSpacingStyle  int16
	LineSpacingFactor float64
	ActualMeasurement float64
	DefPoint12        Point2D
	FlipArrow1        bool
	FlipArrow2        bool
}

// Dimension is one decoded dimension; Kind says which suffix applied.
type Dimension struct {
	Kind   string
	Common DimensionCommon
	// Ordinate / linear-family definition points; which subset is used
	// depends on Kind.
	DefPoint10            Point3D
	DefPoint13            Point3D
	DefPoint14            Point3D
	DefPoint15            Point3D
	FeatureLocation13     Point3D
	LeaderEndpoint14      Point3D
	Flag2                 uint8
	ExtensionLineRotation float64
	DimRotation           float64
	LeaderLength          float64
	DefPoint16            Point2D
}

// dimensionKindName maps a fixed type code to its subtype label, or ""
// when the code is outside the dimension range (spec §5 table 4).
func dimensionKindName(code uint16) string {
	switch code {
	case oDimOrdinate:
		return "DIMENSION_ORDINATE"
	case oDimLinear:
		return "DIMENSION_LINEAR"
	case oDimAligned:
		return "DIMENSION_ALIGNED"
	case oDimAng3Pt:
		return "DIMENSION_ANG_3PT"
	case oDimAng2Ln:
		return "DIMENSION_ANG_2LN"
	case oDimRadius:
		return "DIMENSION_RADIUS"
	case oDimDiameter:
		return "DIMENSION_DIAMETER"
	}
	return ""
}

// readDimensionCommon reads the shared preamble; the user_text TV comes
// from strings on the R2007+ split layout.
func readDimensionCommon(c *BitCursor, v Version, strings *stringReader) (DimensionCommon, error) {
	var d DimensionCommon
	var err error
	if v.IsR2010Plus() {
		var b uint8
		if b, err = c.ReadRC(); err != nil {
			return d, err
		}
		d.VersionFlag = b
	}
	if d.Extrusion, err = ReadBD3(c); err != nil {
		return d, err
	}
	x, err := c.ReadRD()
	if err != nil {
		return d, err
	}
	y, err := c.ReadRD()
	if err != nil {
		return d, err
	}
	d.TextMidpoint = Point2D{X: x, Y: y}
	if d.Elevation, err = c.ReadBD(); err != nil {
		return d, err
	}
	if d.Flags, err = c.ReadRC(); err != nil {
		return d, err
	}
	if d.UserText, err = readTVSource(c, strings, v); err != nil {
		return d, err
	}
	if d.TextRotation, err = c.ReadBD(); err != nil {
		return d, err
	}
	if d.HorizDir, err = c.ReadBD(); err != nil {
		return d, err
	}
	if d.InsScale, err = ReadBD3(c); err != nil {
		return d, err
	}
	if d.InsRotation, err = c.ReadBD(); err != nil {
		return d, err
	}
	if d.Attachment, err = c.ReadBS(); err != nil {
		return d, err
	}
	if d.LineSpacingStyle, err = c.ReadBS(); err != nil {
		return d, err
	}
	if d.LineSpacingFactor, err = c.ReadBD(); err != nil {
		return d, err
	}
	if d.ActualMeasurement, err = c.ReadBD(); err != nil {
		return d, err
	}
	if v.IsR2007Plus() {
		if _, err = c.ReadB(); err != nil { // unknown bit
			return d, err
		}
		if d.FlipArrow1, err = c.ReadB(); err != nil {
			return d, err
		}
		if d.FlipArrow2, err = c.ReadB(); err != nil {
			return d, err
		}
	}
	x, err = c.ReadRD()
	if err != nil {
		return d, err
	}
	y, err = c.ReadRD()
	if err != nil {
		return d, err
	}
	d.DefPoint12 = Point2D{X: x, Y: y}
	return d, nil
}

// decodeDimension reads one dimension of the subtype the fixed type code
// names (caller already consumed the common entity preamble).
func decodeDimension(code uint16, c *BitCursor, v Version, strings *stringReader) (Dimension, error) {
	var dim Dimension
	dim.Kind = dimensionKindName(code)
	common, err := readDimensionCommon(c, v, strings)
	if err != nil {
		return dim, err
	}
	dim.Common = common
	read3 := func(dst *Point3D) error {
		p, err := ReadBD3(c)
		if err != nil {
			return err
		}
		*dst = p
		return nil
	}
	switch code {
	case oDimOrdinate:
		if err = read3(&dim.DefPoint10); err != nil {
			return dim, err
		}
		if err = read3(&dim.FeatureLocation13); err != nil {
			return dim, err
		}
		if err = read3(&dim.LeaderEndpoint14); err != nil {
			return dim, err
		}
		if dim.Flag2, err = c.ReadRC(); err != nil {
			return dim, err
		}
	case oDimLinear:
		if err = read3(&dim.DefPoint13); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint14); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint10); err != nil {
			return dim, err
		}
		if dim.ExtensionLineRotation, err = c.ReadBD(); err != nil {
			return dim, err
		}
		if dim.DimRotation, err = c.ReadBD(); err != nil {
			return dim, err
		}
	case oDimAligned:
		if err = read3(&dim.DefPoint13); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint14); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint10); err != nil {
			return dim, err
		}
		if dim.ExtensionLineRotation, err = c.ReadBD(); err != nil {
			return dim, err
		}
	case oDimAng3Pt:
		if err = read3(&dim.DefPoint10); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint13); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint14); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint15); err != nil {
			return dim, err
		}
	case oDimAng2Ln:
		// §20.4.27 puts the 2RD 16-point first, before the four 3BD def
		// points — the measured record only closes in this order.
		x, err := c.ReadRD()
		if err != nil {
			return dim, err
		}
		y, err := c.ReadRD()
		if err != nil {
			return dim, err
		}
		dim.DefPoint16 = Point2D{X: x, Y: y}
		if err = read3(&dim.DefPoint13); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint14); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint15); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint10); err != nil {
			return dim, err
		}
	case oDimRadius:
		if err = read3(&dim.DefPoint10); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint15); err != nil {
			return dim, err
		}
		if dim.LeaderLength, err = c.ReadBD(); err != nil {
			return dim, err
		}
	case oDimDiameter:
		if err = read3(&dim.DefPoint15); err != nil {
			return dim, err
		}
		if err = read3(&dim.DefPoint10); err != nil {
			return dim, err
		}
		if dim.LeaderLength, err = c.ReadBD(); err != nil {
			return dim, err
		}
	}
	return dim, nil
}
