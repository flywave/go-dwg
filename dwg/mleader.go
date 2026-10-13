package dwg

// MULTILEADER entity (§20.4.48 "MLEADER") — R2010+ custom class. Most of
// the record is an embedded MLeaderAnnotContext (§20.4.86) that carries
// the leader roots, leader lines, vertices and the text or block content;
// the shared style fields follow it. Ported from the reference
// implementation's mleader.rs, including its measured deviations from the
// spec: the embedded context carries none of the inherited
// ObjectContextData fields, and the R2007-and-earlier block plus the two
// undocumented fields (an MC and a B) after `B 293` are as measured.

// MLeaderCmc is one colour reference inside a multileader.
type MLeaderCmc struct {
	Index     int16
	RGB       uint32
	ColorByte uint8
}

// MLeaderLeaderLine is one leader line of a leader root.
type MLeaderLeaderLine struct {
	Points        []Point3D
	Index         int32
	LeaderType    int16
	Color         MLeaderCmc
	LineWeight    int32
	ArrowSize     float64
	OverrideFlags uint32
}

// MLeaderBreakPair is one invisible-leader segment pair.
type MLeaderBreakPair struct {
	Start Point3D
	End   Point3D
}

// MLeaderLeaderRoot is one leader root (a dogleg with its lines).
type MLeaderLeaderRoot struct {
	ContentValid        bool
	UnknownFlag         bool
	ConnectionPoint     Point3D
	Direction           Vec3D
	BreakPoints         []MLeaderBreakPair
	Index               int32
	LandingDistance     float64
	Lines               []MLeaderLeaderLine
	AttachmentDirection int16
}

// MLeaderText is the text content block.
type MLeaderText struct {
	Label                  string
	Normal                 Vec3D
	Location               Point3D
	Direction              Vec3D
	Rotation               float64
	BoundaryWidth          float64
	BoundaryHeight         float64
	LineSpacingFactor      float64
	LineSpacingStyle       int16
	Color                  MLeaderCmc
	Alignment              int16
	FlowDirection          int16
	BackgroundColor        MLeaderCmc
	BackgroundScale        float64
	BackgroundTransparency int32
	BackgroundFillEnabled  bool
	BackgroundMaskFillOn   bool
	ColumnType             int16
	TextHeightAutomatic    bool
	ColumnWidth            float64
	ColumnGutter           float64
	ColumnFlowReversed     bool
	ColumnSizes            []float64
	WordBreak              bool
	UnknownFlag            bool
}

// MLeaderBlock is the block content block.
type MLeaderBlock struct {
	Normal    Vec3D
	Location  Point3D
	Scale     Vec3D
	Rotation  float64
	Color     MLeaderCmc
	Transform [16]float64
}

// MLeaderContext is the embedded annotation context.
type MLeaderContext struct {
	LeaderRoots      []MLeaderLeaderRoot
	OverallScale     float64
	ContentBasePoint Point3D
	TextHeight       float64
	ArrowHeadSize    float64
	LandingGap       float64
	LeftAttachment   int16
	RightAttachment  int16
	TextAlignType    int16
	AttachmentType   int16
	HasText          bool
	Text             *MLeaderText
	Block            *MLeaderBlock
	BasePoint        Point3D
	BaseDirection    Vec3D
	BaseVertical     Vec3D
	NormalReversed   bool
	TopAttachment    int16
	BottomAttachment int16
}

// MLeader is a decoded MULTILEADER entity.
type MLeader struct {
	ClassVersion         int16
	Context              MLeaderContext
	OverrideFlags        uint32
	LeaderType           int16
	LeaderColor          MLeaderCmc
	LineWeight           int32
	LandingEnabled       bool
	DoglegEnabled        bool
	LandingDistance      float64
	ArrowHeadSize        float64
	ContentType          int16
	LeftAttachment       int16
	RightAttachment      int16
	TextAngleType        int16
	Unknown175           int16
	TextColor            MLeaderCmc
	TextFrameEnabled     bool
	BlockColor           MLeaderCmc
	BlockScale           Point3D
	BlockRotation        float64
	BlockAttachmentType  int16
	IsAnnotative         bool
	UndocumentedMC       int64
	UndocumentedFlag     bool
	AttachmentDirection  int16
	TopAttachment        int16
	BottomAttachment     int16
	LeaderExtendedToText bool
}

const (
	mleaderMaxLeaderPoints = 65_535
	mleaderMaxBreakPairs   = 65_535
	mleaderMaxLeaderLines  = 65_535
	mleaderMaxLeaderRoots  = 65_535
	mleaderMaxColumnSizes  = 65_535
)

