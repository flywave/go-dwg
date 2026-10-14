package dxf

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"strings"
)

// AutoCAD binary DXF: the same (group code, value) stream with the sentinel
// string "AutoCAD Binary DXF\r\n\x1a\x00" up front, group codes as
// little-endian uint16, and value encodings selected by code class. Group
// 310-319 binary chunks are surfaced as uppercase-hex tag values so the
// ASCII and binary paths share one tag model.

// binarySentinel is the 22-byte file prologue of a binary DXF.
var binarySentinel = []byte("AutoCAD Binary DXF\r\n\x1a\x00")

func isBinaryDxf(data []byte) bool {
	return bytes.HasPrefix(data, binarySentinel)
}

type binValueType uint8

const (
	binStr binValueType = iota
	binDouble
	binInt16
	binInt32
	binInt64
	binByte
	binChunk // 1-byte length + raw bytes (hex string in the tag model)
)

func binaryValueType(code int) binValueType {
	switch {
	case code >= 0 && code <= 9:
		return binStr
	case code >= 10 && code <= 59:
		return binDouble
	case code >= 60 && code <= 79:
		return binInt16
	case code >= 90 && code <= 99:
		return binInt32
	case code == 100, code == 102, code == 105:
		return binStr
	case code >= 110 && code <= 149:
		return binDouble
	case code >= 160 && code <= 169:
		return binInt64
	case code >= 170 && code <= 179:
		return binInt16
	case code >= 210 && code <= 239:
		return binDouble
	case code >= 270 && code <= 289:
		return binInt16
	case code >= 290 && code <= 299:
		return binByte
	case code >= 300 && code <= 309:
		return binStr
	case code >= 310 && code <= 319:
		return binChunk
	case code >= 320 && code <= 369:
		return binStr
	case code >= 370 && code <= 389:
		return binInt16
	case code >= 390 && code <= 399:
		return binInt16
	case code >= 400 && code <= 409:
		return binInt16
	case code >= 410 && code <= 419:
		return binStr
	case code >= 420 && code <= 429:
		return binInt32
	case code >= 430 && code <= 439:
		return binStr
	case code >= 440 && code <= 449:
		return binInt32
	case code >= 450 && code <= 459:
		return binInt32
	case code >= 460 && code <= 469:
		return binDouble
	case code >= 470 && code <= 479:
		return binStr
	case code == 480, code == 481:
		return binStr
	case code == 999:
		return binStr
	case code >= 1000 && code <= 1009:
		return binStr
	case code >= 1010 && code <= 1059:
		return binDouble
	case code >= 1060 && code <= 1070:
		return binInt16
	case code == 1071:
		return binInt32
	}
	return binStr
}

// parseBinaryTags decodes a complete binary DXF payload (sentinel included).
func parseBinaryTags(data []byte) ([]Tag, error) {
	if !isBinaryDxf(data) {
		return nil, &parseError{"dxf: not a binary DXF stream"}
	}
	r := bytes.NewReader(data[len(binarySentinel):])
	var tags []Tag
	for {
		var code uint16
		if err := binary.Read(r, binary.LittleEndian, &code); err != nil {
			if err == io.EOF {
				return tags, nil
			}
			return tags, err
		}
		switch binaryValueType(int(code)) {
		case binStr:
			s, err := readBinString(r)
			if err != nil {
				return tags, err
			}
			tags = append(tags, Tag{Code: int(code), Value: s})
		case binChunk:
			var n uint8
			if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
				return tags, err
			}
			raw := make([]byte, n)
			if _, err := io.ReadFull(r, raw); err != nil {
				return tags, err
			}
			tags = append(tags, Tag{Code: int(code), Value: strings.ToUpper(hexEncode(raw))})
		case binDouble:
			var v float64
			if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
				return tags, err
			}
			tags = append(tags, Tag{Code: int(code), Value: FormatFloat(v)})
		case binInt16:
			var v int16
			if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
				return tags, err
			}
			tags = append(tags, Tag{Code: int(code), Value: itoa(int64(v))})
		case binInt32:
			var v int32
			if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
				return tags, err
			}
			tags = append(tags, Tag{Code: int(code), Value: itoa(int64(v))})
		case binInt64:
			var v int64
			if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
				return tags, err
			}
			tags = append(tags, Tag{Code: int(code), Value: itoa(v)})
		case binByte:
			var v uint8
			if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
				return tags, err
			}
			tags = append(tags, Tag{Code: int(code), Value: itoa(int64(v))})
		}
	}
}

// readBinString reads a NUL-terminated string.
func readBinString(r io.ByteReader) (string, error) {
	var sb strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			return sb.String(), err
		}
		if b == 0 {
			return sb.String(), nil
		}
		sb.WriteByte(b)
	}
}

// writeBinary emits the document in AutoCAD binary DXF form: canonical tag
// stream, binary-encoded per the code-class table.
func (d *Document) writeBinary(w io.Writer) error {
	data, err := encodeBinaryDoc(d)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// encodeBinaryDoc re-encodes a document's tag stream into binary form.
func encodeBinaryDoc(d *Document) ([]byte, error) {
	var ascii bytes.Buffer
	if err := d.SaveTo(&ascii); err != nil {
		return nil, err
	}
	tags, err := ParseTags(bytes.NewReader(ascii.Bytes()))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.Write(binarySentinel)
	for _, t := range tags {
		if err := writeBinaryTag(&out, t); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

func writeBinaryTag(w io.Writer, t Tag) error {
	var hdr [2]byte
	binary.LittleEndian.PutUint16(hdr[:], uint16(t.Code))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	switch binaryValueType(t.Code) {
	case binStr:
		return writeBinString(w, t.Value)
	case binChunk:
		raw, err := hexDecodeString(t.Value)
		if err != nil {
			return err
		}
		var n [1]byte
		n[0] = uint8(len(raw))
		if _, err := w.Write(n[:]); err != nil {
			return err
		}
		_, err = w.Write(raw)
		return err
	case binDouble:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], float64bits(t.Float()))
		_, err := w.Write(buf[:])
		return err
	case binInt16:
		var buf [2]byte
		binary.LittleEndian.PutUint16(buf[:], uint16(int16(t.Int())))
		_, err := w.Write(buf[:])
		return err
	case binInt32:
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], uint32(int32(t.Int())))
		_, err := w.Write(buf[:])
		return err
	case binInt64:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(t.Int()))
		_, err := w.Write(buf[:])
		return err
	case binByte:
		_, err := w.Write([]byte{uint8(t.Int())})
		return err
	}
	return writeBinString(w, t.Value)
}

func writeBinString(w io.Writer, s string) error {
	if _, err := w.Write([]byte(s)); err != nil {
		return err
	}
	_, err := w.Write([]byte{0})
	return err
}

func float64bits(f float64) uint64 { return math.Float64bits(f) }

const hexDigits = "0123456789ABCDEF"

func hexEncode(b []byte) string {
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexDigits[v>>4]
		out[i*2+1] = hexDigits[v&0xF]
	}
	return string(out)
}

func hexDecodeString(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, &parseError{"dxf: odd-length hex chunk"}
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi := hexVal(s[i*2])
		lo := hexVal(s[i*2+1])
		if hi < 0 || lo < 0 {
			return nil, &parseError{"dxf: invalid hex chunk"}
		}
		out[i] = byte(hi<<4 | lo)
	}
	return out, nil
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return -1
}
