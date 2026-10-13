package dwg

// HATCH entity (§19.4.75) — a filled hatch region: optional gradient block,
// elevation/extrusion, pattern name, a boundary-path tree and a pattern-line
// table. Field order and the per-array caps mirror the reference.

const (
	hatchCapGradientColors  = 16
	hatchCapPaths           = 10000
	hatchCapPathSegs        = 100000
	hatchCapBoundaryHandles = 1024
	hatchCapPatternLines    = 4096
	hatchCapLineDashes      = 64
	hatchCapSeedPoints      = 1024
)

const (
	hatchFlagPolyline uint32 = 0x02
	hatchFlagDerived  uint32 = 0x04
)

// Hatch is a decoded HATCH entity.
type Hatch struct {
	Gradient      *HatchGradientFill
	Elevation     float64
	Extrusion     PlotPoint3D
	PatternName   string
	SolidFill     bool
	Associative   bool
	Paths         []HatchPath
	PatternStyle  uint16
	PatternType   uint16
	PatternAngle  float64
	PatternScale  float64
	PatternDouble bool
	PatternLines  []PatternLine
	PixelSize     float64
	SeedPoints    []PlotPoint2D
}

// HatchGradientFill is the R2004+ gradient block of a HATCH.
type HatchGradientFill struct {
	Angle         float64
	Shift         float64
	IsSingleColor uint32
	Tint          float64
	Colors        []HatchGradientColor
	Name          string
}

// HatchGradientColor is one gradient stop.
type HatchGradientColor struct {
	UnknownDouble    float64
	UnknownShort     int16
	RGB              uint32
	IgnoredColorByte uint8
}

// HatchPath is one boundary path.
type HatchPath struct {
	Flags              uint32
	Segments           HatchPathSegments
	NumBoundaryHandles uint32
}

// HatchPathSegments is either a polyline or an edge list.
type HatchPathSegments interface{ hatchSegments() }

// HatchPolylineSegments is a polyline boundary path.
type HatchPolylineSegments struct {
	HasBulge bool
	IsClosed bool
	Vertices []HatchVertex
}

func (*HatchPolylineSegments) hatchSegments() {}

// HatchVertex is one polyline vertex with an optional bulge.
type HatchVertex struct {
	Point PlotPoint2D
	Bulge *float64
}

// HatchEdgeSegments is an edge-list boundary path.
type HatchEdgeSegments struct {
	Edges []HatchEdge
}

func (*HatchEdgeSegments) hatchSegments() {}

// HatchEdge is one edge of an edge-list path.
type HatchEdge interface{ hatchEdge() }

// HatchEdgeLine is a straight edge.
type HatchEdgeLine struct {
	Start PlotPoint2D
	End   PlotPoint2D
}

func (*HatchEdgeLine) hatchEdge() {}

// HatchEdgeArc is a circular arc edge.
type HatchEdgeArc struct {
	Center           PlotPoint2D
	Radius           float64
	StartAngle       float64
	EndAngle         float64
	CounterClockwise bool
}

func (*HatchEdgeArc) hatchEdge() {}

// HatchEdgeEllipse is an elliptical arc edge.
type HatchEdgeEllipse struct {
	Center           PlotPoint2D
	Endpoint         PlotPoint2D
	AxisRatio        float64
	StartAngle       float64
	EndAngle         float64
	CounterClockwise bool
}

func (*HatchEdgeEllipse) hatchEdge() {}

// HatchEdgeSpline is a spline edge.
type HatchEdgeSpline struct {
	Degree        uint32
	IsRational    bool
	IsPeriodic    bool
	Knots         []float64
	ControlPoints []PlotPoint2D
	Weights       []float64
	FitPoints     []PlotPoint2D
	StartTangent  PlotPoint2D
	EndTangent    PlotPoint2D
}

func (*HatchEdgeSpline) hatchEdge() {}

// PatternLine is one hatch pattern-line definition.
type PatternLine struct {
	Angle  float64
	Origin PlotPoint2D
	Offset PlotPoint2D
	Dashes []float64
}

