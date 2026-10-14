package dwg

// ACIS-based surface entities and the subdivision MESH, ported from the
// reference implementation. All four surfaces lead with the same SAT
// envelope the 3DSOLID record uses (readSatBlob), then add a handful of
// their own fields. MLEADER remains the one large entity not ported here.

// ExtrudedSurface is a decoded ACDBEXTRUDEDSURFACE.
type ExtrudedSurface struct {
	Sat         SatBlob
	SweepVector Vec3D
	DraftAngle  float64
}

// RevolvedSurface is a decoded ACDBREVOLVEDSURFACE.
type RevolvedSurface struct {
	Sat           SatBlob
	AxisOrigin    Point3D
	AxisDirection Vec3D
	SweepAngle    float64
}

// SweptSurface is a decoded ACDBSWEPTSURFACE.
type SweptSurface struct {
	Sat         SatBlob
	PathHandle  uint64
	HasPath     bool
	AlignOption uint8 // 0 none, 1 normal, 2 translate, 3 forced alignment
}

// LoftedSurface is a decoded ACDBLOFTEDSURFACE; the cross-section handles
// live in the record body (pre-R2007) or handle stream.
type LoftedSurface struct {
	Sat              SatBlob
	NumCrossSections int32
	CrossSections    []uint64
}

func decodeExtrudedSurface(c *BitCursor, v Version, common *CommonEntityData) (ExtrudedSurface, error) {
	var s ExtrudedSurface
	var err error
	if s.Sat, err = decodeSurfaceSatBlob(c, v, common); err != nil {
		return s, err
	}
	if s.SweepVector, err = ReadBD3(c); err != nil {
		return s, err
	}
	s.DraftAngle, err = c.ReadBD()
	return s, err
}

func decodeRevolvedSurface(c *BitCursor, v Version, common *CommonEntityData) (RevolvedSurface, error) {
	var s RevolvedSurface
	var err error
	if s.Sat, err = decodeSurfaceSatBlob(c, v, common); err != nil {
		return s, err
	}
	if s.AxisOrigin, err = ReadBD3(c); err != nil {
		return s, err
	}
	if s.AxisDirection, err = ReadBD3(c); err != nil {
		return s, err
	}
	s.SweepAngle, err = c.ReadBD()
	return s, err
}

func decodeSweptSurface(c *BitCursor, v Version, common *CommonEntityData) (SweptSurface, error) {
	var s SweptSurface
	var err error
	if s.Sat, err = decodeSurfaceSatBlob(c, v, common); err != nil {
		return s, err
	}
	if h, err := c.ReadHandle(); err != nil {
		return s, err
	} else if h.Value != 0 {
		s.PathHandle = h.Value
		s.HasPath = true
	}
	s.AlignOption, err = c.ReadBB()
	if err != nil {
		return s, err
	}
	if s.AlignOption > 3 {
		return s, newError(ErrSectionMap, "SWEPTSURFACE alignment option %d not in 0..=3", s.AlignOption)
	}
	return s, nil
}

func decodeLoftedSurface(c *BitCursor, v Version, common *CommonEntityData) (LoftedSurface, error) {
	var s LoftedSurface
	var err error
	if s.Sat, err = decodeSurfaceSatBlob(c, v, common); err != nil {
		return s, err
	}
	if s.NumCrossSections, err = c.ReadBL(); err != nil {
		return s, err
	}
	if s.NumCrossSections < 0 || int(s.NumCrossSections) > c.RemainingBits() {
		return s, newError(ErrSectionMap, "LOFTEDSURFACE cross-section count %d out of range", s.NumCrossSections)
	}
	for i := int32(0); i < s.NumCrossSections; i++ {
		h, err := c.ReadHandle()
		if err != nil {
			return s, err
		}
		if h.Value != 0 {
			s.CrossSections = append(s.CrossSections, h.Value)
		}
	}
	return s, nil
}

