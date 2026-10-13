package dwg

// DWG LZ77 decompressor (spec §4.7).
//
// The R2004+ section pages compress their payloads with a custom LZ77
// dialect that differs from zlib in three ways:
//
//  1. 0x11 terminates the stream.
//  2. Literal lengths use a 0x00-byte run-length extension.
//  3. Offset encoding depends on opcode class.
//
// RLE-style copies (comp_bytes > comp_offset) are supported: the output
// buffer is re-read byte-by-byte as it grows.

// DecompressLimits bounds LZ77 decompression (decompression-bomb defense).
type DecompressLimits struct {
	// MaxOutputBytes is a hard ceiling on the returned length.
	MaxOutputBytes int
	// MaxBackrefLen caps any single back-reference copy length.
	MaxBackrefLen int
}

// DefaultDecompressLimits returns the conservative default profile.
func DefaultDecompressLimits() DecompressLimits {
	return DecompressLimits{MaxOutputBytes: 256 * 1024 * 1024, MaxBackrefLen: 1024 * 1024}
}

// PermissiveDecompressLimits returns the looser test profile.
func PermissiveDecompressLimits() DecompressLimits {
	return DecompressLimits{MaxOutputBytes: 4 * 1024 * 1024 * 1024, MaxBackrefLen: 64 * 1024 * 1024}
}

// Decompress decompresses a DWG LZ77 stream using the default limits.
// expectedSize is a capacity hint, clamped to the output limit.
func Decompress(input []byte, expectedSize int) ([]byte, error) {
	return DecompressWithLimits(input, expectedSize, DefaultDecompressLimits())
}

// DecompressWithLimits decompresses with caller-supplied safety limits.
func DecompressWithLimits(input []byte, expectedSize int, limits DecompressLimits) ([]byte, error) {
	r := &lz77Reader{bytes: input}
	capHint := expectedSize
	if capHint <= 0 {
		capHint = 4096
	}
	if capHint > limits.MaxOutputBytes {
		capHint = limits.MaxOutputBytes
	}
	out := make([]byte, 0, capHint)

	n, ok, err := readLiteralLengthOrPeekOpcode(r)
	if err != nil {
		return nil, err
	}
	if ok {
		if err := copyLiteral(r, &out, n, limits); err != nil {
			return nil, err
		}
	}

	for {
		opPos := r.pos
		op1, err := r.read()
		if err != nil {
			return nil, err
		}
		var compBytes, compOffset int
		var litCountOpt uint8
		// Spec §4.7 documents offsets as +0x3FFF (long) / +0 (short),
		// but AutoCAD-produced files use 1-indexed offsets (+0x4000 long,
		// +1 short). Cross-checked against the MIT-licensed ACadSharp.
		switch {
		case op1 <= 0x0F:
			return nil, newError(ErrLZ77InvalidOpcode, "reserved opcode 0x%02X at input position %d (out len %d)", op1, opPos, len(out))
		case op1 == 0x11:
			return out, nil
		case op1 == 0x10:
			x, err := readLongCompressionOffset(r)
			if err != nil {
				return nil, err
			}
			compBytes = x + 9
			off, lit, err := readTwoByteOffset(r)
			if err != nil {
				return nil, err
			}
			compOffset = off + 0x4000
			litCountOpt = lit
		case op1 >= 0x12 && op1 <= 0x1F:
			cb, err := readCompressedBytesExtended(op1, 0x07, r)
			if err != nil {
				return nil, err
			}
			compBytes = cb
			off := int(op1&0x08) << 11
			extra, lit, err := readTwoByteOffset(r)
			if err != nil {
				return nil, err
			}
			off |= extra
			compOffset = off + 0x4000
			litCountOpt = lit
		case op1 == 0x20:
			x, err := readLongCompressionOffset(r)
			if err != nil {
				return nil, err
			}
			compBytes = x + 0x21
			off, lit, err := readTwoByteOffset(r)
			if err != nil {
				return nil, err
			}
			compOffset = off + 1
			litCountOpt = lit
		case op1 >= 0x21 && op1 <= 0x3F:
			cb, err := readCompressedBytesExtended(op1, 0x1F, r)
			if err != nil {
				return nil, err
			}
			compBytes = cb
			off, lit, err := readTwoByteOffset(r)
			if err != nil {
				return nil, err
			}
			compOffset = off + 1
			litCountOpt = lit
		default: // 0x40..=0xFF
			compBytes = int((op1&0xF0)>>4) - 1
			op2, err := r.read()
			if err != nil {
				return nil, err
			}
			compOffset = (int(op2)<<2 | int((op1&0x0C)>>2)) + 1
			litCountOpt = op1 & 0x03
		}

		litCount := int(litCountOpt)
		if litCountOpt == 0 {
			v, ok, err := readLiteralLengthOrPeekOpcode(r)
			if err != nil {
				return nil, err
			}
			if ok {
				litCount = v
			} else {
				litCount = 0
			}
		}

		if compOffset == 0 {
			return nil, ErrLZ77InvalidOffset
		}
		if compBytes > limits.MaxBackrefLen {
			return nil, newError(ErrLZ77BackrefTooLong, "back-reference length %d exceeds limit %d", compBytes, limits.MaxBackrefLen)
		}
		if len(out)+compBytes > limits.MaxOutputBytes {
			return nil, newError(ErrLZ77OutputLimitExceeded, "output would exceed limit %d", limits.MaxOutputBytes)
		}
		if compOffset > len(out) {
			return nil, ErrLZ77InvalidOffset
		}
		start := len(out) - compOffset
		for i := 0; i < compBytes; i++ {
			out = append(out, out[start+i])
		}
		if err := copyLiteral(r, &out, litCount, limits); err != nil {
			return nil, err
		}
	}
}

