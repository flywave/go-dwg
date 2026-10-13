package dwg

import (
	"encoding/binary"
	"math"
	"testing"
)

// Minimal MSB-first bit writer for synthetic entity payloads, mirroring the
// reference's BitWriter test helper.
type bitWriter struct {
	buf []byte
	bit uint8
}

func newBitWriter() *bitWriter { return &bitWriter{} }

func (w *bitWriter) writeBits(n int, v uint64) {
	for i := n - 1; i >= 0; i-- {
		bit := uint8((v >> uint(i)) & 1)
		if w.bit == 0 {
			w.buf = append(w.buf, 0)
			w.bit = 8
		}
		b := &w.buf[len(w.buf)-1]
		*b |= bit << (w.bit - 1)
		w.bit--
	}
}

func (w *bitWriter) WriteB(v bool) {
	if v {
		w.writeBits(1, 1)
	} else {
		w.writeBits(1, 0)
	}
}

func (w *bitWriter) WriteBB(v uint8) { w.writeBits(2, uint64(v)) }

func (w *bitWriter) WriteRC(v uint8) { w.writeBits(8, uint64(v)) }

func (w *bitWriter) WriteRS(v uint16) {
	w.writeBits(8, uint64(v&0xff))
	w.writeBits(8, uint64(v>>8))
}

func (w *bitWriter) WriteRL(v uint32) {
	for i := 0; i < 4; i++ {
		w.writeBits(8, uint64((v>>(8*i))&0xff))
	}
}

// WriteBS writes the short form when possible, else the full word.
func (w *bitWriter) WriteBS(v int16) {
	switch {
	case v == 0:
		w.WriteBB(0b10)
	case v == 256:
		w.WriteBB(0b11)
	case v >= -128 && v <= 127:
		w.WriteBB(0b01)
		w.writeBits(8, uint64(uint8(v)))
	default:
		w.WriteBB(0b00)
		w.WriteRS(uint16(v))
	}
}

func (w *bitWriter) WriteBSU(v uint16) { w.WriteBS(int16(v)) }

func (w *bitWriter) WriteBL(v int32) {
	switch {
	case v == 0:
		w.WriteBB(0b10)
	case v >= -128 && v <= 127:
		w.WriteBB(0b01)
		w.writeBits(8, uint64(uint8(v)))
	default:
		w.WriteBB(0b00)
		w.WriteRL(uint32(v))
	}
}

func (w *bitWriter) WriteRD(v float64) {
	bits := math.Float64bits(v)
	for i := 0; i < 8; i++ {
		w.writeBits(8, (bits>>(8*i))&0xff)
	}
}

func (w *bitWriter) WriteBD(v float64) {
	switch v {
	case 0:
		w.WriteBB(0b10)
	case 1:
		w.WriteBB(0b01)
	default:
		w.WriteBB(0b00)
		w.WriteRD(v)
	}
}

// WriteDD writes a bitdouble-with-default against def (§2.6), choosing the
// shortest encoding the reader accepts.
func (w *bitWriter) WriteDD(v, def float64) {
	vb := math.Float64bits(v)
	db := math.Float64bits(def)
	if vb == db {
		w.WriteBB(0b00)
		return
	}
	var vbLe [8]byte
	var dbLe [8]byte
	binary.LittleEndian.PutUint64(vbLe[:], vb)
	binary.LittleEndian.PutUint64(dbLe[:], db)
	highEqual := [4]byte(vbLe[4:]) == [4]byte(dbLe[4:])
	tailEqual := vbLe[6] == dbLe[6] && vbLe[7] == dbLe[7]
	switch {
	case highEqual:
		// tag 01 patches the low four bytes.
		w.WriteBB(0b01)
		for i := 0; i < 4; i++ {
			w.writeBits(8, uint64(vbLe[i]))
		}
	case tailEqual:
		// tag 10 patches bytes 4,5 then the low four bytes.
		w.WriteBB(0b10)
		w.writeBits(8, uint64(vbLe[4]))
		w.writeBits(8, uint64(vbLe[5]))
		for i := 0; i < 4; i++ {
			w.writeBits(8, uint64(vbLe[i]))
		}
	default:
		w.WriteBB(0b11)
		w.WriteRD(v)
	}
}

