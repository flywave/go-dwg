package dxf

import (
	"fmt"
	"strconv"
)

// Entity is one drawable entry in the ENTITIES section (or inside a BLOCK).
// Type carries the canonical DXF name ("LINE", "CIRCLE", …); Data holds the
// typed payload. Entities this package does not model carry Data of type
// *UnknownEntity, which preserves the full tag run verbatim.
type Entity struct {
	Type string
	Data AnyEntityData
}

// AnyEntityData is implemented by every typed entity payload.
type AnyEntityData interface {
	// EntityType returns the canonical DXF entity name.
	EntityType() string
}

// commonHolder is satisfied by every typed payload that embeds Common.
type commonHolder interface {
	EntityCommon() *Common
}

// UnknownEntity preserves an unmodeled entity's full tag run (everything
// after the `0 <TYPE>` pair) verbatim.
type UnknownEntity struct {
	Type string
	Tags []Tag
}

// EntityType implements AnyEntityData.
func (u *UnknownEntity) EntityType() string { return u.Type }

// EntityCommon returns nil for unknown entities.
func (u *UnknownEntity) EntityCommon() *Common { return nil }

// AccessCommon returns the common preamble of the payload when modeled.
func (e *Entity) AccessCommon() *Common {
	if c, ok := e.Data.(commonHolder); ok {
		return c.EntityCommon()
	}
	return nil
}

// entityCodec binds one entity type to its reader/writer.
type entityCodec struct {
	read  func(tags []Tag) (AnyEntityData, error)
	write func(w *TagWriter, data AnyEntityData, ver DxfVersion) error
}

var entityCodecs = map[string]*entityCodec{}

func registerEntity(name string, codec *entityCodec) {
	entityCodecs[name] = codec
}

// parseEntity builds one Entity from a tag run (the tags after the
// `0 <TYPE>` pair up to the next 0-code). Unmodeled types come back as
// *UnknownEntity; a modeled type with a malformed body degrades to
// *UnknownEntity as well rather than dropping the record.
func parseEntity(typeName string, tags []Tag) *Entity {
	codec, ok := entityCodecs[typeName]
	if !ok {
		return &Entity{Type: typeName, Data: &UnknownEntity{Type: typeName, Tags: cloneTags(tags)}}
	}
	data, err := codec.read(tags)
	if err != nil || data == nil {
		return &Entity{Type: typeName, Data: &UnknownEntity{Type: typeName, Tags: cloneTags(tags)}}
	}
	return &Entity{Type: typeName, Data: data}
}

// writeEntity emits one entity (without section framing).
func writeEntity(w *TagWriter, e *Entity, ver DxfVersion) error {
	if e == nil || e.Type == "" {
		return fmt.Errorf("dxf: nil or unnamed entity")
	}
	if u, ok := e.Data.(*UnknownEntity); ok {
		w.Str(0, u.Type)
		w.Pairs(u.Tags)
		return nil
	}
	codec, ok := entityCodecs[e.Type]
	if !ok {
		return fmt.Errorf("dxf: no codec for entity %q", e.Type)
	}
	if e.Data == nil {
		return fmt.Errorf("dxf: entity %q has no payload", e.Type)
	}
	w.Str(0, e.Type)
	return codec.write(w, e.Data, ver)
}

// cloneTags copies a tag slice (nil-safe).
func cloneTags(tags []Tag) []Tag {
	if len(tags) == 0 {
		return nil
	}
	out := make([]Tag, len(tags))
	copy(out, tags)
	return out
}

// parseUintHex parses a hexadecimal handle ("1A3F"), falling back to
// decimal for sloppy producers ("42").
func parseUintHex(s string) uint64 {
	s = trimSpace(s)
	if s == "" {
		return 0
	}
	if v, err := strconv.ParseUint(s, 16, 64); err == nil {
		return v
	}
	if v, err := strconv.ParseUint(s, 10, 64); err == nil {
		return v
	}
	return 0
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// cornerSlots absorbs group codes 10-13/20-23/30-33 (four 3D corner points,
// shared by SOLID/TRACE/3DFACE) into slot storage. It reports whether the
// tag belonged to the corner block.
type cornerSlots struct {
	p    [4]Point3
	seen bool
}

func (cs *cornerSlots) absorb(t Tag) bool {
	switch {
	case t.Code >= 10 && t.Code <= 13:
		cs.p[t.Code-10].X = t.Float()
		cs.seen = true
	case t.Code >= 20 && t.Code <= 23:
		cs.p[t.Code-20].Y = t.Float()
		cs.seen = true
	case t.Code >= 30 && t.Code <= 33:
		cs.p[t.Code-30].Z = t.Float()
		cs.seen = true
	default:
		return false
	}
	return true
}
