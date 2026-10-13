package dwg

// MENTALRAYRENDERSETTINGS object (OdDbMentalRayRenderSettings) — render
// settings. Field order recovered from `OdDbMentalRayRenderSettings::dwgInFields`
// (SCENEOE.tx:0x81d46) plus its base `OdDbRenderSettings::dwgInFields`
// (0x7f45a): a base block of `BL, [TV], B×4, [TV×2], BL`, then
// `BL×3, BS, BD×6, BS, B×2, BL×3, B, BL, B, BD, BL×4, B, BL, B×3, BD×3,
// BS×2, BD, BS×2, B, [TV], BL, BS, BL, B, BD`. The corpus record closes
// with delta 0. `TV` fields cost no data-stream bits on R2007+.

// MentalRayRenderSettings is a decoded MENTALRAYRENDERSETTINGS record.
type MentalRayRenderSettings struct {
	RenderFlags int32
	Flags       [4]bool
	Trailing    int32
	Ints        []int32   // the BL words, in wire order
	Shorts      []int16   // the BS words, in wire order
	Doubles     []float64 // the BD words, in wire order
	Bools       []bool    // the B words, in wire order
	Strings     []string  // the TV words, in wire order
}

// decodeMentalRayRenderSettingsObject decodes a MENTALRAYRENDERSETTINGS
// straight from its raw object payload.
func decodeMentalRayRenderSettingsObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (MentalRayRenderSettings, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return MentalRayRenderSettings{}, err
	}
	var m MentalRayRenderSettings
	readTV := func() error {
		s, err := o.readTV(version)
		if err != nil {
			return err
		}
		m.Strings = append(m.Strings, s)
		return nil
	}
	// Base: BL, TV, B×4, TV, TV, BL.
	if m.RenderFlags, err = o.data.ReadBL(); err != nil {
		return m, err
	}
	if err := readTV(); err != nil {
		return m, err
	}
	for i := range m.Flags {
		if m.Flags[i], err = o.data.ReadB(); err != nil {
			return m, err
		}
	}
	if err := readTV(); err != nil {
		return m, err
	}
	if err := readTV(); err != nil {
		return m, err
	}
	if m.Trailing, err = o.data.ReadBL(); err != nil {
		return m, err
	}
	// Derived body.
	bl := func() error { v, e := o.data.ReadBL(); m.Ints = append(m.Ints, v); return e }
	bs := func() error { v, e := o.data.ReadBS(); m.Shorts = append(m.Shorts, v); return e }
	bd := func() error { v, e := o.data.ReadBD(); m.Doubles = append(m.Doubles, v); return e }
	bb := func() error { v, e := o.data.ReadB(); m.Bools = append(m.Bools, v); return e }
	seq := []func() error{
		bl, bl, bl, bs, bd, bd, bd, bd, bd, bd, bs, bb, bb,
		bl, bl, bl, bb, bl, bb, bd,
		bl, bl, bl, bl, bb, bl, bb, bb, bb,
		bd, bd, bd, bs, bs, bd, bs, bs, bb,
		readTV, bl, bs, bl, bb, bd,
	}
	for _, fn := range seq {
		if err := fn(); err != nil {
			return m, err
		}
	}
	if err := o.finish("MENTALRAYRENDERSETTINGS"); err != nil {
		return m, err
	}
	return m, nil
}