func (w *bitWriter) WriteTV(s string) {
	w.WriteBSU(uint16(len(s)))
	for i := 0; i < len(s); i++ {
		w.WriteRC(s[i])
	}
}

func (w *bitWriter) bytes() []byte { return w.buf }

func TestDecodeSyntheticText(t *testing.T) {
	w := newBitWriter()
	w.WriteRC(0xFF) // every optional field elided (set bit = default)
	w.WriteRD(10)
	w.WriteRD(20)
	w.WriteB(true) // BE default extrusion
	w.WriteB(true) // BT default thickness
	w.WriteBD(2.5) // height (BD on the inline layout)
	w.WriteTV("HELLO")
	text, err := decodeText(NewBitCursor(w.bytes()), R2000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if text.InsertionPoint != (Point2D{X: 10, Y: 20}) || text.Height != 2.5 ||
		text.WidthFactor != 1 || text.Text != "HELLO" || text.Elevation != 0 ||
		text.AlignmentPoint != nil || text.Thickness != 0 {
		t.Errorf("text = %+v", text)
	}
}

func TestDecodeSyntheticTextFullFlags(t *testing.T) {
	w := newBitWriter()
	w.WriteRC(0x01 | 0x04 | 0x08 | 0x10 | 0x20 | 0x40 | 0x80) // only the alignment point is present
	w.WriteRD(10)
	w.WriteRD(20)
	w.WriteDD(11, 10)
	w.WriteDD(21, 20)
	w.WriteB(true)
	w.WriteB(true)
	w.WriteBD(2.5)
	w.WriteTV("Hi")
	text, err := decodeText(NewBitCursor(w.bytes()), R2000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if text.AlignmentPoint == nil || *text.AlignmentPoint != (Point2D{X: 11, Y: 21}) {
		t.Errorf("alignment = %+v", text.AlignmentPoint)
	}
	if text.Text != "Hi" || text.Height != 2.5 {
		t.Errorf("text = %+v", text)
	}
}

func TestDecodeSyntheticInsert(t *testing.T) {
	w := newBitWriter()
	w.WriteBD(1)
	w.WriteBD(2)
	w.WriteBD(3) // insertion point
	w.WriteBB(0b10)
	w.WriteRD(2.5) // x scale; y=z=x
	w.WriteBD(0.5) // rotation
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(1)   // extrusion
	w.WriteB(true) // has attribs
	w.WriteBL(3)   // owned object count
	ins, err := decodeInsert(NewBitCursor(w.bytes()), R2013)
	if err != nil {
		t.Fatal(err)
	}
	if ins.InsertionPoint != (Point3D{X: 1, Y: 2, Z: 3}) {
		t.Errorf("insertion = %+v", ins.InsertionPoint)
	}
	if ins.Scale != (Point3D{X: 2.5, Y: 2.5, Z: 2.5}) || ins.Rotation != 0.5 ||
		!ins.HasAttribs || ins.OwnedObjectCount != 3 {
		t.Errorf("insert = %+v", ins)
	}
}

func TestDecodeSyntheticLwPolyline(t *testing.T) {
	w := newBitWriter()
	w.WriteBSU(lwHasBulges | lwClosed)
	w.WriteBL(3) // points
	w.WriteBL(1) // bulges
	w.WriteRD(0)
	w.WriteRD(0)
	w.WriteDD(10, 0)
	w.WriteDD(0, 0)
	w.WriteDD(10, 10)
	w.WriteDD(10, 0)
	w.WriteBD(0.5)
	p, err := decodeLwPolyline(NewBitCursor(w.bytes()), R2013)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Vertices) != 3 || p.Vertices[2] != (Point2D{X: 10, Y: 10}) {
		t.Errorf("vertices = %+v", p.Vertices)
	}
	if len(p.Bulges) != 1 || p.Bulges[0] != 0.5 || !p.Closed {
		t.Errorf("polyline = %+v", p)
	}
}

func TestDecodeSyntheticSplineControl(t *testing.T) {
	w := newBitWriter()
	w.WriteBL(1) // scenario: control
	w.WriteBL(3) // degree
	w.WriteB(false)
	w.WriteB(true) // closed
	w.WriteB(false)
	w.WriteBD(1e-9) // knot tolerance
	w.WriteBD(1e-9) // control tolerance
	w.WriteBL(2)    // knots
	w.WriteBL(2)    // control points
	w.WriteB(false) // no weights
	w.WriteBD(0)
	w.WriteBD(1)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(1)
	w.WriteBD(1)
	w.WriteBD(0)
	s, err := decodeSpline(NewBitCursor(w.bytes()), R2010)
	if err != nil {
		t.Fatal(err)
	}
	if s.Degree != 3 || s.Control == nil || len(s.Control.Knots) != 2 ||
		len(s.Control.ControlPoints) != 2 || !s.Control.Closed {
		t.Fatalf("spline = %+v %+v", s, s.Control)
	}
	if s.Control.Knots[1] != 1 || s.Control.ControlPoints[1].Y != 1 {
		t.Errorf("spline = %+v", s.Control)
	}
}

func TestDecodeSyntheticSplineFitDerived(t *testing.T) {
	// R2013+: scenario word is 1 but the fit-points flag + non-custom knot
	// parameter derive the fit branch (§20.4.40).
	w := newBitWriter()
	w.WriteBL(1) // scenario
	w.WriteBL(splineFlagMethodFitPts | 0x8)
	w.WriteBL(2)     // knot param (uniform)
	w.WriteBL(3)     // degree
	w.WriteBD(1e-10) // fit tolerance
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(1) // begin tangent
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0) // end tangent
	w.WriteBL(2) // fit points
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(5)
	w.WriteBD(5)
	w.WriteBD(0)
	s, err := decodeSpline(NewBitCursor(w.bytes()), R2013)
	if err != nil {
		t.Fatal(err)
	}
	if s.Fit == nil || s.Control != nil {
		t.Fatalf("spline fit/control = %+v %+v", s.Fit, s.Control)
	}
	if s.Fit.Tolerance != 1e-10 || len(s.Fit.FitPoints) != 2 {
		t.Errorf("fit = %+v", s.Fit)
	}
}

