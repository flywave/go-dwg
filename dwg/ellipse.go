package dwg

// ELLIPSE entity (§19.4.17) — center, major-axis vector, a full BD3
// extrusion (rather than the BE most entities use) and the parametric
// start/end angles.

// Ellipse is a decoded ELLIPSE entity.
type Ellipse struct {
	Center     PlotPoint3D
	MajorAxis  PlotPoint3D
	Extrusion  PlotPoint3D
	AxisRatio  float64
	StartParam float64
	EndParam   float64
}

func decodeEllipse(c *BitCursor) (Ellipse, error) {
	var e Ellipse
	var err error
	if e.Center, err = readBD3(c); err != nil {
		return e, err
	}
	if e.MajorAxis, err = readBD3(c); err != nil {
		return e, err
	}
	if e.Extrusion, err = readBD3(c); err != nil {
		return e, err
	}
	if e.AxisRatio, err = c.ReadBD(); err != nil {
		return e, err
	}
	if e.StartParam, err = c.ReadBD(); err != nil {
		return e, err
	}
	if e.EndParam, err = c.ReadBD(); err != nil {
		return e, err
	}
	return e, nil
}
