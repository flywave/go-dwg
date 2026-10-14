package dwg

import "testing"

// The MLINE vertex count is a 32-bit BL (ODA OdDbMlineImpl::dwgInFields
// reads it with rdInt32), not a BS. A count of 128 encodes differently
// under the two readings, so this locks the ODA-faithful width.
func TestDecodeMLineVertexCountBL(t *testing.T) {
	const verts = 128
	w := newBitWriter()
	w.WriteBD(0) // scale_factor
	w.WriteRC(0) // justification
	w.WriteBD(0) // scale_point
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0) // extrusion
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBS(0)     // open_closed_flags
	w.WriteRC(0)     // num_lines (no per-vertex sub-records)
	w.WriteBL(verts) // num_verts — 32-bit BL
	for i := 0; i < verts; i++ {
		// vertex_point, segment_direction, miter_direction.
		for j := 0; j < 9; j++ {
			w.WriteBD(0)
		}
	}
	c := NewBitCursor(w.bytes())
	m, err := decodeMLine(c)
	if err != nil {
		t.Fatal(err)
	}
	if m.NumLines != 0 || len(m.Vertices) != verts {
		t.Fatalf("mline lines=%d verts=%d, want 0/%d", m.NumLines, len(m.Vertices), verts)
	}
	if c.RemainingBits() >= 8 {
		t.Fatalf("MLINE left %d bits unconsumed", c.RemainingBits())
	}
}
