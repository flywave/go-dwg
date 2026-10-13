package dwg

// ACAD_LIGHT object (custom class) — reconstructed from
// `OdDbLight::dwgInFields` in SCENEOE.tx. LIGHT extends **OdDbEntity**, so its
// common data is the entity preamble (§19.4.1), not the object prefix; the
// reference leaves it Undecoded.
//
// `OdDbLightImpl::dwgInFields` body:
//
//	BL, TV name, BL, B, CmColor, B, BD, rdPoint3d, rdPoint3d, BL, B, BD x4,
//	B, BL, BS, RC, if(filerType()!=0){ B; if (B) rdPhotometricData }

// Light is a decoded ACAD_LIGHT record.
type Light struct {
	Name string
}

func readLightPhotometricData(c *BitCursor, strings *stringReader, version Version) error {
	readTVs := func() (string, error) {
		if strings != nil {
			return strings.readTV()
		}
		return readTV(c, version)
	}
	hasName, err := c.ReadB()
	if err != nil {
		return err
	}
	if hasName {
		if _, err := readTVs(); err != nil {
			return err
		}
	}
	steps := []func() error{
		func() error { _, e := c.ReadBS(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBS(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBS(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBS(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBS(); return e },
		func() error { _, e := c.ReadBS(); return e },
		func() error { _, e := c.ReadBS(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBD(); return e },
		func() error { _, e := c.ReadBS(); return e },
	}
	for _, st := range steps {
		if err := st(); err != nil {
			return err
		}
	}
	return nil
}

// decodeLightEntity decodes a LIGHT via the common entity preamble.
func decodeLightEntity(raw *RawObject, version Version) (Light, error) {
	c, err := BodyCursor(raw, version)
	if err != nil {
		return Light{}, err
	}
	dataEnd, hasEnd := entityDataEnd(raw, version)
	strings := entityStrings(raw, version)
	if _, err := ReadCommonEntityData(c, version); err != nil {
		return Light{}, err
	}
	var l Light
	if _, err := c.ReadBL(); err != nil {
		return Light{}, err
	}
	if l.Name, err = readFromStringsOrInline(c, strings, version); err != nil {
		return Light{}, err
	}
	if _, err := c.ReadBL(); err != nil {
		return Light{}, err
	}
	if _, err := c.ReadB(); err != nil {
		return Light{}, err
	}
	if err := readCmColorCursor(c, strings, version); err != nil {
		return Light{}, err
	}
	if _, err := c.ReadB(); err != nil {
		return Light{}, err
	}
	if _, err := c.ReadBD(); err != nil {
		return Light{}, err
	}
	if _, err := ReadBD3(c); err != nil {
		return Light{}, err
	}
	if _, err := ReadBD3(c); err != nil {
		return Light{}, err
	}
	if _, err := c.ReadBL(); err != nil {
		return Light{}, err
	}
	if _, err := c.ReadB(); err != nil {
		return Light{}, err
	}
	for i := 0; i < 4; i++ {
		if _, err := c.ReadBD(); err != nil {
			return Light{}, err
		}
	}
	if _, err := c.ReadB(); err != nil {
		return Light{}, err
	}
	if _, err := c.ReadBL(); err != nil {
		return Light{}, err
	}
	if _, err := c.ReadBS(); err != nil {
		return Light{}, err
	}
	if _, err := c.ReadRC(); err != nil {
		return Light{}, err
	}
	// The final block is gated on `filerType() != 0`. A file-read filer
	// reports kFileFiler (0), so the photometric block is not present when
	// reading a drawing; `readLightPhotometricData` handles the other filers.
	if hasEnd && c.PositionBits() != dataEnd {
		return Light{}, newError(ErrSectionMap, "LIGHT data fields ended at bit %d, data stream ends at %d (delta %d)",
			c.PositionBits(), dataEnd, c.PositionBits()-dataEnd)
	}
	return l, nil
}

func entityStrings(raw *RawObject, version Version) *stringReader {
	if !version.IsR2007Plus() {
		return nil
	}
	if s, ok := LocateStringStream(raw.Raw, version); ok {
		if r, err := newStringReader(raw.Raw, s); err == nil {
			return r
		}
	}
	return emptyStringReader(raw.Raw)
}

func readFromStringsOrInline(c *BitCursor, strings *stringReader, version Version) (string, error) {
	if strings != nil {
		return strings.readTV()
	}
	return readTV(c, version)
}

// readCmColorCursor reads an OdCmColor (§2.11 CMC) from a cursor + string stream.
func readCmColorCursor(c *BitCursor, strings *stringReader, version Version) error {
	if _, err := c.ReadBSU(); err != nil {
		return err
	}
	if _, err := c.ReadBLU(); err != nil {
		return err
	}
	cb, err := c.ReadRC()
	if err != nil {
		return err
	}
	if cb&0x01 != 0 {
		if _, err := readFromStringsOrInline(c, strings, version); err != nil {
			return err
		}
	}
	if cb&0x02 != 0 {
		if _, err := readFromStringsOrInline(c, strings, version); err != nil {
			return err
		}
	}
	return nil
}
