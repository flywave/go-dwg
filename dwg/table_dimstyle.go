package dwg

// DIMSTYLE table entry (spec §19.5.5 / §20.4.68) — dimension style. This port
// surfaces the 15 most-consulted dimension variables; the decoder consumes the
// whole body so the data-stream boundary can be verified.

// DimStyleEntry is a DIMSTYLE table entry (partial: 15 surfaced variables).
type DimStyleEntry struct {
	Header    TableEntryHeader
	Dimscale  float64
	Dimasz    float64
	Dimexo    float64
	Dimexe    float64
	Dimtxt    float64
	Dimcen    float64
	Dimtfac   float64
	Dimlfac   float64
	Dimtih    bool
	Dimtoh    bool
	Dimtad    uint8
	Dimtolj   uint8
	Dimaltf   float64
	Dimaltrnd float64
	Dimupt    bool
}

func decodeDimStyleInline(c *BitCursor, version Version) (DimStyleEntry, error) {
	header, err := readTableEntryHeader(c, version)
	if err != nil {
		return DimStyleEntry{}, err
	}
	if _, err := readTV(c, version); err != nil { // dimpost
		return DimStyleEntry{}, err
	}
	if _, err := readTV(c, version); err != nil { // dimapost
		return DimStyleEntry{}, err
	}
	var d DimStyleEntry
	d.Header = header
	readBD := func(dst *float64) error { v, err := c.ReadBD(); *dst = v; return err }
	if err := readBD(&d.Dimscale); err != nil {
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimasz); err != nil {
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimexo); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipBD(c); err != nil { // dimdli
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimexe); err != nil {
		return DimStyleEntry{}, err
	}
	for i := 0; i < 3; i++ { // dimrnd, dimdle, dimtp, dimtm? (3 then dimtm)
		if err := skipBD(c); err != nil {
			return DimStyleEntry{}, err
		}
	}
	if err := skipBD(c); err != nil { // dimtm
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // dimtol
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // dimlim
		return DimStyleEntry{}, err
	}
	if d.Dimtih, err = c.ReadB(); err != nil {
		return DimStyleEntry{}, err
	}
	if d.Dimtoh, err = c.ReadB(); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // dimse1
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // dimse2
		return DimStyleEntry{}, err
	}
	dimtad, err := c.ReadBS()
	if err != nil {
		return DimStyleEntry{}, err
	}
	d.Dimtad = uint8(dimtad)
	if err := skipBS(c); err != nil { // dimzin
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimazin
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimtxt); err != nil {
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimcen); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipBD(c); err != nil { // dimtsz
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimaltf); err != nil {
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimlfac); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipBD(c); err != nil { // dimtvp
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimtfac); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipBD(c); err != nil { // dimgap
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimaltrnd); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // dimalt
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimaltd
		return DimStyleEntry{}, err
	}
	for i := 0; i < 4; i++ { // dimtofl, dimsah, dimtix, dimsoxd
		if err := skipB(c); err != nil {
			return DimStyleEntry{}, err
		}
	}
	for i := 0; i < 3; i++ { // dimclrd, dimclre, dimclrt
		if _, err := readCMCInlineVersion(c, version); err != nil {
			return DimStyleEntry{}, err
		}
	}
	for i := 0; i < 11; i++ { // dimadec..dimjust
		if err := skipBS(c); err != nil {
			return DimStyleEntry{}, err
		}
	}
	if err := skipB(c); err != nil { // dimsd1
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // dimsd2
		return DimStyleEntry{}, err
	}
	dimtolj, err := c.ReadBS()
	if err != nil {
		return DimStyleEntry{}, err
	}
	d.Dimtolj = uint8(dimtolj)
	if err := skipBS(c); err != nil { // dimtzin
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimaltz
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimalttz
		return DimStyleEntry{}, err
	}
	if d.Dimupt, err = c.ReadB(); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimfit (dimatfit)
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimlwd
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimlwe
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // unknown trailing bit
		return DimStyleEntry{}, err
	}
	return d, nil
}

