package dwg

// ACAD_GROUP object (spec §19.6.7) — a named, ordered set of entity
// handles. The member handles are handle references, not data-stream
// fields, so the data stream carries only the name, the two flags and
// the member count; the i-th member handle follows in the handle stream.
//
// Ported from the reference implementation's acad_group.rs, including its
// measured deviation from the spec: the two flags are `BS` words (10 bits
// in the measured records), not `B`, and the first reads 2 on an
// AutoCAD-generated group rather than a 0/1 boolean — so it is surfaced
// as a raw int16 and the semantics of that value stay undetermined.

// maxGroupMembers caps the member-handle count, matching the reference's
// spec-safety guidance (a decompression-bomb-adjacent allocation vector).
const maxGroupMembers = 100_000

// AcadGroup is a decoded ACAD_GROUP object.
type AcadGroup struct {
	Name string
	// Unnamed is the raw first BS; non-zero on AutoCAD-generated "*A..."
	// groups. Not a bool because real records carry 2, not 1.
	Unnamed int16
	// Selectable is the raw second BS; non-zero when selecting one member
	// selects the whole group.
	Selectable int16
	// NumMembers counts the member handles that follow the data stream.
	NumMembers uint32
}

// IsUnnamed reports whether AutoCAD generated the group rather than a user.
func (g *AcadGroup) IsUnnamed() bool { return g.Unnamed != 0 }

// IsSelectable reports whether selecting one member selects the group.
func (g *AcadGroup) IsSelectable() bool { return g.Selectable != 0 }

func decodeGroupBody(o *objectStream, version Version) (AcadGroup, error) {
	name, err := o.readTV(version)
	if err != nil {
		return AcadGroup{}, err
	}
	unnamed, err := o.data.ReadBS()
	if err != nil {
		return AcadGroup{}, err
	}
	selectable, err := o.data.ReadBS()
	if err != nil {
		return AcadGroup{}, err
	}
	n, err := o.data.ReadBL()
	if err != nil {
		return AcadGroup{}, err
	}
	if n < 0 || int(n) > maxGroupMembers {
		return AcadGroup{}, newError(ErrSectionMap,
			"ACAD_GROUP claims %d member handles (>%d sanity cap)", n, maxGroupMembers)
	}
	return AcadGroup{Name: name, Unnamed: unnamed, Selectable: selectable, NumMembers: uint32(n)}, nil
}

// decodeGroupObject decodes an ACAD_GROUP straight from its raw object
// payload, taking the name from the R2007+ string stream when present and
// checking the data fields end exactly on the data-stream boundary.
func decodeGroupObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (AcadGroup, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return AcadGroup{}, err
	}
	g, err := decodeGroupBody(o, version)
	if err != nil {
		return AcadGroup{}, err
	}
	if err := o.finish("ACAD_GROUP"); err != nil {
		return AcadGroup{}, err
	}
	return g, nil
}
