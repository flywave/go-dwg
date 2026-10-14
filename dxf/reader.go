package dxf

import "strings"

// documentFromTags assembles a Document from a flat tag stream. The parser
// is section-directed and forgiving: a malformed body inside one section
// never poisons the others, and unknown record types are preserved rather
// than dropped.
func documentFromTags(tags []Tag) (*Document, error) {
	d := &Document{}
	s := &tagStream{tags: tags}
	for {
		t, ok := s.next()
		if !ok {
			break
		}
		if t.Code != 0 {
			continue // stray pair between sections — tolerated, dropped
		}
		switch strings.ToUpper(t.Value) {
		case "SECTION":
			nameTag, ok := s.next()
			if !ok || nameTag.Code != 2 {
				continue
			}
			sectionTags := collectUntil(s, "ENDSEC")
			d.absorbSection(nameTag.Value, sectionTags)
		case "EOF":
			d.inferVersion()
			return d, nil
		}
	}
	d.inferVersion()
	return d, nil
}

// collectUntil consumes tags until a 0-code record whose value is stop
// (consumed too), or the stream ends.
func collectUntil(s *tagStream, stop string) []Tag {
	var out []Tag
	for {
		t, ok := s.next()
		if !ok {
			return out
		}
		if t.Code == 0 && strings.EqualFold(t.Value, stop) {
			return out
		}
		out = append(out, t)
	}
}

// absorbSection routes one section body to its model.
func (d *Document) absorbSection(name string, body []Tag) {
	switch strings.ToUpper(name) {
	case "HEADER":
		d.parseHeader(body)
	case "CLASSES":
		d.parseClasses(body)
	case "TABLES":
		d.parseTables(body)
	case "BLOCKS":
		d.parseBlocks(body)
	case "ENTITIES":
		d.Entities = append(d.Entities, parseEntityList(body)...)
	case "OBJECTS":
		d.parseObjects(body)
	case "THUMBNAILIMAGE":
		d.Thumbnail = append(d.Thumbnail, body...)
	default:
		d.OtherSections = append(d.OtherSections, RawSection{Name: name, Tags: cloneTags(body)})
	}
}

func (d *Document) parseHeader(body []Tag) {
	var cur *HeaderVar
	for _, t := range body {
		if t.Code == 9 {
			d.Header = append(d.Header, HeaderVar{Name: strings.TrimSpace(t.Value)})
			cur = &d.Header[len(d.Header)-1]
			continue
		}
		if cur == nil {
			continue
		}
		cur.Values = append(cur.Values, t)
	}
}

func (d *Document) parseClasses(body []Tag) {
	s := &tagStream{tags: body}
	for {
		t, ok := s.next()
		if !ok {
			return
		}
		if t.Code != 0 || !strings.EqualFold(t.Value, "CLASS") {
			continue
		}
		runTags := collectUntilZero(s)
		c := ClassEntry{}
		for _, rt := range runTags {
			switch rt.Code {
			case 1:
				c.CPPClassName = rt.Value
			case 2:
				c.AppName = rt.Value
			case 3:
				c.ItemName = rt.Value
			case 90:
				c.ProxyFlags = rt.Int()
			case 280:
				c.WasZombie = rt.Int() != 0
			case 281:
				c.IsAnEntity = rt.Int() != 0
			default:
				c.Extra = append(c.Extra, rt)
			}
		}
		d.Classes = append(d.Classes, c)
	}
}

// collectUntilZero consumes tags until the next 0-code tag (which stays in
// the stream).
func collectUntilZero(s *tagStream) []Tag {
	var out []Tag
	for {
		t, ok := s.peek()
		if !ok || t.Code == 0 {
			return out
		}
		s.next()
		out = append(out, t)
	}
}

func (d *Document) parseTables(body []Tag) {
	s := &tagStream{tags: body}
	for {
		t, ok := s.next()
		if !ok {
			return
		}
		if t.Code != 0 || !strings.EqualFold(t.Value, "TABLE") {
			continue
		}
		tbl := &Table{}
		for {
			h, ok := s.peek()
			if !ok || h.Code == 0 {
				break
			}
			s.next()
			switch h.Code {
			case 2:
				tbl.Name = h.Value
			case 5:
				tbl.Handle = parseHandle(h.Value)
			case 330:
				tbl.Owner = parseHandle(h.Value)
			case 70:
				tbl.Flags = h.Int()
			default:
				tbl.Extra = append(tbl.Extra, h)
			}
		}
		// Entries until ENDTAB.
		for {
			e, ok := s.peek()
			if !ok {
				break
			}
			if e.Code != 0 {
				s.next()
				continue
			}
			if strings.EqualFold(e.Value, "ENDTAB") {
				s.next()
				break
			}
			s.next()
			entry := &TableEntry{Type: strings.ToUpper(e.Value)}
			// Only a leading 5/330 pair is the record's handle/owner —
			// DIMSTYLE reuses group 5 (dimpost) deeper in the stream.
			headed := true
			for {
				h, ok := s.peek()
				if !ok || h.Code == 0 {
					break
				}
				s.next()
				switch {
				case headed && h.Code == 5:
					entry.Handle = parseHandle(h.Value)
				case headed && h.Code == 330:
					entry.Owner = parseHandle(h.Value)
					entry.HasOwner = true
				default:
					headed = false
					entry.Rest = append(entry.Rest, h)
				}
			}
			tbl.Entries = append(tbl.Entries, entry)
		}
		d.Tables = append(d.Tables, tbl)
	}
}

