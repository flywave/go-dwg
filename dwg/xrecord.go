package dwg

// XRECORD object (spec §19.6.5 / §19.5.67) — opaque key/value storage under
// a parent DICTIONARY.
//
// Stream shape:
//
//	BL  num_databytes   -- size of the raw body (<= 16 MiB)
//	// body: concatenated raw bytes of DXF-style (code, value) pairs
//	BS  cloning_flags   -- R2000+ only
//
// XRECORD holds no TV at all, so unlike its dictionary siblings it needs no
// string-stream port — only the data-stream boundary check.

// MaxXRecordBytes is a sanity cap on the XRECORD body size (16 MiB).
const MaxXRecordBytes = 16 * 1024 * 1024

// XRecord is a decoded XRECORD object.
type XRecord struct {
	// Data is the opaque raw bytes of the (code, value) pair sequence.
	Data []byte
	// CloningFlags is the R2000+ cloning flags (0 for R14).
	CloningFlags int16
}

func decodeXRecordBody(c *BitCursor, version Version) (XRecord, error) {
	dataBytes, err := c.ReadBL()
	if err != nil {
		return XRecord{}, err
	}
	n := int(dataBytes)
	if n < 0 || n > MaxXRecordBytes {
		return XRecord{}, newError(ErrSectionMap, "XRECORD data bytes %d exceeds %d sanity cap", n, MaxXRecordBytes)
	}
	data := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		b, err := c.ReadRC()
		if err != nil {
			return XRecord{}, err
		}
		data = append(data, b)
	}
	var cloning int16
	if version != R14 {
		cloning, err = c.ReadBS()
		if err != nil {
			return XRecord{}, err
		}
	}
	return XRecord{Data: data, CloningFlags: cloning}, nil
}

// decodeXRecordObject decodes an XRECORD from its raw payload, checking the
// body ends exactly on the data-stream boundary.
func decodeXRecordObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (XRecord, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return XRecord{}, err
	}
	x, err := decodeXRecordBody(o.data, version)
	if err != nil {
		return XRecord{}, err
	}
	if err := o.finish("XRECORD"); err != nil {
		return XRecord{}, err
	}
	return x, nil
}
