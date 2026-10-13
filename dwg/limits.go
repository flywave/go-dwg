package dwg

// ParseLimits groups the parser's count caps.
type ParseLimits struct {
	MaxHandleEntries      int
	MaxClassEntries       int
	MaxXDataIterations    int
	MaxLWPolylineCount    int
	MaxImageClipVerts     int
	MaxLeaderPoints       int
	MaxSplineCount        int
	MaxRetainedDispatchEr int
}

// SafeParseLimits returns the conservative default profile.
func SafeParseLimits() ParseLimits {
	return ParseLimits{
		MaxHandleEntries:      1_000_000,
		MaxClassEntries:       4_096,
		MaxXDataIterations:    256,
		MaxLWPolylineCount:    1_000_000,
		MaxImageClipVerts:     100_000,
		MaxLeaderPoints:       100_000,
		MaxSplineCount:        100_000,
		MaxRetainedDispatchEr: 1_000,
	}
}

// WalkerLimits bounds graph iteration.
type WalkerLimits struct {
	MaxHandles      int
	MaxScanBytes    int
	MaxBlockNesting int
}

// SafeWalkerLimits returns the conservative default profile.
func SafeWalkerLimits() WalkerLimits {
	return WalkerLimits{
		MaxHandles:      1_000_000,
		MaxScanBytes:    512 * 1024 * 1024,
		MaxBlockNesting: 128,
	}
}

// MaxHandleEntries is the defensive cap on parsed handle map entries.
const MaxHandleEntries = 1_000_000

// MaxHandleSectionBytes is the maximum bytes per handle section (spec §4.3).
const MaxHandleSectionBytes = 2032
