package dwg

// LAYOUT (fixed type code 0x52, §20.4.84) — a paper-space or model-space
// tab: its embedded plot-settings block, limits, extents and UCS.
//
//	RC   ds_block[0]           -- R2013+, only when the object's
//	RC   ds_block[1]              `has AcDs binary data` bit is set
//	<plot-settings block — see plot_settings.go>
//	TV   layout_name           (1)
//	BL   tab_order             (71)
//	BS   flags                 (70)
//	3BD  ucs_origin            (13)
//	2RD  limmin                (10)
//	2RD  limmax                (11)
//	3BD  insertion_point       (12)
//	3BD  ucs_x_axis            (16)
//	3BD  ucs_y_axis            (17)
//	BD   elevation             (146)
//	BS   ucs_ortho_view_type   (76)
//	3BD  extent_min            (14)
//	3BD  extent_max            (15)
//	BL   viewport_count        -- R2004+ (spec prints RL; measured BL)
//
// The handle references §20.4.84 lists after viewport_count (plot view,
// visual style, paperspace block record, last active viewport, base and
// named UCS, and one per viewport) live in the object's handle stream,
// past the data-stream boundary this decoder checks.

// AcadLayout is a decoded LAYOUT record.
type AcadLayout struct {
	// DsBlock is the R2013+ data-store block: the two RC bytes a LAYOUT
	// writes ahead of its plot-settings block when the object's
	// `has AcDs binary data` bit is set. Nil otherwise.
	DsBlock          []byte
	PlotSettings     AcadPlotSettings
	LayoutName       string
	TabOrder         int32
	Flags            int16
	UcsOrigin        PlotPoint3D
	LimMin           PlotPoint2D
	LimMax           PlotPoint2D
	InsertionPt      PlotPoint3D
	UcsXAxis         PlotPoint3D
	UcsYAxis         PlotPoint3D
	Elevation        float64
	UcsOrthoViewType int16
	ExtentMin        PlotPoint3D
	ExtentMax        PlotPoint3D
	ViewportCount    int32
}

// decodeLayoutObject decodes a LAYOUT record straight from its raw
// object payload.
func decodeLayoutObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (AcadLayout, error) {
	if err := checkPlotSettingsVersion(version, "LAYOUT"); err != nil {
		return AcadLayout{}, err
	}
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return AcadLayout{}, err
	}
	var l AcadLayout
	if o.hasDSBinaryData {
		b0, err := o.data.ReadRC()
		if err != nil {
			return l, err
		}
		b1, err := o.data.ReadRC()
		if err != nil {
			return l, err
		}
		l.DsBlock = []byte{b0, b1}
	}
	if l.PlotSettings, err = readPlotSettingsFields(o, version); err != nil {
		return l, err
	}
	if l.LayoutName, err = o.readTV(version); err != nil {
		return l, err
	}
	c := o.data
	if l.TabOrder, err = c.ReadBL(); err != nil {
		return l, err
	}
	if l.Flags, err = c.ReadBS(); err != nil {
		return l, err
	}
	if l.UcsOrigin, err = readBD3(c); err != nil {
		return l, err
	}
	if l.LimMin, err = readRD2(c); err != nil {
		return l, err
	}
	if l.LimMax, err = readRD2(c); err != nil {
		return l, err
	}
	if l.InsertionPt, err = readBD3(c); err != nil {
		return l, err
	}
	if l.UcsXAxis, err = readBD3(c); err != nil {
		return l, err
	}
	if l.UcsYAxis, err = readBD3(c); err != nil {
		return l, err
	}
	if l.Elevation, err = c.ReadBD(); err != nil {
		return l, err
	}
	if l.UcsOrthoViewType, err = c.ReadBS(); err != nil {
		return l, err
	}
	if l.ExtentMin, err = readBD3(c); err != nil {
		return l, err
	}
	if l.ExtentMax, err = readBD3(c); err != nil {
		return l, err
	}
	if l.ViewportCount, err = c.ReadBL(); err != nil {
		return l, err
	}
	if err := o.finish("LAYOUT"); err != nil {
		return l, err
	}
	return l, nil
}
