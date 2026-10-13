package dwg

// ACAD_MLEADERSTYLE object (custom class, measured) — multileader style.

// MLeaderStyle is a decoded ACAD_MLEADERSTYLE record.
type MLeaderStyle struct {
	Version                       int16
	ContentType                   int16
	DrawMLeaderOrderType          int16
	DrawLeaderOrderType           int16
	MaxLeaderSegmentPoints        int32
	FirstSegmentAngle             float64
	SecondSegmentAngle            float64
	LeaderLineType                int16
	LeaderLineColor               ObjectColor
	LeaderLineweight              int32
	EnableLanding                 bool
	LandingGap                    float64
	EnableDogleg                  bool
	DoglegLength                  float64
	Description                   string
	ArrowheadSize                 float64
	DefaultMTextContents          string
	TextLeftAttachment            int16
	TextRightAttachment           int16
	TextAngleType                 int16
	TextAlignmentType             int16
	TextColor                     ObjectColor
	TextHeight                    float64
	EnableFrameText               bool
	TextAlignAlwaysLeft           bool
	AlignSpace                    float64
	BlockContentColor             ObjectColor
	BlockContentScaleX            float64
	BlockContentScaleY            float64
	BlockContentScaleZ            float64
	EnableBlockContentScale       bool
	BlockContentRotation          float64
	EnableBlockContentRotation    bool
	BlockContentConnectionType    int16
	Scale                         float64
	OverwritePropertyValue        bool
	IsAnnotative                  bool
	BreakGapSize                  float64
	TextAttachmentDirection       int16
	BottomTextAttachmentDirection int16
	TopTextAttachmentDirection    int16
	ExtendedFlag                  bool
}

func decodeMLeaderStyleObject(payload []byte, bodyStart int, inlineEnd *uint32, version Version) (MLeaderStyle, error) {
	o, err := openObjectStream(payload, bodyStart, inlineEnd, version)
	if err != nil {
		return MLeaderStyle{}, err
	}
	hasVersion := version.IsR2010Plus()
	c := o.data
	var m MLeaderStyle
	rdBS := func(dst *int16) error { v, err := c.ReadBS(); *dst = v; return err }
	rdBD := func(dst *float64) error { v, err := c.ReadBD(); *dst = v; return err }
	rdB := func(dst *bool) error { v, err := c.ReadB(); *dst = v; return err }
	if hasVersion {
		if err := rdBS(&m.Version); err != nil {
			return MLeaderStyle{}, err
		}
	}
	if err := rdBS(&m.ContentType); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBS(&m.DrawMLeaderOrderType); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBS(&m.DrawLeaderOrderType); err != nil {
		return MLeaderStyle{}, err
	}
	bl, err := c.ReadBL()
	if err != nil {
		return MLeaderStyle{}, err
	}
	m.MaxLeaderSegmentPoints = bl
	if err := rdBD(&m.FirstSegmentAngle); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.SecondSegmentAngle); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBS(&m.LeaderLineType); err != nil {
		return MLeaderStyle{}, err
	}
	if m.LeaderLineColor, err = readObjectColor(c, o.strings, version); err != nil {
		return MLeaderStyle{}, err
	}
	bl, err = c.ReadBL()
	if err != nil {
		return MLeaderStyle{}, err
	}
	m.LeaderLineweight = bl
	if err := rdB(&m.EnableLanding); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.LandingGap); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdB(&m.EnableDogleg); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.DoglegLength); err != nil {
		return MLeaderStyle{}, err
	}
	if m.Description, err = o.readTV(version); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.ArrowheadSize); err != nil {
		return MLeaderStyle{}, err
	}
	if m.DefaultMTextContents, err = o.readTV(version); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBS(&m.TextLeftAttachment); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBS(&m.TextRightAttachment); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBS(&m.TextAngleType); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBS(&m.TextAlignmentType); err != nil {
		return MLeaderStyle{}, err
	}
	if m.TextColor, err = readObjectColor(c, o.strings, version); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.TextHeight); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdB(&m.EnableFrameText); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdB(&m.TextAlignAlwaysLeft); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.AlignSpace); err != nil {
		return MLeaderStyle{}, err
	}
	if m.BlockContentColor, err = readObjectColor(c, o.strings, version); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.BlockContentScaleX); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.BlockContentScaleY); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.BlockContentScaleZ); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdB(&m.EnableBlockContentScale); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.BlockContentRotation); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdB(&m.EnableBlockContentRotation); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBS(&m.BlockContentConnectionType); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.Scale); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdB(&m.OverwritePropertyValue); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdB(&m.IsAnnotative); err != nil {
		return MLeaderStyle{}, err
	}
	if err := rdBD(&m.BreakGapSize); err != nil {
		return MLeaderStyle{}, err
	}
	if hasVersion {
		if err := rdBS(&m.TextAttachmentDirection); err != nil {
			return MLeaderStyle{}, err
		}
		if err := rdBS(&m.BottomTextAttachmentDirection); err != nil {
			return MLeaderStyle{}, err
		}
		if err := rdBS(&m.TopTextAttachmentDirection); err != nil {
			return MLeaderStyle{}, err
		}
	}
	if version == R2013 || version == R2018 {
		if m.ExtendedFlag, err = c.ReadB(); err != nil {
			return MLeaderStyle{}, err
		}
	}
	if err := o.finish("MLEADERSTYLE"); err != nil {
		return MLeaderStyle{}, err
	}
	return m, nil
}
