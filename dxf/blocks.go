package dxf

// BLOCKS section — a block is the `0 BLOCK` header run, its entity list,
// and the `0 ENDBLK` terminator. Header and terminator tag runs are
// preserved verbatim (same discipline as TableEntry.Rest) so extension
// dictionaries, R12 dialects and unknown fields round-trip byte-faithfully;
// typed accessors expose the codes consumers actually need.

// Block is one block definition.
type Block struct {
	// Tags is the BLOCK record verbatim (everything after the `0 BLOCK`
	// pair, in stream order).
	Tags []Tag
	// Entities are the drawables between BLOCK and ENDBLK.
	Entities []*Entity
	// EndTags is the ENDBLK record verbatim.
	EndTags []Tag

	// allocEnd is a synthesized ENDBLK handle set before setEndblk runs.
	allocEnd uint64
}

// SetEndHandle records the ENDBLK handle used by setEndblk.
func (b *Block) SetEndHandle(h uint64) { b.allocEnd = h }

// Block flags (group 70).
const (
	BlockAnonymous       int64 = 1
	BlockHasAttribs      int64 = 2
	BlockIsXref          int64 = 4
	BlockXrefOverlay     int64 = 8
	BlockIsExternallyDep int64 = 16
	BlockIsResolved      int64 = 32
	BlockIsReferenced    int64 = 64
)

// scanGroup returns the first tag with the given code.
func scanGroup(tags []Tag, code int) (Tag, bool) {
	for _, t := range tags {
		if t.Code == code {
			return t, true
		}
	}
	return Tag{}, false
}

// Name returns the block name (group 2).
func (b *Block) Name() string {
	if t, ok := scanGroup(b.Tags, 2); ok {
		return t.Value
	}
	return ""
}

// SetName updates the block name (groups 2 and 3 stay in sync).
func (b *Block) SetName(name string) {
	seen2 := false
	for i := range b.Tags {
		if b.Tags[i].Code == 2 {
			b.Tags[i].Value = name
			seen2 = true
			break
		}
	}
	if !seen2 {
		b.Tags = append(b.Tags, Tag{Code: 2, Value: name})
	}
	for i := range b.Tags {
		if b.Tags[i].Code == 3 {
			b.Tags[i].Value = name
			break
		}
	}
}

// Handle returns the block-record handle (group 5).
func (b *Block) Handle() uint64 {
	if t, ok := scanGroup(b.Tags, 5); ok {
		return parseHandle(t.Value)
	}
	return 0
}

// Owner returns the owner handle (first group 330).
func (b *Block) Owner() uint64 {
	if t, ok := scanGroup(b.Tags, 330); ok {
		return parseHandle(t.Value)
	}
	return 0
}

// Layer returns the block entity's layer (group 8).
func (b *Block) Layer() string {
	if t, ok := scanGroup(b.Tags, 8); ok {
		return t.Value
	}
	return ""
}

// Flags returns group 70.
func (b *Block) Flags() int64 {
	if t, ok := scanGroup(b.Tags, 70); ok {
		return t.Int()
	}
	return 0
}

// BasePoint returns the block base point (groups 10/20/30).
func (b *Block) BasePoint() Point3 {
	var p Point3
	for _, t := range b.Tags {
		switch t.Code {
		case 10:
			p.X = t.Float()
		case 20:
			p.Y = t.Float()
		case 30:
			p.Z = t.Float()
		}
	}
	return p
}

// SetBasePoint updates the base point triple in place.
func (b *Block) SetBasePoint(p Point3) {
	setCoord := func(code int, v float64) {
		for i := range b.Tags {
			if b.Tags[i].Code == code {
				b.Tags[i].Value = FormatFloat(v)
				return
			}
		}
		b.Tags = append(b.Tags, Tag{Code: code, Value: FormatFloat(v)})
	}
	setCoord(10, p.X)
	setCoord(20, p.Y)
	setCoord(30, p.Z)
}

// NewBlock builds a BLOCK record with the canonical tag layout for the
// target dialect.
func NewBlock(name string, ver DxfVersion) *Block {
	b := &Block{}
	sub := ver.SupportsSubclassMarkers()
	if sub {
		b.Tags = append(b.Tags,
			Tag{Code: 100, Value: "AcDbEntity"},
			Tag{Code: 8, Value: "0"},
			Tag{Code: 100, Value: "AcDbBlockBegin"},
		)
	}
	b.Tags = append(b.Tags,
		Tag{Code: 2, Value: name},
		Tag{Code: 70, Value: "0"},
		Tag{Code: 10, Value: "0"},
		Tag{Code: 20, Value: "0"},
		Tag{Code: 30, Value: "0"},
		Tag{Code: 3, Value: name},
		Tag{Code: 1, Value: ""},
	)
	return b
}

// setEndblk canonicalizes the ENDBLK terminator for the target dialect.
func (b *Block) setEndblk(ver DxfVersion) {
	b.EndTags = b.EndTags[:0]
	if ver.SupportsSubclassMarkers() {
		b.EndTags = append(b.EndTags,
			Tag{Code: 100, Value: "AcDbEntity"},
			Tag{Code: 8, Value: "0"},
			Tag{Code: 100, Value: "AcDbBlockEnd"},
		)
	}
}

func (w *TagWriter) writeBlock(b *Block, ver DxfVersion) {
	w.Str(0, "BLOCK")
	// Tags are verbatim — handles and owners ride inside them.
	w.Pairs(b.Tags)
	for _, e := range b.Entities {
		if err := writeEntity(w, e, ver); err != nil {
			continue // malformed payload degrades to a skip
		}
	}
	w.Str(0, "ENDBLK")
	w.Pairs(b.EndTags)
}

// SetHandleOwner sets (or replaces) the leading 5/330 pair of the BLOCK
// record for synthesized blocks.
func (b *Block) SetHandleOwner(h, owner uint64) {
	setLeading(&b.Tags, 5, h)
	setLeading(&b.Tags, 330, owner)
}

// setLeading updates the first tag with the given code, or prepends it
// ahead of any subclass markers.
func setLeading(tags *[]Tag, code int, v uint64) {
	for i := range *tags {
		if (*tags)[i].Code == code {
			(*tags)[i].Value = padHandle(v)
			return
		}
	}
	// Prepend after existing 5/330 pairs but before 100 markers.
	pos := 0
	for pos < len(*tags) {
		c := (*tags)[pos].Code
		if c == 5 || c == 330 {
			pos++
			continue
		}
		break
	}
	*tags = append(*tags, Tag{Code: code, Value: padHandle(v)})
	copy((*tags)[pos+1:], (*tags)[pos:])
	(*tags)[pos] = Tag{Code: code, Value: padHandle(v)}
}