// hatchBoundsCheck rejects an array count that exceeds its cap or the bits
// remaining in the record.
func hatchBoundsCheck(n, cap int, field string, c *BitCursor) error {
	if n > cap || n > c.RemainingBits() {
		return newError(ErrSectionMap, "HATCH %s count %d exceeds cap (%d) or remaining bits (%d)",
			field, n, cap, c.RemainingBits())
	}
	return nil
}

// decodeHatch decodes a HATCH body from its data cursor, taking its name
// strings from the R2007+ string stream when one is present.
func decodeHatch(c *BitCursor, version Version, strings *stringReader) (Hatch, error) {
	var h Hatch
	g, err := decodeHatchGradient(c, version, strings)
	if err != nil {
		return h, err
	}
	h.Gradient = g
	if h.Elevation, err = c.ReadBD(); err != nil {
		return h, err
	}
	if h.Extrusion, err = readBD3(c); err != nil {
		return h, err
	}
	if h.PatternName, err = readTVSource(c, strings, version); err != nil {
		return h, err
	}
	if h.SolidFill, err = c.ReadB(); err != nil {
		return h, err
	}
	if h.Associative, err = c.ReadB(); err != nil {
		return h, err
	}
	numPaths, err := c.ReadBLU()
	if err != nil {
		return h, err
	}
	if err := hatchBoundsCheck(int(numPaths), hatchCapPaths, "num_paths", c); err != nil {
		return h, err
	}
	h.Paths = make([]HatchPath, 0, numPaths)
	anyDerived := false
	for i := uint32(0); i < numPaths; i++ {
		p, err := decodeHatchPath(c, version)
		if err != nil {
			return h, err
		}
		anyDerived = anyDerived || p.Flags&hatchFlagDerived != 0
		h.Paths = append(h.Paths, p)
	}
	if h.PatternStyle, err = c.ReadBSU(); err != nil {
		return h, err
	}
	if h.PatternType, err = c.ReadBSU(); err != nil {
		return h, err
	}
	if !h.SolidFill {
		if h.PatternAngle, err = c.ReadBD(); err != nil {
			return h, err
		}
		if h.PatternScale, err = c.ReadBD(); err != nil {
			return h, err
		}
		if h.PatternDouble, err = c.ReadB(); err != nil {
			return h, err
		}
		numLines, err := c.ReadBSU()
		if err != nil {
			return h, err
		}
		if err := hatchBoundsCheck(int(numLines), hatchCapPatternLines, "num_pattern_lines", c); err != nil {
			return h, err
		}
		h.PatternLines = make([]PatternLine, 0, numLines)
		for i := uint16(0); i < numLines; i++ {
			pl, err := decodeHatchPatternLine(c)
			if err != nil {
				return h, err
			}
			h.PatternLines = append(h.PatternLines, pl)
		}
	}
	if anyDerived {
		if h.PixelSize, err = c.ReadBD(); err != nil {
			return h, err
		}
	}
	numSeed, err := c.ReadBLU()
	if err != nil {
		return h, err
	}
	if err := hatchBoundsCheck(int(numSeed), hatchCapSeedPoints, "num_seed_points", c); err != nil {
		return h, err
	}
	h.SeedPoints = make([]PlotPoint2D, 0, numSeed)
	for i := uint32(0); i < numSeed; i++ {
		p, err := readRD2(c)
		if err != nil {
			return h, err
		}
		h.SeedPoints = append(h.SeedPoints, p)
	}
	return h, nil
}

func decodeHatchGradient(c *BitCursor, version Version, strings *stringReader) (*HatchGradientFill, error) {
	if !version.IsR2004Plus() {
		return nil, nil
	}
	isGradient, err := c.ReadBLU()
	if err != nil {
		return nil, err
	}
	if _, err := c.ReadBL(); err != nil { // reserved
		return nil, err
	}
	var g HatchGradientFill
	if g.Angle, err = c.ReadBD(); err != nil {
		return nil, err
	}
	if g.Shift, err = c.ReadBD(); err != nil {
		return nil, err
	}
	if g.IsSingleColor, err = c.ReadBLU(); err != nil {
		return nil, err
	}
	if g.Tint, err = c.ReadBD(); err != nil {
		return nil, err
	}
	numColors, err := c.ReadBLU()
	if err != nil {
		return nil, err
	}
	if err := hatchBoundsCheck(int(numColors), hatchCapGradientColors, "num_gradient_colors", c); err != nil {
		return nil, err
	}
	g.Colors = make([]HatchGradientColor, 0, numColors)
	for i := uint32(0); i < numColors; i++ {
		var gc HatchGradientColor
		if gc.UnknownDouble, err = c.ReadBD(); err != nil {
			return nil, err
		}
		if gc.UnknownShort, err = c.ReadBS(); err != nil {
			return nil, err
		}
		if gc.RGB, err = c.ReadBLU(); err != nil {
			return nil, err
		}
		if gc.IgnoredColorByte, err = c.ReadRC(); err != nil {
			return nil, err
		}
		g.Colors = append(g.Colors, gc)
	}
	if g.Name, err = readTVSource(c, strings, version); err != nil {
		return nil, err
	}
	if isGradient == 0 {
		return nil, nil
	}
	return &g, nil
}

