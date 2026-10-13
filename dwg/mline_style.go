package dwg

// MLINESTYLE object (§20.4.73) — the style record backing the MLINE
// entity: name/description, flags, fill colour, start/end angles and the
// per-line offset/colour/linetype elements.

// maxMlineStyleLines mirrors §20.4.73's one-RC line count (AutoCAD caps a
// style at 16 elements).
const maxMlineStyleLines = 16

// MlineStyleLine is one parallel line element of an MLINESTYLE.
type MlineStyleLine struct {
	Offset        float64
	Color         ObjectColor
	LinetypeIndex int16
}

// AcadMlinestyle is a decoded MLINESTYLE record.
type AcadMlinestyle struct {
	Name        string
	Description string
	Flags       int16
	FillColor   ObjectColor
	StartAngle  float64
	EndAngle    float64
	Lines       []MlineStyleLine
}

// decodeMlineStyleObject decodes an MLINESTYLE straight from its raw object
// payload.
func decodeMlineStyleObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (AcadMlinestyle, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return AcadMlinestyle{}, err
	}
	var m AcadMlinestyle
	if m.Name, err = o.readTV(version); err != nil {
		return m, err
	}
	if m.Description, err = o.readTV(version); err != nil {
		return m, err
	}
	if m.Flags, err = o.data.ReadBS(); err != nil {
		return m, err
	}
	if m.FillColor, err = readObjectColor(o.data, o.strings, version); err != nil {
		return m, err
	}
	if m.StartAngle, err = o.data.ReadBD(); err != nil {
		return m, err
	}
	if m.EndAngle, err = o.data.ReadBD(); err != nil {
		return m, err
	}
	numLines, err := o.data.ReadRC()
	if err != nil {
		return m, err
	}
	if int(numLines) > maxMlineStyleLines {
		return m, newError(ErrSectionMap, "MLINESTYLE claims %d lines (max %d)", numLines, maxMlineStyleLines)
	}
	m.Lines = make([]MlineStyleLine, 0, numLines)
	for i := uint8(0); i < numLines; i++ {
		var line MlineStyleLine
		if line.Offset, err = o.data.ReadBD(); err != nil {
			return m, err
		}
		if line.Color, err = readObjectColor(o.data, o.strings, version); err != nil {
			return m, err
		}
		// R2018 moved the per-line linetype to a handle, which lives in the
		// handle stream and costs no data-stream bits.
		if version != R2018 {
			if line.LinetypeIndex, err = o.data.ReadBS(); err != nil {
				return m, err
			}
		}
		m.Lines = append(m.Lines, line)
	}
	if err := o.finish("MLINESTYLE"); err != nil {
		return m, err
	}
	return m, nil
}
