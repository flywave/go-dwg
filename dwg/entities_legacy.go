package dwg

// Legacy POLYLINE-family entities (§19.4.45/§19.4.46 and the mesh
// variants) plus OLE2FRAME, ported from the reference implementation.
// The geometry of a POLYLINE lives in the VERTEX sub-entities it owns in
// the object stream; only the headers are decoded here.

// Polyline is a decoded legacy 2D POLYLINE header.
type Polyline struct {
	Flag              uint16
	CurveType         int16
	DefaultStartWidth float64
	DefaultEndWidth   float64
	Thickness         float64
	Elevation         float64
	Extrusion         Vec3D
}

// IsClosed reports flag bit 0x01.
func (p *Polyline) IsClosed() bool { return p.Flag&0x01 != 0 }

// Is3D reports flag bit 0x08.
func (p *Polyline) Is3D() bool { return p.Flag&0x08 != 0 }

// IsPolyface reports flag bit 0x40.
func (p *Polyline) IsPolyface() bool { return p.Flag&0x40 != 0 }

// Polyline3d is the 3D polyline header: two flag bytes and, on R2004+,
// the owned VERTEX_3D count.
type Polyline3d struct {
	Flags           [2]uint8
	NumOwnedObjects uint32
	HasOwnedCount   bool
}

// PolylineFlags1/2 aliases for the two POLYLINE_3D flag bytes.
func decodePolyline(c *BitCursor) (Polyline, error) {
	var p Polyline
	var err error
	if p.Flag, err = c.ReadBSU(); err != nil {
		return p, err
	}
	if p.CurveType, err = c.ReadBS(); err != nil {
		return p, err
	}
	if p.DefaultStartWidth, err = c.ReadBD(); err != nil {
		return p, err
	}
	if p.DefaultEndWidth, err = c.ReadBD(); err != nil {
		return p, err
	}
	if p.Thickness, err = ReadBT(c); err != nil {
		return p, err
	}
	if p.Elevation, err = c.ReadBD(); err != nil {
		return p, err
	}
	p.Extrusion, err = ReadBE(c)
	return p, err
}

func decodePolyline3d(c *BitCursor, v Version) (Polyline3d, error) {
	var p Polyline3d
	var err error
	if p.Flags[0], err = c.ReadRC(); err != nil {
		return p, err
	}
	if p.Flags[1], err = c.ReadRC(); err != nil {
		return p, err
	}
	if v.IsR2004Plus() {
		n, err := c.ReadBLU()
		if err != nil {
			return p, err
		}
		p.NumOwnedObjects = n
		p.HasOwnedCount = true
	}
	return p, nil
}

// Vertex is a decoded VERTEX_2D record.
type Vertex struct {
	Flag             uint8
	Location         Point3D
	StartWidth       float64
	EndWidth         float64
	Bulge            float64
	VertexID         uint32
	HasVertexID      bool
	TangentDirection *float64
}

// VertexPoint is a VERTEX_3D / VERTEX_MESH / VERTEX_PFACE record: just a
// flag byte and the location.
type VertexPoint struct {
	Flag     uint8
	Location Point3D
}

// VertexPfaceFace is one POLYFACE face: four 1-based vertex indices; a
// negative index marks the edge from that vertex invisible, a zero index
// means the face has fewer than four corners.
type VertexPfaceFace struct {
	VertexIndices [4]int16
}

func decodeVertex(c *BitCursor, v Version) (Vertex, error) {
	var x Vertex
	var err error
	if x.Flag, err = c.ReadRC(); err != nil {
		return x, err
	}
	if x.Location, err = ReadBD3(c); err != nil {
		return x, err
	}
	if x.StartWidth, err = c.ReadBD(); err != nil {
		return x, err
	}
	if x.EndWidth, err = c.ReadBD(); err != nil {
		return x, err
	}
	if x.Bulge, err = c.ReadBD(); err != nil {
		return x, err
	}
	if v.IsR2010Plus() {
		id, err := c.ReadBLU()
		if err != nil {
			return x, err
		}
		x.VertexID = id
		x.HasVertexID = true
	}
	if x.Flag&0x02 != 0 {
		t, err := c.ReadBD()
		if err != nil {
			return x, err
		}
		x.TangentDirection = &t
	}
	return x, nil
}