func decodeHatchPath(c *BitCursor, version Version) (HatchPath, error) {
	var p HatchPath
	flags, err := c.ReadBLU()
	if err != nil {
		return p, err
	}
	p.Flags = flags
	if flags&hatchFlagPolyline != 0 {
		seg, err := decodeHatchPolylinePath(c)
		if err != nil {
			return p, err
		}
		p.Segments = seg
	} else {
		seg, err := decodeHatchEdgePath(c, version)
		if err != nil {
			return p, err
		}
		p.Segments = seg
	}
	if p.NumBoundaryHandles, err = c.ReadBLU(); err != nil {
		return p, err
	}
	if err := hatchBoundsCheck(int(p.NumBoundaryHandles), hatchCapBoundaryHandles, "num_boundary_handles", c); err != nil {
		return p, err
	}
	return p, nil
}

func decodeHatchPolylinePath(c *BitCursor) (*HatchPolylineSegments, error) {
	var seg HatchPolylineSegments
	var err error
	if seg.HasBulge, err = c.ReadB(); err != nil {
		return nil, err
	}
	if seg.IsClosed, err = c.ReadB(); err != nil {
		return nil, err
	}
	num, err := c.ReadBLU()
	if err != nil {
		return nil, err
	}
	if err := hatchBoundsCheck(int(num), hatchCapPathSegs, "num_path_segs (polyline)", c); err != nil {
		return nil, err
	}
	seg.Vertices = make([]HatchVertex, 0, num)
	for i := uint32(0); i < num; i++ {
		pt, err := readRD2(c)
		if err != nil {
			return nil, err
		}
		v := HatchVertex{Point: pt}
		if seg.HasBulge {
			b, err := c.ReadBD()
			if err != nil {
				return nil, err
			}
			v.Bulge = &b
		}
		seg.Vertices = append(seg.Vertices, v)
	}
	return &seg, nil
}

func decodeHatchEdgePath(c *BitCursor, version Version) (*HatchEdgeSegments, error) {
	num, err := c.ReadBLU()
	if err != nil {
		return nil, err
	}
	if err := hatchBoundsCheck(int(num), hatchCapPathSegs, "num_path_segs (edges)", c); err != nil {
		return nil, err
	}
	seg := &HatchEdgeSegments{Edges: make([]HatchEdge, 0, num)}
	for i := uint32(0); i < num; i++ {
		e, err := decodeHatchEdge(c, version)
		if err != nil {
			return nil, err
		}
		seg.Edges = append(seg.Edges, e)
	}
	return seg, nil
}

