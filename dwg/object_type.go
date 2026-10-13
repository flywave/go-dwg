package dwg

import "fmt"

// ObjectType is a DWG object type code (spec §20.3). Codes below 0x1F2
// are fixed; 0x1F2/0x1F3 are proxies; from 500 upward the codes are
// dynamic (class-indexed into AcDb:Classes).
type ObjectType struct {
	Code uint16
}

// fixed object type codes
const (
	oText            = 0x01
	oAttrib          = 0x02
	oAttdef          = 0x03
	oBlock           = 0x04
	oEndBlk          = 0x05
	oSeqEnd          = 0x06
	oInsert          = 0x07
	oMInsert         = 0x08
	oVertex2d        = 0x0A
	oVertex3d        = 0x0B
	oVertexMesh      = 0x0C
	oVertexPface     = 0x0D
	oVertexPfaceFace = 0x0E
	oPolyline2d      = 0x0F
	oPolyline3d      = 0x10
	oArc             = 0x11
	oCircle          = 0x12
	oLine            = 0x13
	oDimOrdinate     = 0x14
	oDimLinear       = 0x15
	oDimAligned      = 0x16
	oDimAng3Pt       = 0x17
	oDimAng2Ln       = 0x18
	oDimRadius       = 0x19
	oDimDiameter     = 0x1A
	oPoint           = 0x1B
	oFace3D          = 0x1C
	oPolylinePface   = 0x1D
	oPolylineMesh    = 0x1E
	oSolid           = 0x1F
	oTrace           = 0x20
	oShape           = 0x21
	oViewport        = 0x22
	oEllipse         = 0x23
	oSpline          = 0x24
	oRegion          = 0x25
	oSolid3d         = 0x26
	oBody            = 0x27
	oRay             = 0x28
	oXLine           = 0x29
	oDictionary      = 0x2A
	oOleFrame        = 0x2B
	oMText           = 0x2C
	oLeader          = 0x2D
	oTolerance       = 0x2E
	oMLine           = 0x2F
	oBlockControl    = 0x30
	oBlockHeader     = 0x31
	oLayerControl    = 0x32
	oLayer           = 0x33
	oStyleControl    = 0x34
	oStyle           = 0x35
	oLtypeControl    = 0x38
	oLtype           = 0x39
	oViewControl     = 0x3C
	oView            = 0x3D
	oUcsControl      = 0x3E
	oUcs             = 0x3F
	oVportControl    = 0x40
	oVport           = 0x41
	oAppIdControl    = 0x42
	oAppId           = 0x43
	oDimStyleControl = 0x44
	oDimStyle        = 0x45
	oVpEntHdrCtrl    = 0x46
	oVpEntHdr        = 0x47
	oGroup           = 0x48
	oMLineStyle      = 0x49
	oOle2Frame       = 0x4A
	oDummy           = 0x4B
	oLongTransaction = 0x4C
	oLwPolyline      = 0x4D
	oHatch           = 0x4E
	oXRecord         = 0x4F
	oAcDbPlaceholder = 0x50
	oVbaProject      = 0x51
	oLayout          = 0x52
	oAcadProxyEntity = 0x1F2
	oAcadProxyObject = 0x1F3
	oCamera          = 0x4F8
	oSun             = 0x4F9
	oLight           = 0x4FA
	oGeoData         = 0x4FB
)

// ObjectTypeFromCode maps a raw object-type code to its ObjectType.
func ObjectTypeFromCode(code uint16) ObjectType { return ObjectType{Code: code} }

