package dxf

// DIMENSION — one entity, seven subtype families discriminated by group 70.
// The common block (AcDbDimension) carries the block name, definition/text
// points and text override; each subtype adds its own definition points.

// Dimension subtype codes (low bits of group 70).
const (
	DimTypeLinear    int64 = 0
	DimTypeAligned   int64 = 1
	DimTypeAngular   int64 = 2 // two-line angular
	DimTypeDiameter  int64 = 3
	DimTypeRadius    int64 = 4
	DimTypeAngular3P int64 = 5 // 3-point angular
	DimTypeOrdinate  int64 = 6
)

// Group 70 flag bits.
const (
	DimFlagBlockRef  int64 = 32  // references a block (geometry only)
	DimFlagOrdinateX int64 = 64  // ordinate measures X
	DimFlagUserText  int64 = 128 // user-placed text
)

// Dimension models all seven subtypes.
type Dimension struct {
	Common
	Block             string // 2 — anonymous geometry block
	Subtype           int64  // 70 low bits
	Flags             int64  // full group 70 value
	Text              string // 1 — "<>" substitutes the measurement
	DefPoint          Point3 // 10-30 — dimension line / vertex (WCS)
	TextMidpoint      Point3 // 11-31 — text midpoint (OCS)
	DefPoint1         Point3 // 13-33
	DefPoint2         Point3 // 14-34
	DefPoint3         Point3 // 15-25-35 (angular/diameter/radius)
	DefPoint4         Point3 // 16-26-36 (angular arc point)
	HasPoint1         bool
	HasPoint2         bool
	HasPoint3         bool
	HasPoint4         bool
	Angle             float64 // 50 — rotation (linear/aligned) or arc angle
	ObliqueAngle      float64 // 52
	LeaderLength      float64 // 40 — radius/diameter leader
	ActualText        float64 // 42 — measured value (R2000+)
	AttachmentPt      int64   // 71
	LineSpacing       int64   // 72
	LineSpacingFactor float64 // 41
}

func (*Dimension) EntityType() string      { return "DIMENSION" }
func (e *Dimension) EntityCommon() *Common { return &e.Common }

func (e *Dimension) subtypeMarker() string {
	switch e.Subtype {
	case DimTypeLinear, DimTypeAligned:
		return "AcDbRotatedDimension"
	case DimTypeAngular:
		return "AcDb2LineAngularDimension"
	case DimTypeAngular3P:
		return "AcDb3PointAngularDimension"
	case DimTypeDiameter:
		return "AcDbDiametricDimension"
	case DimTypeRadius:
		return "AcDbRadialDimension"
	case DimTypeOrdinate:
		return "AcDbOrdinateDimension"
	}
	return "AcDbRotatedDimension"
}

