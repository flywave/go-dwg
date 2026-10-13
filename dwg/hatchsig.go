package dwg

// hatchSignature renders a HATCH record to a canonical, deterministic string
// so the differential test can compare every field bit-for-bit against the
// reference oracle's identical rendering (tools/dwgref/src/main.rs). Floats
// are emitted as IEEE-754 bit patterns.

import (
	"fmt"
	"math"
	"strings"
)

func hb(v float64) string { return fmt.Sprint(math.Float64bits(v)) }
func bb(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// hatchSignature builds the canonical HATCH signature.
func hatchSignature(h *Hatch) string {
	var b strings.Builder
	// Gradient.
	if h.Gradient == nil {
		b.WriteString("G0")
	} else {
		g := h.Gradient
		fmt.Fprintf(&b, "G1,%s,%s,%d,%s,%s,%d", hb(g.Angle), hb(g.Shift), g.IsSingleColor, hb(g.Tint), g.Name, len(g.Colors))
		for _, c := range g.Colors {
			fmt.Fprintf(&b, ",(%s,%d,%d,%d)", hb(c.UnknownDouble), c.UnknownShort, c.RGB, c.IgnoredColorByte)
		}
	}
	// Head + elevations.
	fmt.Fprintf(&b, " H%s,%s,%s,%s,%s,%s,%s", hb(h.Elevation), hb(h.Extrusion.X), hb(h.Extrusion.Y), hb(h.Extrusion.Z),
		h.PatternName, bb(h.SolidFill), bb(h.Associative))
	// Paths.
	fmt.Fprintf(&b, " N%d", len(h.Paths))
	for _, p := range h.Paths {
		fmt.Fprintf(&b, " [%d,%d,%s]", p.Flags, p.NumBoundaryHandles, hatchSegmentsSignature(p.Segments))
	}
	// Pattern style/type and pattern lines.
	fmt.Fprintf(&b, " T%d,%d,%s,%s,%s,L%d", h.PatternStyle, h.PatternType, hb(h.PatternAngle), hb(h.PatternScale), bb(h.PatternDouble), len(h.PatternLines))
	for _, pl := range h.PatternLines {
		fmt.Fprintf(&b, " {%s,%s,%s,%s,%s,%d", hb(pl.Angle), hb(pl.Origin.X), hb(pl.Origin.Y), hb(pl.Offset.X), hb(pl.Offset.Y), len(pl.Dashes))
		for _, d := range pl.Dashes {
			fmt.Fprintf(&b, ",%s", hb(d))
		}
		b.WriteString("}")
	}
	// Pixel size and seed points.
	fmt.Fprintf(&b, " P%s,S%d", hb(h.PixelSize), len(h.SeedPoints))
	for _, sp := range h.SeedPoints {
		fmt.Fprintf(&b, " (%s,%s)", hb(sp.X), hb(sp.Y))
	}
	return b.String()
}

func hatchSegmentsSignature(seg HatchPathSegments) string {
	switch s := seg.(type) {
	case *HatchPolylineSegments:
		var b strings.Builder
		fmt.Fprintf(&b, "P%s,%s,%d", bb(s.HasBulge), bb(s.IsClosed), len(s.Vertices))
		for _, v := range s.Vertices {
			if s.HasBulge && v.Bulge != nil {
				fmt.Fprintf(&b, ",(%s,%s,%s)", hb(v.Point.X), hb(v.Point.Y), hb(*v.Bulge))
			} else {
				fmt.Fprintf(&b, ",(%s,%s)", hb(v.Point.X), hb(v.Point.Y))
			}
		}
		return b.String()
	case *HatchEdgeSegments:
		var b strings.Builder
		fmt.Fprintf(&b, "E%d", len(s.Edges))
		for _, e := range s.Edges {
			fmt.Fprintf(&b, ",%s", hatchEdgeSignature(e))
		}
		return b.String()
	default:
		return "?"
	}
}

func hatchEdgeSignature(e HatchEdge) string {
	switch v := e.(type) {
	case *HatchEdgeLine:
		return fmt.Sprintf("L(%s,%s,%s,%s)", hb(v.Start.X), hb(v.Start.Y), hb(v.End.X), hb(v.End.Y))
	case *HatchEdgeArc:
		return fmt.Sprintf("A(%s,%s,%s,%s,%s,%s)", hb(v.Center.X), hb(v.Center.Y), hb(v.Radius),
			hb(v.StartAngle), hb(v.EndAngle), bb(v.CounterClockwise))
	case *HatchEdgeEllipse:
		return fmt.Sprintf("X(%s,%s,%s,%s,%s,%s,%s,%s)", hb(v.Center.X), hb(v.Center.Y), hb(v.Endpoint.X), hb(v.Endpoint.Y),
			hb(v.AxisRatio), hb(v.StartAngle), hb(v.EndAngle), bb(v.CounterClockwise))
	case *HatchEdgeSpline:
		var b strings.Builder
		fmt.Fprintf(&b, "S(%d,%s,%s,%d", v.Degree, bb(v.IsRational), bb(v.IsPeriodic), len(v.Knots))
		for _, k := range v.Knots {
			fmt.Fprintf(&b, ",%s", hb(k))
		}
		fmt.Fprintf(&b, ";%d", len(v.ControlPoints))
		for i, cp := range v.ControlPoints {
			fmt.Fprintf(&b, ",(%s,%s", hb(cp.X), hb(cp.Y))
			if v.IsRational {
				fmt.Fprintf(&b, ",%s", hb(v.Weights[i]))
			}
			b.WriteString(")")
		}
		fmt.Fprintf(&b, ";%d", len(v.FitPoints))
		for _, fp := range v.FitPoints {
			fmt.Fprintf(&b, ",(%s,%s)", hb(fp.X), hb(fp.Y))
		}
		fmt.Fprintf(&b, ";%s,%s,%s,%s)", hb(v.StartTangent.X), hb(v.StartTangent.Y), hb(v.EndTangent.X), hb(v.EndTangent.Y))
		return b.String()
	default:
		return "?"
	}
}
