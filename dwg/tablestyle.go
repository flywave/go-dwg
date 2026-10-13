package dwg

// TABLESTYLE custom class (AcDbTableStyle) — the table/grid style behind
// AcDbTable entities. Field order recovered from `OdDbTableStyle::dwgInFields`
// in two ODA builds (TD_DbEntities.tx:0xd44fd8 in the 2021 lnxX64_11dll ELF,
// exported ?dwgInFields@OdDbTableStyle@@ at RVA 0x3c5360 in the 25.4 PE DLL)
// plus the file-path behaviour of `OdCellStyle::dwgIn` and
// `OdCmColor::dwgInAsTrueColor`:
//
//   dwgVersion() > 0x1b (R2010+) — branch B:
//     RC, TV name, BL, BL, hard-ownership handle (handle stream, no data
//     bits), one default cell style (below), BL, BL, TV, BL count and
//     count × { BL type, cell style, BL key1, BL key2, TV name }; the 25.4
//     build routes keys 1/2/3 into slots 1/2/0 and appends the rest. The
//     2021 build reads only a BL+BS prologue per cell style here and
//     therefore *under-reads* records written by newer revisions; the full
//     cell-style layout is what closes the corpus records on the boundary.
//   dwgVersion() <= 0x1b (≤ R2007) — branch A:
//     TV name, BS, BS, BD, BD, B, B, then exactly three direction blocks
//     { BD, BS, CMC, CMC, B, 6×{ BS, B, CMC } }, closed per direction by
//     2×BL + TV when dwgVersion() > 0x19 (R2007 beta/release only).
//
//   Cell style (25.4 OdCellStyle::dwgIn full file-path list):
//     BL, BS, BL, BL, CMC, BL×5, TV, BD, BD, BL, CMC, soft-pointer handle
//     (0 bits), BD, BS, BD×6, BL grid count and grid count × { BL, BL, BL,
//     CMC, BL, handle (0 bits), BL, BD }.
//
// All handles ride in the object's handle chunk and cost no data-stream
// bits on either layout. The corpus records close at delta 0 on both
// branches (AC1018 branch A: margins 0.18/0.25/0.18; AC1027 branch B:
// default style + 3 named cell styles with 6 grid lines each). Not to be
// confused with table_style.go, which is the STYLE symbol-table entry.

// TableCellStyleHeader is the OdCellStyle version pair leading each style.
type TableCellStyleHeader struct {
	Word  int32
	Extra int16
}

// TableStyleGridLine is one grid-line record inside a cell style.
type TableStyleGridLine struct {
	IntA  int32
	IntB  int32
	IntC  int32
	Color ObjectColor
	IntD  int32
	IntE  int32
	Value float64
}

// TableStyleCell is one full cell style (default or named).
type TableStyleCell struct {
	Header    TableCellStyleHeader
	IntA      int32
	IntB      int32
	ColorA    ObjectColor
	Ints      [5]int32
	Name      string
	DoubleA   float64
	DoubleB   float64
	IntC      int32
	ColorB    ObjectColor
	DoubleC   float64
	IntD      int16
	Doubles   [6]float64
	GridCount int32
	GridLines []TableStyleGridLine
}

// TableStyleCustomCell is one named cell style plus its routing keys.
type TableStyleCustomCell struct {
	Type int32
	Cell TableStyleCell
	Key1 int32
	Key2 int32
	Name string
}

// TableStyleDirection is one branch-A direction block (title/header/data).
type TableStyleDirection struct {
	Double   float64
	Int16    int16
	ColorA   ObjectColor
	ColorB   ObjectColor
	Flag     bool
	Cells    [6]TableStyleCellEntry
	HasTail  bool
	TailA    int32
	TailB    int32
	TailName string
}

// TableStyleCellEntry is one inner-loop cell entry of a direction block.
type TableStyleCellEntry struct {
	Int16 int16
	Flag  bool
	Color ObjectColor
}

// TableStyleClass is a decoded TABLESTYLE record; the branch fields not
// used by the record's version stay zero.
type TableStyleClass struct {
	// Branch A (≤ R2007).
	Name       string
	IntA       int16
	IntB       int16
	DoubleA    float64
	DoubleB    float64
	FlagA      bool
	FlagB      bool
	Directions [3]TableStyleDirection

	// Branch B (R2010+).
	Byte        uint8
	IntA32      int32
	IntB32      int32
	DefaultCell TableStyleCell
	IntC32      int32
	IntD32      int32
	TailName    string
	CellCount   int32
	Cells       []TableStyleCustomCell
}

// decodeTableStyleClassObject decodes a TABLESTYLE straight from its raw
// payload.
func decodeTableStyleClassObject(payload []byte, bodyStart int, inlineDataEnd *uint32, version Version) (TableStyleClass, error) {
	o, err := openObjectStream(payload, bodyStart, inlineDataEnd, version)
	if err != nil {
		return TableStyleClass{}, err
	}
	var s TableStyleClass
	if version > R2007 {
		return decodeTableStyleBranchB(o, s, version)
	}
	return decodeTableStyleBranchA(o, s, version)
}

