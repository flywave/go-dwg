package dwg

// RawObject is a single undecoded object extracted from the
// AcDb:AcDbObjects stream.
type RawObject struct {
	// StreamOffset is the byte offset where this object begins.
	StreamOffset int
	// SizeBytes is the record's leading MS value verbatim. On R2010+ it
	// excludes the record's own leading MC handle-stream-size field.
	SizeBytes uint32
	// TypeCode is the raw type code as encoded in the stream.
	TypeCode uint16
	// Kind is the classified ObjectType.
	Kind ObjectType
	// Handle is the object's handle.
	Handle Handle
	// Raw is the record's bytes between the MS and the CRC (including the
	// R2010+ leading MC field).
	Raw []byte
	// ObjSizeBits is the R2000-R2007 RL object-data-size-in-bits, or nil.
	ObjSizeBits *uint32
}

// IsEntity reports whether this object is a drawable entity.
func (o *RawObject) IsEntity() bool { return o.Kind.IsEntity() }

// SkippedEntry describes a handle-driven record that failed to parse.
type SkippedEntry struct {
	Handle uint64
	Offset uint64
	Reason string
}

// ObjectWalkSummary is a summary of a lossy walk.
type ObjectWalkSummary struct {
	DecodedCount     int
	Skipped          []SkippedEntry
	ErroredCount     int
	UnknownTypes     [][2]uint64
	HandleMismatches [][3]uint64
}

// SkippedCount returns the number of skipped records.
func (s *ObjectWalkSummary) SkippedCount() int { return len(s.Skipped) }

// Confidence is decoded / (decoded + skipped + errored), clamped to
// [0,1]. An empty walk returns 1.0.
func (s *ObjectWalkSummary) Confidence() float64 {
	total := s.DecodedCount + s.SkippedCount() + s.ErroredCount
	if total == 0 {
		return 1.0
	}
	r := float64(s.DecodedCount) / float64(total)
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// ObjectWalker iterates every object in an AcDb:AcDbObjects byte stream.
type ObjectWalker struct {
	bytes          []byte
	pos            int
	version        Version
	handleMap      *HandleMap
	handleIdx      int
	limits         WalkerLimits
	recordsVisited int
	mismatches     [][3]uint64
}

// NewObjectWalker starts a sequential walker over the object-section bytes.
func NewObjectWalker(bytes []byte, version Version) *ObjectWalker {
	initial := 0
	if len(bytes) >= 4 && version.IsR2004Plus() {
		initial = 4
	}
	return &ObjectWalker{bytes: bytes, pos: initial, version: version, limits: SafeWalkerLimits()}
}

// WithHandleMap starts a handle-driven walker.
func WithHandleMap(bytes []byte, version Version, map_ *HandleMap) *ObjectWalker {
	return &ObjectWalker{bytes: bytes, version: version, handleMap: map_, limits: SafeWalkerLimits()}
}

// WithLimits overrides the walker count cap.
func (w *ObjectWalker) WithLimits(limits WalkerLimits) *ObjectWalker {
	w.limits = limits
	return w
}

// CollectAll reads all objects, silently skipping records that fail to
// parse in handle-driven mode.
func (w *ObjectWalker) CollectAll() ([]RawObject, error) {
	var out []RawObject
	for {
		raw, ok, err := w.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			return out, nil
		}
		out = append(out, raw)
	}
}

// CollectAllLossy reads all objects alongside a walk summary.
func (w *ObjectWalker) CollectAllLossy() ([]RawObject, ObjectWalkSummary) {
	var out []RawObject
	var summary ObjectWalkSummary
	for {
		item, ok, err := w.nextItem()
		if err != nil {
			break
		}
		if !ok {
			break
		}
		if item.object != nil {
			raw := *item.object
			if raw.Kind.IsUnknown() {
				summary.UnknownTypes = append(summary.UnknownTypes, [2]uint64{uint64(raw.TypeCode), uint64(raw.StreamOffset)})
			}
			summary.DecodedCount++
			out = append(out, raw)
		} else {
			summary.Skipped = append(summary.Skipped, *item.skipped)
		}
	}
	summary.HandleMismatches = w.mismatches
	return out, summary
}

type walkItem struct {
	object  *RawObject
	skipped *SkippedEntry
}

func (w *ObjectWalker) next() (RawObject, bool, error) {
	for {
		item, ok, err := w.nextItem()
		if err != nil {
			return RawObject{}, false, err
		}
		if !ok {
			return RawObject{}, false, nil
		}
		if item.object != nil {
			return *item.object, true, nil
		}
	}
}

func (w *ObjectWalker) nextItem() (walkItem, bool, error) {
	if w.recordsVisited >= w.limits.MaxHandles {
		return walkItem{}, false, newError(ErrWalkerLimitExceeded, "visited %d records, exceeds limit %d", w.recordsVisited, w.limits.MaxHandles)
	}
	if w.handleMap != nil {
		if w.handleIdx >= len(w.handleMap.Entries) {
			return walkItem{}, false, nil
		}
		entry := w.handleMap.Entries[w.handleIdx]
		w.handleIdx++
		w.recordsVisited++
		pos := int(entry.Offset)
		if pos >= len(w.bytes) {
			return walkItem{skipped: &SkippedEntry{Handle: entry.Handle, Offset: entry.Offset,
				Reason: "offset past end of object stream"}}, true, nil
		}
		w.pos = pos
		raw, ok, err := w.readOneAtPos()
		if err != nil {
			return walkItem{skipped: &SkippedEntry{Handle: entry.Handle, Offset: entry.Offset, Reason: err.Error()}}, true, nil
		}
		if !ok {
			return walkItem{skipped: &SkippedEntry{Handle: entry.Handle, Offset: entry.Offset,
				Reason: "record read returned None (truncated or zero-length)"}}, true, nil
		}
		if raw.Handle.Value != entry.Handle {
			w.mismatches = append(w.mismatches, [3]uint64{entry.Handle, raw.Handle.Value, entry.Offset})
		}
		raw.Handle.Value = entry.Handle
		return walkItem{object: &raw}, true, nil
	}
	if w.pos >= len(w.bytes) {
		return walkItem{}, false, nil
	}
	w.recordsVisited++
	raw, ok, err := w.readOneAtPos()
	if err != nil {
		return walkItem{}, false, err
	}
	if !ok {
		return walkItem{}, false, nil
	}
	return walkItem{object: &raw}, true, nil
}