func TestDecodeSyntheticDimensionAligned(t *testing.T) {
	w := newBitWriter()
	w.WriteRC(0) // version flag (R2010+)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(1) // extrusion
	w.WriteRD(1)
	w.WriteRD(2)  // text midpoint
	w.WriteBD(0)  // elevation
	w.WriteRC(0)  // flags
	w.WriteTV("") // user text
	w.WriteBD(0)  // text rotation
	w.WriteBD(0)  // horiz dir
	w.WriteBD(1)
	w.WriteBD(1)
	w.WriteBD(1)    // ins scale
	w.WriteBD(0)    // ins rotation
	w.WriteBS(0)    // attachment
	w.WriteBS(0)    // line spacing style
	w.WriteBD(1)    // line spacing factor
	w.WriteBD(42)   // actual measurement
	w.WriteB(false) // unknown bit (R2007+)
	w.WriteB(false) // flip arrow 1
	w.WriteB(false) // flip arrow 2
	w.WriteRD(30)
	w.WriteRD(40) // 12-pt definition point
	// aligned suffix
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0) // def 13
	w.WriteBD(10)
	w.WriteBD(0)
	w.WriteBD(0) // def 14
	w.WriteBD(10)
	w.WriteBD(5)
	w.WriteBD(0) // def 10
	w.WriteBD(0) // ext line rotation
	dim, err := decodeDimension(oDimAligned, NewBitCursor(w.bytes()), R2013, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dim.Kind != "DIMENSION_ALIGNED" || dim.Common.ActualMeasurement != 42 {
		t.Errorf("dimension = %+v", dim)
	}
	if dim.DefPoint10 != (Point3D{X: 10, Y: 5}) || dim.DefPoint14.X != 10 {
		t.Errorf("points = %+v", dim)
	}
}

