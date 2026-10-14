package dwg

// Custom-class object decoders reached through the AcDb:Classes table. A
// custom class's object type code is assigned per file; the dispatcher
// resolves it by DXF class name.

// DictionaryVar is a decoded DICTIONARYVAR object (AcDbDictionaryVar).
type DictionaryVar struct {
	Schema uint8
	Value  string
}

// Scale is a decoded ACAD_SCALE object (AcDbScale).
type Scale struct {
	Version      int16
	ScaleName    string
	PaperUnits   float64
	DrawingUnits float64
	IsUnitScale  bool
}

// decodeDictionaryVarObject decodes a DICTIONARYVAR straight from its raw
// object payload.
func decodeDictionaryVarObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (DictionaryVar, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return DictionaryVar{}, err
	}
	schema, err := o.data.ReadRC()
	if err != nil {
		return DictionaryVar{}, err
	}
	value, err := o.readTV(version)
	if err != nil {
		return DictionaryVar{}, err
	}
	if err := o.finish("DICTIONARYVAR"); err != nil {
		return DictionaryVar{}, err
	}
	return DictionaryVar{Schema: schema, Value: value}, nil
}

// decodeScaleObject decodes an ACAD_SCALE straight from its raw object payload.
func decodeScaleObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (Scale, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return Scale{}, err
	}
	ver, err := o.data.ReadBS()
	if err != nil {
		return Scale{}, err
	}
	name, err := o.readTV(version)
	if err != nil {
		return Scale{}, err
	}
	paper, err := o.data.ReadBD()
	if err != nil {
		return Scale{}, err
	}
	drawing, err := o.data.ReadBD()
	if err != nil {
		return Scale{}, err
	}
	unit, err := o.data.ReadB()
	if err != nil {
		return Scale{}, err
	}
	if err := o.finish("SCALE"); err != nil {
		return Scale{}, err
	}
	return Scale{Version: ver, ScaleName: name, PaperUnits: paper, DrawingUnits: drawing, IsUnitScale: unit}, nil
}

// decodeDictionaryObjectWithDefault decodes an ACDBDICTIONARYWDFLT, which
// shares DICTIONARY's field list plus — on R13/R14 only — one leading TV.
func decodeDictionaryObjectWithDefault(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (Dictionary, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return Dictionary{}, err
	}
	numItems, err := o.data.ReadBL()
	if err != nil {
		return Dictionary{}, err
	}
	n := int(numItems)
	if n < 0 || n > maxDictionaryItems {
		return Dictionary{}, newError(ErrSectionMap, "DICTIONARY claims %d items (>%d sanity cap)", n, maxDictionaryItems)
	}
	var cloning int16
	var hardFlag uint8
	if version == R14 {
		if _, err := o.data.ReadRC(); err != nil {
			return Dictionary{}, err
		}
	} else {
		c, err := o.data.ReadBS()
		if err != nil {
			return Dictionary{}, err
		}
		cloning = c
		h, err := o.data.ReadRC()
		if err != nil {
			return Dictionary{}, err
		}
		hardFlag = h
	}
	if version == R14 {
		if _, err := o.readTV(version); err != nil { // default slot
			return Dictionary{}, err
		}
	}
	keys := make([]string, 0, n)
	for i := 0; i < n; i++ {
		k, err := o.readTV(version)
		if err != nil {
			return Dictionary{}, err
		}
		keys = append(keys, k)
	}
	if err := o.finish("DICTIONARY"); err != nil {
		return Dictionary{}, err
	}
	return Dictionary{CloningFlag: cloning, HardOwner: hardFlag != 0, Keys: keys}, nil
}