func mleaderBoundsCheck(n int, what string, remaining int) error {
	if n > mleaderMaxLeaderPoints || n > remaining {
		return newError(ErrSectionMap, "MLEADER %s count %d exceeds cap or %d remaining bits", what, n, remaining)
	}
	return nil
}

// readMLeaderCmc reads one CMC; colour names come from the string stream.
func readMLeaderCmc(c *BitCursor, strings *stringReader) (MLeaderCmc, error) {
	var cm MLeaderCmc
	var err error
	if cm.Index, err = c.ReadBS(); err != nil {
		return cm, err
	}
	if cm.RGB, err = c.ReadBLU(); err != nil {
		return cm, err
	}
	if cm.ColorByte, err = c.ReadRC(); err != nil {
		return cm, err
	}
	if cm.ColorByte&1 != 0 {
		if _, err = strings.readTV(); err != nil {
			return cm, err
		}
	}
	if cm.ColorByte&2 != 0 {
		if _, err = strings.readTV(); err != nil {
			return cm, err
		}
	}
	return cm, nil
}

func readMLeaderLeaderLine(c *BitCursor, strings *stringReader) (MLeaderLeaderLine, error) {
	var l MLeaderLeaderLine
	var err error
	np, err := c.ReadBLU()
	if err != nil {
		return l, err
	}
	if err = mleaderBoundsCheck(int(np), "leader-line points", c.RemainingBits()); err != nil {
		return l, err
	}
	for i := uint32(0); i < np; i++ {
		p, err := ReadBD3(c)
		if err != nil {
			return l, err
		}
		l.Points = append(l.Points, p)
	}
	nb, err := c.ReadBLU()
	if err != nil {
		return l, err
	}
	if err = mleaderBoundsCheck(int(nb), "leader-line break info", c.RemainingBits()); err != nil {
		return l, err
	}
	for i := uint32(0); i < nb; i++ {
		if _, err = c.ReadBL(); err != nil { // segment index
			return l, err
		}
		pairs, err := c.ReadBLU()
		if err != nil {
			return l, err
		}
		if err = mleaderBoundsCheck(int(pairs), "leader-line break pairs", c.RemainingBits()); err != nil {
			return l, err
		}
		for j := uint32(0); j < pairs; j++ {
			if _, err = ReadBD3(c); err != nil {
				return l, err
			}
			if _, err = ReadBD3(c); err != nil {
				return l, err
			}
		}
	}
	if l.Index, err = c.ReadBL(); err != nil {
		return l, err
	}
	if l.LeaderType, err = c.ReadBS(); err != nil {
		return l, err
	}
	if l.Color, err = readMLeaderCmc(c, strings); err != nil {
		return l, err
	}
	if l.LineWeight, err = c.ReadBL(); err != nil {
		return l, err
	}
	if l.ArrowSize, err = c.ReadBD(); err != nil {
		return l, err
	}
	if l.OverrideFlags, err = c.ReadBLU(); err != nil {
		return l, err
	}
	return l, nil
}

func readMLeaderLeaderRoot(c *BitCursor, strings *stringReader) (MLeaderLeaderRoot, error) {
	var r MLeaderLeaderRoot
	var err error
	if r.ContentValid, err = c.ReadB(); err != nil {
		return r, err
	}
	if r.UnknownFlag, err = c.ReadB(); err != nil {
		return r, err
	}
	if r.ConnectionPoint, err = ReadBD3(c); err != nil {
		return r, err
	}
	if r.Direction, err = ReadBD3(c); err != nil {
		return r, err
	}
	np, err := c.ReadBLU()
	if err != nil {
		return r, err
	}
	if err = mleaderBoundsCheck(int(np), "leader-root break pairs", c.RemainingBits()); err != nil {
		return r, err
	}
	for i := uint32(0); i < np; i++ {
		start, err := ReadBD3(c)
		if err != nil {
			return r, err
		}
		end, err := ReadBD3(c)
		if err != nil {
			return r, err
		}
		r.BreakPoints = append(r.BreakPoints, MLeaderBreakPair{Start: start, End: end})
	}
	if r.Index, err = c.ReadBL(); err != nil {
		return r, err
	}
	if r.LandingDistance, err = c.ReadBD(); err != nil {
		return r, err
	}
	nl, err := c.ReadBLU()
	if err != nil {
		return r, err
	}
	if err = mleaderBoundsCheck(int(nl), "leader lines", c.RemainingBits()); err != nil {
		return r, err
	}
	for i := uint32(0); i < nl; i++ {
		line, err := readMLeaderLeaderLine(c, strings)
		if err != nil {
			return r, err
		}
		r.Lines = append(r.Lines, line)
	}
	r.AttachmentDirection, err = c.ReadBS()
	return r, err
}

