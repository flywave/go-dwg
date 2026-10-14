package dxf

import "strings"

// HeaderVar is one HEADER variable: the `9 $NAME` group plus its value
// pairs, stored verbatim so unknown variables round-trip losslessly.
type HeaderVar struct {
	Name   string
	Values []Tag
}

// Header is the ordered variable list of the HEADER section.
type Header []HeaderVar

// Find returns the variable with the given name (case-insensitive).
func (h Header) Find(name string) (*HeaderVar, bool) {
	for i := range h {
		if strings.EqualFold(h[i].Name, name) {
			return &h[i], true
		}
	}
	return nil, false
}

// Str returns the first string value of a variable.
func (h Header) Str(name string) (string, bool) {
	v, ok := h.Find(name)
	if !ok || len(v.Values) == 0 {
		return "", false
	}
	return v.Values[0].Value, true
}

// Int returns the first integer value of a variable.
func (h Header) Int(name string) (int64, bool) {
	v, ok := h.Find(name)
	if !ok || len(v.Values) == 0 {
		return 0, false
	}
	return v.Values[0].Int(), true
}

// Float returns the first float value of a variable.
func (h Header) Float(name string) (float64, bool) {
	v, ok := h.Find(name)
	if !ok || len(v.Values) == 0 {
		return 0, false
	}
	return v.Values[0].Float(), true
}

// Point returns the 10/20/30 triple of a variable.
func (h Header) Point(name string) (Point3, bool) {
	v, ok := h.Find(name)
	if !ok {
		return Point3{}, false
	}
	var p Point3
	for _, t := range v.Values {
		switch t.Code {
		case 10:
			p.X = t.Float()
		case 20:
			p.Y = t.Float()
		case 30:
			p.Z = t.Float()
		}
	}
	return p, true
}

// Set replaces (or appends) a variable with a single string value.
func (h *Header) Set(name, value string) {
	if v, ok := h.Find(name); ok {
		v.Values = []Tag{{Code: 1, Value: value}}
		return
	}
	*h = append(*h, HeaderVar{Name: name, Values: []Tag{{Code: 1, Value: value}}})
}

// SetInt replaces (or appends) a variable with a group-70 integer.
func (h *Header) SetInt(name string, value int64) {
	if v, ok := h.Find(name); ok {
		v.Values = []Tag{{Code: 70, Value: itoa(value)}}
		return
	}
	*h = append(*h, HeaderVar{Name: name, Values: []Tag{{Code: 70, Value: itoa(value)}}})
}

// SetFloat replaces (or appends) a variable with a group-40 float.
func (h *Header) SetFloat(name string, value float64) {
	if v, ok := h.Find(name); ok {
		v.Values = []Tag{{Code: 40, Value: FormatFloat(value)}}
		return
	}
	*h = append(*h, HeaderVar{Name: name, Values: []Tag{{Code: 40, Value: FormatFloat(value)}}})
}

// SetPoint replaces (or appends) a variable with a 10/20/30 triple.
func (h *Header) SetPoint(name string, p Point3) {
	vals := []Tag{
		{Code: 10, Value: FormatFloat(p.X)},
		{Code: 20, Value: FormatFloat(p.Y)},
		{Code: 30, Value: FormatFloat(p.Z)},
	}
	if v, ok := h.Find(name); ok {
		v.Values = vals
		return
	}
	*h = append(*h, HeaderVar{Name: name, Values: vals})
}

// Remove drops a variable if present.
func (h *Header) Remove(name string) {
	out := (*h)[:0]
	for _, v := range *h {
		if !strings.EqualFold(v.Name, name) {
			out = append(out, v)
		}
	}
	*h = out
}