// decodeSurfaceSatBlob reads the ACIS envelope; R2013+ records carrying
// binary-chain data store no inline SAT bytes, matching 3DSOLID.
func decodeSurfaceSatBlob(c *BitCursor, v Version, common *CommonEntityData) (SatBlob, error) {
	if common != nil && common.BinaryChain {
		return SatBlob{Empty: true}, nil
	}
	return readSatBlob(c)
}

// Mesh is a decoded ACDBSUBDMESH (R2010+).
type Mesh struct {
	Version          uint16
	BlendCrease      bool
	SubdivisionLevel uint16
	Vertices         []Point3D
	Faces            [][]uint32
	Edges            [][2]uint32
	Creases          []float64
}

const (
	meshCapVertices  = 1_000_000
	meshCapFaceList  = 8_000_000
	meshCapFaceVerts = 64
	meshCapEdges     = 4_000_000
	meshCapCreases   = 4_000_000
)

func meshBoundsCheck(n int, what string, cap, remaining int) error {
	if n < 0 || n > cap || n > remaining {
		return newError(ErrSectionMap, "MESH %s count %d exceeds cap %d or %d remaining bits", what, n, cap, remaining)
	}
	return nil
}

func decodeMesh(c *BitCursor, v Version) (Mesh, error) {
	var m Mesh
	if !v.IsR2010Plus() {
		return m, newError(ErrUnsupported, "MESH requires R2010+")
	}
	var err error
	if m.Version, err = c.ReadBSU(); err != nil {
		return m, err
	}
	if m.BlendCrease, err = c.ReadB(); err != nil {
		return m, err
	}
	if m.SubdivisionLevel, err = c.ReadBSU(); err != nil {
		return m, err
	}
	nv, err := c.ReadBSU()
	if err != nil {
		return m, err
	}
	if err = meshBoundsCheck(int(nv), "vertex", meshCapVertices, c.RemainingBits()); err != nil {
		return m, err
	}
	for i := 0; i < int(nv); i++ {
		p, err := ReadBD3(c)
		if err != nil {
			return m, err
		}
		m.Vertices = append(m.Vertices, p)
	}
	listSize, err := c.ReadBLU()
	if err != nil {
		return m, err
	}
	if err = meshBoundsCheck(int(listSize), "face list", meshCapFaceList, c.RemainingBits()); err != nil {
		return m, err
	}
	// Measured: the count is the length of the face list in BL entries,
	// each face costing 1 + its vertex count.
	consumed := 0
	for consumed < int(listSize) {
		fvc, err := c.ReadBLU()
		if err != nil {
			return m, err
		}
		if int(fvc) > meshCapFaceVerts {
			return m, newError(ErrSectionMap, "MESH face vertex count %d exceeds cap %d", fvc, meshCapFaceVerts)
		}
		consumed++
		if consumed+int(fvc) > int(listSize) {
			return m, newError(ErrSectionMap, "MESH face of %d vertices overruns the %d-entry face list", fvc, listSize)
		}
		face := make([]uint32, 0, fvc)
		for k := 0; k < int(fvc); k++ {
			idx, err := c.ReadBLU()
			if err != nil {
				return m, err
			}
			if int(idx) >= int(nv) {
				return m, newError(ErrSectionMap, "MESH face vertex index %d out of range (%d vertices)", idx, nv)
			}
			face = append(face, idx)
		}
		consumed += int(fvc)
		m.Faces = append(m.Faces, face)
	}
	ne, err := c.ReadBLU()
	if err != nil {
		return m, err
	}
	if err = meshBoundsCheck(int(ne), "edge", meshCapEdges, c.RemainingBits()); err != nil {
		return m, err
	}
	for i := 0; i < int(ne); i++ {
		a, err := c.ReadBLU()
		if err != nil {
			return m, err
		}
		b, err := c.ReadBLU()
		if err != nil {
			return m, err
		}
		if int(a) >= int(nv) || int(b) >= int(nv) {
			return m, newError(ErrSectionMap, "MESH edge index out of range (%d vertices)", nv)
		}
		m.Edges = append(m.Edges, [2]uint32{a, b})
	}
	nc, err := c.ReadBLU()
	if err != nil {
		return m, err
	}
	if err = meshBoundsCheck(int(nc), "crease", meshCapCreases, c.RemainingBits()); err != nil {
		return m, err
	}
	for i := 0; i < int(nc); i++ {
		x, err := c.ReadBD()
		if err != nil {
			return m, err
		}
		m.Creases = append(m.Creases, x)
	}
	return m, nil
}

