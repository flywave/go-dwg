package dwg

// R2007 (AC1021) LZ decompression — ODA spec §5.10.
//
// R2007 uses a different LZ77 variant from the one R2004 and R2010+ use
// (lz77.go, spec §4.7). The stream interleaves literal runs (copied out of
// the source buffer with a byte permutation) with decompressed chunks
// (back-references into the output).

type rsBlock struct {
	size int
	idx  int
}

// copyBlocks is the §5.10.1 literal copy table, indexed by byte count.
// Slot 0 is unused; slot 1 is the base case. Every other slot lists the
// sub-blocks that make up that count's copy function, in output order.
var copyBlocks [33][]rsBlock = [33][]rsBlock{
	{},
	{{1, 0}},
	{{1, 1}, {1, 0}},
	{{1, 2}, {1, 1}, {1, 0}},
	{{1, 0}, {1, 1}, {1, 2}, {1, 3}},
	{{1, 4}, {4, 0}},
	{{1, 5}, {4, 1}, {1, 0}},
	{{2, 5}, {4, 1}, {1, 0}},
	{{4, 0}, {4, 4}},
	{{1, 8}, {8, 0}},
	{{1, 9}, {8, 1}, {1, 0}},
	{{2, 9}, {8, 1}, {1, 0}},
	{{4, 8}, {8, 0}},
	{{1, 12}, {4, 8}, {8, 0}},
	{{1, 13}, {4, 9}, {8, 1}, {1, 0}},
	{{2, 13}, {4, 9}, {8, 1}, {1, 0}},
	{{8, 8}, {8, 0}},
	{{8, 9}, {1, 8}, {8, 0}},
	{{1, 17}, {16, 1}, {1, 0}},
	{{3, 16}, {16, 0}},
	{{4, 16}, {16, 0}},
	{{1, 20}, {4, 16}, {16, 0}},
	{{2, 20}, {4, 16}, {16, 0}},
	{{3, 20}, {4, 16}, {16, 0}},
	{{8, 16}, {16, 0}},
	{{8, 17}, {1, 16}, {16, 0}},
	{{1, 25}, {8, 17}, {1, 16}, {16, 0}},
	{{2, 25}, {8, 17}, {1, 16}, {16, 0}},
	{{4, 24}, {8, 16}, {16, 0}},
	{{1, 28}, {4, 24}, {8, 16}, {16, 0}},
	{{2, 28}, {4, 24}, {8, 16}, {16, 0}},
	{{1, 30}, {4, 26}, {8, 18}, {16, 2}, {2, 0}},
	{{16, 16}, {16, 0}},
}

// emitPermuted appends count bytes of src starting at base to out in the
// order the §5.10.1 table prescribes.
func emitPermuted(src []byte, base, count int, out *[]byte) error {
	if base+count > len(src) {
		return ErrLZ77Truncated
	}
	if count == 1 {
		*out = append(*out, src[base])
		return nil
	}
	for _, b := range copyBlocks[count] {
		if err := emitPermuted(src, base+b.idx, b.size, out); err != nil {
			return err
		}
	}
	return nil
}

func r21CopyLiteral(src []byte, pos, count int, out *[]byte) (int, error) {
	for count >= 32 {
		if err := emitPermuted(src, pos, 32, out); err != nil {
			return pos, err
		}
		pos += 32
		count -= 32
	}
	if count > 0 {
		if err := emitPermuted(src, pos, count, out); err != nil {
			return pos, err
		}
		pos += count
	}
	return pos, nil
}

func r21ByteAt(src []byte, pos int) (byte, error) {
	if pos < 0 || pos >= len(src) {
		return 0, ErrLZ77Truncated
	}
	return src[pos], nil
}

// r21ReadInstructions implements the spec's ReadInstructions (§5.10.2).
// Returns (newPos, newOpcode, offset, length).
func r21ReadInstructions(src []byte, pos int, opcode byte) (int, byte, uint64, uint64, error) {
	var length, offset uint64
	var b byte
	var err error
	switch opcode >> 4 {
	case 0:
		length = uint64(opcode&0x0F) + 0x13
		b, err = r21ByteAt(src, pos)
		if err != nil {
			return pos, opcode, 0, 0, err
		}
		lo := uint64(b)
		pos++
		opcode, err = r21ByteAt(src, pos)
		if err != nil {
			return pos, opcode, 0, 0, err
		}
		pos++
		length += uint64((opcode >> 3) & 0x10)
		offset = (uint64(opcode&0x78) << 5) + 1 + lo
	case 1:
		length = uint64(opcode&0x0F) + 3
		b, err = r21ByteAt(src, pos)
		if err != nil {
			return pos, opcode, 0, 0, err
		}
		lo := uint64(b)
		pos++
		opcode, err = r21ByteAt(src, pos)
		if err != nil {
			return pos, opcode, 0, 0, err
		}
		pos++
		offset = (uint64(opcode&0xF8) << 5) + 1 + lo
	case 2:
		blo, err := r21ByteAt(src, pos)
		if err != nil {
			return pos, opcode, 0, 0, err
		}
		pos++
		bhi, err := r21ByteAt(src, pos)
		if err != nil {
			return pos, opcode, 0, 0, err
		}
		pos++
		lo := uint64(blo)
		hi := uint64(bhi)
		off := ((hi << 8) & 0xFF00) | lo
		length = uint64(opcode & 0x07)
		if opcode&0x08 == 0 {
			opcode, err = r21ByteAt(src, pos)
			if err != nil {
				return pos, opcode, 0, 0, err
			}
			pos++
			length += uint64(opcode & 0xF8)
		} else {
			off++
			b, err = r21ByteAt(src, pos)
			if err != nil {
				return pos, opcode, 0, 0, err
			}
			length += uint64(b) << 3
			pos++
			opcode, err = r21ByteAt(src, pos)
			if err != nil {
				return pos, opcode, 0, 0, err
			}
			pos++
			length += (uint64(opcode&0xF8) << 8) + 0x100
		}
		offset = off
	default:
		length = uint64(opcode >> 4)
		lo := uint64(opcode & 0x0F)
		opcode, err = r21ByteAt(src, pos)
		if err != nil {
			return pos, opcode, 0, 0, err
		}
		pos++
		offset = (uint64(opcode&0xF8) << 1) + lo + 1
	}
	return pos, opcode, offset, length, nil
}