func decodeTableStyleBranchA(o *objectStream, s TableStyleClass, version Version) (TableStyleClass, error) {
	var err error
	if s.Name, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.IntA, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.IntB, err = o.data.ReadBS(); err != nil {
		return s, err
	}
	if s.DoubleA, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.DoubleB, err = o.data.ReadBD(); err != nil {
		return s, err
	}
	if s.FlagA, err = o.data.ReadB(); err != nil {
		return s, err
	}
	if s.FlagB, err = o.data.ReadB(); err != nil {
		return s, err
	}
	withTail := version > R2004 // dwgVersion() > 0x19
	for i := range s.Directions {
		d := &s.Directions[i]
		if d.Double, err = o.data.ReadBD(); err != nil {
			return s, err
		}
		if d.Int16, err = o.data.ReadBS(); err != nil {
			return s, err
		}
		if d.ColorA, err = readObjectColor(o.data, o.strings, version); err != nil {
			return s, err
		}
		if d.ColorB, err = readObjectColor(o.data, o.strings, version); err != nil {
			return s, err
		}
		if d.Flag, err = o.data.ReadB(); err != nil {
			return s, err
		}
		for k := range d.Cells {
			if d.Cells[k].Int16, err = o.data.ReadBS(); err != nil {
				return s, err
			}
			if d.Cells[k].Flag, err = o.data.ReadB(); err != nil {
				return s, err
			}
			if d.Cells[k].Color, err = readObjectColor(o.data, o.strings, version); err != nil {
				return s, err
			}
		}
		if withTail {
			d.HasTail = true
			if d.TailA, err = o.data.ReadBL(); err != nil {
				return s, err
			}
			if d.TailB, err = o.data.ReadBL(); err != nil {
				return s, err
			}
			if d.TailName, err = o.readTV(version); err != nil {
				return s, err
			}
		}
	}
	if err := o.finish("TABLESTYLE"); err != nil {
		return s, err
	}
	return s, nil
}

// readTableStyleCell reads one OdCellStyle's full file-path field list.
func readTableStyleCell(o *objectStream, cell *TableStyleCell, version Version) error {
	var err error
	if cell.Header.Word, err = o.data.ReadBL(); err != nil {
		return err
	}
	if cell.Header.Extra, err = o.data.ReadBS(); err != nil {
		return err
	}
	if cell.IntA, err = o.data.ReadBL(); err != nil {
		return err
	}
	if cell.IntB, err = o.data.ReadBL(); err != nil {
		return err
	}
	if cell.ColorA, err = readObjectColor(o.data, o.strings, version); err != nil {
		return err
	}
	for i := range cell.Ints {
		if cell.Ints[i], err = o.data.ReadBL(); err != nil {
			return err
		}
	}
	if cell.Name, err = o.readTV(version); err != nil {
		return err
	}
	if cell.DoubleA, err = o.data.ReadBD(); err != nil {
		return err
	}
	if cell.DoubleB, err = o.data.ReadBD(); err != nil {
		return err
	}
	if cell.IntC, err = o.data.ReadBL(); err != nil {
		return err
	}
	if cell.ColorB, err = readObjectColor(o.data, o.strings, version); err != nil {
		return err
	}
	// Soft-pointer handle rides in the handle stream.
	if cell.DoubleC, err = o.data.ReadBD(); err != nil {
		return err
	}
	if cell.IntD, err = o.data.ReadBS(); err != nil {
		return err
	}
	for i := range cell.Doubles {
		if cell.Doubles[i], err = o.data.ReadBD(); err != nil {
			return err
		}
	}
	if cell.GridCount, err = o.data.ReadBL(); err != nil {
		return err
	}
	if cell.GridCount < 0 || int(cell.GridCount) > maxObjectEEDIterations {
		return newError(ErrSectionMap, "TABLESTYLE grid count %d out of range", cell.GridCount)
	}
	cell.GridLines = make([]TableStyleGridLine, 0, cell.GridCount)
	for i := int32(0); i < cell.GridCount; i++ {
		var g TableStyleGridLine
		if g.IntA, err = o.data.ReadBL(); err != nil {
			return err
		}
		if g.IntB, err = o.data.ReadBL(); err != nil {
			return err
		}
		if g.IntC, err = o.data.ReadBL(); err != nil {
			return err
		}
		if g.Color, err = readObjectColor(o.data, o.strings, version); err != nil {
			return err
		}
		if g.IntD, err = o.data.ReadBL(); err != nil {
			return err
		}
		// Grid-line handle rides in the handle stream.
		if g.IntE, err = o.data.ReadBL(); err != nil {
			return err
		}
		if g.Value, err = o.data.ReadBD(); err != nil {
			return err
		}
		cell.GridLines = append(cell.GridLines, g)
	}
	return nil
}

func decodeTableStyleBranchB(o *objectStream, s TableStyleClass, version Version) (TableStyleClass, error) {
	var err error
	if s.Byte, err = o.data.ReadRC(); err != nil {
		return s, err
	}
	if s.Name, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.IntA32, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	if s.IntB32, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	// Hard-ownership handle rides in the handle stream.
	if err = readTableStyleCell(o, &s.DefaultCell, version); err != nil {
		return s, err
	}
	if s.IntC32, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	if s.IntD32, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	if s.TailName, err = o.readTV(version); err != nil {
		return s, err
	}
	if s.CellCount, err = o.data.ReadBL(); err != nil {
		return s, err
	}
	if s.CellCount < 0 || int(s.CellCount) > maxObjectEEDIterations {
		return s, newError(ErrSectionMap, "TABLESTYLE cell count %d out of range", s.CellCount)
	}
	s.Cells = make([]TableStyleCustomCell, 0, s.CellCount)
	for i := int32(0); i < s.CellCount; i++ {
		var c TableStyleCustomCell
		if c.Type, err = o.data.ReadBL(); err != nil {
			return s, err
		}
		if err = readTableStyleCell(o, &c.Cell, version); err != nil {
			return s, err
		}
		if c.Key1, err = o.data.ReadBL(); err != nil {
			return s, err
		}
		if c.Key2, err = o.data.ReadBL(); err != nil {
			return s, err
		}
		if c.Name, err = o.readTV(version); err != nil {
			return s, err
		}
		s.Cells = append(s.Cells, c)
	}
	if err := o.finish("TABLESTYLE"); err != nil {
		return s, err
	}
	return s, nil
}
