package dxf

import (
	"io"
	"os"
)

// Document is a parsed DXF file. Every section this package models is
// typed; sections it does not understand are preserved verbatim in
// OtherSections so a read→write round trip is lossless.
type Document struct {
	Version DxfVersion

	Header   Header
	Classes  []ClassEntry
	Tables   []*Table
	Blocks   []*Block
	Entities []*Entity
	Objects  []*Object

	// Thumbnail preserves the THUMBNAILIMAGE section verbatim.
	Thumbnail []Tag
	// Comments are diagnostic lines emitted as group-999 comments ahead
	// of EOF.
	Comments []string
	// OtherSections preserves unrecognized named sections.
	OtherSections []RawSection

	// nextHandle backs AllocHandle for synthesized documents.
	nextHandle uint64
}

// RawSection is an unrecognized named section preserved verbatim.
type RawSection struct {
	Name string
	Tags []Tag
}

// Open parses a DXF file from disk (ASCII or binary — auto-detected).
func Open(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads a DXF document from r.
func Parse(r io.Reader) (*Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if isBinaryDxf(data) {
		tags, err := parseBinaryTags(data)
		if err != nil {
			return nil, err
		}
		return documentFromTags(tags)
	}
	tags, err := ParseTags(newByteReader(data))
	if err != nil {
		return nil, err
	}
	return documentFromTags(tags)
}

// WriteTo emits the document as canonical ASCII DXF.
func (d *Document) SaveTo(w io.Writer) error {
	return d.writeTo(NewTagWriter(w))
}

// Save writes the document to path as ASCII DXF.
func (d *Document) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := d.SaveTo(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// SaveBinary writes the document to path in AutoCAD binary DXF form.
func (d *Document) SaveBinary(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := d.writeBinary(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Table returns the named symbol table, or nil.
func (d *Document) Table(name string) *Table {
	for _, t := range d.Tables {
		if equalFold(t.Name, name) {
			return t
		}
	}
	return nil
}

// Block returns the named block, or nil.
func (d *Document) Block(name string) *Block {
	for _, b := range d.Blocks {
		if equalFold(b.Name(), name) {
			return b
		}
	}
	return nil
}

// LayerNames lists the LAYER table entry names in table order.
func (d *Document) LayerNames() []string {
	lt := d.Table("LAYER")
	if lt == nil {
		return nil
	}
	out := make([]string, 0, len(lt.Entries))
	for _, e := range lt.Entries {
		out = append(out, e.Name())
	}
	return out
}

// EntityCount returns the number of top-level entities.
func (d *Document) EntityCount() int { return len(d.Entities) }

// AllocHandle returns the next synthesized handle. Only meaningful for
// documents built in memory (NewDocument seeds the sequence).
func (d *Document) AllocHandle() uint64 {
	d.nextHandle++
	return d.nextHandle
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'a' && ca <= 'z' {
			ca -= 'a' - 'A'
		}
		if cb >= 'a' && cb <= 'z' {
			cb -= 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// newByteReader avoids importing bytes just for ReadAll's common case;
// io.ReadAll already buffers, so this simply wraps the slice.
func newByteReader(b []byte) io.Reader {
	return &sliceReader{b: b}
}

type sliceReader struct {
	b []byte
	i int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