// Camera is a decoded ACDBCAMERA entity (R2007+).
type Camera struct {
	Origin      Point3D
	Direction   Vec3D
	FocalLength float64
	FieldOfView float64
	ViewHandle  uint64
	HasView     bool
}

func decodeCamera(c *BitCursor, v Version) (Camera, error) {
	var cam Camera
	if !v.IsR2007Plus() {
		return cam, newError(ErrUnsupported, "CAMERA requires R2007+")
	}
	var err error
	if cam.Origin, err = ReadBD3(c); err != nil {
		return cam, err
	}
	if cam.Direction, err = ReadBD3(c); err != nil {
		return cam, err
	}
	if cam.FocalLength, err = c.ReadBD(); err != nil {
		return cam, err
	}
	if cam.FieldOfView, err = c.ReadBD(); err != nil {
		return cam, err
	}
	if h, err := c.ReadHandle(); err != nil {
		return cam, err
	} else if h.Value != 0 {
		cam.ViewHandle = h.Value
		cam.HasView = true
	}
	return cam, nil
}

// GeoData is a decoded GEODATA object (R2010+); the five TV fields live in
// the string stream on real files.
type GeoData struct {
	Version                uint32
	BlockHandle            uint64
	HasBlock               bool
	Origin                 Point3D
	DesignToRefXAxis       Point3D
	DesignToRefYAxis       Point3D
	ReferencePoint         Point3D
	ReferenceNorthDir      Point3D
	DesignToRefXScale      float64
	DesignToRefYScale      float64
	CoordinateSystemDef    string
	GeoRSSTag              string
	ObservationFromTag     string
	ObservationToTag       string
	ObservationCoverageTag string
}

// decodeGeoDataObject reads a GEODATA from its raw payload, with the TV
// fields taken from the R2007+/R2010+ string stream.
func decodeGeoDataObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (GeoData, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return GeoData{}, err
	}
	var g GeoData
	if !version.IsR2010Plus() {
		return g, newError(ErrUnsupported, "GEODATA requires R2010+")
	}
	ver, err := o.data.ReadBLU()
	if err != nil {
		return g, err
	}
	g.Version = ver
	if h, err := o.data.ReadHandle(); err != nil {
		return g, err
	} else if h.Value != 0 {
		g.BlockHandle = h.Value
		g.HasBlock = true
	}
	if g.Origin, err = ReadBD3(o.data); err != nil {
		return g, err
	}
	if g.DesignToRefXAxis, err = ReadBD3(o.data); err != nil {
		return g, err
	}
	if g.DesignToRefYAxis, err = ReadBD3(o.data); err != nil {
		return g, err
	}
	if g.ReferencePoint, err = ReadBD3(o.data); err != nil {
		return g, err
	}
	if g.ReferenceNorthDir, err = ReadBD3(o.data); err != nil {
		return g, err
	}
	if g.DesignToRefXScale, err = o.data.ReadBD(); err != nil {
		return g, err
	}
	if g.DesignToRefYScale, err = o.data.ReadBD(); err != nil {
		return g, err
	}
	read := func() (string, error) {
		if o.strings != nil {
			return o.strings.readTV()
		}
		return readTV(o.data, version)
	}
	if g.CoordinateSystemDef, err = read(); err != nil {
		return g, err
	}
	if g.GeoRSSTag, err = read(); err != nil {
		return g, err
	}
	if g.ObservationFromTag, err = read(); err != nil {
		return g, err
	}
	if g.ObservationToTag, err = read(); err != nil {
		return g, err
	}
	g.ObservationCoverageTag, err = read()
	if err != nil {
		return g, err
	}
	if err := o.finish("GEODATA"); err != nil {
		return g, err
	}
	return g, nil
}

