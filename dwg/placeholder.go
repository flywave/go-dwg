package dwg

// ACDB_PLACEHOLDER object (type code 0x50) — a body-less object. It carries
// the common object data (whose reactor count is the only number it holds)
// and its handle references, and nothing else.

// Placeholder is a decoded ACDB_PLACEHOLDER.
type Placeholder struct {
	NumReactors uint32
}

// decodePlaceholderObject decodes a placeholder and requires its common
// object data to land exactly on the data-stream boundary.
func decodePlaceholderObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (Placeholder, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return Placeholder{}, err
	}
	if err := o.finish("ACDB_PLACEHOLDER"); err != nil {
		return Placeholder{}, err
	}
	return Placeholder{NumReactors: o.numReactors}, nil
}
