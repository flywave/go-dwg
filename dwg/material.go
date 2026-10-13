package dwg

// ACAD_MATERIAL object — rendering material definition. There is no §20.4
// prescription (the reference decodes only its strings). Field order
// recovered from `OdDbMaterialImpl::dwgInFields` (libTD_DbCore.so:0xa0b840)
// and its helpers `rdColor` (0xa09a42), `rdMap` (0xa0b0c2), `rdMapper`
// (0xa0ae9c) and `rdMatrix3d` (0xa09302):
//
//	TV name, TV description,
//	rdColor, rdColor, rdMap, rdColor, rdMap, BD, rdMap, BD, rdMap, rdMap, BD, rdMap,
//	BD x3, BL x3
//
//	rdColor  = RC method, BD factor, [method==1] BL colour
//	rdMap    = BD blend, rdMapper, RC source, [source==1] TV file, [source==2] rdTexture
//	rdMapper = RC projection, RC uTiling, [filerType!=0] RC vTiling, RC autoTransform,
//	           16 x BD (4x4 transform)
//
// The corpus records close with delta 0. `rdAdvMaterial` is gated on a
// non-file filer, so it is absent on the read path. A texture map
// (`source==2`) is not decoded — such a record is reported as an error
// rather than mis-read.

// MaterialColor is one MATERIAL colour slot.
type MaterialColor struct {
	Method uint8
	Factor float64
	Color  uint32 // only when Method == 1
}

// MaterialMapper is a texture-placer transform.
type MaterialMapper struct {
	Projection    uint8
	UTiling       uint8
	VTiling       uint8
	AutoTransform uint8
	Transform     [16]float64
}

// MaterialMap is one texture-map slot.
type MaterialMap struct {
	Blend      float64
	Mapper     MaterialMapper
	Source     uint8
	SourceFile string
}

// Material is a decoded ACAD_MATERIAL record.
type Material struct {
	Name        string
	Description string
	Colors      []MaterialColor
	Maps        []MaterialMap
	Scalars     []float64 // the top-level/version BD words, in wire order
	VersionInts []int32   // the trailing BL words, in wire order
}

func readMaterialColor(c *BitCursor, version Version) (MaterialColor, error) {
	var mc MaterialColor
	var err error
	if mc.Method, err = c.ReadRC(); err != nil {
		return mc, err
	}
	if mc.Factor, err = c.ReadBD(); err != nil {
		return mc, err
	}
	if mc.Method == 1 {
		if mc.Color, err = c.ReadBLU(); err != nil {
			return mc, err
		}
	}
	return mc, nil
}

func readMaterialMatrix(c *BitCursor) ([16]float64, error) {
	var m [16]float64
	for i := range m {
		v, err := c.ReadBD()
		if err != nil {
			return m, err
		}
		m[i] = v
	}
	return m, nil
}

func readMaterialMapper(c *BitCursor, version Version) (MaterialMapper, error) {
	var m MaterialMapper
	var err error
	if m.Projection, err = c.ReadRC(); err != nil {
		return m, err
	}
	if m.UTiling, err = c.ReadRC(); err != nil {
		return m, err
	}
	// A non-file filer (`filerType() != 0`) writes a third tiling byte here;
	// the file read path does not, so only three RCs are consumed.
	if m.AutoTransform, err = c.ReadRC(); err != nil {
		return m, err
	}
	if m.Transform, err = readMaterialMatrix(c); err != nil {
		return m, err
	}
	return m, nil
}

func readMaterialMap(c *BitCursor, o *objectStream, version Version) (MaterialMap, error) {
	var mm MaterialMap
	var err error
	if mm.Blend, err = c.ReadBD(); err != nil {
		return mm, err
	}
	if mm.Mapper, err = readMaterialMapper(c, version); err != nil {
		return mm, err
	}
	if mm.Source, err = c.ReadRC(); err != nil {
		return mm, err
	}
	switch mm.Source {
	case 1:
		if mm.SourceFile, err = o.readTV(version); err != nil {
			return mm, err
		}
	case 2:
		return mm, newError(ErrUnsupported, "MATERIAL texture map (source==2) is not decoded")
	}
	return mm, nil
}

// decodeMaterialObject decodes an ACAD_MATERIAL straight from its raw object
// payload.
func decodeMaterialObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (Material, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return Material{}, err
	}
	var m Material
	if m.Name, err = o.readTV(version); err != nil {
		return m, err
	}
	if m.Description, err = o.readTV(version); err != nil {
		return m, err
	}
	c := o.data
	if m.Colors, err = appendColor(m.Colors, c, version); err != nil {
		return m, err
	}
	if m.Colors, err = appendColor(m.Colors, c, version); err != nil {
		return m, err
	}
	if m.Maps, err = appendMap(m.Maps, c, o, version); err != nil {
		return m, err
	}
	if m.Colors, err = appendColor(m.Colors, c, version); err != nil {
		return m, err
	}
	if m.Maps, err = appendMap(m.Maps, c, o, version); err != nil {
		return m, err
	}
	if err := appendScalar(&m, c); err != nil {
		return m, err
	}
	if m.Maps, err = appendMap(m.Maps, c, o, version); err != nil {
		return m, err
	}
	if err := appendScalar(&m, c); err != nil {
		return m, err
	}
	if m.Maps, err = appendMap(m.Maps, c, o, version); err != nil {
		return m, err
	}
	if m.Maps, err = appendMap(m.Maps, c, o, version); err != nil {
		return m, err
	}
	if err := appendScalar(&m, c); err != nil {
		return m, err
	}
	if m.Maps, err = appendMap(m.Maps, c, o, version); err != nil {
		return m, err
	}
	for i := 0; i < 3; i++ {
		if err := appendScalar(&m, c); err != nil {
			return m, err
		}
	}
	for i := 0; i < 3; i++ {
		v, err := c.ReadBL()
		if err != nil {
			return m, err
		}
		m.VersionInts = append(m.VersionInts, v)
	}
	if err := o.finish("MATERIAL"); err != nil {
		return m, err
	}
	return m, nil
}

func appendColor(dst []MaterialColor, c *BitCursor, version Version) ([]MaterialColor, error) {
	v, err := readMaterialColor(c, version)
	if err != nil {
		return dst, err
	}
	return append(dst, v), nil
}

func appendMap(dst []MaterialMap, c *BitCursor, o *objectStream, version Version) ([]MaterialMap, error) {
	v, err := readMaterialMap(c, o, version)
	if err != nil {
		return dst, err
	}
	return append(dst, v), nil
}

func appendScalar(m *Material, c *BitCursor) error {
	v, err := c.ReadBD()
	if err != nil {
		return err
	}
	m.Scalars = append(m.Scalars, v)
	return nil
}