// MLine is a decoded MLINE entity (multiple parallel lines).
type MLine struct {
	ScaleFactor     float64
	Justification   uint8 // 0 top, 1 zero, 2 bottom
	ScalePoint      Point3D
	Extrusion       Vec3D
	OpenClosedFlags int16
	NumLines        int
	Vertices        []MLineVertex
}

// MLineVertex is one MLINE vertex with its per-line segment parameters.
type MLineVertex struct {
	Position           Point3D
	SegmentDirection   Point3D
	MiterDirection     Point3D
	SegmentParameters  []float64
	AreaFillParameters []float64
}

const (
	mlineCapLines          = 1_024
	mlineCapVerts          = 100_000
	mlineCapSegmentParams  = 1_024
	mlineCapAreaFillParams = 1_024
)

func decodeMLine(c *BitCursor) (MLine, error) {
	var m MLine
	var err error
	if m.ScaleFactor, err = c.ReadBD(); err != nil {
		return m, err
	}
	if m.Justification, err = c.ReadRC(); err != nil {
		return m, err
	}
	if m.ScalePoint, err = ReadBD3(c); err != nil {
		return m, err
	}
	if m.Extrusion, err = ReadBD3(c); err != nil {
		return m, err
	}
	if m.OpenClosedFlags, err = c.ReadBS(); err != nil {
		return m, err
	}
	nl, err := c.ReadRC()
	if err != nil {
		return m, err
	}
	m.NumLines = int(nl)
	if m.NumLines > mlineCapLines {
		return m, newError(ErrSectionMap, "MLINE line count %d exceeds cap %d", m.NumLines, mlineCapLines)
	}
	// num_verts is a 32-bit BL, not a BS. ODA's OdDbMlineImpl::dwgInFields
	// reads it with the 32-bit filer method (rdInt32) and passes the result
	// to OdArray<MLVertex>::resize; a BS here only coincides with a BL for
	// counts 0..127.
	nv, err := c.ReadBL()
	if err != nil {
		return m, err
	}
	if nv < 0 || int(nv) > mlineCapVerts || int(nv)*18 > c.RemainingBits() {
		return m, newError(ErrSectionMap, "MLINE implausible counts (lines=%d, verts=%d; %d bits left)", nl, nv, c.RemainingBits())
	}
	for k := 0; k < int(nv); k++ {
		var vx MLineVertex
		if vx.Position, err = ReadBD3(c); err != nil {
			return m, err
		}
		if vx.SegmentDirection, err = ReadBD3(c); err != nil {
			return m, err
		}
		if vx.MiterDirection, err = ReadBD3(c); err != nil {
			return m, err
		}
		for line := 0; line < m.NumLines; line++ {
			sc, err := c.ReadBS()
			if err != nil {
				return m, err
			}
			if int(sc) > mlineCapSegmentParams || int(sc)*2 > c.RemainingBits() {
				return m, newError(ErrSectionMap, "MLINE segment param count %d out of range", sc)
			}
			for i := int16(0); i < sc; i++ {
				x, err := c.ReadBD()
				if err != nil {
					return m, err
				}
				vx.SegmentParameters = append(vx.SegmentParameters, x)
			}
			ac, err := c.ReadBS()
			if err != nil {
				return m, err
			}
			if int(ac) > mlineCapAreaFillParams || int(ac)*2 > c.RemainingBits() {
				return m, newError(ErrSectionMap, "MLINE area fill param count %d out of range", ac)
			}
			for i := int16(0); i < ac; i++ {
				x, err := c.ReadBD()
				if err != nil {
					return m, err
				}
				vx.AreaFillParameters = append(vx.AreaFillParameters, x)
			}
		}
		m.Vertices = append(m.Vertices, vx)
	}
	return m, nil
}
