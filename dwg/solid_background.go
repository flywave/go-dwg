package dwg

// ACAD solid-background object (custom class) — reconstructed from
// `OdDbSolidBackground::dwgInFields` in SCENEOE.tx: the common object data
// followed by one `rdInt32` (an OdCmEntityColor word). The reference leaves
// this object Undecoded.

// SolidBackground is a decoded solid-background object.
type SolidBackground struct {
	Color uint32
}

func decodeSolidBackgroundObject(payload []byte, bodyStart int, inlineEnd *uint32, version Version) (SolidBackground, error) {
	o, err := openObjectStream(payload, bodyStart, inlineEnd, version)
	if err != nil {
		return SolidBackground{}, err
	}
	color, err := o.data.ReadBLU()
	if err != nil {
		return SolidBackground{}, err
	}
	if err := o.finish("SOLID_BACKGROUND"); err != nil {
		return SolidBackground{}, err
	}
	return SolidBackground{Color: color}, nil
}
