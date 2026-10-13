package dwg

// Raster/underlay custom-class entities (IMAGE §19.4.35, WIPEOUT —
// measured identical to IMAGE, IMAGEDEF, UNDERLAY) and HELIX, ported from
// the reference implementation. Dispatched by DXF class name through the
// AcDb:Classes map.

// Helix is a decoded ACDBHELIX entity.
type Helix struct {
	AxisBasePoint Point3D
	StartPoint    Point3D
	AxisVector    Vec3D
	Radius        float64
	Turns         float64
	TurnHeight    float64
	RightHanded   bool
	ConstrainType int16 // 0 turn-height, 1 turns, 2 height
}

func decodeHelix(c *BitCursor) (Helix, error) {
	var h Helix
	var err error
	if h.AxisBasePoint, err = ReadBD3(c); err != nil {
		return h, err
	}
	if h.StartPoint, err = ReadBD3(c); err != nil {
		return h, err
	}
	if h.AxisVector, err = ReadBD3(c); err != nil {
		return h, err
	}
	if h.Radius, err = c.ReadBD(); err != nil {
		return h, err
	}
	if h.Turns, err = c.ReadBD(); err != nil {
		return h, err
	}
	if h.TurnHeight, err = c.ReadBD(); err != nil {
		return h, err
	}
	if h.RightHanded, err = c.ReadB(); err != nil {
		return h, err
	}
	if h.ConstrainType, err = c.ReadBS(); err != nil {
		return h, err
	}
	if h.ConstrainType < 0 || h.ConstrainType > 2 {
		return h, newError(ErrSectionMap, "HELIX constrain type %d not in 0..=2", h.ConstrainType)
	}
	return h, nil
}

// Image is a decoded IMAGE (and WIPEOUT) record; the clip boundary is a
// rectangle (type 1) or a polygon (type 2).
type Image struct {
	InsertionPoint     Point3D
	UVector            Vec3D
	VVector            Vec3D
	ImageSize          Point2D
	DisplayFlags       int16
	Clipping           bool
	Brightness         uint8
	Contrast           uint8
	Fade               uint8
	ClipMode           bool
	ClipBoundaryIsRect bool
	ClipRect           [2]Point2D
	ClipPolygon        []Point2D
}

const imageMaxClipVerts = 100_000

// decodeImage reads the IMAGE/WIPEOUT body.
func decodeImage(c *BitCursor, v Version) (Image, error) {
	var im Image
	var err error
	if _, err = c.ReadBL(); err != nil { // class version
		return im, err
	}
	if im.InsertionPoint, err = ReadBD3(c); err != nil {
		return im, err
	}
	if im.UVector, err = ReadBD3(c); err != nil {
		return im, err
	}
	if im.VVector, err = ReadBD3(c); err != nil {
		return im, err
	}
	x, err := c.ReadRD()
	if err != nil {
		return im, err
	}
	y, err := c.ReadRD()
	if err != nil {
		return im, err
	}
	im.ImageSize = Point2D{X: x, Y: y}
	if im.DisplayFlags, err = c.ReadBS(); err != nil {
		return im, err
	}
	if im.Clipping, err = c.ReadB(); err != nil {
		return im, err
	}
	if im.Brightness, err = c.ReadRC(); err != nil {
		return im, err
	}
	if im.Contrast, err = c.ReadRC(); err != nil {
		return im, err
	}
	if im.Fade, err = c.ReadRC(); err != nil {
		return im, err
	}
	if v.IsR2010Plus() {
		if im.ClipMode, err = c.ReadB(); err != nil {
			return im, err
		}
	}
	boundaryType, err := c.ReadBS()
	if err != nil {
		return im, err
	}
	numVerts := 2
	if boundaryType != 1 {
		n, err := c.ReadBL()
		if err != nil {
			return im, err
		}
		numVerts = int(n)
	}
	if boundaryType != 1 && boundaryType != 2 {
		return im, newError(ErrSectionMap, "IMAGE clip boundary type %d not in {1, 2}", boundaryType)
	}
	if numVerts < 0 || numVerts > imageMaxClipVerts || numVerts*128 > c.RemainingBits() {
		return im, newError(ErrSectionMap, "IMAGE clip verts %d out of range", numVerts)
	}
	readPt := func() (Point2D, error) {
		px, err := c.ReadRD()
		if err != nil {
			return Point2D{}, err
		}
		py, err := c.ReadRD()
		if err != nil {
			return Point2D{}, err
		}
		return Point2D{X: px, Y: py}, nil
	}
	if boundaryType == 1 {
		if im.ClipRect[0], err = readPt(); err != nil {
			return im, err
		}
		im.ClipRect[1], err = readPt()
		im.ClipBoundaryIsRect = true
		return im, err
	}
	for k := 0; k < numVerts; k++ {
		p, err := readPt()
		if err != nil {
			return im, err
		}
		im.ClipPolygon = append(im.ClipPolygon, p)
	}
	return im, nil
}

