package dxf

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Tag is one (group code, raw value) pair — the atom of a DXF file. The
// value is kept as text exactly as read; typed accessors (Int, Float, Str)
// parse on demand so round trips never lose formatting information.
type Tag struct {
	Code  int
	Value string
}

// Str returns the raw value.
func (t Tag) Str() string { return t.Value }

// Int parses the value as an integer; errors keep the zero.
func (t Tag) Int() int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(t.Value), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// Float parses the value as a float; errors keep the zero.
func (t Tag) Float() float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(t.Value), 64)
	if err != nil {
		return 0
	}
	return f
}

// errUnexpectedEOF wraps io.ErrUnexpectedEOF with tag context.
var errUnexpectedEOF = errors.New("dxf: unexpected end of file inside tag pair")

// ParseTags reads a complete ASCII tag stream. It tolerates CRLF and LF,
// leading/trailing blank lines and a UTF-8 BOM. A truncated final pair is
// an error.
func ParseTags(r io.Reader) ([]Tag, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	var tags []Tag
	first := true
	for {
		codeLine, err := readCleanLine(br, &first)
		if err == io.EOF {
			return tags, nil
		}
		if err != nil {
			return tags, err
		}
		if codeLine == "" {
			continue // tolerate stray blank lines between pairs
		}
		code, err := strconv.Atoi(strings.TrimSpace(codeLine))
		if err != nil {
			return tags, fmt.Errorf("dxf: group code line %q is not an integer", clip(codeLine))
		}
		valueLine, err := readCleanLine(br, &first)
		if err == io.EOF {
			return tags, fmt.Errorf("dxf: group code %d at end of file has no value", code)
		}
		if err != nil {
			return tags, err
		}
		tags = append(tags, Tag{Code: code, Value: valueLine})
	}
}

// readCleanLine reads one line, stripping the trailing CR of CRLF pairs and
// a leading BOM on the very first line.
func readCleanLine(br *bufio.Reader, first *bool) (string, error) {
	line, err := br.ReadString('\n')
	if len(line) == 0 && err != nil {
		return "", err
	}
	line = strings.TrimRight(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	if *first {
		*first = false
		line = strings.TrimPrefix(line, "\ufeff")
	}
	if err != nil && err != io.EOF {
		return "", err
	}
	return line, nil
}

func clip(s string) string {
	if len(s) > 64 {
		return s[:64] + "…"
	}
	return s
}

// FormatFloat renders a float the way every DXF reader expects: shortest
// round-trip representation, always parseable, deterministic.
func FormatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// sanitizeValue strips line breaks — a value containing a newline would
// corrupt the strictly line-based pair structure.
func sanitizeValue(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return s
	}
	r := strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")
	return r.Replace(s)
}
