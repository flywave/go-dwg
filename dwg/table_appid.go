package dwg

// APPID table entry (spec §19.5.50) — the registered application name used as
// a key in XDATA appid handles. Stream shape: entry header + RC unknown.

// AppID is a decoded APPID table entry.
type AppID struct {
	Header  TableEntryHeader
	Unknown uint8
}

func decodeAppIDInline(c *BitCursor, version Version) (AppID, error) {
	header, err := readTableEntryHeader(c, version)
	if err != nil {
		return AppID{}, err
	}
	unknown, err := c.ReadRC()
	if err != nil {
		return AppID{}, err
	}
	return AppID{Header: header, Unknown: unknown}, nil
}

func decodeAppIDModern(payload []byte, bodyStart int, version Version) (AppID, error) {
	split, err := openTableEntry(payload, bodyStart, version)
	if err != nil {
		return AppID{}, err
	}
	flag64, xrefIdx, xdep, err := readEntryFlags(split.data)
	if err != nil {
		return AppID{}, err
	}
	unknown, err := split.data.ReadRC()
	if err != nil {
		return AppID{}, err
	}
	if err := split.finish("APPID"); err != nil {
		return AppID{}, err
	}
	name, err := split.strings.readTV()
	if err != nil {
		return AppID{}, err
	}
	return AppID{
		Header:  TableEntryHeader{Name: name, IsXrefDependent: xdep, XrefIndexPlus1: xrefIdx, IsXrefResolved: flag64},
		Unknown: unknown,
	}, nil
}
