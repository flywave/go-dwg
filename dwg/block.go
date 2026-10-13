package dwg

import "unicode/utf16"

// BLOCK / ENDBLK / SEQEND entities.

// Block is a decoded BLOCK entity — its only field is the block name.
type Block struct {
	Name string
}

// EndBlk is a decoded ENDBLK entity (no payload).
type EndBlk struct{}

// SeqEnd is a decoded SEQEND entity (no payload).
type SeqEnd struct{}

// decodeBlockField decodes a BLOCK, taking its one TV from whichever stream
// holds it.
func decodeBlockField(c *BitCursor, version Version, strings *stringReader) (Block, error) {
	if strings != nil {
		name, err := strings.readTV()
		if err != nil {
			return Block{}, err
		}
		return Block{Name: name}, nil
	}
	length, err := c.ReadBSU()
	if err != nil {
		return Block{}, err
	}
	if length == 0 {
		return Block{}, nil
	}
	if version.IsR2007Plus() {
		units := make([]uint16, 0, length)
		for i := 0; i < int(length); i++ {
			lo, err := c.ReadRC()
			if err != nil {
				return Block{}, err
			}
			hi, err := c.ReadRC()
			if err != nil {
				return Block{}, err
			}
			units = append(units, uint16(hi)<<8|uint16(lo))
		}
		if len(units) > 0 && units[len(units)-1] == 0 {
			units = units[:len(units)-1]
		}
		return Block{Name: string(utf16.Decode(units))}, nil
	}
	bs := make([]byte, 0, length)
	for i := 0; i < int(length); i++ {
		b, err := c.ReadRC()
		if err != nil {
			return Block{}, err
		}
		bs = append(bs, b)
	}
	if len(bs) > 0 && bs[len(bs)-1] == 0 {
		bs = bs[:len(bs)-1]
	}
	return Block{Name: string(bs)}, nil
}