// handledFixed reports whether code has an explicit spec label.
func handledFixed(code uint16) bool {
	switch code {
	case 0x00, oText, oAttrib, oAttdef, oBlock, oEndBlk, oSeqEnd, oInsert,
		oMInsert, 0x09, oVertex2d, oVertex3d, oVertexMesh, oVertexPface,
		oVertexPfaceFace, oPolyline2d, oPolyline3d, oArc, oCircle, oLine,
		oDimOrdinate, oDimLinear, oDimAligned, oDimAng3Pt, oDimAng2Ln,
		oDimRadius, oDimDiameter, oPoint, oFace3D, oPolylinePface,
		oPolylineMesh, oSolid, oTrace, oShape, oViewport, oEllipse, oSpline,
		oRegion, oSolid3d, oBody, oRay, oXLine, oDictionary, oOleFrame, oMText,
		oLeader, oTolerance, oMLine, oBlockControl, oBlockHeader, oLayerControl,
		oLayer, oStyleControl, oStyle, 0x36, 0x37, oLtypeControl, oLtype, 0x3A,
		0x3B, oViewControl, oView, oUcsControl, oUcs, oVportControl, oVport,
		oAppIdControl, oAppId, oDimStyleControl, oDimStyle, oVpEntHdrCtrl,
		oVpEntHdr, oGroup, oMLineStyle, oOle2Frame, oDummy, oLongTransaction,
		oLwPolyline, oHatch, oXRecord, oAcDbPlaceholder, oVbaProject, oLayout,
		oAcadProxyEntity, oAcadProxyObject, oCamera, oSun, oLight, oGeoData:
		return true
	}
	return false
}

// IsCustom reports a dynamic (class-indexed) code >= 500 that is not one
// of the fixed late-introduced codes.
func (t ObjectType) IsCustom() bool {
	return t.Code >= 500 && !handledFixed(t.Code)
}

// IsUnknown reports a code in a reserved range the spec does not assign.
func (t ObjectType) IsUnknown() bool {
	if t.IsCustom() {
		return false
	}
	return !handledFixed(t.Code)
}

// IsEntity reports whether the type is a drawable entity.
func (t ObjectType) IsEntity() bool {
	switch t.Code {
	case oText, oAttrib, oAttdef, oBlock, oEndBlk, oSeqEnd, oInsert, oMInsert,
		oVertex2d, oVertex3d, oVertexMesh, oVertexPface, oVertexPfaceFace,
		oPolyline2d, oPolyline3d, oArc, oCircle, oLine, oDimOrdinate, oDimLinear,
		oDimAligned, oDimAng3Pt, oDimAng2Ln, oDimRadius, oDimDiameter, oPoint,
		oFace3D, oPolylinePface, oPolylineMesh, oSolid, oTrace, oShape, oViewport,
		oEllipse, oSpline, oRegion, oSolid3d, oBody, oRay, oXLine, oOleFrame,
		oMText, oLeader, oTolerance, oMLine, oOle2Frame, oLwPolyline, oHatch,
		oAcadProxyEntity, oCamera, oSun, oLight, oGeoData:
		return true
	}
	return false
}

// IsTableEntry reports whether the type is a symbol-table entry.
func (t ObjectType) IsTableEntry() bool {
	switch t.Code {
	case oLayer, oStyle, oLtype, oView, oUcs, oVport, oAppId, oDimStyle, oBlockHeader:
		return true
	}
	return false
}

// IsControl reports whether the type is a *_CONTROL object.
func (t ObjectType) IsControl() bool {
	switch t.Code {
	case oBlockControl, oLayerControl, oStyleControl, oLtypeControl,
		oViewControl, oUcsControl, oVportControl, oAppIdControl,
		oDimStyleControl, oVpEntHdrCtrl:
		return true
	}
	return false
}