// dispatchObjectClass decodes a custom-class object by DXF class name.
// Returns (result, true) when the class is handled here.
func dispatchObjectClass(raw *RawObject, dxfName string, version Version) (DecodedEntity, bool) {
	bs, err := BodyCursor(raw, version)
	if err != nil {
		return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: "failed to position cursor: " + err.Error()}, true
	}
	bodyStart := bs.PositionBits()
	switch dxfName {
	case "DICTIONARYVAR", "ACDBDICTIONARYVAR":
		v, err := decodeDictionaryVarObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "DICTIONARYVAR", TypeCode: raw.TypeCode, Value: &v}, true
	case "SCALE", "ACDBSCALE":
		s, err := decodeScaleObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "SCALE", TypeCode: raw.TypeCode, Value: &s}, true
	case "ACDBDICTIONARYWDFLT", "ACDBDICTIONARYWITHDEFAULT":
		d, err := decodeDictionaryObjectWithDefault(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "DICTIONARY", TypeCode: raw.TypeCode, Value: &d}, true
	case "VISUALSTYLE", "ACDBVISUALSTYLE":
		v, err := decodeVisualStyleObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "VISUALSTYLE", TypeCode: raw.TypeCode, Value: &v}, true
	case "MLEADERSTYLE", "ACDBMLEADERSTYLE":
		m, err := decodeMLeaderStyleObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "MLEADERSTYLE", TypeCode: raw.TypeCode, Value: &m}, true
	case "SOLID_BACKGROUND", "ACDBSOLIDBACKGROUND":
		s, err := decodeSolidBackgroundObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "SOLID_BACKGROUND", TypeCode: raw.TypeCode, Value: &s}, true
	case "SUN", "ACDBSUN":
		s, err := decodeSunObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "SUN", TypeCode: raw.TypeCode, Value: &s}, true
	case "MATERIAL", "ACDBMATERIAL":
		m, err := decodeMaterialObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "MATERIAL", TypeCode: raw.TypeCode, Value: &m}, true
	case "MENTALRAYRENDERSETTINGS", "ACDBMENTALRAYRENDERSETTINGS":
		m, err := decodeMentalRayRenderSettingsObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "MENTALRAYRENDERSETTINGS", TypeCode: raw.TypeCode, Value: &m}, true
	case "ACAD_EVALUATION_GRAPH", "EVALUATION_GRAPH":
		g, err := decodeEvalGraphObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "EVALUATION_GRAPH", TypeCode: raw.TypeCode, Value: &g}, true
	case "SH_HISTORY", "ACDBSH_HISTORY", "ACDBSHHISTORY", "ACSH_HISTORY_CLASS":
		h, err := decodeShHistoryObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "SH_HISTORY", TypeCode: raw.TypeCode, Value: &h}, true
	case "ACSH_SPHERE_CLASS", "ACDBSH_SPHERE":
		s, err := decodeShPrimitiveObject(raw.Raw, bodyStart, raw.ObjSizeBits, version, 1)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "SH_SPHERE", TypeCode: raw.TypeCode, Value: &s}, true
	case "ACSH_CYLINDER_CLASS", "ACDBSH_CYLINDER":
		s, err := decodeShPrimitiveObject(raw.Raw, bodyStart, raw.ObjSizeBits, version, 4)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "SH_CYLINDER", TypeCode: raw.TypeCode, Value: &s}, true
	case "ACSH_CONE_CLASS", "ACDBSH_CONE":
		s, err := decodeShPrimitiveObject(raw.Raw, bodyStart, raw.ObjSizeBits, version, 4)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "SH_CONE", TypeCode: raw.TypeCode, Value: &s}, true
	case "ACSH_WEDGE_CLASS", "ACDBSH_WEDGE":
		s, err := decodeShPrimitiveObject(raw.Raw, bodyStart, raw.ObjSizeBits, version, 3)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "SH_WEDGE", TypeCode: raw.TypeCode, Value: &s}, true
	case "SORTENTSTABLE", "ACDBSORTENTSTABLE":
		s, err := decodeSortentsTableObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "SORTENTSTABLE", TypeCode: raw.TypeCode, Value: &s}, true
	case "SECTIONVIEWSTYLE", "ACDBSECTIONVIEWSTYLE":
		s, err := decodeSectionViewStyleObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "SECTIONVIEWSTYLE", TypeCode: raw.TypeCode, Value: &s}, true
	case "DETAILVIEWSTYLE", "ACDBDETAILVIEWSTYLE":
		s, err := decodeDetailViewStyleObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "DETAILVIEWSTYLE", TypeCode: raw.TypeCode, Value: &s}, true
	case "TABLESTYLE", "ACDBTABLESTYLE":
		s, err := decodeTableStyleClassObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "TABLESTYLE", TypeCode: raw.TypeCode, Value: &s}, true
	case "IMAGEDEF", "ACDBIMAGEDEF":
		d, err := decodeImageDefObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "IMAGEDEF", TypeCode: raw.TypeCode, Value: &d}, true
	case "GEODATA":
		g, err := decodeGeoDataObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "GEODATA", TypeCode: raw.TypeCode, Value: &g}, true
	case "PLOTSETTINGS", "ACDBPLOTSETTINGS":
		s, err := decodePlotSettingsObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "PLOTSETTINGS", TypeCode: raw.TypeCode, Value: &s}, true
	case "ACAD_GROUP", "GROUP":
		g, err := decodeGroupObject(raw.Raw, bodyStart, raw.ObjSizeBits, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}, true
		}
		return DecodedEntity{Label: "GROUP", TypeCode: raw.TypeCode, Value: &g}, true
	}
	return DecodedEntity{}, false
}

