package dxf

import (
	"fmt"
	"io"
	"strings"
)

// TagWriter emits (group code, value) pairs in the canonical DXF line
// format: a right-aligned width-3 code line followed by the value line,
// LF-terminated. It owns no section state — the document writer composes
// sections on top of it. Writes go straight to the underlying writer;
// callers that need buffering wrap it themselves.
type TagWriter struct {
	w     io.Writer
	count int
}

// NewTagWriter wraps w.
func NewTagWriter(w io.Writer) *TagWriter {
	return &TagWriter{w: w}
}

// Pairs writes raw tags verbatim (used for preserved Extra content).
func (w *TagWriter) Pairs(tags []Tag) {
	for _, t := range tags {
		w.Pair(t.Code, t.Value)
	}
}

// Pair writes one raw-string pair.
func (w *TagWriter) Pair(code int, value string) {
	var buf [8]byte
	n := copy(buf[:], fmt.Sprintf("%3d\n", code))
	w.w.Write(buf[:n])
	w.w.Write([]byte(sanitizeValue(value)))
	w.w.Write([]byte{'\n'})
	w.count++
}

// Str writes a string value.
func (w *TagWriter) Str(code int, s string) { w.Pair(code, s) }

// Int writes an integer value.
func (w *TagWriter) Int(code int, v int64) { w.Pair(code, itoa(v)) }

// Bool writes a boolean as 0/1.
func (w *TagWriter) Bool(code int, v bool) {
	if v {
		w.Int(code, 1)
	} else {
		w.Int(code, 0)
	}
}

// Float writes a double value.
func (w *TagWriter) Float(code int, v float64) { w.Pair(code, FormatFloat(v)) }

// Point writes a 3D point as (code, x), (code+10, y), (code+20, z). The
// canonical X codes are 10 (first point), 11 (second), 12/13 (third/fourth).
func (w *TagWriter) Point(code int, p Point3) {
	w.Float(code, p.X)
	w.Float(code+10, p.Y)
	w.Float(code+20, p.Z)
}

// Point2 writes a 2D point as (code, x), (code+10, y) — the LWPOLYLINE
// vertex form.
func (w *TagWriter) Point2(code int, p Point2) {
	w.Float(code, p.X)
	w.Float(code+10, p.Y)
}

// Handle writes a hexadecimal handle with the given code (5 for the
// object's own handle, 330/340/350 for references).
func (w *TagWriter) Handle(code int, h uint64) {
	w.Pair(code, padHandle(h))
}

// Subclass writes the `100 AcDb*` marker.
func (w *TagWriter) Subclass(name string) { w.Str(100, name) }

// Written reports how many pairs were emitted.
func (w *TagWriter) Written() int { return w.count }

// itoa renders an int64 without allocations beyond the result string.
func itoa(v int64) string {
	return fmt.Sprint(v)
}

// padHandle renders a handle in canonical uppercase hex without the 0x
// prefix; zero handles render as "0".
func padHandle(h uint64) string {
	return strings.ToUpper(fmt.Sprintf("%X", h))
}
