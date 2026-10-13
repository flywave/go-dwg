package dwg

// Common entity data (spec §19.4.1) — the shared preamble every drawable
// entity writes before its type-specific body.

// EntityMode is the 2-bit entity-mode code.
type EntityMode uint8

const (
	// EntityModeByLayer is `00`.
	EntityModeByLayer EntityMode = iota
	// EntityModeByPreviousEntity is `01`.
	EntityModeByPreviousEntity
	// EntityModeInBlock is `10`.
	EntityModeInBlock
	// EntityModeReserved is `11`.
	EntityModeReserved
)

// CommonEntityData is the decoded common entity preamble. Fields that are
// skipped are still consumed from the cursor so the entity-specific payload
// starts in the right place.
type CommonEntityData struct {
	RawMode         uint8
	Mode            EntityMode
	NumReactors     uint32
	NoXDictionary   bool
	BinaryChain     bool
	IsOnLayer       bool
	NonFixedLtype   bool
	PlotstyleFlag   uint8
	MaterialFlag    uint8
	ShadowFlags     uint8
	Invisibility    int16
	Lineweight      uint8
	HadGraphics     bool
	HadExtendedData bool
	// DataEndBits is the R13/R14 object-data-size-in-bits the preamble
	// carries; nil on other releases.
	DataEndBits *uint32
}

const maxXDataIterations = 256

// skipExtendedData consumes the EED chain, returning whether any group was
// present.
func skipExtendedData(c *BitCursor) (bool, error) {
	hadExtended := false
	for i := 0; i < maxXDataIterations; i++ {
		size, err := c.ReadBSU()
		if err != nil {
			return hadExtended, err
		}
		if size == 0 {
			return hadExtended, nil
		}
		hadExtended = true
		if _, err := c.ReadHandle(); err != nil {
			return hadExtended, err
		}
		for j := uint16(0); j < size; j++ {
			if _, err := c.ReadRC(); err != nil {
				return hadExtended, err
			}
		}
	}
	return hadExtended, newError(ErrSectionMap, "common object XDATA loop exceeded %d iterations", maxXDataIterations)
}

func readGraphicsSize(c *BitCursor, version Version) (uint64, error) {
	if version.IsR2010Plus() {
		return c.ReadBLL()
	}
	v, err := c.ReadRL()
	return uint64(v), err
}

// ReadCommonEntityData reads the common entity preamble from c, advancing
// past it. The cursor must be positioned immediately after the object header
// handle.
func ReadCommonEntityData(c *BitCursor, version Version) (CommonEntityData, error) {
	hadExtended, err := skipExtendedData(c)
	if err != nil {
		return CommonEntityData{}, err
	}
	hadGraphics, err := c.ReadB()
	if err != nil {
		return CommonEntityData{}, err
	}
	if hadGraphics {
		gfxSize, err := readGraphicsSize(c, version)
		if err != nil {
			return CommonEntityData{}, err
		}
		for i := uint64(0); i < gfxSize; i++ {
			if _, err := c.ReadRC(); err != nil {
				return CommonEntityData{}, err
			}
		}
	}
	var dataEnd *uint32
	if version == R14 {
		v, err := c.ReadRL()
		if err != nil {
			return CommonEntityData{}, err
		}
		dataEnd = &v
	}
	common, err := readEntityModeOnwards(c, version, hadExtended, hadGraphics)
	if err != nil {
		return CommonEntityData{}, err
	}
	common.DataEndBits = dataEnd
	return common, nil
}