type lz77Reader struct {
	bytes []byte
	pos   int
}

func (r *lz77Reader) read() (byte, error) {
	if r.pos >= len(r.bytes) {
		return 0, ErrLZ77Truncated
	}
	b := r.bytes[r.pos]
	r.pos++
	return b, nil
}

func (r *lz77Reader) rewindOne() { r.pos-- }

// readCompressedBytesExtended reads an extended compressed-byte count for
// opcode classes 0x10-0x1F and 0x20-0x3F: if (op1 & validBits) is
// non-zero use it directly, otherwise accumulate 0xFF per 0x00 byte and
// terminate on the first non-zero byte. Always add 2.
func readCompressedBytesExtended(op1, validBits uint8, r *lz77Reader) (int, error) {
	cb := int(op1 & validBits)
	if cb == 0 {
		for {
			b, err := r.read()
			if err != nil {
				return 0, err
			}
			if b == 0 {
				cb += 0xFF
			} else {
				cb += int(b) + int(validBits)
				break
			}
		}
	}
	return cb + 2, nil
}

// readLiteralLengthOrPeekOpcode reads a literal-length byte. Returns
// (n, true) when a valid length was consumed, or (0, false) when the
// peeked byte was an opcode (high nibble set) and was rolled back.
func readLiteralLengthOrPeekOpcode(r *lz77Reader) (int, bool, error) {
	b, err := r.read()
	if err != nil {
		return 0, false, err
	}
	if b == 0x00 {
		total := 0x0F
		for {
			bb, err := r.read()
			if err != nil {
				return 0, false, err
			}
			if bb == 0x00 {
				total += 0xFF
			} else {
				total += int(bb)
				return total + 3, true, nil
			}
		}
	}
	if b&0xF0 != 0 {
		r.rewindOne()
		return 0, false, nil
	}
	return int(b) + 3, true, nil
}

func copyLiteral(r *lz77Reader, out *[]byte, n int, limits DecompressLimits) error {
	if len(*out)+n > limits.MaxOutputBytes {
		return newError(ErrLZ77OutputLimitExceeded, "output would exceed limit %d", limits.MaxOutputBytes)
	}
	for i := 0; i < n; i++ {
		b, err := r.read()
		if err != nil {
			return err
		}
		*out = append(*out, b)
	}
	return nil
}

// readTwoByteOffset reads the two-byte offset form:
// offset = (first >> 2) | (second << 6); litCount = first & 0x03.
func readTwoByteOffset(r *lz77Reader) (int, uint8, error) {
	first, err := r.read()
	if err != nil {
		return 0, 0, err
	}
	second, err := r.read()
	if err != nil {
		return 0, 0, err
	}
	offset := int(first>>2) | int(second)<<6
	return offset, first & 0x03, nil
}

// readLongCompressionOffset reads the long compression offset form.
func readLongCompressionOffset(r *lz77Reader) (int, error) {
	b, err := r.read()
	if err != nil {
		return 0, err
	}
	if b != 0x00 {
		return int(b), nil
	}
	total := 0xFF
	for {
		bb, err := r.read()
		if err != nil {
			return 0, err
		}
		if bb == 0x00 {
			total += 0xFF
		} else {
			return total + int(bb), nil
		}
	}
}
