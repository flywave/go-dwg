package dwg

// Entity decoders — spec §19.4.x. The caller must have already consumed the
// object header and the common entity preamble before invoking a decoder.

// Line is a fully-decoded LINE entity.
type Line struct {
	Start     Point3D
	End       Point3D
	Thickness float64
	Extrusion Vec3D
	Is2D      bool
}

// Circle is a fully-decoded CIRCLE entity.
type Circle struct {
	Center    Point3D
	Radius    float64
	Thickness float64
	Extrusion Vec3D
}

// Arc is a fully-decoded ARC entity.
type Arc struct {
	Center     Point3D
	Radius     float64
	Thickness  float64
	Extrusion  Vec3D
	StartAngle float64
	EndAngle   float64
}

// Point is a fully-decoded POINT entity.
type Point struct {
	Position   Point3D
	Thickness  float64
	Extrusion  Vec3D
	XAxisAngle float64
}

// decodeLineVersioned routes to the R13/R14 3BD form or the R2000+ form.
func decodeLineVersioned(c *BitCursor, version Version) (Line, error) {
	if version == R14 {
		return decodeLineR13R14(c)
	}
	return decodeLine(c)
}

func decodeLineR13R14(c *BitCursor) (Line, error) {
	start, err := ReadBD3(c)
	if err != nil {
		return Line{}, err
	}
	end, err := ReadBD3(c)
	if err != nil {
		return Line{}, err
	}
	thickness, err := ReadBT(c)
	if err != nil {
		return Line{}, err
	}
	extrusion, err := ReadBE(c)
	if err != nil {
		return Line{}, err
	}
	return Line{Start: start, End: end, Thickness: thickness, Extrusion: extrusion,
		Is2D: start.Z == 0 && end.Z == 0}, nil
}

func decodeLine(c *BitCursor) (Line, error) {
	zflag, err := c.ReadB()
	if err != nil {
		return Line{}, err
	}
	sx, err := c.ReadRD()
	if err != nil {
		return Line{}, err
	}
	ex, err := c.ReadDD(sx)
	if err != nil {
		return Line{}, err
	}
	sy, err := c.ReadRD()
	if err != nil {
		return Line{}, err
	}
	ey, err := c.ReadDD(sy)
	if err != nil {
		return Line{}, err
	}
	var sz, ez float64
	if !zflag {
		sz, err = c.ReadRD()
		if err != nil {
			return Line{}, err
		}
		ez, err = c.ReadDD(sz)
		if err != nil {
			return Line{}, err
		}
	}
	thickness, err := ReadBT(c)
	if err != nil {
		return Line{}, err
	}
	extrusion, err := ReadBE(c)
	if err != nil {
		return Line{}, err
	}
	return Line{
		Start:     Point3D{X: sx, Y: sy, Z: sz},
		End:       Point3D{X: ex, Y: ey, Z: ez},
		Thickness: thickness,
		Extrusion: extrusion,
		Is2D:      zflag,
	}, nil
}

func decodeCircle(c *BitCursor) (Circle, error) {
	cx, err := c.ReadBD()
	if err != nil {
		return Circle{}, err
	}
	cy, err := c.ReadBD()
	if err != nil {
		return Circle{}, err
	}
	cz, err := c.ReadBD()
	if err != nil {
		return Circle{}, err
	}
	radius, err := c.ReadBD()
	if err != nil {
		return Circle{}, err
	}
	thickness, err := ReadBT(c)
	if err != nil {
		return Circle{}, err
	}
	extrusion, err := ReadBE(c)
	if err != nil {
		return Circle{}, err
	}
	return Circle{Center: Point3D{X: cx, Y: cy, Z: cz}, Radius: radius, Thickness: thickness, Extrusion: extrusion}, nil
}

func decodeArc(c *BitCursor) (Arc, error) {
	cx, err := c.ReadBD()
	if err != nil {
		return Arc{}, err
	}
	cy, err := c.ReadBD()
	if err != nil {
		return Arc{}, err
	}
	cz, err := c.ReadBD()
	if err != nil {
		return Arc{}, err
	}
	radius, err := c.ReadBD()
	if err != nil {
		return Arc{}, err
	}
	thickness, err := ReadBT(c)
	if err != nil {
		return Arc{}, err
	}
	extrusion, err := ReadBE(c)
	if err != nil {
		return Arc{}, err
	}
	start, err := c.ReadBD()
	if err != nil {
		return Arc{}, err
	}
	end, err := c.ReadBD()
	if err != nil {
		return Arc{}, err
	}
	return Arc{Center: Point3D{X: cx, Y: cy, Z: cz}, Radius: radius, Thickness: thickness,
		Extrusion: extrusion, StartAngle: start, EndAngle: end}, nil
}

