package dwg

// SH_HISTORY object (OdDbShHistory) — the associative-shape history. Field
// order recovered from `OdDbShHistory::dwgInFields` (TD_3DSolidHistory.tx:0xb77e8),
// which calls `OdDbObject::dwgInFields` then `OdDbShHistoryImpl::dwgInFields`
// (0xb82bc): three `rdInt32` words, one `rdDbHandle` (an `H` field, which on
// R2007+ lives in the object's handle stream and costs no data-stream bits),
// then two `rdBool` flags. The data stream is therefore `BL, BL, BL, B, B`
// and the corpus records close with delta 0.

// ShHistory is a decoded SH_HISTORY record.
type ShHistory struct {
	Field0 int32
	Field1 int32
	Field2 int32
	FlagA  bool
	FlagB  bool
}

// decodeShHistoryObject decodes an SH_HISTORY straight from its raw object
// payload.
func decodeShHistoryObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (ShHistory, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return ShHistory{}, err
	}
	var h ShHistory
	if h.Field0, err = o.data.ReadBL(); err != nil {
		return h, err
	}
	if h.Field1, err = o.data.ReadBL(); err != nil {
		return h, err
	}
	if h.Field2, err = o.data.ReadBL(); err != nil {
		return h, err
	}
	if h.FlagA, err = o.data.ReadB(); err != nil {
		return h, err
	}
	if h.FlagB, err = o.data.ReadB(); err != nil {
		return h, err
	}
	if err := o.finish("SH_HISTORY"); err != nil {
		return h, err
	}
	return h, nil
}
