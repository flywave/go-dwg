// Package dwg is a pure-Go reader for Autodesk DWG files (R13 -> R2018 /
// AC1032), ported from the Apache-2.0 dwg-rs reference implementation.
//
// The two DWG container families are supported:
//
//   - R13-R15 (AC1014/AC1015): flat section-locator list.
//   - R2004 family (AC1018/AC1024/AC1027/AC1032): XOR-encrypted header,
//     section page map, section info, LZ77 section payloads.
//   - R2007 (AC1021): its own §5.1-§5.4 container (Reed-Solomon + a
//     distinct LZ variant).
//
// Nothing in this package executes code from the target file.
package dwg

import (
	"errors"
	"fmt"
)

// Sentinel errors, one per semantic failure class. Callers can use
// errors.Is to branch on a failure without matching message text.
var (
	// ErrNotDwg means byte offsets 0..2 were not the ASCII prefix "AC".
	ErrNotDwg = errors.New("not a DWG file")
	// ErrUnsupportedVersion means the magic is AC10xx but not a version
	// this package understands (known: AC1014 .. AC1032).
	ErrUnsupportedVersion = errors.New("unsupported DWG version")
	// ErrTruncated means the file ended before a required field.
	ErrTruncated = errors.New("file truncated")
	// ErrBitsExhausted means a bit read ran past the cursor end.
	ErrBitsExhausted = errors.New("bit cursor exhausted")
	// ErrCRC means a stored checksum did not match its recomputation.
	ErrCRC = errors.New("checksum mismatch")
	// ErrR2004Decrypt means the R2004+ header XOR decrypt failed its
	// "AcFssFcAJMB" file-id sanity check.
	ErrR2004Decrypt = errors.New("R2004+ header decrypt failed")
	// ErrSectionMap covers section page map / section info failures.
	ErrSectionMap = errors.New("section map parse failed")
	// ErrSectionLocator covers R13-R15 locator failures.
	ErrSectionLocator = errors.New("section locator malformed")
	// ErrReservedBitPattern means a reserved BS/BL/BD prefix ("11") was
	// encountered where a value was expected.
	ErrReservedBitPattern = errors.New("reserved bit pattern encountered")
	// ErrLZ77Truncated means the LZ77 stream ended before a terminator.
	ErrLZ77Truncated = errors.New("lz77 stream truncated")
	// ErrLZ77InvalidOffset means a back-reference pointed before the
	// start of the output buffer.
	ErrLZ77InvalidOffset = errors.New("lz77 back-reference before output start")
	// ErrLZ77InvalidOpcode means opcode 0x00-0x0F appeared mid-stream.
	ErrLZ77InvalidOpcode = errors.New("lz77 reserved opcode")
	// ErrLZ77OutputLimitExceeded is the decompression-bomb defense.
	ErrLZ77OutputLimitExceeded = errors.New("lz77 output limit exceeded")
	// ErrLZ77BackrefTooLong bounds a single back-reference copy.
	ErrLZ77BackrefTooLong = errors.New("lz77 back-reference too long")
	// ErrBLLOverflow means a BLL value needed more than 7 bytes.
	ErrBLLOverflow = errors.New("BLL value overflows 7-byte encoding")
	// ErrHandleCounterTooLarge means a handle nibble asked for more than
	// 8 value bytes; the cursor is not on a handle field.
	ErrHandleCounterTooLarge = errors.New("handle counter exceeds 8 bytes")
	// ErrWalkerLimitExceeded bounds object-walk record counts.
	ErrWalkerLimitExceeded = errors.New("object walker limit exceeded")
	// ErrUnknownObjectType means a strict walk hit a reserved type code.
	ErrUnknownObjectType = errors.New("unknown object type code")
	// ErrUnsupported means a feature exists in the format but is not yet
	// implemented.
	ErrUnsupported = errors.New("unsupported feature")
)

// Error is a typed parse error. It wraps one of the sentinel values so
// errors.Is works, and carries a human-readable context string.
type Error struct {
	Kind error
	Msg  string
}

func (e *Error) Error() string {
	if e.Msg == "" {
		return e.Kind.Error()
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Kind }

// newError builds a typed error wrapping kind with a formatted message.
func newError(kind error, format string, args ...any) *Error {
	return &Error{Kind: kind, Msg: fmt.Sprintf(format, args...)}
}
