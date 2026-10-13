package dwg

// LWPOLYLINE entity (§20.4.85 "LWPLINE") — the lightweight polyline whose
// vertices, bulges, vertex ids and widths all pack into one record. The
// leading BS flag word marks which optional groups are present; 0x200 is
// the closed flag, not a presence bit.

const lwpolylineMaxPoints = 1_000_000

// LWPOLYLINE flag bits (§20.4.85).
const (
	lwHasExtrusion     = 0x0001
	lwHasThickness     = 0x0002
	lwConstantWidth    = 0x0004
	lwHasElevation     = 0x0008
	lwHasBulges        = 0x0010
	lwHasVariableWidth = 0x0020
	lwClosed           = 0x0200
	lwHasVertexID      = 0x0400
)

// LwPolyline is a decoded lightweight polyline.
type LwPolyline struct {
	Flag          uint16
	ConstantWidth *float64
	Elevation     *float64
	Thickness     *float64
	Extrusion     *Vec3D
	Vertices      []Point2D
	Bulges        []float64
	VertexIDs     []uint32
	Widths        [][2]float64
	Closed        bool
}

// decodeLwPolyline reads the LWPOLYLINE payload that follows the common
// entity header.
func decodeLwPolyline(c *BitCursor, v Version) (LwPolyline, error) {
	var p LwPolyline
	flag, err := c.ReadBSU()
	if err != nil {
		return p, err
	}
	p.Flag = flag
	if flag&lwConstantWidth != 0 {
		x, err := c.ReadBD()
		if err != nil {
			return p, err
		}
		p.ConstantWidth = &x
	}
	if flag&lwHasElevation != 0 {
		x, err := c.ReadBD()
		if err != nil {
			return p, err
		}
		p.Elevation = &x
	}
	if flag&lwHasThickness != 0 {
		x, err := c.ReadBD()
		if err != nil {
			return p, err
		}
		p.Thickness = &x
	}
	if flag&lwHasExtrusion != 0 {
		x, err := c.ReadBD()
		if err != nil {
			return p, err
		}
		y, err := c.ReadBD()
		if err != nil {
			return p, err
		}
		z, err := c.ReadBD()
		if err != nil {
			return p, err
		}
		p.Extrusion = &Vec3D{X: x, Y: y, Z: z}
	}
	numPoints, err := c.ReadBL()
	if err != nil {
		return p, err
	}
	np := int(numPoints)
	numBulges := 0
	if flag&lwHasBulges != 0 {
		if numBulges, err = intErr(c.ReadBL()); err != nil {
			return p, err
		}
	}
	numIDs := 0
	if v.IsR2010Plus() && flag&lwHasVertexID != 0 {
		if numIDs, err = intErr(c.ReadBL()); err != nil {
			return p, err
		}
	}
	numWidths := 0
	if flag&lwHasVariableWidth != 0 {
		if numWidths, err = intErr(c.ReadBL()); err != nil {
			return p, err
		}
	}
	// Layered defensive caps: a hard ceiling, a coarse whole-record claim
	// check against the remaining bits, and a per-item minimum-bits floor
	// (each point/bulge/id/width costs at least its two DD/BD/BL tag bits).
	if np < 0 || numBulges < 0 || numIDs < 0 || numWidths < 0 ||
		np > lwpolylineMaxPoints || numBulges > lwpolylineMaxPoints ||
		numIDs > lwpolylineMaxPoints || numWidths > lwpolylineMaxPoints {
		return p, newError(ErrSectionMap, "LWPOLYLINE has implausible counts (p=%d, b=%d, i=%d, w=%d)", np, numBulges, numIDs, numWidths)
	}
	total := np + numBulges + numIDs + numWidths
	remaining := c.RemainingBits()
	if total > remaining || 4*np+2*numBulges+2*numIDs+4*numWidths > remaining {
		return p, newError(ErrSectionMap, "LWPOLYLINE claims %d items with %d bits left", total, remaining)
	}
	p.Vertices = make([]Point2D, 0, np)
	if np > 0 {
		x, err := c.ReadRD()
		if err != nil {
			return p, err
		}
		y, err := c.ReadRD()
		if err != nil {
			return p, err
		}
		prev := Point2D{X: x, Y: y}
		p.Vertices = append(p.Vertices, prev)
		for k := 1; k < np; k++ {
			x, err := c.ReadDD(prev.X)
			if err != nil {
				return p, err
			}
			y, err := c.ReadDD(prev.Y)
			if err != nil {
				return p, err
			}
			prev = Point2D{X: x, Y: y}
			p.Vertices = append(p.Vertices, prev)
		}
	}
	for k := 0; k < numBulges; k++ {
		b, err := c.ReadBD()
		if err != nil {
			return p, err
		}
		p.Bulges = append(p.Bulges, b)
	}
	for k := 0; k < numIDs; k++ {
		id, err := c.ReadBLU()
		if err != nil {
			return p, err
		}
		p.VertexIDs = append(p.VertexIDs, id)
	}
	for k := 0; k < numWidths; k++ {
		w1, err := c.ReadBD()
		if err != nil {
			return p, err
		}
		w2, err := c.ReadBD()
		if err != nil {
			return p, err
		}
		p.Widths = append(p.Widths, [2]float64{w1, w2})
	}
	p.Closed = flag&lwClosed != 0
	return p, nil
}

func intErr(v int32, err error) (int, error) {
	return int(v), err
}