// entity-run boundary: any 0-code record the entity parsers own. INSERT
// claims ATTRIB/SEQEND; POLYLINE claims VERTEX/SEQEND.
func entityOwned(typeName, cand string) bool {
	switch typeName {
	case "INSERT":
		return cand == "ATTRIB" || cand == "SEQEND"
	case "POLYLINE":
		return cand == "VERTEX" || cand == "SEQEND"
	}
	return false
}

// parseEntityList parses a flat list of `0 <TYPE>` runs into entities,
// attaching ATTRIB/VERTEX/SEQEND children to their owner.
func parseEntityList(body []Tag) []*Entity {
	s := &tagStream{tags: body}
	var out []*Entity
	var pending *Entity // INSERT/POLYLINE collecting children
	for {
		t, ok := s.next()
		if !ok {
			return out
		}
		if t.Code != 0 {
			continue
		}
		typeName := strings.ToUpper(t.Value)
		runTags := collectUntilZero(s)
		if pending != nil {
			if typeName == "SEQEND" {
				attachSeqend(pending, parseEntity("SEQEND", runTags))
				pending = nil
				continue
			}
			if entityOwned(pending.Type, typeName) {
				attachChild(pending, parseEntity(typeName, runTags))
				continue
			}
			pending = nil
		}
		e := parseEntity(typeName, runTags)
		if typeName == "INSERT" || typeName == "POLYLINE" {
			if _, hasChildren := e.Data.(childOwner); hasChildren {
				pending = e
			}
		}
		out = append(out, e)
	}
}

// childOwner marks payloads that can carry ATTRIB/VERTEX children.
type childOwner interface {
	ownsChildren()
}

func (*Insert) ownsChildren()   {}
func (*Polyline) ownsChildren() {}

func attachChild(owner *Entity, child *Entity) {
	switch o := owner.Data.(type) {
	case *Insert:
		o.Attribs = append(o.Attribs, child)
	case *Polyline:
		o.Vertices = append(o.Vertices, child)
	}
}

func attachSeqend(owner *Entity, seq *Entity) {
	switch o := owner.Data.(type) {
	case *Insert:
		o.Seqend = seq
	case *Polyline:
		o.Seqend = seq
	}
}

func (d *Document) parseBlocks(body []Tag) {
	s := &tagStream{tags: body}
	for {
		t, ok := s.next()
		if !ok {
			return
		}
		if t.Code != 0 || !strings.EqualFold(t.Value, "BLOCK") {
			continue
		}
		// Header run: everything up to the first 0-code.
		var headerTags []Tag
		for {
			h, ok := s.peek()
			if !ok || h.Code == 0 {
				break
			}
			s.next()
			headerTags = append(headerTags, h)
		}
		b := &Block{Tags: cloneTags(headerTags)}
		// Entities until ENDBLK.
		var entTags []Tag
		for {
			e, ok := s.peek()
			if !ok {
				break
			}
			if e.Code == 0 && strings.EqualFold(e.Value, "ENDBLK") {
				s.next()
				// ENDBLK record body.
				var endTags []Tag
				for {
					h, ok := s.peek()
					if !ok || h.Code == 0 {
						break
					}
					s.next()
					endTags = append(endTags, h)
				}
				b.EndTags = cloneTags(endTags)
				break
			}
			s.next()
			if e.Code == 0 {
				entTags = append(entTags, Tag{Code: 0, Value: e.Value})
				entTags = append(entTags, collectUntilZero(s)...)
			} else {
				entTags = append(entTags, e)
			}
		}
		b.Entities = parseEntityList(entTags)
		d.Blocks = append(d.Blocks, b)
	}
}

func (d *Document) parseObjects(body []Tag) {
	s := &tagStream{tags: body}
	for {
		t, ok := s.next()
		if !ok {
			return
		}
		if t.Code != 0 {
			continue
		}
		runTags := collectUntilZero(s)
		d.Objects = append(d.Objects, &Object{Type: strings.ToUpper(t.Value), Tags: cloneTags(runTags)})
	}
}

// inferVersion sets Version from $ACADVER when the header was parsed.
func (d *Document) inferVersion() {
	if acadver, ok := d.Header.Str("$ACADVER"); ok {
		d.Version = DxfVersionFromAcadVer(acadver)
	}
}
