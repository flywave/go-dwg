package dwg

// ACDBSECTIONVIEWSTYLE object (custom class) — the style behind a model
// documentation *section view*: identifier, cutting-plane arrows, view
// label and the cut-surface hatch. There is no §20.4 prescription; the
// field list is the reference's byte-measured layout, which closes every
// corpus record on the data-stream boundary.

const sectionViewHatchAngleCount = 5

// AcadSectionViewStyle is a decoded ACDBSECTIONVIEWSTYLE record.
type AcadSectionViewStyle struct {
	UnknownHead                 int16
	Name                        string
	NameAlias                   string
	UnknownFlagA                bool
	UnknownFlagB                bool
	Flags                       int16
	FlagC                       bool
	FlagD                       bool
	FlagE                       bool
	IdentifierColor             ObjectColor
	IdentifierHeight            float64
	ArrowSymbolColor            ObjectColor
	ArrowSymbolSize             float64
	IdentifierExcludeCharacters string
	IdentifierOffset            float64
	CuttingPlaneLineWeight      int16
	CuttingPlaneLineColor       ObjectColor
	EndLineWeight               int16
	EndLineColor                ObjectColor
	EndLineLength               float64
	EndLineOvershoot            float64
	ViewLabelColor              ObjectColor
	ViewLabelTextHeight         float64
	ViewLabelPosition           int16
	ViewLabelOffset             float64
	ViewLabelAttachment         int16
	ViewLabelPattern            string
	HatchColor                  ObjectColor
	HatchBackgroundColor        ObjectColor
	HatchPatternName            string
	HatchFlags                  uint8
	HatchScale                  float64
	HatchTransparency           int16
	HatchSpacing                float64
	HatchAngleCount             int16
	UnknownTail                 int16
	HatchAngles                 [sectionViewHatchAngleCount]float64
}

// decodeSectionViewStyleObject decodes an ACDBSECTIONVIEWSTYLE straight from
// its raw object payload.
func decodeSectionViewStyleObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (AcadSectionViewStyle, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return AcadSectionViewStyle{}, err
	}
	var s AcadSectionViewStyle
	r2018 := version == R2018
	if s.UnknownHead, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.Name, err = o.readTV(version); err != nil {
		return s, err
	}
	if r2018 {
		if s.NameAlias, err = o.readTV(version); err != nil {
			return s, err
		}
		if s.UnknownFlagA, err = o.data.ReadB(); err != nil {
			return s, err
		}
		if s.UnknownFlagB, err = o.data.ReadB(); err != nil {
			return s, err
		}
	}
	if s.Flags, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.FlagC, err = o.data.ReadB(); err != nil {
		return s, err
	}
	if s.FlagD, err = o.data.ReadB(); err != nil {
		return s, err
	}
	if s.FlagE, err = o.data.ReadB(); err != nil {
		return s, err
	}
	if s.IdentifierColor, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.IdentifierHeight, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.ArrowSymbolColor, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.ArrowSymbolSize, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.IdentifierExcludeCharacters, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.IdentifierOffset, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.CuttingPlaneLineWeight, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.CuttingPlaneLineColor, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.EndLineWeight, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.EndLineColor, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.EndLineLength, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.EndLineOvershoot, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.ViewLabelColor, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.ViewLabelTextHeight, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.ViewLabelPosition, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.ViewLabelOffset, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.ViewLabelAttachment, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.ViewLabelPattern, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.HatchColor, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.HatchBackgroundColor, err = readObjectColor(o.data, o.strings, version); err != nil {
		return s, err
	}
	if s.HatchPatternName, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.HatchFlags, err = o.data.ReadRC(); err != nil {
		return s, err
	}
	if s.HatchScale, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.HatchTransparency, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.HatchSpacing, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.HatchAngleCount, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.UnknownTail, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	for i := range s.HatchAngles {
		if s.HatchAngles[i], err = o.data.ReadBD(); err != nil {
			return s, err
		}
	}
	if err := o.finish("ACDBSECTIONVIEWSTYLE"); err != nil {
		return s, err
	}
	return s, nil
}
