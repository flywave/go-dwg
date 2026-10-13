package dwg

// Simple fixed-code entities ported from the reference implementation:
// RAY/XLINE (§19.4.36/§19.4.37), SOLID (§19.4.42), TRACE (§19.4.45 —
// shares SOLID's encoding), TEXT (§19.4.46) and INSERT (§20.4.9).

// Point2D is a 2D coordinate pair.
type Point2D struct {
	X float64
	Y float64
}

// Ray is a decoded RAY: origin plus a direction.
type Ray struct {
	Start     Point3D
	Direction Vec3D
}

// XLine is a decoded construction line (infinite in both directions).
type XLine struct {
	Point     Point3D
	Direction Vec3D
}

// Solid is a decoded SOLID/TRACE quad.
type Solid struct {
	Thickness float64
	Elevation float64
	Corners   [4]Point2D
	Extrusion Vec3D
}

// Text is a decoded single-line annotation (§19.4.46). The optional-field
// data flag sets one bit per elided member — a set bit means "use the
// default", per the measured R2018 records (DataFlags = 0xFF leaves only
// the flag, insertion point, BE, BT and height on the stream).
type Text struct {
	Elevation      float64
	InsertionPoint Point2D
	AlignmentPoint *Point2D
	Extrusion      Vec3D
	Thickness      float64
	ObliqueAngle   float64
	RotationAngle  float64
	Height         float64
	WidthFactor    float64
	Text           string
	Generation     int16
	HAlign         int16
	VAlign         int16
}

// Insert is a decoded block reference (§20.4.9).
type Insert struct {
	InsertionPoint   Point3D
	Scale            Point3D
	Rotation         float64
	Extrusion        Vec3D
	HasAttribs       bool
	OwnedObjectCount uint32
}

// maxInsertOwnedObjects caps the INSERT owned-object count.
const maxInsertOwnedObjects = 65536

// decodeSimpleEntity dispatches the entities defined in this file; called
// from DecodeFromRaw's body switch.
func decodeSimpleEntity(code uint16, c *BitCursor, v Version, strings *stringReader) (any, error) {
	switch code {
	case oRay:
		return decodeRay(c)
	case oXLine:
		return decodeXLine(c)
	case oSolid, oTrace:
		return decodeSolid(c)
	case oText:
		return decodeText(c, v, strings)
	case oInsert:
		return decodeInsert(c, v)
	}
	return nil, nil
}

func decodeRay(c *BitCursor) (Ray, error) {
	start, err := ReadBD3(c)
	if err != nil {
		return Ray{}, err
	}
	dir, err := ReadBD3(c)
	if err != nil {
		return Ray{}, err
	}
	return Ray{Start: start, Direction: dir}, nil
}

func decodeXLine(c *BitCursor) (XLine, error) {
	point, err := ReadBD3(c)
	if err != nil {
		return XLine{}, err
	}
	dir, err := ReadBD3(c)
	if err != nil {
		return XLine{}, err
	}
	return XLine{Point: point, Direction: dir}, nil
}

func decodeSolid(c *BitCursor) (Solid, error) {
	thickness, err := ReadBT(c)
	if err != nil {
		return Solid{}, err
	}
	elevation, err := c.ReadBD()
	if err != nil {
		return Solid{}, err
	}
	var s Solid
	s.Thickness = thickness
	s.Elevation = elevation
	for i := range s.Corners {
		x, err := c.ReadRD()
		if err != nil {
			return s, err
		}
		y, err := c.ReadRD()
		if err != nil {
			return s, err
		}
		s.Corners[i] = Point2D{X: x, Y: y}
	}
	s.Extrusion, err = ReadBE(c)
	return s, err
}

