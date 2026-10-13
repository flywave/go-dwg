package dwg

// SORTENTSTABLE object (AcDbSortentsTable) — the draw-order table a
// block record owns. Field order recovered from
// `OdDbSortentsTableImpl::dwgInFields` (libTD_DbCore.so:0x96eb90) via the
// filer-vtable offset method and confirmed against the record's own
// data-stream boundary: `BL count`, then `count` object handles (`H`).
// The corpus record's handles come out as exactly the draw-order entity
// set and the cursor lands on the boundary with delta 0.

// SortentsEntry is one SORTENTSTABLE entry.
type SortentsEntry struct {
	Handle Handle
}

// SortentsTable is a decoded SORTENTSTABLE record.
type SortentsTable struct {
	Count   int32
	Entries []SortentsEntry
}

// decodeSortentsTableObject decodes a SORTENTSTABLE straight from its raw
// object payload.
func decodeSortentsTableObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (SortentsTable, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return SortentsTable{}, err
	}
	var s SortentsTable
	count, err := o.data.ReadBL()
	if err != nil {
		return s, err
	}
	if count < 0 || int(count) > o.data.RemainingBits() {
		return s, newError(ErrSectionMap, "SORTENTSTABLE count %d exceeds remaining bits (%d)", count, o.data.RemainingBits())
	}
	s.Count = count
	s.Entries = make([]SortentsEntry, 0, count)
	for i := int32(0); i < count; i++ {
		var e SortentsEntry
		if e.Handle, err = o.data.ReadHandle(); err != nil {
			return s, err
		}
		s.Entries = append(s.Entries, e)
	}
	if err := o.finish("SORTENTSTABLE"); err != nil {
		return s, err
	}
	return s, nil
}
