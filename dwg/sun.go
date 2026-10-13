package dwg

// ACAD_SUN object (custom class) — reconstructed from
// `OdDbSun::dwgInFields` in SCENEOE.tx via vtable-offset call mapping:
//
//	BL color-index? , B, CmColor, BD, B, Date(BL,BL), B, BL, BS, RC
//
// CmColor is the standard §2.11 CMC (BS index, BL rgb, RC byte, optional
// name/book TV from the string stream). The reference leaves this object
// Undecoded.

// Sun is a decoded ACAD_SUN record.
type Sun struct {
	ColorIndex int16
}

func readCmColor(o *objectStream, version Version) error {
	if _, err := o.data.ReadBSU(); err != nil {
		return err
	}
	if _, err := o.data.ReadBLU(); err != nil {
		return err
	}
	cb, err := o.data.ReadRC()
	if err != nil {
		return err
	}
	if cb&0x01 != 0 {
		if _, err := o.readTV(version); err != nil {
			return err
		}
	}
	if cb&0x02 != 0 {
		if _, err := o.readTV(version); err != nil {
			return err
		}
	}
	return nil
}

func decodeSunObject(payload []byte, bodyStart int, inlineEnd *uint32, version Version) (Sun, error) {
	o, err := openObjectStream(payload, bodyStart, inlineEnd, version)
	if err != nil {
		return Sun{}, err
	}
	var s Sun
	if s.ColorIndex, err = o.data.ReadBS(); err != nil {
		return Sun{}, err
	}
	if _, err := o.data.ReadB(); err != nil {
		return Sun{}, err
	}
	if err := readCmColor(o, version); err != nil {
		return Sun{}, err
	}
	if _, err := o.data.ReadBD(); err != nil {
		return Sun{}, err
	}
	if _, err := o.data.ReadB(); err != nil {
		return Sun{}, err
	}
	// OdDbDate::dwgIn = BL, BL.
	if _, err := o.data.ReadBL(); err != nil {
		return Sun{}, err
	}
	if _, err := o.data.ReadBL(); err != nil {
		return Sun{}, err
	}
	if _, err := o.data.ReadB(); err != nil {
		return Sun{}, err
	}
	if _, err := o.data.ReadBL(); err != nil {
		return Sun{}, err
	}
	if _, err := o.data.ReadBS(); err != nil {
		return Sun{}, err
	}
	if _, err := o.data.ReadRC(); err != nil {
		return Sun{}, err
	}
	if err := o.finish("SUN"); err != nil {
		return Sun{}, err
	}
	return s, nil
}
