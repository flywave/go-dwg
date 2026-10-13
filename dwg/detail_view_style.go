package dwg

// ACDBDETAILVIEWSTYLE object (custom class) — the style behind a model
// documentation *detail view*. Field order recovered from
// `OdDbDetailViewStyle::dwgInFields` (TD_DbEntities.tx:0xd8b98e): the
// `OdDbModelDocViewStyle` base (`BS` class version that must be 0, `TV`
// name, `B`, plus `TV`+`BL` on R2018+), then a straight-line derived body —
// `BS` version, `BL`, soft-pointer handle, CMC, `BD`, `TV`, `BD`, `RC`,
// handle, CMC, `BD`, handle, `BL`, CMC, handle, CMC, `BD`, `BL`, `BD`,
// `BL`, `TV`, handle, `BL`, CMC, handle, `BL`, CMC, `RC`, closed by a
// `filerType() != 0` gate that never fires on the file-read path. The six
// soft-pointer handles ride in the object's handle chunk, which is empty
// on the corpus record, so they cost no data-stream bits. The record
// closes on the data-stream boundary with delta 0.

// AcadDetailViewStyle is a decoded ACDBDETAILVIEWSTYLE record.
type AcadDetailViewStyle struct {
	UnknownHead int16 // base class-version word, 0
	Name        string
	UnknownFlag bool
	UnknownA    int16 // derived class-version word, 0
	UnknownB    int32
	ColorA      ObjectColor
	DoubleA     float64
	StringA     string
	DoubleB     float64
	ByteA       uint8
	ColorB      ObjectColor
	DoubleC     float64
	IntA        int32
	ColorC      ObjectColor
	ColorD      ObjectColor
	DoubleD     float64
	IntB        int32
	DoubleE     float64
	IntC        int32
	StringB     string
	IntD        int32
	ColorE      ObjectColor
	IntE        int32
	ColorF      ObjectColor
	ByteB       uint8
}

// decodeDetailViewStyleObject decodes an ACDBDETAILVIEWSTYLE straight from
// its raw object payload.
func decodeDetailViewStyleObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (AcadDetailViewStyle, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return AcadDetailViewStyle{}, err
	}
	var s AcadDetailViewStyle
	r2018 := version == R2018
	if s.UnknownHead, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.Name, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.UnknownFlag, err = o.data.ReadB(); err != nil {
		return s, err
	}
	if r2018 {
		if _, err = o.readTV(version); err != nil { // name alias
			return s, err
		}
		if _, err = o.data.ReadBL(); err != nil {
			return s, err
		}
	}
	if s.UnknownA, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.UnknownB, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	// Soft-pointer handle (handle stream; no data-stream bits).
	if s.ColorA, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.DoubleA, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.StringA, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.DoubleB, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.ByteA, err = o.data.ReadRC(); err != nil {
		return s, err
	}
	// Handle.
	if s.ColorB, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.DoubleC, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	// Handle.
	if s.IntA, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	if s.ColorC, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	// Handle.
	if s.ColorD, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.DoubleD, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.IntB, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	if s.DoubleE, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.IntC, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	if s.StringB, err = o.readTV(version); err != nil {
		return s, err
	}
	// Handle.
	if s.IntD, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	if s.ColorE, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	// Handle.
	if s.IntE, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	if s.ColorF, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.ByteB, err = o.data.ReadRC(); err != nil {
		return s, err
	}
	if err := o.finish("ACDBDETAILVIEWSTYLE"); err != nil {
		return s, err
	}
	return s, nil
}
