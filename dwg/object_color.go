package dwg

// The CMC color form the R2004+ non-entity objects carry (§2.11): a BS index
// optionally followed by a BL true-color word and an RC color byte. Every
// R2004+ record writes all three; the two name-string bits are conditional.

// ObjectColor is one CMC color.
type ObjectColor struct {
	Index     uint16
	RGB       uint32
	ColorByte uint8
	ColorName string
	BookName  string
}

// Method is the color-method octet (top byte of the true-color word).
func (c *ObjectColor) Method() uint8 { return uint8(c.RGB >> 24) }

// Payload is the 24-bit payload of the true-color word.
func (c *ObjectColor) Payload() uint32 { return c.RGB & 0x00FFFFFF }

// readObjectColor reads one CMC, taking any name strings from strings on the
// R2007+ split layout.
func readObjectColor(c *BitCursor, strings *stringReader, version Version) (ObjectColor, error) {
	index, err := c.ReadBSU()
	if err != nil {
		return ObjectColor{}, err
	}
	if !version.IsR2004Plus() {
		return ObjectColor{Index: index}, nil
	}
	rgb, err := c.ReadBLU()
	if err != nil {
		return ObjectColor{}, err
	}
	colorByte, err := c.ReadRC()
	if err != nil {
		return ObjectColor{}, err
	}
	var colorName, bookName string
	if colorByte&1 != 0 {
		colorName, err = readTVSource(c, strings, version)
		if err != nil {
			return ObjectColor{}, err
		}
	}
	if colorByte&2 != 0 {
		bookName, err = readTVSource(c, strings, version)
		if err != nil {
			return ObjectColor{}, err
		}
	}
	return ObjectColor{Index: index, RGB: rgb, ColorByte: colorByte, ColorName: colorName, BookName: bookName}, nil
}

// readTVSource reads a TV from whichever stream holds it.
func readTVSource(c *BitCursor, strings *stringReader, version Version) (string, error) {
	if strings != nil {
		return strings.readTV()
	}
	return readTV(c, version)
}
