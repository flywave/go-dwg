package dwg

// ACSH_* shape-history primitive objects — AcDbShSphere / AcDbShCylinder /
// AcDbShCone / AcDbShWedge. The inheritance chain is OdDbObject →
// OdDbEvalExpr → OdDbShHistoryNode → OdDbShPrimitive → Sh*, so the data
// stream after the object prefix reads:
//
//   OdDbEvalExpr::dwgInFields (TD_DynBlocks.tx:0x1de566):
//     BL group_code, BL, BL, BS eval_type, then a variant keyed by
//     OdDxfCode::_getType (13-entry jump table at 0x2a5be8: 1/2/11/12 → TV,
//     4/5 → BS, 6 → BL, 7/8 → BD, 9 → 3×BD, everything else cleared), and
//     one more BL when group_code == -1 (the -1 marks "real group code
//     follows at the end"). ODA itself bails before the variant unless the
//     file is R2004 release 69 and group_code != -1; the finish() boundary
//     check is what guards that case here.
//   OdDbShHistoryNodeImpl::dwgInFields (TD_3DSolidHistory.tx:0xc55b2):
//     BL, BL, 16×BD (OdGeMatrix3d filled row-major through operator[]),
//     CMC, BL — the node's soft-pointer handle rides in the handle stream
//     and costs no data-stream bits.
//   OdDbShPrimitive::dwgInFields: no own fields.
//   Per-primitive Impl tail: sphere BL, BL, BD; cylinder BL, BL, 4×BD;
//     cone reuses the cylinder list verbatim; wedge BL, BL, 3×BD.
//
// All four corpus records (AC1027) close at delta 0.

// EvalVariant is the typed value an eval expression carries next to its
// group code; Kind records which OdDxfCode bucket was actually read.
type EvalVariant struct {
	Kind   uint8
	Text   string
	Int16  int16
	Int32  int32
	Double float64
	Point  [3]float64
}

// ShPrimitive is one decoded ACSH_* shape-history primitive record.
type ShPrimitive struct {
	GroupCode   int32
	EvalFields  [2]int32
	EvalType    int16
	Variant     EvalVariant
	TailCode    int32 // read only when GroupCode == -1
	NodeFields  [3]int32
	Matrix      [16]float64
	Color       ObjectColor
	ShapeFields [2]int32
	Shape       []float64
}

// dxfTypeRun is one [lo, hi] → type-id run of the OdDxfCode::_getType maps.
type dxfTypeRun struct {
	lo, hi int32
	id     uint8
}

// The three lookup tables recovered from OdDxfCode::_getType
// (libTD_DbCore.so:0x7b04a6) and its .rodata maps m_TypeMap / m_XdTypeMap /
// m_RtTypeMap.
var odDxfTypeRuns = []dxfTypeRun{
	{-6, -5, 0}, {-4, -4, 2}, {-3, -3, 0}, {-2, -2, 15}, {-1, -1, 14},
	{0, 4, 2}, {5, 5, 12}, {6, 8, 1}, {9, 9, 2}, {10, 19, 9},
	{20, 49, 7}, {50, 59, 8}, {60, 79, 5}, {80, 89, 0}, {90, 99, 6},
	{100, 102, 2}, {103, 104, 0}, {105, 105, 12}, {106, 109, 0}, {110, 119, 9},
	{120, 149, 7}, {150, 159, 0}, {160, 169, 18}, {170, 179, 5}, {180, 209, 0},
	{210, 219, 9}, {220, 220, 7}, {221, 223, 9}, {224, 229, 0}, {230, 230, 7},
	{231, 233, 9}, {234, 269, 0}, {270, 279, 5}, {280, 289, 4}, {290, 299, 3},
	{300, 309, 2}, {310, 319, 10}, {320, 329, 13}, {330, 339, 14}, {340, 349, 15},
	{350, 359, 16}, {360, 369, 17}, {370, 389, 5}, {390, 399, 13}, {400, 409, 5},
	{410, 419, 2}, {420, 429, 6}, {430, 439, 2}, {440, 459, 6}, {460, 469, 7},
	{470, 479, 2},
}