func readMLeaderTextContent(c *BitCursor, strings *stringReader) (MLeaderText, error) {
	var t MLeaderText
	var err error
	if t.Label, err = strings.readTV(); err != nil {
		return t, err
	}
	if t.Normal, err = ReadBD3(c); err != nil {
		return t, err
	}
	if t.Location, err = ReadBD3(c); err != nil {
		return t, err
	}
	if t.Direction, err = ReadBD3(c); err != nil {
		return t, err
	}
	if t.Rotation, err = c.ReadBD(); err != nil {
		return t, err
	}
	if t.BoundaryWidth, err = c.ReadBD(); err != nil {
		return t, err
	}
	if t.BoundaryHeight, err = c.ReadBD(); err != nil {
		return t, err
	}
	if t.LineSpacingFactor, err = c.ReadBD(); err != nil {
		return t, err
	}
	if t.LineSpacingStyle, err = c.ReadBS(); err != nil {
		return t, err
	}
	if t.Color, err = readMLeaderCmc(c, strings); err != nil {
		return t, err
	}
	if t.Alignment, err = c.ReadBS(); err != nil {
		return t, err
	}
	if t.FlowDirection, err = c.ReadBS(); err != nil {
		return t, err
	}
	if t.BackgroundColor, err = readMLeaderCmc(c, strings); err != nil {
		return t, err
	}
	if t.BackgroundScale, err = c.ReadBD(); err != nil {
		return t, err
	}
	if t.BackgroundTransparency, err = c.ReadBL(); err != nil {
		return t, err
	}
	if t.BackgroundFillEnabled, err = c.ReadB(); err != nil {
		return t, err
	}
	if t.BackgroundMaskFillOn, err = c.ReadB(); err != nil {
		return t, err
	}
	if t.ColumnType, err = c.ReadBS(); err != nil {
		return t, err
	}
	if t.TextHeightAutomatic, err = c.ReadB(); err != nil {
		return t, err
	}
	if t.ColumnWidth, err = c.ReadBD(); err != nil {
		return t, err
	}
	if t.ColumnGutter, err = c.ReadBD(); err != nil {
		return t, err
	}
	if t.ColumnFlowReversed, err = c.ReadB(); err != nil {
		return t, err
	}
	ns, err := c.ReadBLU()
	if err != nil {
		return t, err
	}
	if err = mleaderBoundsCheck(int(ns), "column sizes", c.RemainingBits()); err != nil {
		return t, err
	}
	for i := uint32(0); i < ns; i++ {
		x, err := c.ReadBD()
		if err != nil {
			return t, err
		}
		t.ColumnSizes = append(t.ColumnSizes, x)
	}
	if t.WordBreak, err = c.ReadB(); err != nil {
		return t, err
	}
	t.UnknownFlag, err = c.ReadB()
	return t, err
}

func readMLeaderBlockContent(c *BitCursor, strings *stringReader) (MLeaderBlock, error) {
	var b MLeaderBlock
	var err error
	if b.Normal, err = ReadBD3(c); err != nil {
		return b, err
	}
	if b.Location, err = ReadBD3(c); err != nil {
		return b, err
	}
	if b.Scale, err = ReadBD3(c); err != nil {
		return b, err
	}
	if b.Rotation, err = c.ReadBD(); err != nil {
		return b, err
	}
	if b.Color, err = readMLeaderCmc(c, strings); err != nil {
		return b, err
	}
	for i := range b.Transform {
		if b.Transform[i], err = c.ReadBD(); err != nil {
			return b, err
		}
	}
	return b, nil
}