func TestDecodeSyntheticSimpleEntities(t *testing.T) {
	w := newBitWriter()
	w.WriteBD(1)
	w.WriteBD(2)
	w.WriteBD(3)
	w.WriteBD(1)
	w.WriteBD(0)
	w.WriteBD(0)
	ray, err := decodeRay(NewBitCursor(w.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if ray.Start != (Point3D{X: 1, Y: 2, Z: 3}) || ray.Direction != (Vec3D{X: 1}) {
		t.Errorf("ray = %+v", ray)
	}

	w2 := newBitWriter()
	w2.WriteB(true) // default thickness
	w2.WriteBD(0)   // elevation
	for _, c := range [][2]float64{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		w2.WriteRD(c[0])
		w2.WriteRD(c[1])
	}
	w2.WriteB(true) // default extrusion
	solid, err := decodeSolid(NewBitCursor(w2.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if solid.Thickness != 0 || solid.Corners[3] != (Point2D{X: 1, Y: 1}) {
		t.Errorf("solid = %+v", solid)
	}
}

func TestDecodeSyntheticBatch1(t *testing.T) {
	// VIEWPORT: BD3 center + width + height.
	w := newBitWriter()
	w.WriteBD(100)
	w.WriteBD(200)
	w.WriteBD(0)
	w.WriteBD(50)
	w.WriteBD(25)
	v, err := decodeViewportEntity(NewBitCursor(w.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if v.Center != (Point3D{X: 100, Y: 200}) || v.Width != 50 || v.Height != 25 {
		t.Errorf("viewport = %+v", v)
	}

	// LEADER: 3 points.
	w2 := newBitWriter()
	w2.WriteB(false)
	w2.WriteBS(0)
	w2.WriteBS(0)
	w2.WriteBL(3)
	for _, p := range [][3]float64{{0, 0, 0}, {5, 5, 0}, {10, 5, 0}} {
		w2.WriteBD(p[0])
		w2.WriteBD(p[1])
		w2.WriteBD(p[2])
	}
	w2.WriteBD(10)
	w2.WriteBD(5)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(1)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	l, err := decodeLeader(NewBitCursor(w2.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Points) != 3 || l.Points[2].X != 10 || l.EndProjection.X != 10 {
		t.Errorf("leader = %+v", l)
	}

	// TOLERANCE inline (pre-R2007): no leading unknown-short here.
	w3 := newBitWriter()
	w3.WriteBD(1)
	w3.WriteBD(0.5)
	w3.WriteBD(0)
	w3.WriteBD(0)
	w3.WriteBD(0)
	w3.WriteBD(1)
	w3.WriteBD(0)
	w3.WriteBD(0)
	w3.WriteBD(0)
	w3.WriteBD(0)
	w3.WriteBD(1)
	w3.WriteTV("{\\Fgdt;j}")
	w3.writeBits(4, 5)
	w3.writeBits(4, 0)
	tol, err := decodeTolerance(NewBitCursor(w3.bytes()), R2004, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tol.Height != 1 || tol.DimGap != 0.5 || tol.TextString != "{\\Fgdt;j}" {
		t.Errorf("tolerance = %+v", tol)
	}
}

func TestDecodeSyntheticBatch2(t *testing.T) {
	// 2D POLYLINE (closed).
	w := newBitWriter()
	w.WriteBSU(0x01)
	w.WriteBS(0)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteB(true)
	w.WriteBD(0)
	w.WriteB(true)
	p, err := decodePolyline(NewBitCursor(w.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsClosed() || p.Is3D() || p.Thickness != 0 {
		t.Errorf("polyline = %+v", p)
	}

	// POLYLINE_3D measured shape: 2 flag bytes + owned count.
	w2 := newBitWriter()
	w2.WriteRC(0)
	w2.WriteRC(0)
	w2.WriteBL(5)
	p3, err := decodePolyline3d(NewBitCursor(w2.bytes()), R2018)
	if err != nil {
		t.Fatal(err)
	}
	if p3.Flags != [2]uint8{0, 0} || !p3.HasOwnedCount || p3.NumOwnedObjects != 5 {
		t.Errorf("polyline3d = %+v", p3)
	}

	// VERTEX_PFACE_FACE measured shape: BS ×4.
	w3 := newBitWriter()
	w3.WriteBS(1)
	w3.WriteBS(2)
	w3.WriteBS(3)
	w3.WriteBS(0)
	f, err := decodeVertexPfaceFace(NewBitCursor(w3.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if f.VertexIndices != [4]int16{1, 2, 3, 0} {
		t.Errorf("pface face = %+v", f)
	}

	// POLYFACE_MESH header measured: 5/2/7 in 30 bits.
	w4 := newBitWriter()
	w4.WriteBSU(5)
	w4.WriteBSU(2)
	w4.WriteBL(7)
	m, err := decodePolyfaceMesh(NewBitCursor(w4.bytes()), R2018)
	if err != nil {
		t.Fatal(err)
	}
	if m.VertexCount != 5 || m.FaceCount != 2 || m.NumOwnedObjects != 7 {
		t.Errorf("polyface = %+v", m)
	}

	// POLYGON_MESH header.
	w5 := newBitWriter()
	w5.WriteBSU(0)
	w5.WriteBSU(6)
	w5.WriteBSU(6)
	w5.WriteBSU(5)
	w5.WriteBSU(5)
	pm, err := decodePolygonMesh(NewBitCursor(w5.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if pm.MDensity != 6 || pm.NVertCount != 5 {
		t.Errorf("polygon mesh = %+v", pm)
	}

	// OLE2FRAME with a 4-byte blob.
	w6 := newBitWriter()
	w6.WriteBS(1)
	w6.WriteBL(4)
	for _, b := range []byte{1, 2, 3, 4} {
		w6.WriteRC(b)
	}
	w6.WriteBS(0)
	w6.WriteRC(1)
	o, err := decodeOle2Frame(NewBitCursor(w6.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if o.OleVersion != 1 || len(o.Data) != 4 || o.Data[3] != 4 || o.Mode != 1 {
		t.Errorf("ole2frame = %+v", o)
	}
}

func TestDecodeSyntheticAttribAttdef(t *testing.T) {
	// ATTRIB single-line, inline layout: shared TEXT fields (BD height)
	// with the value TV inline, then the attribute body and the tag TV.
	w := newBitWriter()
	w.WriteRC(0xFF) // all TEXT optional fields elided
	w.WriteRD(1)
	w.WriteRD(2)
	w.WriteB(true)
	w.WriteB(true)
	w.WriteBD(2.5) // height (BD on the inline layout)
	w.WriteTV("VALUE")
	w.WriteBS(0) // field length
	w.WriteRC(0) // flags
	w.WriteTV("TAG1")
	a, err := decodeAttrib(NewBitCursor(w.bytes()), R2004, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.AttributeType != 1 || a.Text.Height != 2.5 || a.Text.InsertionPoint.X != 1 ||
		a.Text.Text != "VALUE" || a.Tag != "TAG1" {
		t.Errorf("attrib = %+v", a)
	}

	// ATTDEF inline (pre-R2007): TEXT fields with BD height, then tag
	// and prompt TVs.
	w2 := newBitWriter()
	w2.WriteRC(0xFF)
	w2.WriteRD(1)
	w2.WriteRD(2)
	w2.WriteB(true)
	w2.WriteB(true)
	w2.WriteBD(2.5)
	w2.WriteTV("VALUE")
	w2.WriteTV("Enter value")
	w2.WriteTV("TAG1")
	w2.WriteBS(0) // field length
	w2.WriteRC(0) // flags
	d, err := decodeAttdef(NewBitCursor(w2.bytes()), R2004, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Tag != "TAG1" || d.Prompt != "Enter value" || d.Text.Height != 2.5 {
		t.Errorf("attdef = %+v", d)
	}
}

func TestDecodeSyntheticBatch4(t *testing.T) {
	// HELIX.
	w := newBitWriter()
	for _, v := range [][3]float64{{0, 0, 0}, {1, 0, 0}, {0, 0, 1}} {
		w.WriteBD(v[0])
		w.WriteBD(v[1])
		w.WriteBD(v[2])
	}
	w.WriteBD(2.5) // radius
	w.WriteBD(5)   // turns
	w.WriteBD(0.5) // turn height
	w.WriteB(true) // right handed
	w.WriteBS(0)   // constrain: turn height
	h, err := decodeHelix(NewBitCursor(w.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if h.Radius != 2.5 || h.Turns != 5 || !h.RightHanded || h.ConstrainType != 0 {
		t.Errorf("helix = %+v", h)
	}

	// IMAGE with the measured WIPEOUT polygon boundary (R2013).
	w2 := newBitWriter()
	w2.WriteBL(0) // class version
	w2.WriteBD(271.9)
	w2.WriteBD(3.9)
	w2.WriteBD(0)
	w2.WriteBD(31.7)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(31.7)
	w2.WriteBD(0)
	w2.WriteRD(1)
	w2.WriteRD(1)
	w2.WriteBS(7)
	w2.WriteB(true)
	w2.WriteRC(50)
	w2.WriteRC(50)
	w2.WriteRC(0)
	w2.WriteB(false) // clip mode (R2010+)
	w2.WriteBS(2)    // polygon boundary
	w2.WriteBL(4)
	for _, p := range [][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		w2.WriteRD(p[0])
		w2.WriteRD(p[1])
	}
	im, err := decodeImage(NewBitCursor(w2.bytes()), R2013)
	if err != nil {
		t.Fatal(err)
	}
	if len(im.ClipPolygon) != 4 || !im.Clipping || im.Brightness != 50 || im.ImageSize.X != 1 {
		t.Errorf("image = %+v", im)
	}

	// IMAGEDEF inline, behind a minimal common object prefix.
	w3 := newBitWriter()
	w3.WriteBSU(0) // no EED
	w3.WriteBL(0)  // no reactors
	w3.WriteB(false)
	w3.WriteBL(0)
	w3.WriteRD(640)
	w3.WriteRD(480)
	w3.WriteTV("textures/wood.jpg")
	w3.WriteB(true)
	w3.WriteRC(0)
	w3.WriteRD(0.01)
	w3.WriteRD(0.01)
	d, err := decodeImageDefObject(w3.bytes(), 0, nil, R2004)
	if err != nil {
		t.Fatal(err)
	}
	if d.FilePath != "textures/wood.jpg" || d.ImageSizePx[0] != 640 || !d.IsLoaded {
		t.Errorf("imagedef = %+v", d)
	}

	// UNDERLAY (PDF) inline.
	w4 := newBitWriter()
	w4.WriteBD(0)
	w4.WriteBD(0)
	w4.WriteBD(1)
	w4.WriteBD(5)
	w4.WriteBD(5)
	w4.WriteBD(0)
	w4.WriteBD(0) // rotation
	w4.WriteBD(2)
	w4.WriteBD(0)
	w4.WriteBD(10) // scale
	w4.WriteRC(0)
	w4.WriteRC(80)
	w4.WriteRC(20)
	w4.WriteBS(0) // no clip vertices
	w4.writeBits(4, 5)
	w4.writeBits(4, 0)
	u, err := decodeUnderlay(NewBitCursor(w4.bytes()), UnderlayPDF, R2004)
	if err != nil {
		t.Fatal(err)
	}
	if u.Kind != UnderlayPDF || u.Contrast != 80 || len(u.ClipPolygon) != 0 {
		t.Errorf("underlay = %+v", u)
	}
}

func TestDecodeSyntheticBatch5(t *testing.T) {
	// MESH: 3 vertices, one triangle face, one edge, one crease.
	w := newBitWriter()
	w.WriteBSU(1)
	w.WriteB(false)
	w.WriteBSU(0)
	w.WriteBSU(3)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(1)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(5)
	w.WriteBD(5)
	w.WriteBD(0)
	w.WriteBL(4)
	w.WriteBL(3)
	w.WriteBL(0)
	w.WriteBL(1)
	w.WriteBL(2)
	w.WriteBL(2)
	w.WriteBL(0)
	w.WriteBL(1)
	w.WriteBL(0)
	w.WriteBL(1)
	w.WriteBL(1)
	w.WriteBD(0.5)
	m, err := decodeMesh(NewBitCursor(w.bytes()), R2013)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Vertices) != 3 || len(m.Faces) != 1 || len(m.Faces[0]) != 3 ||
		len(m.Edges) != 2 || m.Creases[0] != 0.5 {
		t.Errorf("mesh edges=%v creases=%v", m.Edges, m.Creases)
	}

	// MLINE: one line, one vertex with empty parameter lists.
	w2 := newBitWriter()
	w2.WriteBD(1)
	w2.WriteRC(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(1)
	w2.WriteBS(0)
	w2.WriteRC(1)
	w2.WriteBS(1)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(1)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(1)
	w2.WriteBD(0)
	w2.WriteBS(0)
	w2.WriteBS(0)
	ml, err := decodeMLine(NewBitCursor(w2.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if ml.NumLines != 1 || len(ml.Vertices) != 1 || len(ml.Vertices[0].SegmentParameters) != 0 {
		t.Errorf("mline = %+v", ml)
	}

	// EXTRUDEDSURFACE with an empty SAT envelope.
	w3 := newBitWriter()
	w3.WriteB(true)
	w3.WriteBD(0)
	w3.WriteBD(0)
	w3.WriteBD(10)
	w3.WriteBD(0.1)
	common := &CommonEntityData{}
	s, err := decodeExtrudedSurface(NewBitCursor(w3.bytes()), R2013, common)
	if err != nil {
		t.Fatal(err)
	}
	if s.SweepVector != (Vec3D{X: 0, Y: 0, Z: 10}) || s.DraftAngle != 0.1 || !s.Sat.Empty {
		t.Errorf("extruded surface = %+v", s)
	}

	// CAMERA (R2010).
	w4 := newBitWriter()
	w4.WriteBD(1)
	w4.WriteBD(2)
	w4.WriteBD(3)
	w4.WriteBD(0)
	w4.WriteBD(0)
	w4.WriteBD(-1)
	w4.WriteBD(50)
	w4.WriteBD(0.6)
	w4.writeBits(4, 5)
	w4.writeBits(4, 0)
	cam, err := decodeCamera(NewBitCursor(w4.bytes()), R2010)
	if err != nil {
		t.Fatal(err)
	}
	if cam.Origin != (Point3D{X: 1, Y: 2, Z: 3}) || cam.FocalLength != 50 || cam.FieldOfView != 0.6 {
		t.Errorf("camera = %+v", cam)
	}
}

func TestDbgIsoInFile(t *testing.T) {
	w := newBitWriter()
	w.WriteBSU(1)
	w.WriteB(false)
	w.WriteBSU(0)
	w.WriteBSU(3)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(1)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(5)
	w.WriteBD(5)
	w.WriteBD(0)
	w.WriteBL(4)
	w.WriteBL(3)
	w.WriteBL(0)
	w.WriteBL(1)
	w.WriteBL(2)
	w.WriteBL(2)
	w.WriteBL(0)
	w.WriteBL(1)
	w.WriteBL(0)
	w.WriteBL(1)
	w.WriteBL(1)
	w.WriteBD(0.5)
	m, _ := decodeMesh(NewBitCursor(w.bytes()), R2013)
	t.Logf("in-file iso: edges=%v creases=%v bytes=%d buf=% x", m.Edges, m.Creases, len(w.bytes()), w.bytes())
}
