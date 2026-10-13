package dwg

// PLOTSETTINGS — the "Page Setup" block: printer, paper size, margins,
// plot origin/window, print scale and shade-plot mode. The field list is
// prescribed once, as the block §20.4.84 LAYOUT opens with; a LAYOUT
// embeds exactly this block ahead of its own fields, so both are read by
// readPlotSettingsFields.
//
//	TV   page_setup_name             (1)
//	TV   printer_config_name         (2)
//	BS   plot_layout_flags           (70)
//	BD   margin_left/bottom/right/top (40..43)  millimetres
//	BD   paper_width / paper_height  (44,45)
//	TV   paper_size                  (4)
//	2BD  plot_origin                 (46,47)
//	BS   paper_units/plot_rotation/plot_type (72,73,74)
//	2BD  window_min                  (48,49)
//	2BD  window_max                  (140,141)
//	BD   real_world_units/drawing_units (142,143)
//	TV   current_style_sheet         (7)
//	BS   standard_scale_type         (75)
//	BD   scale_factor                (147)
//	2BD  paper_image_origin          (148,149)
//	BS   shade_plot_mode/resolution/custom_dpi (76,77,78)  -- R2004+
//
// R2007 is unsupported here for the same reason as the reference: this
// port cannot locate an R2007 object's string stream. R13/R14/R2000 are
// undetermined (the block inserts a T plot_view_name and drops the
// shade-plot triple).

// PlotPoint2D is a 2D point used for plot origin, window extents and
// image origin.
type PlotPoint2D struct {
	X float64
	Y float64
}

// PlotPoint3D is a 3D point used for insertion base, extents and UCS.
type PlotPoint3D struct {
	X float64
	Y float64
	Z float64
}

// AcadPlotSettings is the §20.4.84 plot-settings block.
type AcadPlotSettings struct {
	PageSetupName            string
	PrinterConfigName        string
	PlotLayoutFlags          int16
	MarginLeft               float64
	MarginBottom             float64
	MarginRight              float64
	MarginTop                float64
	PaperWidth               float64
	PaperHeight              float64
	PaperSize                string
	PlotOrigin               PlotPoint2D
	PaperUnits               int16
	PlotRotation             int16
	PlotType                 int16
	WindowMin                PlotPoint2D
	WindowMax                PlotPoint2D
	RealWorldUnits           float64
	DrawingUnits             float64
	CurrentStyleSheet        string
	StandardScaleType        int16
	ScaleFactor              float64
	PaperImageOrigin         PlotPoint2D
	ShadePlotMode            int16
	ShadePlotResolutionLevel int16
	ShadePlotCustomDPI       int16
}

// checkPlotSettingsVersion rejects release bands whose layout this port
// has not matched against real bytes.
func checkPlotSettingsVersion(version Version, what string) error {
	switch version {
	case R2004, R2010, R2013, R2018:
		return nil
	case R2007:
		return newError(ErrUnsupported,
			"%s on R2007: cannot locate an R2007 object's string stream", what)
	default:
		return newError(ErrUnsupported,
			"%s is only determined for R2004 and R2010+; got %s", what, version)
	}
}

func readBD2(c *BitCursor) (PlotPoint2D, error) {
	x, err := c.ReadBD()
	if err != nil {
		return PlotPoint2D{}, err
	}
	y, err := c.ReadBD()
	if err != nil {
		return PlotPoint2D{}, err
	}
	return PlotPoint2D{X: x, Y: y}, nil
}

func readRD2(c *BitCursor) (PlotPoint2D, error) {
	x, err := c.ReadRD()
	if err != nil {
		return PlotPoint2D{}, err
	}
	y, err := c.ReadRD()
	if err != nil {
		return PlotPoint2D{}, err
	}
	return PlotPoint2D{X: x, Y: y}, nil
}

func readBD3(c *BitCursor) (PlotPoint3D, error) {
	x, err := c.ReadBD()
	if err != nil {
		return PlotPoint3D{}, err
	}
	y, err := c.ReadBD()
	if err != nil {
		return PlotPoint3D{}, err
	}
	z, err := c.ReadBD()
	if err != nil {
		return PlotPoint3D{}, err
	}
	return PlotPoint3D{X: x, Y: y, Z: z}, nil
}

// readPlotSettingsFields reads the shared §20.4.84 block from an
// already-opened object stream, leaving the cursor on the first field
// that follows it.
func readPlotSettingsFields(o *objectStream, version Version) (AcadPlotSettings, error) {
	var s AcadPlotSettings
	var err error
	if s.PageSetupName, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.PrinterConfigName, err = o.readTV(version); err != nil {
		return s, err
	}
	c := o.data
	if s.PlotLayoutFlags, err = c.ReadBS(); err != nil {
		return s, err
	}
	if s.MarginLeft, err = c.ReadBD(); err != nil {
		return s, err
	}
	if s.MarginBottom, err = c.ReadBD(); err != nil {
		return s, err
	}
	if s.MarginRight, err = c.ReadBD(); err != nil {
		return s, err
	}
	if s.MarginTop, err = c.ReadBD(); err != nil {
		return s, err
	}
	if s.PaperWidth, err = c.ReadBD(); err != nil {
		return s, err
	}
	if s.PaperHeight, err = c.ReadBD(); err != nil {
		return s, err
	}
	if s.PaperSize, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.PlotOrigin, err = readBD2(c); err != nil {
		return s, err
	}
	if s.PaperUnits, err = c.ReadBS(); err != nil {
		return s, err
	}
	if s.PlotRotation, err = c.ReadBS(); err != nil {
		return s, err
	}
	if s.PlotType, err = c.ReadBS(); err != nil {
		return s, err
	}
	if s.WindowMin, err = readBD2(c); err != nil {
		return s, err
	}
	if s.WindowMax, err = readBD2(c); err != nil {
		return s, err
	}
	if s.RealWorldUnits, err = c.ReadBD(); err != nil {
		return s, err
	}
	if s.DrawingUnits, err = c.ReadBD(); err != nil {
		return s, err
	}
	if s.CurrentStyleSheet, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.StandardScaleType, err = c.ReadBS(); err != nil {
		return s, err
	}
	if s.ScaleFactor, err = c.ReadBD(); err != nil {
		return s, err
	}
	if s.PaperImageOrigin, err = readBD2(c); err != nil {
		return s, err
	}
	if s.ShadePlotMode, err = c.ReadBS(); err != nil {
		return s, err
	}
	if s.ShadePlotResolutionLevel, err = c.ReadBS(); err != nil {
		return s, err
	}
	if s.ShadePlotCustomDPI, err = c.ReadBS(); err != nil {
		return s, err
	}
	return s, nil
}

// decodePlotSettingsObject decodes a standalone PLOTSETTINGS object.
func decodePlotSettingsObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (AcadPlotSettings, error) {
	if err := checkPlotSettingsVersion(version, "PLOTSETTINGS"); err != nil {
		return AcadPlotSettings{}, err
	}
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return AcadPlotSettings{}, err
	}
	s, err := readPlotSettingsFields(o, version)
	if err != nil {
		return AcadPlotSettings{}, err
	}
	if err := o.finish("PLOTSETTINGS"); err != nil {
		return AcadPlotSettings{}, err
	}
	return s, nil
}