func decodePoint(c *BitCursor) (Point, error) {
	x, err := c.ReadBD()
	if err != nil {
		return Point{}, err
	}
	y, err := c.ReadBD()
	if err != nil {
		return Point{}, err
	}
	z, err := c.ReadBD()
	if err != nil {
		return Point{}, err
	}
	thickness, err := ReadBT(c)
	if err != nil {
		return Point{}, err
	}
	extrusion, err := ReadBE(c)
	if err != nil {
		return Point{}, err
	}
	angle, err := c.ReadBD()
	if err != nil {
		return Point{}, err
	}
	return Point{Position: Point3D{X: x, Y: y, Z: z}, Thickness: thickness, Extrusion: extrusion, XAxisAngle: angle}, nil
}

// DecodedEntity is the result of dispatching one RawObject.
type DecodedEntity struct {
	// Label is "LINE", "CIRCLE", "ARC", "POINT", "UNHANDLED", or "ERROR".
	Label    string
	TypeCode uint16
	// Value is *Line, *Circle, *Arc, or *Point for decoded entities.
	Value   any
	Message string
}

// entityDataEnd returns the bit offset at which an entity's data fields must
// end, when the record states it.
func entityDataEnd(raw *RawObject, version Version) (int, bool) {
	if version.IsR2007Plus() {
		return DataFieldEnd(raw.Raw, version)
	}
	if raw.ObjSizeBits != nil {
		return int(*raw.ObjSizeBits), true
	}
	return 0, false
}

// checkedInline runs an inline entity decoder and requires it to end exactly
// on the record's data-stream boundary. For R2007+ it also opens the record's
// string stream and hands it to the decoder.
func checkedInline(raw *RawObject, version Version, label string,
	decodeBody func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error)) (any, error) {
	c, err := BodyCursor(raw, version)
	if err != nil {
		return nil, err
	}
	var strings *stringReader
	if version.IsR2007Plus() {
		if s, ok := LocateStringStream(raw.Raw, version); ok {
			r, err := newStringReader(raw.Raw, s)
			if err != nil {
				return nil, err
			}
			strings = r
		} else {
			strings = emptyStringReader(raw.Raw)
		}
	}
	dataEnd, hasEnd := entityDataEnd(raw, version)
	common, err := ReadCommonEntityData(c, version)
	if err != nil {
		return nil, err
	}
	if !hasEnd && common.DataEndBits != nil {
		dataEnd = int(*common.DataEndBits)
		hasEnd = true
	}
	value, err := decodeBody(c, version, &common, strings)
	if err != nil {
		return nil, err
	}
	if hasEnd && c.PositionBits() != dataEnd {
		return nil, newError(ErrSectionMap, "%s data fields ended at bit %d, data stream ends at %d (delta %d)",
			label, c.PositionBits(), dataEnd, c.PositionBits()-dataEnd)
	}
	return value, nil
}