// ImageDef is a decoded IMAGEDEF object.
type ImageDef struct {
	ClassVersion    int32
	ImageSizePx     [2]float64
	FilePath        string
	IsLoaded        bool
	PixelSizeUnits  uint8
	PixelWidthSize  float64
	PixelHeightSize float64
}

const imageDefMaxPathUnits = 65_536

// decodeImageDefObject reads an IMAGEDEF straight from its raw payload,
// taking the file path from the R2007+ string stream when present.
func decodeImageDefObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (ImageDef, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return ImageDef{}, err
	}
	var d ImageDef
	if d.ClassVersion, err = o.data.ReadBL(); err != nil {
		return d, err
	}
	if d.ImageSizePx[0], err = o.data.ReadRD(); err != nil {
		return d, err
	}
	if d.ImageSizePx[1], err = o.data.ReadRD(); err != nil {
		return d, err
	}
	if d.FilePath, err = o.readTV(version); err != nil {
		return d, err
	}
	if d.IsLoaded, err = o.data.ReadB(); err != nil {
		return d, err
	}
	if d.PixelSizeUnits, err = o.data.ReadRC(); err != nil {
		return d, err
	}
	if d.PixelWidthSize, err = o.data.ReadRD(); err != nil {
		return d, err
	}
	if d.PixelHeightSize, err = o.data.ReadRD(); err != nil {
		return d, err
	}
	if err := o.finish("IMAGEDEF"); err != nil {
		return d, err
	}
	return d, nil
}

// UnderlayKind discriminates the three underlay classes.
type UnderlayKind int

const (
	UnderlayPDF UnderlayKind = iota
	UnderlayDWF
	UnderlayDGN
)

// Underlay is a decoded PDFUNDERLAY/DWFUNDERLAY/DGNUNDERLAY entity.
type Underlay struct {
	Kind             UnderlayKind
	Normal           Vec3D
	InsertionPoint   Point3D
	Rotation         float64
	Scale            Vec3D
	Flags            uint8
	Contrast         uint8
	Fade             uint8
	ClipPolygon      []Point2D
	DefinitionHandle uint64
	HasDefinition    bool
}

const underlayMaxClipVerts = 100_000

func decodeUnderlay(c *BitCursor, kind UnderlayKind, v Version) (Underlay, error) {
	var u Underlay
	u.Kind = kind
	var err error
	if u.Normal, err = ReadBD3(c); err != nil {
		return u, err
	}
	if u.InsertionPoint, err = ReadBD3(c); err != nil {
		return u, err
	}
	if u.Rotation, err = c.ReadBD(); err != nil {
		return u, err
	}
	if u.Scale, err = ReadBD3(c); err != nil {
		return u, err
	}
	if u.Flags, err = c.ReadRC(); err != nil {
		return u, err
	}
	if u.Contrast, err = c.ReadRC(); err != nil {
		return u, err
	}
	if u.Fade, err = c.ReadRC(); err != nil {
		return u, err
	}
	n, err := c.ReadBS()
	if err != nil {
		return u, err
	}
	if n < 0 || int(n) > underlayMaxClipVerts || int(n) > c.RemainingBits() {
		return u, newError(ErrSectionMap, "UNDERLAY clip verts %d out of range", n)
	}
	for i := int16(0); i < n; i++ {
		x, err := c.ReadBD()
		if err != nil {
			return u, err
		}
		y, err := c.ReadBD()
		if err != nil {
			return u, err
		}
		u.ClipPolygon = append(u.ClipPolygon, Point2D{X: x, Y: y})
	}
	if !v.IsR2007Plus() {
		h, err := c.ReadHandle()
		if err != nil {
			return u, err
		}
		if h.Value != 0 {
			u.DefinitionHandle = h.Value
			u.HasDefinition = true
		}
	}
	return u, nil
}
