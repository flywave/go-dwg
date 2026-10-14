package dwg

import "testing"

// buildLeader encodes a LEADER body (R2018 layout) with a single path
// point and one point per geometry triple: end_projection, extrusion,
// horiz_direction, offset_to_block_insertion, offset_to_text.
func buildLeader(w *bitWriter) {
	w.WriteB(false) // unknown bit
	w.WriteBS(0)    // annot_type
	w.WriteBS(0)    // path_type
	w.WriteBL(1)    // num_points
	for _, p := range [][3]float64{
		{1, 2, 0},  // points[0]
		{10, 5, 0}, // end_projection
		{0, 0, 1},  // extrusion
		{1, 0, 0},  // horiz_direction
		{0, 0, 0},  // offset_to_block_insertion
		{0, 0, 0},  // offset_to_text
	} {
		w.WriteBD(p[0])
		w.WriteBD(p[1])
		w.WriteBD(p[2])
	}
}

// R2018 tail: two flags, arrowhead type, two more flags (no box pair).
func TestDecodeLeaderR2018Tail(t *testing.T) {
	w := newBitWriter()
	buildLeader(w)
	w.WriteB(false) // hookline_on_x_dir
	w.WriteB(false) // arrowhead_on
	w.WriteBS(0)    // arrowhead_type
	w.WriteB(false) // R2000+ flag
	w.WriteB(false) // R2000+ flag
	c := NewBitCursor(w.bytes())
	l, err := decodeLeader(c, R2018)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Points) != 1 || l.Points[0].X != 1 || l.EndProjection.X != 10 {
		t.Fatalf("leader = %+v", l)
	}
	// Only byte-padding (0..7 bits) may remain.
	if c.RemainingBits() >= 8 {
		t.Fatalf("R2018 LEADER tail left %d bits unconsumed", c.RemainingBits())
	}
}

// R14 tail: discarded BD, box pair, then the long R13/R14-only sequence.
func TestDecodeLeaderR14Tail(t *testing.T) {
	w := newBitWriter()
	buildLeader(w)
	w.WriteBD(0) // discarded dimgap
	w.WriteBD(3) // box_height
	w.WriteBD(4) // box_width
	w.WriteB(false)
	w.WriteB(true) // hookline_on_x_dir, arrowhead_on
	w.WriteBS(1)   // arrowhead_type
	w.WriteBD(0)   // dimasz
	w.WriteB(false)
	w.WriteB(false) // unknown_b1, unknown_b2
	w.WriteBS(0)
	w.WriteBS(0) // unknown_shorts
	w.WriteB(false)
	w.WriteB(false) // trailing flags
	c := NewBitCursor(w.bytes())
	l, err := decodeLeader(c, R14)
	if err != nil {
		t.Fatal(err)
	}
	if l.BoxHeight != 3 || l.BoxWidth != 4 || !l.ArrowheadOn || l.ArrowheadType != 1 {
		t.Fatalf("leader = %+v", l)
	}
	if c.RemainingBits() >= 8 {
		t.Fatalf("R14 LEADER tail left %d bits unconsumed", c.RemainingBits())
	}
}
