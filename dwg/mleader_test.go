package dwg

import "testing"

// writeMLeaderCmc encodes one MLeaderCmc: BS index, BLU rgb, RC color byte
// (no string-stream follow-ups when colorByte is 0).
func writeMLeaderCmc(w *bitWriter) {
	w.WriteBS(0)
	w.WriteBL(0)
	w.WriteRC(0)
}

// buildEmptyMLeader encodes a minimal R2010 MULTILEADER body: class version,
// an embedded AnnotContext with zero leader roots and no text/block content,
// then the shared style fields. The color references carry colorByte 0, so the
// (unused here) string reader is never consulted.
func buildEmptyMLeader() *bitWriter {
	w := newBitWriter()
	w.WriteBS(0) // ClassVersion

	// AnnotContext
	w.WriteBL(0) // leader root count
	w.WriteBD(0) // OverallScale
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0) // ContentBasePoint (BD3)
	w.WriteBD(0) // TextHeight
	w.WriteBD(0) // ArrowHeadSize
	w.WriteBD(0) // LandingGap
	w.WriteBS(0) // LeftAttachment
	w.WriteBS(0) // RightAttachment
	w.WriteBS(0) // TextAlignType
	w.WriteBS(0) // AttachmentType
	w.WriteB(false)
	w.WriteB(false) // HasText=false, hasBlock=false
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0) // BasePoint (BD3)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0) // BaseDirection (BD3)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0) // BaseVertical (BD3)
	w.WriteB(false)
	w.WriteBS(0) // TopAttachment
	w.WriteBS(0) // BottomAttachment

	// Shared style fields
	w.WriteBL(0) // OverrideFlags
	w.WriteBS(0) // LeaderType
	writeMLeaderCmc(w)
	w.WriteBL(0) // LineWeight
	w.WriteB(false)
	w.WriteB(false) // LandingEnabled, DoglegEnabled
	w.WriteBD(0)    // LandingDistance
	w.WriteBD(0)    // ArrowHeadSize
	w.WriteBS(0)    // ContentType
	w.WriteBS(0)    // LeftAttachment
	w.WriteBS(0)    // RightAttachment
	w.WriteBS(0)    // TextAngleType
	w.WriteBS(0)    // Unknown175
	writeMLeaderCmc(w)
	w.WriteB(false) // TextFrameEnabled
	writeMLeaderCmc(w)
	w.WriteBD(0)
	w.WriteBD(0)
	w.WriteBD(0) // BlockScale (BD3)
	w.WriteBD(0) // BlockRotation
	w.WriteBS(0) // BlockAttachmentType
	w.WriteB(false)
	w.WriteRC(0) // IsAnnotative, UndocumentedMC(=0)
	w.WriteB(false)
	w.WriteBS(0) // UndocumentedFlag, AttachmentDirection
	w.WriteBS(0) // TopAttachment
	w.WriteBS(0) // BottomAttachment
	return w
}

// The MULTILEADER decoder had no synthetic coverage; this locks the field
// order of the R2010 body and the embedded AnnotContext on an empty record.
func TestDecodeMLeaderEmpty(t *testing.T) {
	w := buildEmptyMLeader()
	strings := emptyStringReader(w.bytes())
	m, err := decodeMLeader(NewBitCursor(w.bytes()), R2010, strings)
	if err != nil {
		t.Fatal(err)
	}
	if m.ClassVersion != 0 {
		t.Errorf("ClassVersion = %d, want 0", m.ClassVersion)
	}
	if len(m.Context.LeaderRoots) != 0 {
		t.Errorf("leader roots = %d, want 0", len(m.Context.LeaderRoots))
	}
	if m.Context.HasText || m.Context.Text != nil || m.Context.Block != nil {
		t.Errorf("unexpected content: text=%v block=%v", m.Context.Text, m.Context.Block)
	}
	if m.Context.OverallScale != 0 || m.Context.TextHeight != 0 || m.Context.LandingGap != 0 {
		t.Errorf("unexpected context scalars: %+v", m.Context)
	}
	if m.ContentType != 0 || m.UndocumentedFlag {
		t.Errorf("unexpected tail: contentType=%d flag=%v", m.ContentType, m.UndocumentedFlag)
	}
}

// The R2013/R2018 tail carries one extra bit; a body built without it must
// still decode on R2010.
func TestDecodeMLeaderRequiresR2010Fields(t *testing.T) {
	if _, err := decodeMLeader(NewBitCursor(nil), R2010, emptyStringReader(nil)); err == nil {
		t.Fatal("expected an error decoding an empty MULTILEADER stream")
	}
}