func readDimension(tags []Tag) (AnyEntityData, error) {
	st := splitCommon(tags)
	e := &Dimension{Common: st.c}
	// Track the innermost subclass marker: 11-31 ownership differs between
	// AcDbDimension (text midpoint) and the subtype block (angular arc
	// point 16-26-36), so the marker order decides where 15/16 points go.
	inSub := ""
	for _, t := range st.rest {
		switch t.Code {
		case 100:
			inSub = t.Value
		case 2:
			e.Block = t.Value
		case 70:
			e.Flags = t.Int()
			e.Subtype = e.Flags & 7
			if e.Subtype == 7 {
				e.Subtype = DimTypeOrdinate // legacy encoding for ordinate
			}
		case 1:
			e.Text = t.Value
		case 10:
			e.DefPoint.X = t.Float()
		case 20:
			e.DefPoint.Y = t.Float()
		case 30:
			e.DefPoint.Z = t.Float()
		case 11:
			if inSub == "AcDb3PointAngularDimension" || inSub == "AcDb2LineAngularDimension" {
				e.DefPoint4.X, e.HasPoint4 = t.Float(), true
			} else {
				e.TextMidpoint.X = t.Float()
			}
		case 21:
			if inSub == "AcDb3PointAngularDimension" || inSub == "AcDb2LineAngularDimension" {
				e.DefPoint4.Y = t.Float()
			} else {
				e.TextMidpoint.Y = t.Float()
			}
		case 31:
			if inSub == "AcDb3PointAngularDimension" || inSub == "AcDb2LineAngularDimension" {
				e.DefPoint4.Z = t.Float()
			} else {
				e.TextMidpoint.Z = t.Float()
			}
		case 13:
			e.DefPoint1.X, e.HasPoint1 = t.Float(), true
		case 23:
			e.DefPoint1.Y = t.Float()
		case 33:
			e.DefPoint1.Z = t.Float()
		case 14:
			e.DefPoint2.X, e.HasPoint2 = t.Float(), true
		case 24:
			e.DefPoint2.Y = t.Float()
		case 34:
			e.DefPoint2.Z = t.Float()
		case 15:
			e.DefPoint3.X, e.HasPoint3 = t.Float(), true
		case 25:
			e.DefPoint3.Y = t.Float()
		case 35:
			e.DefPoint3.Z = t.Float()
		case 16:
			e.DefPoint4.X, e.HasPoint4 = t.Float(), true
		case 26:
			e.DefPoint4.Y = t.Float()
		case 36:
			e.DefPoint4.Z = t.Float()
		case 40:
			e.LeaderLength = t.Float()
		case 41:
			e.LineSpacingFactor = t.Float()
		case 42:
			e.ActualText = t.Float()
		case 50:
			e.Angle = t.Float()
		case 52:
			e.ObliqueAngle = t.Float()
		case 71:
			e.AttachmentPt = t.Int()
		case 72:
			e.LineSpacing = t.Int()
		default:
			e.Extra = append(e.Extra, t)
		}
	}
	return e, nil
}

func writeDimension(w *TagWriter, data AnyEntityData, ver DxfVersion) error {
	e := data.(*Dimension)
	sub := ver.SupportsSubclassMarkers()
	w.writeCommon(&e.Common, sub, "AcDbDimension")
	if e.Block != "" {
		w.Str(2, e.Block)
	}
	w.Int(70, e.Flags)
	if e.Text != "" {
		w.Str(1, e.Text)
	}
	if e.LineSpacingFactor != 0 {
		w.Float(41, e.LineSpacingFactor)
	}
	if e.ActualText != 0 {
		w.Float(42, e.ActualText)
	}
	if e.AttachmentPt != 0 {
		w.Int(71, e.AttachmentPt)
	}
	if e.LineSpacing != 0 {
		w.Int(72, e.LineSpacing)
	}
	angular := e.Subtype == DimTypeAngular || e.Subtype == DimTypeAngular3P
	// 11-31 is the text midpoint except for angular types, where the
	// subtype block repurposes it as the arc definition point.
	if !angular {
		w.Point(11, e.TextMidpoint)
	}
	w.Point(10, e.DefPoint)
	if sub {
		w.Subclass(e.subtypeMarker())
	}
	if angular {
		w.Point(11, e.DefPoint4)
	}
	if e.HasPoint1 {
		w.Point(13, e.DefPoint1)
	}
	if e.HasPoint2 {
		w.Point(14, e.DefPoint2)
	}
	if e.HasPoint3 {
		w.Point(15, e.DefPoint3)
	}
	if e.HasPoint4 && !angular {
		w.Point(16, e.DefPoint4)
	}
	if e.LeaderLength != 0 {
		w.Float(40, e.LeaderLength)
	}
	if e.Angle != 0 {
		w.Float(50, e.Angle)
	}
	if e.ObliqueAngle != 0 {
		w.Float(52, e.ObliqueAngle)
	}
	w.Pairs(e.Extra)
	return nil
}

func init() {
	registerEntity("DIMENSION", &entityCodec{read: readDimension, write: writeDimension})
}