// decodeText reads TEXT (§19.4.46) with the TV taken from strings on the
// R2007+ split layout.
func decodeText(c *BitCursor, v Version, strings *stringReader) (Text, error) {
	t := Text{WidthFactor: 1.0}
	flag, err := c.ReadRC()
	if err != nil {
		return t, err
	}
	if flag&0x01 == 0 {
		if t.Elevation, err = c.ReadRD(); err != nil {
			return t, err
		}
	}
	x, err := c.ReadRD()
	if err != nil {
		return t, err
	}
	y, err := c.ReadRD()
	if err != nil {
		return t, err
	}
	t.InsertionPoint = Point2D{X: x, Y: y}
	if flag&0x02 == 0 {
		ax, err := c.ReadDD(t.InsertionPoint.X)
		if err != nil {
			return t, err
		}
		ay, err := c.ReadDD(t.InsertionPoint.Y)
		if err != nil {
			return t, err
		}
		t.AlignmentPoint = &Point2D{X: ax, Y: ay}
	}
	if t.Extrusion, err = ReadBE(c); err != nil {
		return t, err
	}
	if t.Thickness, err = ReadBT(c); err != nil {
		return t, err
	}
	if flag&0x04 == 0 {
		if t.ObliqueAngle, err = c.ReadBD(); err != nil {
			return t, err
		}
	}
	if flag&0x08 == 0 {
		if t.RotationAngle, err = c.ReadBD(); err != nil {
			return t, err
		}
	}
	// Height: BD on the pre-R2007 inline layout; the measured R2018
	// records (DataFlags 0xFF) read a raw RD on the split layout.
	if strings == nil {
		if t.Height, err = c.ReadBD(); err != nil {
			return t, err
		}
	} else if t.Height, err = c.ReadRD(); err != nil {
		return t, err
	}
	if flag&0x10 == 0 {
		if t.WidthFactor, err = c.ReadBD(); err != nil {
			return t, err
		}
	}
	if t.Text, err = readTVSource(c, strings, v); err != nil {
		return t, err
	}
	if flag&0x20 == 0 {
		if t.Generation, err = c.ReadBS(); err != nil {
			return t, err
		}
	}
	if flag&0x40 == 0 {
		if t.HAlign, err = c.ReadBS(); err != nil {
			return t, err
		}
	}
	if flag&0x80 == 0 {
		if t.VAlign, err = c.ReadBS(); err != nil {
			return t, err
		}
	}
	return t, nil
}

func decodeInsert(c *BitCursor, v Version) (Insert, error) {
	var in Insert
	var err error
	if in.InsertionPoint, err = ReadBD3(c); err != nil {
		return in, err
	}
	flags, err := c.ReadBB()
	if err != nil {
		return in, err
	}
	switch flags {
	case 0b11:
		in.Scale = Point3D{X: 1, Y: 1, Z: 1}
	case 0b01:
		y, err := c.ReadDD(1.0)
		if err != nil {
			return in, err
		}
		z, err := c.ReadDD(1.0)
		if err != nil {
			return in, err
		}
		in.Scale = Point3D{X: 1, Y: y, Z: z}
	case 0b10:
		x, err := c.ReadRD()
		if err != nil {
			return in, err
		}
		in.Scale = Point3D{X: x, Y: x, Z: x}
	default:
		x, err := c.ReadRD()
		if err != nil {
			return in, err
		}
		y, err := c.ReadDD(x)
		if err != nil {
			return in, err
		}
		z, err := c.ReadDD(x)
		if err != nil {
			return in, err
		}
		in.Scale = Point3D{X: x, Y: y, Z: z}
	}
	if in.Rotation, err = c.ReadBD(); err != nil {
		return in, err
	}
	if in.Extrusion, err = ReadBD3(c); err != nil {
		return in, err
	}
	if in.HasAttribs, err = c.ReadB(); err != nil {
		return in, err
	}
	if in.HasAttribs && v.IsR2004Plus() {
		n, err := c.ReadBLU()
		if err != nil {
			return in, err
		}
		if n > maxInsertOwnedObjects {
			return in, newError(ErrSectionMap, "INSERT owned_object_count %d exceeds cap %d", n, maxInsertOwnedObjects)
		}
		in.OwnedObjectCount = n
	}
	return in, nil
}