// fixedEntityLabel returns the decoder label for a fixed type code, or
// ("", false) when the code has no decoder in this port.
func fixedEntityLabel(typeCode uint16) (string, bool) {
	switch typeCode {
	case oLine:
		return "LINE", true
	case oPoint:
		return "POINT", true
	case oCircle:
		return "CIRCLE", true
	case oArc:
		return "ARC", true
	case oBlock:
		return "BLOCK", true
	case oEndBlk:
		return "ENDBLK", true
	case oSeqEnd:
		return "SEQEND", true
	case oSolid3d:
		return "3DSOLID", true
	case oRegion:
		return "REGION", true
	case oBody:
		return "BODY", true
	case oHatch:
		return "HATCH", true
	case oEllipse:
		return "ELLIPSE", true
	case oMText:
		return "MTEXT", true
	case oRay:
		return "RAY", true
	case oXLine:
		return "XLINE", true
	case oSolid:
		return "SOLID", true
	case oTrace:
		return "TRACE", true
	case oText:
		return "TEXT", true
	case oInsert:
		return "INSERT", true
	case oDimOrdinate:
		return "DIMENSION_ORDINATE", true
	case oDimLinear:
		return "DIMENSION_LINEAR", true
	case oDimAligned:
		return "DIMENSION_ALIGNED", true
	case oDimAng3Pt:
		return "DIMENSION_ANG_3PT", true
	case oDimAng2Ln:
		return "DIMENSION_ANG_2LN", true
	case oDimRadius:
		return "DIMENSION_RADIUS", true
	case oDimDiameter:
		return "DIMENSION_DIAMETER", true
	case oSpline:
		return "SPLINE", true
	case oLwPolyline:
		return "LWPOLYLINE", true
	case oViewport:
		return "VIEWPORT", true
	case oFace3D:
		return "3DFACE", true
	case oLeader:
		return "LEADER", true
	case oTolerance:
		return "TOLERANCE", true
	case oPolyline2d:
		return "POLYLINE_2D", true
	case oPolyline3d:
		return "POLYLINE_3D", true
	case oVertex2d:
		return "VERTEX_2D", true
	case oVertex3d:
		return "VERTEX_3D", true
	case oVertexMesh:
		return "VERTEX_MESH", true
	case oVertexPface:
		return "VERTEX_PFACE", true
	case oVertexPfaceFace:
		return "VERTEX_PFACE_FACE", true
	case oPolylinePface:
		return "POLYFACE_MESH", true
	case oPolylineMesh:
		return "POLYGON_MESH", true
	case oOle2Frame:
		return "OLE2FRAME", true
	case oAttrib:
		return "ATTRIB", true
	case oAttdef:
		return "ATTDEF", true
	case oMLine:
		return "MLINE", true
	}
	return "", false
}

// DecodeFromRaw decodes a RawObject to a typed DecodedEntity for the entity
// types this port covers (LINE, POINT, CIRCLE, ARC) and the non-entity
// objects DICTIONARY, *_CONTROL and ACDB_PLACEHOLDER. Other types return
// UNHANDLED.
func DecodeFromRaw(raw *RawObject, version Version) DecodedEntity {
	kind := raw.Kind
	if !kind.IsEntity() && !kind.IsTableEntry() {
		return dispatchObject(raw, version)
	}
	if kind.IsTableEntry() {
		c, err := BodyCursor(raw, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: "failed to position cursor: " + err.Error()}
		}
		return dispatchTableEntry(raw, c, kind, version)
	}
	label, ok := fixedEntityLabel(raw.TypeCode)
	if !ok {
		return DecodedEntity{Label: "UNHANDLED", TypeCode: raw.TypeCode}
	}
	value, err := checkedInline(raw, version, label, func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error) {
		switch raw.TypeCode {
		case oLine:
			l, err := decodeLineVersioned(c, v)
			return &l, err
		case oPoint:
			p, err := decodePoint(c)
			return &p, err
		case oCircle:
			ci, err := decodeCircle(c)
			return &ci, err
		case oBlock:
			b, err := decodeBlockField(c, v, strings)
			return &b, err
		case oEndBlk:
			return &EndBlk{}, nil
		case oSeqEnd:
			return &SeqEnd{}, nil
		case oSolid3d, oRegion, oBody:
			rec, err := decodeModelerRecord(c, v, common.BinaryChain)
			return &rec, err
		case oHatch:
			h, err := decodeHatch(c, v, strings)
			return &h, err
		case oEllipse:
			e, err := decodeEllipse(c)
			return &e, err
		case oMText:
			m, err := decodeMText(c, v, strings)
			return &m, err
		case oRay, oXLine, oSolid, oTrace, oText, oInsert:
			e, err := decodeSimpleEntity(raw.TypeCode, c, v, strings)
			return e, err
		case oViewport:
			e, err := decodeViewportEntity(c)
			return &e, err
		case oFace3D:
			e, err := decodeThreeDFace(c)
			return &e, err
		case oLeader:
			e, err := decodeLeader(c)
			return &e, err
		case oTolerance:
			e, err := decodeTolerance(c, v, strings)
			return &e, err
		case oPolyline2d:
			e, err := decodePolyline(c)
			return &e, err
		case oPolyline3d:
			e, err := decodePolyline3d(c, v)
			return &e, err
		case oVertex2d:
			e, err := decodeVertex(c, v)
			return &e, err
		case oVertex3d, oVertexMesh, oVertexPface:
			e, err := decodeVertexPoint(c)
			return &e, err
		case oVertexPfaceFace:
			e, err := decodeVertexPfaceFace(c)
			return &e, err
		case oPolylinePface:
			e, err := decodePolyfaceMesh(c, v)
			return &e, err
		case oPolylineMesh:
			e, err := decodePolygonMesh(c)
			return &e, err
		case oOle2Frame:
			e, err := decodeOle2Frame(c)
			return &e, err
		case oAttrib:
			e, err := decodeAttrib(c, v, strings)
			return &e, err
		case oAttdef:
			e, err := decodeAttdef(c, v, strings)
			return &e, err
		case oMLine:
			e, err := decodeMLine(c)
			return &e, err
		case oDimOrdinate, oDimLinear, oDimAligned, oDimAng3Pt, oDimAng2Ln, oDimRadius, oDimDiameter:
			d, err := decodeDimension(raw.TypeCode, c, v, strings)
			return &d, err
		case oSpline:
			s, err := decodeSpline(c, v)
			return &s, err
		case oLwPolyline:
			p, err := decodeLwPolyline(c, v)
			return &p, err
		default:
			a, err := decodeArc(c)
			return &a, err
		}
	})
	if err != nil {
		return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
	}
	return DecodedEntity{Label: label, TypeCode: raw.TypeCode, Value: value}
}

