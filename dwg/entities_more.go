package dwg

// More fixed-code entities ported from the reference implementation:
// VIEWPORT (§19.4.40-ish, fixed code 0x22), 3DFACE (§19.4.20), LEADER
// (§19.4.32) and TOLERANCE (§19.4.44).

// ViewportEntity is a decoded VIEWPORT entity (a layout's paper-space
// viewport, not the VPORT table preset).
type ViewportEntity struct {
	Center Point3D
	Width  float64
	Height float64
}

// ThreeDFace is a decoded 3DFACE quad.
type ThreeDFace struct {
	Corners        [4]Point3D
	InvisibleEdges uint16
	HasNoFlag      bool
	IsTriangle     bool
}

// Leader is a decoded LEADER annotation.
type Leader struct {
	AnnotType              int16
	PathType               int16
	Points                 []Point3D
	EndProjection          Point3D
	Extrusion              Vec3D
	HorizDirection         Vec3D
	OffsetToBlockInsertion Vec3D
}

// Tolerance is a decoded feature-control-frame annotation.
type Tolerance struct {
	UnknownShort   int16
	Height         float64
	DimGap         float64
	InsertionPoint Point3D
	XAxisDirection Vec3D
	Extrusion      Vec3D
	TextString     string
	DimstyleHandle uint64
	HasDimstyle    bool
}

const leaderMaxPoints = 100_000

// readDD3 reads three DDs patching the previous 3D point.
func readDD3(c *BitCursor, prev Point3D) (Point3D, error) {
	x, err := c.ReadDD(prev.X)
	if err != nil {
		return Point3D{}, err
	}
	y, err := c.ReadDD(prev.Y)
	if err != nil {
		return Point3D{}, err
	}
	z, err := c.ReadDD(prev.Z)
	if err != nil {
		return Point3D{}, err
	}
	return Point3D{X: x, Y: y, Z: z}, nil
}

func decodeViewportEntity(c *BitCursor) (ViewportEntity, error) {
	var v ViewportEntity
	var err error
	if v.Center, err = ReadBD3(c); err != nil {
		return v, err
	}
	if v.Width, err = c.ReadBD(); err != nil {
		return v, err
	}
	v.Height, err = c.ReadBD()
	return v, err
}

func decodeThreeDFace(c *BitCursor) (ThreeDFace, error) {
	var f ThreeDFace
	var err error
	if f.HasNoFlag, err = c.ReadB(); err != nil {
		return f, err
	}
	zIsZero, err := c.ReadB()
	if err != nil {
		return f, err
	}
	x, err := c.ReadRD()
	if err != nil {
		return f, err
	}
	y, err := c.ReadRD()
	if err != nil {
		return f, err
	}
	z := 0.0
	if !zIsZero {
		if z, err = c.ReadRD(); err != nil {
			return f, err
		}
	}
	f.Corners[0] = Point3D{X: x, Y: y, Z: z}
	if f.Corners[1], err = readDD3(c, f.Corners[0]); err != nil {
		return f, err
	}
	if f.Corners[2], err = readDD3(c, f.Corners[1]); err != nil {
		return f, err
	}
	if f.Corners[3], err = readDD3(c, f.Corners[2]); err != nil {
		return f, err
	}
	if !f.HasNoFlag {
		if f.InvisibleEdges, err = c.ReadBSU(); err != nil {
			return f, err
		}
	}
	f.IsTriangle = f.Corners[3] == f.Corners[2]
	return f, nil
}

func decodeLeader(c *BitCursor) (Leader, error) {
	var l Leader
	if _, err := c.ReadB(); err != nil { // unknown bit
		return l, err
	}
	var err error
	if l.AnnotType, err = c.ReadBS(); err != nil {
		return l, err
	}
	if l.PathType, err = c.ReadBS(); err != nil {
		return l, err
	}
	n, err := c.ReadBL()
	if err != nil {
		return l, err
	}
	if n < 0 || int(n) > leaderMaxPoints || int(n) > c.RemainingBits() {
		return l, newError(ErrSectionMap, "LEADER claims %d points (cap %d, %d bits left)", n, leaderMaxPoints, c.RemainingBits())
	}
	for i := int32(0); i < n; i++ {
		p, err := ReadBD3(c)
		if err != nil {
			return l, err
		}
		l.Points = append(l.Points, p)
	}
	if l.EndProjection, err = ReadBD3(c); err != nil {
		return l, err
	}
	if l.Extrusion, err = ReadBD3(c); err != nil {
		return l, err
	}
	if l.HorizDirection, err = ReadBD3(c); err != nil {
		return l, err
	}
	l.OffsetToBlockInsertion, err = ReadBD3(c)
	return l, err
}

// decodeTolerance reads TOLERANCE. On the R2007+ split layout the three
// leading fields and the trailing DIMSTYLE handle are absent — the
// reference measured its R2018 records as exactly three BD3 triples, and
// the TV then comes from the string stream.
func decodeTolerance(c *BitCursor, v Version, strings *stringReader) (Tolerance, error) {
	var t Tolerance
	var err error
	if strings == nil {
		if v.IsR2007Plus() {
			if t.UnknownShort, err = c.ReadBS(); err != nil {
				return t, err
			}
		}
		if t.Height, err = c.ReadBD(); err != nil {
			return t, err
		}
		if t.DimGap, err = c.ReadBD(); err != nil {
			return t, err
		}
	}
	if t.InsertionPoint, err = ReadBD3(c); err != nil {
		return t, err
	}
	if t.XAxisDirection, err = ReadBD3(c); err != nil {
		return t, err
	}
	if t.Extrusion, err = ReadBD3(c); err != nil {
		return t, err
	}
	if t.TextString, err = readTVSource(c, strings, v); err != nil {
		return t, err
	}
	if strings == nil {
		h, err := c.ReadHandle()
		if err != nil {
			return t, err
		}
		if h.Value != 0 {
			t.DimstyleHandle = h.Value
			t.HasDimstyle = true
		}
	}
	return t, nil
}

