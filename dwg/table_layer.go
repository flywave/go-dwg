package dwg

// LAYER table entry (spec §20.4.54) — named drawing layer with display flags,
// color, linetype, lineweight and plot style.

// Layer is a decoded LAYER table entry.
type LayerEntry struct {
	Header     TableEntryHeader
	Flags      int16
	PlotFlag   bool
	Lineweight int16
	ColorIndex int16
}

// IsFrozen reports bit 0x01 of the values word.
func (l *LayerEntry) IsFrozen() bool { return l.Flags&layerValueFrozen != 0 }

// IsLocked reports bit 0x08 of the values word (DWG, not DXF).
func (l *LayerEntry) IsLocked() bool { return l.Flags&layerValueLocked != 0 }

// IsFrozenInNewViewports reports bit 0x04 of the values word.
func (l *LayerEntry) IsFrozenInNewViewports() bool { return l.Flags&layerValueFrozenInNew != 0 }

// IsPlottable reports the plot flag.
func (l *LayerEntry) IsPlottable() bool { return l.PlotFlag }

func decodeLayerInline(c *BitCursor, version Version) (LayerEntry, error) {
	header, err := readTableEntryHeader(c, version)
	if err != nil {
		return LayerEntry{}, err
	}
	var flags int16
	if version == R14 {
		frozen, err := c.ReadB()
		if err != nil {
			return LayerEntry{}, err
		}
		on, err := c.ReadB()
		if err != nil {
			return LayerEntry{}, err
		}
		frozenNew, err := c.ReadB()
		if err != nil {
			return LayerEntry{}, err
		}
		locked, err := c.ReadB()
		if err != nil {
			return LayerEntry{}, err
		}
		if frozen {
			flags |= layerValueFrozen
		}
		if on {
			flags |= layerValueOff
		}
		if frozenNew {
			flags |= layerValueFrozenInNew
		}
		if locked {
			flags |= layerValueLocked
		}
	} else {
		flags, err = c.ReadBS()
		if err != nil {
			return LayerEntry{}, err
		}
	}
	var colorIndex int16
	if version.IsR2004Plus() {
		idx, _, _, err := readCMCInline(c)
		if err != nil {
			return LayerEntry{}, err
		}
		colorIndex = idx
	} else {
		colorIndex, err = c.ReadBS()
		if err != nil {
			return LayerEntry{}, err
		}
	}
	return LayerEntry{
		Header:     header,
		Flags:      flags,
		PlotFlag:   flags&layerValuePlot != 0,
		Lineweight: (flags >> layerValueLineweightShift) & 0x1F,
		ColorIndex: colorIndex,
	}, nil
}

func decodeLayerModern(payload []byte, bodyStart int, version Version) (LayerEntry, error) {
	split, err := openTableEntry(payload, bodyStart, version)
	if err != nil {
		return LayerEntry{}, err
	}
	flag64, xrefIdx, xdep, err := readEntryFlags(split.data)
	if err != nil {
		return LayerEntry{}, err
	}
	values, err := split.data.ReadBS()
	if err != nil {
		return LayerEntry{}, err
	}
	_, colorRGB, _, err := readCMCFull(split.data)
	if err != nil {
		return LayerEntry{}, err
	}
	if err := split.finish("LAYER"); err != nil {
		return LayerEntry{}, err
	}
	name, err := split.strings.readTV()
	if err != nil {
		return LayerEntry{}, err
	}
	return LayerEntry{
		Header:     TableEntryHeader{Name: name, IsXrefDependent: xdep, XrefIndexPlus1: xrefIdx, IsXrefResolved: flag64},
		Flags:      values,
		PlotFlag:   values&layerValuePlot != 0,
		Lineweight: (values >> layerValueLineweightShift) & 0x1F,
		ColorIndex: aciFromCMC(colorRGB),
	}, nil
}