func readMLeaderContext(c *BitCursor, strings *stringReader) (MLeaderContext, error) {
	var ctx MLeaderContext
	var err error
	nr, err := c.ReadBLU()
	if err != nil {
		return ctx, err
	}
	if err = mleaderBoundsCheck(int(nr), "leader roots", c.RemainingBits()); err != nil {
		return ctx, err
	}
	for i := uint32(0); i < nr; i++ {
		root, err := readMLeaderLeaderRoot(c, strings)
		if err != nil {
			return ctx, err
		}
		ctx.LeaderRoots = append(ctx.LeaderRoots, root)
	}
	if ctx.OverallScale, err = c.ReadBD(); err != nil {
		return ctx, err
	}
	if ctx.ContentBasePoint, err = ReadBD3(c); err != nil {
		return ctx, err
	}
	if ctx.TextHeight, err = c.ReadBD(); err != nil {
		return ctx, err
	}
	if ctx.ArrowHeadSize, err = c.ReadBD(); err != nil {
		return ctx, err
	}
	if ctx.LandingGap, err = c.ReadBD(); err != nil {
		return ctx, err
	}
	if ctx.LeftAttachment, err = c.ReadBS(); err != nil {
		return ctx, err
	}
	if ctx.RightAttachment, err = c.ReadBS(); err != nil {
		return ctx, err
	}
	if ctx.TextAlignType, err = c.ReadBS(); err != nil {
		return ctx, err
	}
	if ctx.AttachmentType, err = c.ReadBS(); err != nil {
		return ctx, err
	}
	if ctx.HasText, err = c.ReadB(); err != nil {
		return ctx, err
	}
	if ctx.HasText {
		t, err := readMLeaderTextContent(c, strings)
		if err != nil {
			return ctx, err
		}
		ctx.Text = &t
	} else if hasBlock, err := c.ReadB(); err != nil {
		return ctx, err
	} else if hasBlock {
		b, err := readMLeaderBlockContent(c, strings)
		if err != nil {
			return ctx, err
		}
		ctx.Block = &b
	}
	if ctx.BasePoint, err = ReadBD3(c); err != nil {
		return ctx, err
	}
	if ctx.BaseDirection, err = ReadBD3(c); err != nil {
		return ctx, err
	}
	if ctx.BaseVertical, err = ReadBD3(c); err != nil {
		return ctx, err
	}
	if ctx.NormalReversed, err = c.ReadB(); err != nil {
		return ctx, err
	}
	if ctx.TopAttachment, err = c.ReadBS(); err != nil {
		return ctx, err
	}
	ctx.BottomAttachment, err = c.ReadBS()
	return ctx, err
}

// decodeMLeader reads the MULTILEADER body from a cursor already past the
// common entity preamble (R2010+ custom class; TVs come from strings).
func decodeMLeader(c *BitCursor, v Version, strings *stringReader) (MLeader, error) {
	var m MLeader
	var err error
	if m.ClassVersion, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.Context, err = readMLeaderContext(c, strings); err != nil {
		return m, err
	}
	if m.OverrideFlags, err = c.ReadBLU(); err != nil {
		return m, err
	}
	if m.LeaderType, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.LeaderColor, err = readMLeaderCmc(c, strings); err != nil {
		return m, err
	}
	if m.LineWeight, err = c.ReadBL(); err != nil {
		return m, err
	}
	if m.LandingEnabled, err = c.ReadB(); err != nil {
		return m, err
	}
	if m.DoglegEnabled, err = c.ReadB(); err != nil {
		return m, err
	}
	if m.LandingDistance, err = c.ReadBD(); err != nil {
		return m, err
	}
	if m.ArrowHeadSize, err = c.ReadBD(); err != nil {
		return m, err
	}
	if m.ContentType, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.LeftAttachment, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.RightAttachment, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.TextAngleType, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.Unknown175, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.TextColor, err = readMLeaderCmc(c, strings); err != nil {
		return m, err
	}
	if m.TextFrameEnabled, err = c.ReadB(); err != nil {
		return m, err
	}
	if m.BlockColor, err = readMLeaderCmc(c, strings); err != nil {
		return m, err
	}
	if m.BlockScale, err = ReadBD3(c); err != nil {
		return m, err
	}
	if m.BlockRotation, err = c.ReadBD(); err != nil {
		return m, err
	}
	if m.BlockAttachmentType, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.IsAnnotative, err = c.ReadB(); err != nil {
		return m, err
	}
	if m.UndocumentedMC, err = c.ReadMC(); err != nil {
		return m, err
	}
	if m.UndocumentedFlag, err = c.ReadB(); err != nil {
		return m, err
	}
	if m.AttachmentDirection, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.TopAttachment, err = c.ReadBS(); err != nil {
		return m, err
	}
	if m.BottomAttachment, err = c.ReadBS(); err != nil {
		return m, err
	}
	if v == R2013 || v == R2018 {
		if m.LeaderExtendedToText, err = c.ReadB(); err != nil {
			return m, err
		}
	}
	return m, nil
}