// ShortLabel returns the upper-case spec-style label.
func (t ObjectType) ShortLabel() string {
	switch t.Code {
	case 0x00:
		return "UNUSED"
	case oText:
		return "TEXT"
	case oAttrib:
		return "ATTRIB"
	case oAttdef:
		return "ATTDEF"
	case oBlock:
		return "BLOCK"
	case oEndBlk:
		return "ENDBLK"
	case oSeqEnd:
		return "SEQEND"
	case oInsert:
		return "INSERT"
	case oMInsert:
		return "MINSERT"
	case 0x09:
		return "UNASSIGNED_09"
	case oVertex2d:
		return "VERTEX_2D"
	case oVertex3d:
		return "VERTEX_3D"
	case oVertexMesh:
		return "VERTEX_MESH"
	case oVertexPface:
		return "VERTEX_PFACE"
	case oVertexPfaceFace:
		return "VERTEX_PFACE_FACE"
	case oPolyline2d:
		return "POLYLINE_2D"
	case oPolyline3d:
		return "POLYLINE_3D"
	case oArc:
		return "ARC"
	case oCircle:
		return "CIRCLE"
	case oLine:
		return "LINE"
	case oDimOrdinate:
		return "DIMENSION_ORDINATE"
	case oDimLinear:
		return "DIMENSION_LINEAR"
	case oDimAligned:
		return "DIMENSION_ALIGNED"
	case oDimAng3Pt:
		return "DIMENSION_ANG_3PT"
	case oDimAng2Ln:
		return "DIMENSION_ANG_2LN"
	case oDimRadius:
		return "DIMENSION_RADIUS"
	case oDimDiameter:
		return "DIMENSION_DIAMETER"
	case oPoint:
		return "POINT"
	case oFace3D:
		return "3DFACE"
	case oPolylinePface:
		return "POLYLINE_PFACE"
	case oPolylineMesh:
		return "POLYLINE_MESH"
	case oSolid:
		return "SOLID"
	case oTrace:
		return "TRACE"
	case oShape:
		return "SHAPE"
	case oViewport:
		return "VIEWPORT"
	case oEllipse:
		return "ELLIPSE"
	case oSpline:
		return "SPLINE"
	case oRegion:
		return "REGION"
	case oSolid3d:
		return "3DSOLID"
	case oBody:
		return "BODY"
	case oRay:
		return "RAY"
	case oXLine:
		return "XLINE"
	case oDictionary:
		return "DICTIONARY"
	case oOleFrame:
		return "OLEFRAME"
	case oMText:
		return "MTEXT"
	case oLeader:
		return "LEADER"
	case oTolerance:
		return "TOLERANCE"
	case oMLine:
		return "MLINE"
	case oBlockControl:
		return "BLOCK_CONTROL"
	case oBlockHeader:
		return "BLOCK_HEADER"
	case oLayerControl:
		return "LAYER_CONTROL"
	case oLayer:
		return "LAYER"
	case oStyleControl:
		return "STYLE_CONTROL"
	case oStyle:
		return "STYLE"
	case 0x36:
		return "UNASSIGNED_36"
	case 0x37:
		return "UNASSIGNED_37"
	case oLtypeControl:
		return "LTYPE_CONTROL"
	case oLtype:
		return "LTYPE"
	case 0x3A:
		return "UNASSIGNED_3A"
	case 0x3B:
		return "UNASSIGNED_3B"
	case oViewControl:
		return "VIEW_CONTROL"
	case oView:
		return "VIEW"
	case oUcsControl:
		return "UCS_CONTROL"
	case oUcs:
		return "UCS"
	case oVportControl:
		return "VPORT_CONTROL"
	case oVport:
		return "VPORT"
	case oAppIdControl:
		return "APPID_CONTROL"
	case oAppId:
		return "APPID"
	case oDimStyleControl:
		return "DIMSTYLE_CONTROL"
	case oDimStyle:
		return "DIMSTYLE"
	case oVpEntHdrCtrl:
		return "VP_ENT_HDR_CTRL"
	case oVpEntHdr:
		return "VP_ENT_HDR"
	case oGroup:
		return "GROUP"
	case oMLineStyle:
		return "MLINESTYLE"
	case oOle2Frame:
		return "OLE2FRAME"
	case oDummy:
		return "DUMMY"
	case oLongTransaction:
		return "LONG_TRANSACTION"
	case oLwPolyline:
		return "LWPOLYLINE"
	case oHatch:
		return "HATCH"
	case oXRecord:
		return "XRECORD"
	case oAcDbPlaceholder:
		return "ACDB_PLACEHOLDER"
	case oVbaProject:
		return "VBA_PROJECT"
	case oLayout:
		return "LAYOUT"
	case oAcadProxyEntity:
		return "ACAD_PROXY_ENTITY"
	case oAcadProxyObject:
		return "ACAD_PROXY_OBJECT"
	case oCamera:
		return "CAMERA"
	case oSun:
		return "SUN"
	case oLight:
		return "LIGHT"
	case oGeoData:
		return "GEODATA"
	}
	if t.IsCustom() {
		return "CUSTOM"
	}
	return "UNKNOWN"
}

func (t ObjectType) String() string {
	switch {
	case t.IsCustom():
		return fmt.Sprintf("CUSTOM(%d)", t.Code)
	case t.IsUnknown():
		return fmt.Sprintf("UNKNOWN(0x%x)", t.Code)
	default:
		return t.ShortLabel()
	}
}