func decodeDimStyleModern(payload []byte, bodyStart int, version Version) (DimStyleEntry, error) {
	split, err := openTableEntry(payload, bodyStart, version)
	if err != nil {
		return DimStyleEntry{}, err
	}
	flag64, xrefIdx, xdep, err := readEntryFlags(split.data)
	if err != nil {
		return DimStyleEntry{}, err
	}
	c := split.data
	var d DimStyleEntry
	readBD := func(dst *float64) error { v, err := c.ReadBD(); *dst = v; return err }
	if err := readBD(&d.Dimscale); err != nil {
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimasz); err != nil {
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimexo); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipBD(c); err != nil { // dimdli
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimexe); err != nil {
		return DimStyleEntry{}, err
	}
	for i := 0; i < 3; i++ { // dimrnd, dimdle, dimtp
		if err := skipBD(c); err != nil {
			return DimStyleEntry{}, err
		}
	}
	if err := skipBD(c); err != nil { // dimtm
		return DimStyleEntry{}, err
	}
	if err := skipBD(c); err != nil { // dimfxl
		return DimStyleEntry{}, err
	}
	if err := skipBD(c); err != nil { // dimjogang
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimtfill
		return DimStyleEntry{}, err
	}
	if _, _, _, err := readCMCFull(c); err != nil { // dimtfillclr
		return DimStyleEntry{}, err
	}
	for i := 0; i < 6; i++ { // dimtol, dimlim, dimtih, dimtoh, dimse1, dimse2
		if v, err := c.ReadB(); err != nil {
			return DimStyleEntry{}, err
		} else if i == 2 {
			d.Dimtih = v
		} else if i == 3 {
			d.Dimtoh = v
		}
	}
	dimtad, err := c.ReadBS()
	if err != nil {
		return DimStyleEntry{}, err
	}
	d.Dimtad = uint8(dimtad)
	for i := 0; i < 3; i++ { // dimzin, dimazin, dimarcsym
		if err := skipBS(c); err != nil {
			return DimStyleEntry{}, err
		}
	}
	if err := readBD(&d.Dimtxt); err != nil {
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimcen); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipBD(c); err != nil { // dimtsz
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimaltf); err != nil {
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimlfac); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipBD(c); err != nil { // dimtvp
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimtfac); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipBD(c); err != nil { // dimgap
		return DimStyleEntry{}, err
	}
	if err := readBD(&d.Dimaltrnd); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // dimalt
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimaltd
		return DimStyleEntry{}, err
	}
	for i := 0; i < 4; i++ { // dimtofl, dimsah, dimtix, dimsoxd
		if err := skipB(c); err != nil {
			return DimStyleEntry{}, err
		}
	}
	for i := 0; i < 3; i++ { // dimclrd, dimclre, dimclrt
		if _, _, _, err := readCMCFull(c); err != nil {
			return DimStyleEntry{}, err
		}
	}
	for i := 0; i < 11; i++ { // dimadec..dimjust
		if err := skipBS(c); err != nil {
			return DimStyleEntry{}, err
		}
	}
	if err := skipB(c); err != nil { // dimsd1
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // dimsd2
		return DimStyleEntry{}, err
	}
	dimtolj, err := c.ReadBS()
	if err != nil {
		return DimStyleEntry{}, err
	}
	d.Dimtolj = uint8(dimtolj)
	for i := 0; i < 3; i++ { // dimtzin, dimaltz, dimalttz
		if err := skipBS(c); err != nil {
			return DimStyleEntry{}, err
		}
	}
	if d.Dimupt, err = c.ReadB(); err != nil {
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimatfit
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // dimfxlon
		return DimStyleEntry{}, err
	}
	if version.IsR2010Plus() {
		if err := skipB(c); err != nil { // dimtxtdirection
			return DimStyleEntry{}, err
		}
		if err := skipBD(c); err != nil { // dimaltmzf
			return DimStyleEntry{}, err
		}
		if err := skipBD(c); err != nil { // dimmzf
			return DimStyleEntry{}, err
		}
	}
	if err := skipBS(c); err != nil { // dimlwd
		return DimStyleEntry{}, err
	}
	if err := skipBS(c); err != nil { // dimlwe
		return DimStyleEntry{}, err
	}
	if err := skipB(c); err != nil { // unknown trailing bit
		return DimStyleEntry{}, err
	}
	if err := split.finish("DIMSTYLE"); err != nil {
		return DimStyleEntry{}, err
	}
	if d.Header.Name, err = split.strings.readTV(); err != nil {
		return DimStyleEntry{}, err
	}
	if _, err := split.strings.readTV(); err != nil { // dimpost
		return DimStyleEntry{}, err
	}
	if _, err := split.strings.readTV(); err != nil { // dimapost
		return DimStyleEntry{}, err
	}
	if version.IsR2010Plus() {
		if _, err := split.strings.readTV(); err != nil { // dimaltmzs
			return DimStyleEntry{}, err
		}
		if _, err := split.strings.readTV(); err != nil { // dimmzs
			return DimStyleEntry{}, err
		}
	}
	d.Header.IsXrefDependent = xdep
	d.Header.XrefIndexPlus1 = xrefIdx
	d.Header.IsXrefResolved = flag64
	return d, nil
}

func skipBD(c *BitCursor) error { _, err := c.ReadBD(); return err }
func skipBS(c *BitCursor) error { _, err := c.ReadBS(); return err }
func skipB(c *BitCursor) error  { _, err := c.ReadB(); return err }
