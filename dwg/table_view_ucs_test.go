package dwg

import "testing"

// Synthetic round-trips for the VIEW and UCS table entries; the corpus has
// no records, so these byte-level fixtures pin the field order (a wrong
// order trips the split-stream boundary check or the value expectations).
func TestDecodeSyntheticViewUcs(t *testing.T) {
	// Inline VIEW (pre-R2007): TV name + xref bits, then the body.
	w := newBitWriter()
	w.WriteTV("Top")
	w.WriteB(false) // 64 flag
	w.WriteBS(0)    // xref index
	w.WriteB(false) // xref dependent
	w.WriteBD(106.5)
	w.WriteBD(234.0)
	w.WriteRD(65)
	w.WriteRD(15)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0) // target
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(1) // direction
	w.WriteBD(0) // twist
	w.WriteBD(50)
	w.WriteBD(0) // front clip
	w.WriteBD(0) // back clip
	w.WriteBS(0) // view mode
	w.WriteRC(0) // render mode
	w.WriteB(false)
	w.WriteB(false) // no associated UCS
	v, err := decodeViewInline(NewBitCursor(w.bytes()), R2004)
	if err != nil {
		t.Fatal(err)
	}
	if v.Header.Name != "Top" || v.ViewHeight != 106.5 || v.ViewWidth != 234 ||
		v.LensLength != 50 || v.ViewDirection.Z != 1 {
		t.Errorf("view = %+v", v)
	}

	// Inline UCS: name + body + base-UCS handle.
	w2 := newBitWriter()
	w2.WriteTV("World")
	w2.WriteB(false)
	w2.WriteBS(0)
	w2.WriteB(false)
	w2.WriteBD(0)
	w2.WriteBD(0)
	w2.WriteBD(0) // origin
	w2.WriteBD(1)
	w2.WriteBD(0)
	w2.WriteBD(0) // x axis
	w2.WriteBD(0)
	w2.WriteBD(1)
	w2.WriteBD(0)      // y axis
	w2.WriteBD(0)      // elevation
	w2.WriteBS(0)      // ortho type
	w2.writeBits(4, 5) // handle code 5 (hard owner)…
	w2.writeBits(4, 0) // …with a zero counter → null handle
	u, err := decodeUcsInline(NewBitCursor(w2.bytes()), R2004)
	if err != nil {
		t.Fatal(err)
	}
	if u.Header.Name != "World" || u.XAxis.X != 1 || u.YAxis.Y != 1 {
		t.Errorf("ucs = %+v", u)
	}
	if u.HasBaseUcs {
		t.Errorf("base ucs should be unset: %+v", u)
	}
}
