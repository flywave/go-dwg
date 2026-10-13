package dwg

// SPLINE entity (§19.4.44 / §20.4.40) — NURBS curve. The record stores
// either a fit form (interpolated points) or a control form (degree +
// knots + control points + weights). On R2013+ the effective branch is
// derived from the spline flags and the knot parameter — both measured
// AC1032 records store Scenario=1 yet follow the fit branch when the
// knot parameter is not Custom and the fit-points flag is set.

// Spline is a decoded NURBS curve.
type Spline struct {
	Scenario  uint32
	Flag1     *uint32
	KnotParam *uint32
	Degree    int32
	Fit       *SplineFit
	Control   *SplineControl
}

// SplineFit is the fit-based branch.
type SplineFit struct {
	Tolerance    float64
	BeginTangent Vec3D
	EndTangent   Vec3D
	FitPoints    []Point3D
}

// SplineControl is the control-point-based branch.
type SplineControl struct {
	Rational         bool
	Closed           bool
	Periodic         bool
	KnotTolerance    float64
	ControlTolerance float64
	Knots            []float64
	ControlPoints    []Point3D
	HasWeights       bool
	Weights          []float64
}

const (
	splineMaxItems         = 1_000_000
	knotParamCustom        = 15
	splineFlagMethodFitPts = 1
)

// decodeSpline reads the SPLINE payload that follows the common entity
// header.
func decodeSpline(c *BitCursor, v Version) (Spline, error) {
	var s Spline
	scenario, err := c.ReadBL()
	if err != nil {
		return s, err
	}
	s.Scenario = uint32(scenario)
	r2013Plus := v == R2013 || v == R2018
	var flag1, knotParam uint32
	if r2013Plus {
		f, err := c.ReadBL()
		if err != nil {
			return s, err
		}
		flag1 = uint32(f)
		s.Flag1 = &flag1
		k, err := c.ReadBL()
		if err != nil {
			return s, err
		}
		knotParam = uint32(k)
		s.KnotParam = &knotParam
	}
	if s.Degree, err = c.ReadBL(); err != nil {
		return s, err
	}
	// §20.4.40: derive the effective scenario on R2013+.
	effective := int(s.Scenario)
	if r2013Plus {
		if knotParam == knotParamCustom || flag1&splineFlagMethodFitPts == 0 {
			effective = 1
		} else {
			effective = 2
		}
	}
	remaining := c.RemainingBits()
	var fit SplineFit
	var ctrl SplineControl
	numFit, numKnots, numControl := 0, 0, 0
	switch effective {
	case 2:
		if fit.Tolerance, err = c.ReadBD(); err != nil {
			return s, err
		}
		if fit.BeginTangent, err = ReadBD3(c); err != nil {
			return s, err
		}
		if fit.EndTangent, err = ReadBD3(c); err != nil {
			return s, err
		}
		var n int32
		if n, err = c.ReadBL(); err != nil {
			return s, err
		}
		numFit = int(n)
	case 1:
		if ctrl.Rational, err = c.ReadB(); err != nil {
			return s, err
		}
		if ctrl.Closed, err = c.ReadB(); err != nil {
			return s, err
		}
		if ctrl.Periodic, err = c.ReadB(); err != nil {
			return s, err
		}
		if ctrl.KnotTolerance, err = c.ReadBD(); err != nil {
			return s, err
		}
		if ctrl.ControlTolerance, err = c.ReadBD(); err != nil {
			return s, err
		}
		var n int32
		if n, err = c.ReadBL(); err != nil {
			return s, err
		}
		numKnots = int(n)
		if n, err = c.ReadBL(); err != nil {
			return s, err
		}
		numControl = int(n)
		if ctrl.HasWeights, err = c.ReadB(); err != nil {
			return s, err
		}
	default:
		return s, newError(ErrSectionMap, "SPLINE scenario %d not in {1, 2}", effective)
	}
	if numFit < 0 || numKnots < 0 || numControl < 0 ||
		numFit > splineMaxItems || numKnots > splineMaxItems || numControl > splineMaxItems ||
		numFit+numKnots+numControl > remaining {
		return s, newError(ErrSectionMap, "SPLINE has implausible counts (f=%d, k=%d, c=%d; %d bits left)", numFit, numKnots, numControl, remaining)
	}
	knots := make([]float64, 0, numKnots)
	for k := 0; k < numKnots; k++ {
		x, err := c.ReadBD()
		if err != nil {
			return s, err
		}
		knots = append(knots, x)
	}
	ctrl.Knots = knots
	ctrl.ControlPoints = make([]Point3D, 0, numControl)
	if effective == 1 {
		for k := 0; k < numControl; k++ {
			p, err := ReadBD3(c)
			if err != nil {
				return s, err
			}
			ctrl.ControlPoints = append(ctrl.ControlPoints, p)
			if ctrl.HasWeights {
				w, err := c.ReadBD()
				if err != nil {
					return s, err
				}
				ctrl.Weights = append(ctrl.Weights, w)
			}
		}
		s.Control = &ctrl
		return s, nil
	}
	fit.FitPoints = make([]Point3D, 0, numFit)
	for k := 0; k < numFit; k++ {
		p, err := ReadBD3(c)
		if err != nil {
			return s, err
		}
		fit.FitPoints = append(fit.FitPoints, p)
	}
	s.Fit = &fit
	return s, nil
}
