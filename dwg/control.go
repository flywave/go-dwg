package dwg

// Control objects (spec §19.5.1..§19.5.10) — table owners. Nine of the ten
// have the body `BL num_entries`; DIMSTYLE_CONTROL carries one extra RC.

// Control is a decoded *_CONTROL object.
type Control struct {
	// Kind is the control's spec label (e.g. "LAYER_CONTROL").
	Kind       string
	NumEntries uint32
	// DimstyleTrailingRC is the extra RC DIMSTYLE_CONTROL records carry;
	// HasDimstyleRC distinguishes "not present" from "present with value 0".
	DimstyleTrailingRC uint8
	HasDimstyleRC      bool
}

func decodeControlBody(o *objectStream, version Version, kind ObjectType) (Control, error) {
	n, err := o.data.ReadBL()
	if err != nil {
		return Control{}, err
	}
	c := Control{Kind: kind.ShortLabel(), NumEntries: uint32(n)}
	if kind.Code == oDimStyleControl && version != R14 {
		rc, err := o.data.ReadRC()
		if err != nil {
			return Control{}, err
		}
		c.DimstyleTrailingRC = rc
		c.HasDimstyleRC = true
	}
	return c, nil
}

// decodeControlObject decodes a *_CONTROL object from its raw payload.
func decodeControlObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version, kind ObjectType) (Control, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return Control{}, err
	}
	c, err := decodeControlBody(o, version, kind)
	if err != nil {
		return Control{}, err
	}
	if err := o.finish(kind.ShortLabel()); err != nil {
		return Control{}, err
	}
	return c, nil
}