// AttributeBody carries the attribute-specific fields shared by ATTRIB
// and ATTDEF (§20.4.4/§20.4.5).
type AttributeBody struct {
	AttributeType uint8
	FieldLength   int16
	Flags         uint8
	LockPosition  bool
	// MultiLine is set when the R2018 record embeds an MTEXT.
	MultiLine bool
}

// maxAnnotativeData caps the annotative-data byte count of a multi-line
// attribute.
const maxAnnotativeData = 65_535

// readAttributeBody reads the R2010+ attribute body following the shared
// TEXT fields.
func readAttributeBody(c *BitCursor, v Version, strings *stringReader) (AttributeBody, error) {
	var b AttributeBody
	var err error
	if v.IsR2010Plus() {
		if _, err = c.ReadRC(); err != nil { // version byte
			return b, err
		}
	}
	if v == R2018 {
		if b.AttributeType, err = c.ReadRC(); err != nil {
			return b, err
		}
	} else {
		b.AttributeType = 1
	}
	if b.AttributeType == 1 {
		if b.FieldLength, err = c.ReadBS(); err != nil {
			return b, err
		}
		if b.Flags, err = c.ReadRC(); err != nil {
			return b, err
		}
		if v.IsR2007Plus() {
			if b.LockPosition, err = c.ReadB(); err != nil {
				return b, err
			}
		}
		return b, nil
	}
	// Multi-line: an embedded MTEXT starting at its entity-mode bits.
	if _, err = readEntityModeOnwards(c, v, false, false); err != nil {
		return b, err
	}
	if _, err = decodeMText(c, v, strings); err != nil {
		return b, err
	}
	b.MultiLine = true
	size, err := c.ReadBSU()
	if err != nil {
		return b, err
	}
	if int(size) > maxAnnotativeData || int(size)*8 > c.RemainingBits() {
		return b, newError(ErrSectionMap, "ATTRIB annotative data size %d out of range", size)
	}
	if size > 0 {
		for i := uint16(0); i < size; i++ {
			if _, err = c.ReadRC(); err != nil {
				return b, err
			}
		}
		// H registered application — handle stream, no data bits; the
		// two trailing BS words of the annotative block and the tag TV
		// (0 data bits on R2007+) follow.
		if _, err = c.ReadBS(); err != nil {
			return b, err
		}
		if _, err = c.ReadBS(); err != nil {
			return b, err
		}
	}
	if b.Flags, err = c.ReadRC(); err != nil {
		return b, err
	}
	b.LockPosition, err = c.ReadB()
	return b, err
}

// Attrib is a decoded ATTRIB entity.
type Attrib struct {
	Text          Text
	Tag           string
	FieldLength   int16
	Flags         uint8
	LockPosition  bool
	AttributeType uint8
	MultiLine     bool
}

// decodeAttrib reads an ATTRIB body (single-line and R2018 multi-line).
func decodeAttrib(c *BitCursor, v Version, strings *stringReader) (Attrib, error) {
	var a Attrib
	var err error
	if a.Text, err = decodeText(c, v, strings); err != nil {
		return a, err
	}
	body, err := readAttributeBody(c, v, strings)
	if err != nil {
		return a, err
	}
	a.AttributeType = body.AttributeType
	a.FieldLength = body.FieldLength
	a.Flags = body.Flags
	a.LockPosition = body.LockPosition
	a.MultiLine = body.MultiLine
	if strings != nil {
		if a.Tag, err = strings.readTV(); err != nil {
			return a, err
		}
	} else if a.Tag, err = readTV(c, v); err != nil {
		return a, err
	}
	return a, nil
}

// AttDef is a decoded ATTDEF entity.
type AttDef struct {
	Text          Text
	Prompt        string
	Tag           string
	FieldLength   int16
	Flags         uint8
	LockPosition  bool
	AttributeType uint8
	MultiLine     bool
}

// decodeAttdef reads an ATTDEF body (§20.4.5): the shared TEXT fields,
// then prompt and tag TVs, then the attribute body.
func decodeAttdef(c *BitCursor, v Version, strings *stringReader) (AttDef, error) {
	var d AttDef
	var err error
	if d.Text, err = decodeText(c, v, strings); err != nil {
		return d, err
	}
	read := func() (string, error) {
		if strings != nil {
			return strings.readTV()
		}
		return readTV(c, v)
	}
	if d.Prompt, err = read(); err != nil {
		return d, err
	}
	if d.Tag, err = read(); err != nil {
		return d, err
	}
	body, err := readAttributeBody(c, v, strings)
	if err != nil {
		return d, err
	}
	d.AttributeType = body.AttributeType
	d.FieldLength = body.FieldLength
	d.Flags = body.Flags
	d.LockPosition = body.LockPosition
	d.MultiLine = body.MultiLine
	return d, nil
}