// dispatchTableEntry decodes a symbol-table entry — APPID and LAYER. Other
// kinds return UNHANDLED.
func dispatchTableEntry(raw *RawObject, c *BitCursor, kind ObjectType, version Version) DecodedEntity {
	var inlineEnd *uint32
	if !version.IsR2007Plus() {
		prefix, err := readCommonObjectPrefix(c, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: "common object data: " + err.Error()}
		}
		if prefix.objSizeBits != nil {
			inlineEnd = prefix.objSizeBits
		} else {
			inlineEnd = raw.ObjSizeBits
		}
	}
	bodyStart := c.PositionBits()
	var label string
	var value any
	var derr error
	switch kind.Code {
	case oAppId:
		label = "APPID"
		if version.IsR2007Plus() {
			var a AppID
			a, derr = decodeAppIDModern(raw.Raw, bodyStart, version)
			value = &a
		} else {
			var a AppID
			a, derr = decodeAppIDInline(c, version)
			value = &a
		}
	case oLayer:
		label = "LAYER"
		if version.IsR2007Plus() {
			var l LayerEntry
			l, derr = decodeLayerModern(raw.Raw, bodyStart, version)
			value = &l
		} else {
			var l LayerEntry
			l, derr = decodeLayerInline(c, version)
			value = &l
		}
	case oLtype:
		label = "LTYPE"
		if version.IsR2007Plus() {
			var l LtypeEntry
			l, derr = decodeLtypeModern(raw.Raw, bodyStart, version)
			value = &l
		} else {
			var l LtypeEntry
			l, derr = decodeLtypeInline(c, version)
			value = &l
		}
	case oStyle:
		label = "STYLE"
		if version.IsR2007Plus() {
			var s StyleEntry
			s, derr = decodeStyleModern(raw.Raw, bodyStart, version)
			value = &s
		} else {
			var s StyleEntry
			s, derr = decodeStyleInline(c, version)
			value = &s
		}
	case oVport:
		label = "VPORT"
		if version.IsR2007Plus() {
			var v VportEntry
			v, derr = decodeVportModern(raw.Raw, bodyStart, version)
			value = &v
		} else {
			var v VportEntry
			v, derr = decodeVportInline(c, version)
			value = &v
		}
	case oView:
		label = "VIEW"
		if version.IsR2007Plus() {
			var v ViewEntry
			v, derr = decodeViewModern(raw.Raw, bodyStart, version)
			value = &v
		} else {
			var v ViewEntry
			v, derr = decodeViewInline(c, version)
			value = &v
		}
	case oUcs:
		label = "UCS"
		if version.IsR2007Plus() {
			var u UcsEntry
			u, derr = decodeUcsModern(raw.Raw, bodyStart, version)
			value = &u
		} else {
			var u UcsEntry
			u, derr = decodeUcsInline(c, version)
			value = &u
		}
	case oBlockHeader:
		label = "BLOCK_HEADER"
		if version.IsR2007Plus() {
			var b BlockRecord
			b, derr = decodeBlockRecordModern(raw.Raw, bodyStart, version)
			value = &b
		} else {
			var b BlockRecord
			b, derr = decodeBlockRecordInline(c, version)
			value = &b
		}
	case oDimStyle:
		if version == R14 {
			return DecodedEntity{Label: "UNHANDLED", TypeCode: raw.TypeCode}
		}
		label = "DIMSTYLE"
		if version.IsR2007Plus() {
			var d DimStyleEntry
			d, derr = decodeDimStyleModern(raw.Raw, bodyStart, version)
			value = &d
		} else {
			var d DimStyleEntry
			d, derr = decodeDimStyleInline(c, version)
			value = &d
		}
	default:
		return DecodedEntity{Label: "UNHANDLED", TypeCode: raw.TypeCode}
	}
	if derr != nil {
		return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: derr.Error()}
	}
	if inlineEnd != nil && c.PositionBits() != int(*inlineEnd) {
		return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode,
			Message: kind.ShortLabel() + " data fields did not land on the data-stream boundary"}
	}
	return DecodedEntity{Label: label, TypeCode: raw.TypeCode, Value: value}
}

