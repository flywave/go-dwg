package dwg

// DICTIONARY object (spec §19.5.19) — string-keyed handle map.
//
// The value handles live in the object's handle references, past the end of
// its data stream, so the data stream carries the names only and the i-th
// name pairs with the i-th item handle.

const maxDictionaryItems = 1_000_000

// Dictionary is a decoded DICTIONARY object.
type Dictionary struct {
	CloningFlag int16
	HardOwner   bool
	// Keys are the entry names in stream order; the i-th key pairs with the
	// i-th value handle in the object's handle list.
	Keys []string
}

// IndexOf returns the position of name in Keys, or (-1, false).
func (d *Dictionary) IndexOf(name string) (int, bool) {
	for i, k := range d.Keys {
		if k == name {
			return i, true
		}
	}
	return -1, false
}

// Len returns the entry count.
func (d *Dictionary) Len() int { return len(d.Keys) }

func decodeDictionaryBody(o *objectStream, version Version) (Dictionary, error) {
	numItems, err := o.data.ReadBL()
	if err != nil {
		return Dictionary{}, err
	}
	n := int(numItems)
	if n < 0 || n > maxDictionaryItems {
		return Dictionary{}, newError(ErrSectionMap, "DICTIONARY claims %d items (>%d sanity cap)", n, maxDictionaryItems)
	}
	var cloning int16
	var hardFlag uint8
	if version == R14 {
		if _, err := o.data.ReadRC(); err != nil {
			return Dictionary{}, err
		}
	} else {
		c, err := o.data.ReadBS()
		if err != nil {
			return Dictionary{}, err
		}
		cloning = c
		h, err := o.data.ReadRC()
		if err != nil {
			return Dictionary{}, err
		}
		hardFlag = h
	}
	keys := make([]string, 0, n)
	for i := 0; i < n; i++ {
		k, err := o.readTV(version)
		if err != nil {
			return Dictionary{}, err
		}
		keys = append(keys, k)
	}
	return Dictionary{CloningFlag: cloning, HardOwner: hardFlag != 0, Keys: keys}, nil
}

// decodeDictionaryObject decodes a DICTIONARY straight from its raw object
// payload.
func decodeDictionaryObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (Dictionary, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return Dictionary{}, err
	}
	d, err := decodeDictionaryBody(o, version)
	if err != nil {
		return Dictionary{}, err
	}
	if err := o.finish("DICTIONARY"); err != nil {
		return Dictionary{}, err
	}
	return d, nil
}