func readEntityModeOnwards(c *BitCursor, version Version, hadExtended, hadGraphics bool) (CommonEntityData, error) {
	var ce CommonEntityData
	rawMode, err := c.ReadBB()
	if err != nil {
		return ce, err
	}
	ce.RawMode = rawMode
	switch rawMode {
	case 0b00:
		ce.Mode = EntityModeByLayer
	case 0b01:
		ce.Mode = EntityModeByPreviousEntity
	case 0b10:
		ce.Mode = EntityModeInBlock
	default:
		ce.Mode = EntityModeReserved
	}
	reactors, err := c.ReadBL()
	if err != nil {
		return ce, err
	}
	ce.NumReactors = uint32(reactors)
	if version.IsR2004Plus() {
		b, err := c.ReadB()
		if err != nil {
			return ce, err
		}
		ce.NoXDictionary = b
	} else {
		ce.NoXDictionary = true
	}
	if version == R2013 || version == R2018 {
		b, err := c.ReadB()
		if err != nil {
			return ce, err
		}
		ce.BinaryChain = b
	}
	if version == R14 {
		isByLayerLtype, err := c.ReadB()
		if err != nil {
			return ce, err
		}
		ce.IsOnLayer = true
		ce.NonFixedLtype = !isByLayerLtype
	} else {
		ce.IsOnLayer = true
	}
	if version == R14 || version == R2000 {
		if _, err := c.ReadB(); err != nil {
			return ce, err
		}
	}
	colorRaw, err := c.ReadBSU()
	if err != nil {
		return ce, err
	}
	if version.IsR2004Plus() {
		flags := uint8(colorRaw >> 8)
		if flags&0x20 != 0 {
			if _, err := c.ReadBL(); err != nil {
				return ce, err
			}
		}
		if flags&0x40 == 0 && flags&0x80 != 0 {
			if _, err := c.ReadBL(); err != nil {
				return ce, err
			}
		}
		if flags&0x41 == 0x41 {
			if _, err := readTV(c, version); err != nil {
				return ce, err
			}
		}
		if flags&0x42 == 0x42 {
			if _, err := readTV(c, version); err != nil {
				return ce, err
			}
		}
	}
	if _, err := c.ReadBD(); err != nil {
		return ce, err
	}
	if version.IsR2004Plus() || version == R2000 {
		if _, err := c.ReadBB(); err != nil {
			return ce, err
		}
		ps, err := c.ReadBB()
		if err != nil {
			return ce, err
		}
		ce.PlotstyleFlag = ps
	}
	if version.IsR2007Plus() {
		mf, err := c.ReadBB()
		if err != nil {
			return ce, err
		}
		sf, err := c.ReadRC()
		if err != nil {
			return ce, err
		}
		ce.MaterialFlag = mf
		ce.ShadowFlags = sf
	}
	if version.IsR2010Plus() {
		for i := 0; i < 3; i++ {
			if _, err := c.ReadB(); err != nil {
				return ce, err
			}
		}
	}
	inv, err := c.ReadBS()
	if err != nil {
		return ce, err
	}
	ce.Invisibility = inv
	if version != R14 {
		lw, err := c.ReadRC()
		if err != nil {
			return ce, err
		}
		ce.Lineweight = lw
	}
	ce.HadGraphics = hadGraphics
	ce.HadExtendedData = hadExtended
	return ce, nil
}

// Point3D is a 3D coordinate triple.
type Point3D struct {
	X float64
	Y float64
	Z float64
}

// Vec3D is a 3D direction vector.
type Vec3D = Point3D

// ReadBD3 reads a 3D point encoded as three BDs.
func ReadBD3(c *BitCursor) (Point3D, error) {
	x, err := c.ReadBD()
	if err != nil {
		return Point3D{}, err
	}
	y, err := c.ReadBD()
	if err != nil {
		return Point3D{}, err
	}
	z, err := c.ReadBD()
	if err != nil {
		return Point3D{}, err
	}
	return Point3D{X: x, Y: y, Z: z}, nil
}

// ReadBE reads a BE (bit-extrusion) per spec §2.11.
func ReadBE(c *BitCursor) (Vec3D, error) {
	b, err := c.ReadB()
	if err != nil {
		return Vec3D{}, err
	}
	if b {
		return Vec3D{X: 0, Y: 0, Z: 1}, nil
	}
	return ReadBD3(c)
}

// ReadBT reads a BT (bit-thickness) per spec §2.12.
func ReadBT(c *BitCursor) (float64, error) {
	b, err := c.ReadB()
	if err != nil {
		return 0, err
	}
	if b {
		return 0, nil
	}
	return c.ReadBD()
}