// dispatchObject decodes a non-entity object — DICTIONARY, the *_CONTROL
// owners, and ACDB_PLACEHOLDER. Types with no decoder here return UNHANDLED.
func dispatchObject(raw *RawObject, version Version) DecodedEntity {
	kind := raw.Kind
	hasDecoder := kind.Code == oDictionary || kind.Code == oAcDbPlaceholder ||
		kind.Code == oXRecord || kind.Code == oLayout || kind.Code == oMLineStyle || kind.IsControl()
	if !hasDecoder {
		return DecodedEntity{Label: "UNHANDLED", TypeCode: raw.TypeCode}
	}
	c, err := BodyCursor(raw, version)
	if err != nil {
		return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: "failed to position cursor: " + err.Error()}
	}
	bodyStart := c.PositionBits()
	var value any
	switch {
	case kind.Code == oDictionary:
		d, err := decodeDictionaryObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		value = &d
		return DecodedEntity{Label: "DICTIONARY", TypeCode: raw.TypeCode, Value: value}
	case kind.Code == oXRecord:
		x, err := decodeXRecordObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		value = &x
		return DecodedEntity{Label: "XRECORD", TypeCode: raw.TypeCode, Value: value}
	case kind.Code == oAcDbPlaceholder:
		p, err := decodePlaceholderObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		value = &p
		return DecodedEntity{Label: "PLACEHOLDER", TypeCode: raw.TypeCode, Value: value}
	case kind.Code == oLayout:
		l, err := decodeLayoutObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		value = &l
		return DecodedEntity{Label: "LAYOUT", TypeCode: raw.TypeCode, Value: value}
	case kind.Code == oMLineStyle:
		m, err := decodeMlineStyleObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		value = &m
		return DecodedEntity{Label: "MLINESTYLE", TypeCode: raw.TypeCode, Value: value}
	default:
		ct, err := decodeControlObject(raw.Raw, bodyStart, raw.ObjSizeBits, version, kind)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		value = &ct
		return DecodedEntity{Label: "CONTROL", TypeCode: raw.TypeCode, Value: value}
	}
}

// DecodeAllEntities walks every object via the handle map and dispatches each
// through DecodeFromRaw, returning the decoded list and a summary.
func (f *DwgFile) DecodeAllEntities() ([]DecodedEntity, DispatchSummary, bool, error) {
	objs, ok, err := f.AllObjects()
	if !ok || err != nil {
		return nil, DispatchSummary{}, ok, err
	}
	var classMap *ClassMap
	if cm, ok, err := f.ClassMap(); ok && err == nil {
		classMap = &cm
	}
	out := make([]DecodedEntity, 0, len(objs))
	var summary DispatchSummary
	for i := range objs {
		raw := &objs[i]
		var d DecodedEntity
		if classMap != nil && raw.Kind.IsCustom() {
			d = DecodeFromRawWithClassMap(raw, f.version, classMap)
		} else {
			d = DecodeFromRaw(raw, f.version)
		}
		summary.Record(d)
		out = append(out, d)
	}
	return out, summary, true, nil
}

// DispatchSummary tallies dispatch outcomes.
type DispatchSummary struct {
	Decoded   int
	Unhandled int
	Errored   int
}

// Record tallies one dispatch outcome.
func (s *DispatchSummary) Record(d DecodedEntity) {
	switch d.Label {
	case "UNHANDLED":
		s.Unhandled++
	case "ERROR":
		s.Errored++
	default:
		s.Decoded++
	}
}

// Total returns decoded + unhandled + errored.
func (s *DispatchSummary) Total() int { return s.Decoded + s.Unhandled + s.Errored }