func decodeHatchEdge(c *BitCursor, version Version) (HatchEdge, error) {
	segType, err := c.ReadRC()
	if err != nil {
		return nil, err
	}
	switch segType {
	case 1:
		var e HatchEdgeLine
		if e.Start, err = readRD2(c); err != nil {
			return nil, err
		}
		if e.End, err = readRD2(c); err != nil {
			return nil, err
		}
		return &e, nil
	case 2:
		var e HatchEdgeArc
		if e.Center, err = readRD2(c); err != nil {
			return nil, err
		}
		if e.Radius, err = c.ReadBD(); err != nil {
			return nil, err
		}
		if e.StartAngle, err = c.ReadBD(); err != nil {
			return nil, err
		}
		if e.EndAngle, err = c.ReadBD(); err != nil {
			return nil, err
		}
		if e.CounterClockwise, err = c.ReadB(); err != nil {
			return nil, err
		}
		return &e, nil
	case 3:
		var e HatchEdgeEllipse
		if e.Center, err = readRD2(c); err != nil {
			return nil, err
		}
		if e.Endpoint, err = readRD2(c); err != nil {
			return nil, err
		}
		if e.AxisRatio, err = c.ReadBD(); err != nil {
			return nil, err
		}
		if e.StartAngle, err = c.ReadBD(); err != nil {
			return nil, err
		}
		if e.EndAngle, err = c.ReadBD(); err != nil {
			return nil, err
		}
		if e.CounterClockwise, err = c.ReadB(); err != nil {
			return nil, err
		}
		return &e, nil
	case 4:
		return decodeHatchSplineEdge(c, version)
	default:
		return nil, newError(ErrSectionMap, "HATCH edge seg_type %d not in {1 line, 2 arc, 3 ellipse, 4 spline}", segType)
	}
}

func decodeHatchSplineEdge(c *BitCursor, version Version) (HatchEdge, error) {
	var e HatchEdgeSpline
	var err error
	if e.Degree, err = c.ReadBLU(); err != nil {
		return nil, err
	}
	if e.IsRational, err = c.ReadB(); err != nil {
		return nil, err
	}
	if e.IsPeriodic, err = c.ReadB(); err != nil {
		return nil, err
	}
	numKnots, err := c.ReadBLU()
	if err != nil {
		return nil, err
	}
	if err := hatchBoundsCheck(int(numKnots), hatchCapPathSegs, "spline knots", c); err != nil {
		return nil, err
	}
	numControl, err := c.ReadBLU()
	if err != nil {
		return nil, err
	}
	if err := hatchBoundsCheck(int(numControl), hatchCapPathSegs, "spline control_points", c); err != nil {
		return nil, err
	}
	e.Knots = make([]float64, 0, numKnots)
	for i := uint32(0); i < numKnots; i++ {
		k, err := c.ReadBD()
		if err != nil {
			return nil, err
		}
		e.Knots = append(e.Knots, k)
	}
	e.ControlPoints = make([]PlotPoint2D, 0, numControl)
	for i := uint32(0); i < numControl; i++ {
		cp, err := readRD2(c)
		if err != nil {
			return nil, err
		}
		e.ControlPoints = append(e.ControlPoints, cp)
		if e.IsRational {
			w, err := c.ReadBD()
			if err != nil {
				return nil, err
			}
			e.Weights = append(e.Weights, w)
		}
	}
	// Fit-point block is R2010+.
	if version.IsR2010Plus() {
		numFit, err := c.ReadBLU()
		if err != nil {
			return nil, err
		}
		if err := hatchBoundsCheck(int(numFit), hatchCapPathSegs, "spline fit_points", c); err != nil {
			return nil, err
		}
		e.FitPoints = make([]PlotPoint2D, 0, numFit)
		for i := uint32(0); i < numFit; i++ {
			fp, err := readRD2(c)
			if err != nil {
				return nil, err
			}
			e.FitPoints = append(e.FitPoints, fp)
		}
		if e.StartTangent, err = readRD2(c); err != nil {
			return nil, err
		}
		if e.EndTangent, err = readRD2(c); err != nil {
			return nil, err
		}
	}
	return &e, nil
}

func decodeHatchPatternLine(c *BitCursor) (PatternLine, error) {
	var pl PatternLine
	var err error
	if pl.Angle, err = c.ReadBD(); err != nil {
		return pl, err
	}
	if pl.Origin, err = readBD2(c); err != nil {
		return pl, err
	}
	if pl.Offset, err = readBD2(c); err != nil {
		return pl, err
	}
	numDashes, err := c.ReadBSU()
	if err != nil {
		return pl, err
	}
	if err := hatchBoundsCheck(int(numDashes), hatchCapLineDashes, "num_line_dashes", c); err != nil {
		return pl, err
	}
	pl.Dashes = make([]float64, 0, numDashes)
	for i := uint16(0); i < numDashes; i++ {
		d, err := c.ReadBD()
		if err != nil {
			return pl, err
		}
		pl.Dashes = append(pl.Dashes, d)
	}
	return pl, nil
}