// r21LiteralLength implements ReadLiteralLength (§5.10.1): opcode + 8,
// with 0x17 escaping to one extra byte and then 16-bit words.
func r21LiteralLength(src []byte, pos int, opcode byte) (int, uint64, error) {
	length := uint64(opcode) + 8
	if length == 0x17 {
		n, err := r21ByteAt(src, pos)
		if err != nil {
			return pos, 0, err
		}
		pos++
		length += uint64(n)
		if n == 0xFF {
			for {
				lo, err := r21ByteAt(src, pos)
				if err != nil {
					return pos, 0, err
				}
				hi, err := r21ByteAt(src, pos+1)
				if err != nil {
					return pos, 0, err
				}
				pos += 2
				word := (uint64(hi) << 8) | uint64(lo)
				length += word
				if word != 0xFFFF {
					break
				}
			}
		}
	}
	return pos, length, nil
}

// DecompressR2007 decompresses an R2007 (§5.10) LZ stream. expected is the
// recorded decompressed size; the decoder stops once it has produced that
// many bytes. The effective cap is the smaller of expected and
// limits.MaxOutputBytes.
func DecompressR2007(src []byte, expected int, limits DecompressLimits) ([]byte, error) {
	if expected > limits.MaxOutputBytes {
		return nil, newError(ErrLZ77OutputLimitExceeded, "output would exceed limit %d", limits.MaxOutputBytes)
	}
	if len(src) == 0 {
		return []byte{}, nil
	}
	out := make([]byte, 0, expected)
	pos := 0
	opcode, err := r21ByteAt(src, pos)
	if err != nil {
		return nil, err
	}
	pos++
	var length uint64
	var pendingOpcode byte
	hasPending := false
	if opcode>>4 == 2 {
		pos += 2
		b, err := r21ByteAt(src, pos)
		if err != nil {
			return nil, err
		}
		length = uint64(b & 0x07)
		pos++
	} else {
		length = 0
		pendingOpcode = opcode
		hasPending = true
	}

	for {
		if length == 0 {
			var op byte
			if hasPending {
				op = pendingOpcode
				hasPending = false
			} else {
				var err error
				op, err = r21ByteAt(src, pos)
				if err != nil {
					return nil, err
				}
				pos++
			}
			next, l, err := r21LiteralLength(src, pos, op)
			if err != nil {
				return nil, err
			}
			pos = next
			length = l
		}
		hasPending = false
		want := int(length)
		if len(out)+want > expected {
			return nil, newError(ErrLZ77OutputLimitExceeded, "output would exceed expected %d", expected)
		}
		pos, err = r21CopyLiteral(src, pos, want, &out)
		if err != nil {
			return nil, err
		}
		if len(out) >= expected || pos >= len(src) {
			break
		}
		opcode, err = r21ByteAt(src, pos)
		if err != nil {
			return nil, err
		}
		pos++
		for {
			var nextOpcode byte
			var offset, len64 uint64
			var ierr error
			pos, nextOpcode, offset, len64, ierr = r21ReadInstructions(src, pos, opcode)
			if ierr != nil {
				return nil, ierr
			}
			opcode = nextOpcode
			copyLen := int(len64)
			if copyLen > limits.MaxBackrefLen {
				return nil, newError(ErrLZ77BackrefTooLong, "back-reference length %d exceeds limit %d", copyLen, limits.MaxBackrefLen)
			}
			back := int(offset)
			if back > len(out) {
				return nil, ErrLZ77InvalidOffset
			}
			start := len(out) - back
			if len(out)+copyLen > expected {
				return nil, newError(ErrLZ77OutputLimitExceeded, "output would exceed expected %d", expected)
			}
			for i := 0; i < copyLen; i++ {
				out = append(out, out[start+i])
			}
			length = uint64(opcode & 7)
			if length != 0 || pos >= len(src) {
				break
			}
			opcode, err = r21ByteAt(src, pos)
			if err != nil {
				return nil, err
			}
			pos++
			if opcode>>4 == 0 {
				pendingOpcode = opcode
				hasPending = true
				break
			}
			if opcode>>4 == 15 {
				opcode &= 15
			}
		}
		if len(out) >= expected || pos >= len(src) {
			break
		}
	}
	return out, nil
}
