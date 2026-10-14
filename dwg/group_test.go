package dwg

import (
	"strings"
	"testing"
)

// buildGroupInline encodes an ACAD_GROUP body on the inline (pre-R2007)
// layout: TV name, BS unnamed, BS selectable, BL member count.
func buildGroupInline(name string, unnamed, selectable int16, members int32) *objectStream {
	w := newBitWriter()
	w.WriteTV(name)
	w.WriteBS(unnamed)
	w.WriteBS(selectable)
	w.WriteBL(members)
	return &objectStream{data: NewBitCursor(w.bytes())}
}

// Mirrors the reference implementation's `roundtrip_empty_group`.
func TestDecodeGroupEmpty(t *testing.T) {
	g, err := decodeGroupBody(buildGroupInline("MyGroup", 0, 1, 0), R2000)
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "MyGroup" || g.IsUnnamed() || !g.IsSelectable() || g.NumMembers != 0 {
		t.Fatalf("unexpected group: %+v", g)
	}
}

// Mirrors the reference implementation's `roundtrip_populated_group`.
func TestDecodeGroupPopulated(t *testing.T) {
	g, err := decodeGroupBody(buildGroupInline("*A1", 2, 0, 2), R2000)
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "*A1" || !g.IsUnnamed() || g.IsSelectable() || g.NumMembers != 2 {
		t.Fatalf("unexpected group: %+v", g)
	}
	if g.Unnamed != 2 {
		t.Fatalf("Unnamed = %d, want raw 2 (not a boolean)", g.Unnamed)
	}
}

// Mirrors the reference implementation's `rejects_excessive_handle_count`.
func TestDecodeGroupRejectsExcessiveCount(t *testing.T) {
	_, err := decodeGroupBody(buildGroupInline("G", 0, 1, maxGroupMembers+1), R2000)
	if err == nil {
		t.Fatal("expected an error for a group past the member cap")
	}
	if !strings.Contains(err.Error(), "ACAD_GROUP") {
		t.Fatalf("error %q does not name ACAD_GROUP", err)
	}
}