func decodeVertexPoint(c *BitCursor) (VertexPoint, error) {
	var x VertexPoint
	var err error
	if x.Flag, err = c.ReadRC(); err != nil {
		return x, err
	}
	x.Location, err = ReadBD3(c)
	return x, err
}

func decodeVertexPfaceFace(c *BitCursor) (VertexPfaceFace, error) {
	var f VertexPfaceFace
	for i := range f.VertexIndices {
		v, err := c.ReadBS()
		if err != nil {
			return f, err
		}
		f.VertexIndices[i] = v
	}
	return f, nil
}

// PolyfaceMesh is a POLYLINE_PFACE header; its VERTEX_PFACE and
// VERTEX_PFACE_FACE sub-entities live in the object stream.
type PolyfaceMesh struct {
	VertexCount     uint16
	FaceCount       uint16
	NumOwnedObjects uint32
	HasOwnedCount   bool
}

// PolygonMesh is a POLYLINE_MESH header; its VERTEX_MESH grid is reached
// through the handle stream.
type PolygonMesh struct {
	Flags      uint16
	MDensity   uint16
	NDensity   uint16
	MVertCount uint16
	NVertCount uint16
}

func decodePolyfaceMesh(c *BitCursor, v Version) (PolyfaceMesh, error) {
	var m PolyfaceMesh
	var err error
	if m.VertexCount, err = c.ReadBSU(); err != nil {
		return m, err
	}
	if m.FaceCount, err = c.ReadBSU(); err != nil {
		return m, err
	}
	if v.IsR2004Plus() {
		n, err := c.ReadBLU()
		if err != nil {
			return m, err
		}
		m.NumOwnedObjects = n
		m.HasOwnedCount = true
	}
	return m, nil
}

func decodePolygonMesh(c *BitCursor) (PolygonMesh, error) {
	var m PolygonMesh
	var err error
	if m.Flags, err = c.ReadBSU(); err != nil {
		return m, err
	}
	if m.MDensity, err = c.ReadBSU(); err != nil {
		return m, err
	}
	if m.NDensity, err = c.ReadBSU(); err != nil {
		return m, err
	}
	if m.MVertCount, err = c.ReadBSU(); err != nil {
		return m, err
	}
	m.NVertCount, err = c.ReadBSU()
	return m, err
}

// Ole2Frame is a decoded embedded-OLE frame; Data carries the raw OLE
// stream bytes.
type Ole2Frame struct {
	OleVersion    int16
	Data          []byte
	OleObjectType int16
	Mode          uint8
}

const ole2FrameMaxDataBytes = 16 * 1024 * 1024

func decodeOle2Frame(c *BitCursor) (Ole2Frame, error) {
	var o Ole2Frame
	var err error
	if o.OleVersion, err = c.ReadBS(); err != nil {
		return o, err
	}
	length, err := c.ReadBL()
	if err != nil {
		return o, err
	}
	if length < 0 || int(length) > ole2FrameMaxDataBytes {
		return o, newError(ErrSectionMap, "OLE2FRAME data_length %d out of range", length)
	}
	if int(length)*8 > c.RemainingBits() {
		return o, newError(ErrSectionMap, "OLE2FRAME data_length %d exceeds remaining %d bits", length, c.RemainingBits())
	}
	o.Data = make([]byte, length)
	for i := range o.Data {
		if o.Data[i], err = c.ReadRC(); err != nil {
			return o, err
		}
	}
	if o.OleObjectType, err = c.ReadBS(); err != nil {
		return o, err
	}
	o.Mode, err = c.ReadRC()
	return o, err
}