var odDxfXdTypeRuns = []dxfTypeRun{
	{999, 1002, 2}, {1003, 1003, 11}, {1004, 1004, 10}, {1005, 1005, 12}, {1006, 1009, 0},
	{1010, 1013, 9}, {1014, 1019, 0}, {1020, 1023, 7}, {1024, 1029, 0}, {1030, 1033, 7},
	{1034, 1039, 0}, {1040, 1042, 7}, {1043, 1069, 0}, {1070, 1070, 5}, {1071, 1071, 6},
}

var odDxfRtTypeRuns = []dxfTypeRun{
	{5000, 5000, 0}, {5001, 5001, 7}, {5002, 5002, 9}, {5003, 5003, 5}, {5004, 5004, 8},
}

// odDxfGetType mirrors OdDxfCode::_getType: map the raw DXF group code to
// the type bucket the eval-variant switch dispatches on.
func odDxfGetType(gc int32) uint8 {
	lookup := func(runs []dxfTypeRun) uint8 {
		for _, r := range runs {
			if gc >= r.lo && gc <= r.hi {
				return r.id
			}
		}
		return 0
	}
	switch {
	case gc >= -6 && gc <= 0x1df:
		return lookup(odDxfTypeRuns)
	case gc > 0x3e6 && gc <= 0x42f:
		return lookup(odDxfXdTypeRuns)
	case gc > 0x1387 && gc <= 0x1388+4:
		return lookup(odDxfRtTypeRuns)
	default:
		return 0
	}
}

// readEvalVariant reads the variant payload for one eval type, mirroring the
// jump-table cases; kinds the switch does not cover store nothing.
func readEvalVariant(o *objectStream, typeCode int16, version Version) (EvalVariant, error) {
	var v EvalVariant
	v.Kind = odDxfGetType(int32(typeCode))
	var err error
	switch v.Kind {
	case 1, 2, 11, 12:
		v.Text, err = o.readTV(version)
		return v, err
	case 4, 5:
		v.Int16, err = o.data.ReadBS()
		return v, err
	case 6:
		v.Int32, err = o.data.ReadBL()
		return v, err
	case 7, 8:
		v.Double, err = o.data.ReadBD()
		return v, err
	case 9:
		for i := range v.Point {
			if v.Point[i], err = o.data.ReadBD(); err != nil {
				return v, err
			}
		}
		return v, nil
	default:
		return v, nil
	}
}

// decodeShPrimitiveObject decodes an ACSH_* record straight from its raw
// payload; shapeWords is the primitive Impl's trailing BD count.
func decodeShPrimitiveObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version, shapeWords int) (ShPrimitive, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return ShPrimitive{}, err
	}
	var p ShPrimitive
	if p.GroupCode, err = o.data.ReadBL(); err != nil {
		return p, err
	}
	if p.EvalFields[0], err = o.data.ReadBL(); err != nil {
		return p, err
	}
	if p.EvalFields[1], err = o.data.ReadBL(); err != nil {
		return p, err
	}
	if p.EvalType, err = o.data.ReadBS(); err != nil {
		return p, err
	}
	if p.Variant, err = readEvalVariant(o, p.EvalType, version); err != nil {
		return p, err
	}
	if p.GroupCode == -1 {
		if p.TailCode, err = o.data.ReadBL(); err != nil {
			return p, err
		}
	}
	if p.NodeFields[0], err = o.data.ReadBL(); err != nil {
		return p, err
	}
	if p.NodeFields[1], err = o.data.ReadBL(); err != nil {
		return p, err
	}
	for i := range p.Matrix {
		if p.Matrix[i], err = o.data.ReadBD(); err != nil {
			return p, err
		}
	}
	if p.Color, err = readObjectColor(o.data, o.strings, version); err != nil {
		return p, err
	}
	if p.NodeFields[2], err = o.data.ReadBL(); err != nil {
		return p, err
	}
	if p.ShapeFields[0], err = o.data.ReadBL(); err != nil {
		return p, err
	}
	if p.ShapeFields[1], err = o.data.ReadBL(); err != nil {
		return p, err
	}
	p.Shape = make([]float64, shapeWords)
	for i := range p.Shape {
		if p.Shape[i], err = o.data.ReadBD(); err != nil {
			return p, err
		}
	}
	if err := o.finish("SH_PRIMITIVE"); err != nil {
		return p, err
	}
	return p, nil
}