// decodeSurfaceEntity decodes an OdDbSurface-derived custom-class entity
// (e.g. PLANESURFACE): the common entity preamble, the OdDbModelerGeometry
// fields shared with 3DSOLID, then two trailing `BS` words — the shape
// recovered from `OdDbSurface::dwgInFields` (TD_DbEntities.tx:0x9c90e4),
// which calls `OdDbEntity::dwgInFields` + `OdDbModelerGeometryImpl::dwgInFields`
// and then `rdInt16` twice.
func decodeSurfaceEntity(raw *RawObject, version Version, label string) DecodedEntity {
	value, err := checkedInline(raw, version, label, func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error) {
		rec, err := decodeModelerRecord(c, v, common.BinaryChain)
		if err != nil {
			return nil, err
		}
		if _, err := c.ReadBS(); err != nil {
			return nil, err
		}
		if _, err := c.ReadBS(); err != nil {
			return nil, err
		}
		return &rec, nil
	})
	if err != nil {
		return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
	}
	return DecodedEntity{Label: label, TypeCode: raw.TypeCode, Value: value}
}

// decodeSurfaceCustomEntity runs a surface decoder behind the common
// entity preamble.
func decodeSurfaceCustomEntity(raw *RawObject, version Version, label string, body func(c *BitCursor, v Version, common *CommonEntityData) (any, error)) DecodedEntity {
	value, err := checkedInline(raw, version, label, func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error) {
		out, err := body(c, v, common)
		if err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
	}
	return DecodedEntity{Label: label, TypeCode: raw.TypeCode, Value: value}
}

// decodeMLeaderEntity decodes a MULTILEADER through the entity preamble;
// its TVs live in the R2007+ string stream.
func decodeMLeaderEntity(raw *RawObject, version Version) DecodedEntity {
	value, err := checkedInline(raw, version, "MLEADER", func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error) {
		if strings == nil {
			return nil, newError(ErrUnsupported, "MLEADER requires the R2007+ string stream")
		}
		m, err := decodeMLeader(c, v, strings)
		if err != nil {
			return nil, err
		}
		return &m, nil
	})
	if err != nil {
		return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
	}
	return DecodedEntity{Label: "MLEADER", TypeCode: raw.TypeCode, Value: value}
}

// decodeUnderlayEntity decodes the three UNDERLAY custom-class entities,
// which share one body.
func decodeUnderlayEntity(raw *RawObject, version Version, kind UnderlayKind) DecodedEntity {
	value, err := checkedInline(raw, version, "UNDERLAY", func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error) {
		u, err := decodeUnderlay(c, kind, v)
		if err != nil {
			return nil, err
		}
		return &u, nil
	})
	if err != nil {
		return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
	}
	return DecodedEntity{Label: "UNDERLAY", TypeCode: raw.TypeCode, Value: value}
}