func (w *ObjectWalker) readOneAtPos() (RawObject, bool, error) {
	start := w.pos
	sizeBytes, msConsumed, err := readMSByteAligned(w.bytes[start:])
	if err != nil {
		return RawObject{}, false, err
	}
	if sizeBytes == 0 {
		w.pos = len(w.bytes)
		return RawObject{}, false, nil
	}
	payloadStart := start + msConsumed
	if payloadStart > len(w.bytes) {
		w.pos = len(w.bytes)
		return RawObject{}, false, nil
	}
	mcWidth := 0
	if w.version.IsR2010Plus() {
		width, ok := mcFieldWidth(w.bytes[payloadStart:])
		if !ok {
			w.pos = len(w.bytes)
			return RawObject{}, false, nil
		}
		mcWidth = width
	}
	payloadEnd := payloadStart + int(sizeBytes) + mcWidth
	crcEnd := payloadEnd + 2
	if crcEnd > len(w.bytes) {
		w.pos = len(w.bytes)
		return RawObject{}, false, nil
	}
	payload := w.bytes[payloadStart:payloadEnd]
	cur := NewBitCursor(payload)
	if w.version.IsR2010Plus() {
		if _, err := readMCUnsigned(cur); err != nil {
			return RawObject{}, false, err
		}
	}
	typeCode, err := readObjectType(cur, w.version)
	if err != nil {
		return RawObject{}, false, err
	}
	var objSizeBits *uint32
	if w.version.HasObjectSizeField() {
		v, err := cur.ReadRL()
		if err != nil {
			return RawObject{}, false, err
		}
		objSizeBits = &v
	}
	handle, err := cur.ReadHandle()
	if err != nil {
		return RawObject{}, false, err
	}
	raw := append([]byte(nil), payload...)
	w.pos = crcEnd
	return RawObject{
		StreamOffset: start,
		SizeBytes:    sizeBytes,
		TypeCode:     typeCode,
		Kind:         ObjectTypeFromCode(typeCode),
		Handle:       handle,
		Raw:          raw,
		ObjSizeBits:  objSizeBits,
	}, true, nil
}

// readMSByteAligned reads a byte-aligned modular short. Returns
// (value, bytesConsumed).
func readMSByteAligned(bytes []byte) (uint32, int, error) {
	var value uint32
	var shift uint
	i := 0
	for {
		if i+1 >= len(bytes) {
			return 0, 0, newError(ErrTruncated, "modular short needs 2 bytes at offset %d", i)
		}
		lo := uint32(bytes[i])
		hi := uint32(bytes[i+1])
		i += 2
		module := (hi << 8) | lo
		cont := module&0x8000 != 0
		data := module & 0x7FFF
		value |= data << shift
		shift += 15
		if !cont || shift >= 32 {
			return value, i, nil
		}
	}
}

// mcFieldWidth returns the width in bytes of the leading MC field, or
// (0, false) when no byte in the first ten terminates it.
func mcFieldWidth(bytes []byte) (int, bool) {
	limit := len(bytes)
	if limit > 10 {
		limit = 10
	}
	for i := 0; i < limit; i++ {
		if bytes[i]&0x80 == 0 {
			return i + 1, true
		}
	}
	return 0, false
}

// readMCUnsigned reads an unsigned modular char (0x40 is not sign here).
func readMCUnsigned(r *BitCursor) (uint64, error) {
	var value uint64
	var shift uint
	for {
		b, err := r.ReadRC()
		if err != nil {
			return 0, err
		}
		cont := b&0x80 != 0
		value |= uint64(b&0x7F) << shift
		shift += 7
		if !cont || shift >= 64 {
			return value, nil
		}
	}
}

// readObjectType reads the object type field.
func readObjectType(c *BitCursor, version Version) (uint16, error) {
	if version.IsR2010Plus() {
		tag, err := c.ReadBB()
		if err != nil {
			return 0, err
		}
		switch tag {
		case 0:
			b, err := c.ReadRC()
			return uint16(b), err
		case 1:
			b, err := c.ReadRC()
			return uint16(b) + 0x1F0, err
		default:
			lsb, err := c.ReadRC()
			if err != nil {
				return 0, err
			}
			msb, err := c.ReadRC()
			if err != nil {
				return 0, err
			}
			return uint16(msb)<<8 | uint16(lsb), nil
		}
	}
	return c.ReadBSU()
}

// BodyCursor opens a BitCursor on raw positioned at the object body.
func BodyCursor(raw *RawObject, version Version) (*BitCursor, error) {
	cursor := NewBitCursor(raw.Raw)
	if version.IsR2010Plus() {
		if _, err := readMCUnsigned(cursor); err != nil {
			return nil, err
		}
	}
	if _, err := readObjectType(cursor, version); err != nil {
		return nil, err
	}
	if version.HasObjectSizeField() {
		if _, err := cursor.ReadRL(); err != nil {
			return nil, err
		}
	}
	if _, err := cursor.ReadHandle(); err != nil {
		return nil, err
	}
	return cursor, nil
}