// DecodeFromRawWithClassMap decodes a custom-class RawObject by resolving its
// type code through the class map, mirroring the reference dispatcher.
func DecodeFromRawWithClassMap(raw *RawObject, version Version, cm *ClassMap) DecodedEntity {
	classDef := cm.ByTypeCode(raw.TypeCode)
	if classDef == nil {
		return DecodedEntity{Label: "UNHANDLED", TypeCode: raw.TypeCode}
	}
	// Custom classes that extend OdDbEntity use the common *entity* preamble,
	// not the object prefix.
	switch classDef.DxfClassName {
	case "LIGHT", "ACDBLIGHT":
		l, err := decodeLightEntity(raw, version)
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		return DecodedEntity{Label: "LIGHT", TypeCode: raw.TypeCode, Value: &l}
	case "PLANESURFACE", "ACDBPLANESURFACE":
		return decodeSurfaceEntity(raw, version, "PLANESURFACE")
	case "MLEADER", "MULTILEADER":
		return decodeMLeaderEntity(raw, version)
	case "HELIX", "ACDBHELIX":
		value, err := checkedInline(raw, version, "HELIX", func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error) {
			h, err := decodeHelix(c)
			if err != nil {
				return nil, err
			}
			return &h, nil
		})
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		return DecodedEntity{Label: "HELIX", TypeCode: raw.TypeCode, Value: value}
	case "IMAGE", "RASTERIMAGE":
		value, err := checkedInline(raw, version, "IMAGE", func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error) {
			im, err := decodeImage(c, v)
			if err != nil {
				return nil, err
			}
			return &im, nil
		})
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		return DecodedEntity{Label: "IMAGE", TypeCode: raw.TypeCode, Value: value}
	case "WIPEOUT", "ACDBWIPEOUT":
		value, err := checkedInline(raw, version, "WIPEOUT", func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error) {
			im, err := decodeImage(c, v)
			if err != nil {
				return nil, err
			}
			return &im, nil
		})
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		return DecodedEntity{Label: "WIPEOUT", TypeCode: raw.TypeCode, Value: value}
	case "PDFUNDERLAY":
		return decodeUnderlayEntity(raw, version, UnderlayPDF)
	case "DWFUNDERLAY":
		return decodeUnderlayEntity(raw, version, UnderlayDWF)
	case "DGNUNDERLAY":
		return decodeUnderlayEntity(raw, version, UnderlayDGN)
	case "EXTRUDEDSURFACE", "ACDBEXTRUDEDSURFACE":
		return decodeSurfaceCustomEntity(raw, version, "EXTRUDEDSURFACE", func(c *BitCursor, v Version, common *CommonEntityData) (any, error) {
			s, err := decodeExtrudedSurface(c, v, common)
			return &s, err
		})
	case "REVOLVEDSURFACE", "ACDBREVOLVEDSURFACE":
		return decodeSurfaceCustomEntity(raw, version, "REVOLVEDSURFACE", func(c *BitCursor, v Version, common *CommonEntityData) (any, error) {
			s, err := decodeRevolvedSurface(c, v, common)
			return &s, err
		})
	case "SWEPTSURFACE", "ACDBSWEPTSURFACE":
		return decodeSurfaceCustomEntity(raw, version, "SWEPTSURFACE", func(c *BitCursor, v Version, common *CommonEntityData) (any, error) {
			s, err := decodeSweptSurface(c, v, common)
			return &s, err
		})
	case "LOFTEDSURFACE", "ACDBLOFTEDSURFACE":
		return decodeSurfaceCustomEntity(raw, version, "LOFTEDSURFACE", func(c *BitCursor, v Version, common *CommonEntityData) (any, error) {
			s, err := decodeLoftedSurface(c, v, common)
			return &s, err
		})
	case "MESH", "ACDBSUBDMESH", "ACDBMESH":
		value, err := checkedInline(raw, version, "MESH", func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error) {
			m, err := decodeMesh(c, v)
			if err != nil {
				return nil, err
			}
			return &m, nil
		})
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		return DecodedEntity{Label: "MESH", TypeCode: raw.TypeCode, Value: value}
	case "CAMERA", "ACDBCAMERA":
		value, err := checkedInline(raw, version, "CAMERA", func(c *BitCursor, v Version, common *CommonEntityData, strings *stringReader) (any, error) {
			cam, err := decodeCamera(c, v)
			if err != nil {
				return nil, err
			}
			return &cam, nil
		})
		if err != nil {
			return DecodedEntity{Label: "ERROR", TypeCode: raw.TypeCode, Message: err.Error()}
		}
		return DecodedEntity{Label: "CAMERA", TypeCode: raw.TypeCode, Value: value}
	}
	if d, ok := dispatchObjectClass(raw, classDef.DxfClassName, version); ok {
		return d
	}
	if classDef.ItemClassID == oAcadProxyObject {
		return DecodedEntity{Label: "UNHANDLED", TypeCode: raw.TypeCode}
	}
	return DecodedEntity{Label: "UNHANDLED", TypeCode: raw.TypeCode}
}
