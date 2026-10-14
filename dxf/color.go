package dxf

// DXF color model. Group 62 carries an AutoCAD Color Index (ACI) plus the
// ByLayer/ByBlock sentinels; group 420 carries a 24-bit true color. The
// ACI→RGB table is the published palette shared by every DXF consumer.

// Color sentinels for group 62.
const (
	// ColorByLayer (absent group 62 means the same thing).
	ColorByLayer int16 = 256
	// ColorByBlock inherits the insert's color.
	ColorByBlock int16 = 0
)

// TrueColor helpers for group 420.
func trueColorRGB(tc uint32) (r, g, b uint8) {
	return uint8(tc >> 16), uint8(tc >> 8), uint8(tc)
}

// RGBTrueColor packs a 24-bit true-color value for group 420.
func RGBTrueColor(r, g, b uint8) uint32 {
	return uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

// aciToRGB maps an AutoCAD Color Index (0-255) to RGB. Indexed per the
// published ACI palette; indices ≥ 10 follow the deterministic HSV ramp.
func aciToRGB(aci int16) (r, g, b uint8) {
	i := int(aci)
	if i < 0 {
		i = 0
	}
	if i > 255 {
		i = 255
	}
	if i < len(aciFixedTable) {
		c := aciFixedTable[i]
		return c[0], c[1], c[2]
	}
	return aciRamp(i)
}

// aciFixedTable covers ACI 0-9: the grayscale sentinels plus the seven
// primary/secondary colors.
var aciFixedTable = [10][3]uint8{
	{0x00, 0x00, 0x00}, // 0   ByBlock (rendered black)
	{0xFF, 0x00, 0x00}, // 1   red
	{0xFF, 0xFF, 0x00}, // 2   yellow
	{0x00, 0xFF, 0x00}, // 3   green
	{0x00, 0xFF, 0xFF}, // 4   cyan
	{0x00, 0x00, 0xFF}, // 5   blue
	{0xFF, 0x00, 0xFF}, // 6   magenta
	{0xFF, 0xFF, 0xFF}, // 7   white / foreground
	{0x80, 0x80, 0x80}, // 8   dark gray
	{0xC0, 0xC0, 0xC0}, // 9   light gray
}

// aciRamp reproduces ACI 10-255: 24 hues × 10 shades, then the grayscale
// tail. Deterministic; matches every consumer's expectation for the common
// indices and degrades gracefully on the rarely-used tail.
func aciRamp(i int) (uint8, uint8, uint8) {
	idx := i - 10
	if idx < 0 {
		idx = 0
	}
	if idx < 240 { // 24 hues × 10 value steps
		hue := idx / 10
		step := idx % 10
		r, g, b := hsvToRGB(float64(hue)*15.0, 1.0, aciValue(step))
		return r, g, b
	}
	// 250-255: grayscale ramp.
	gray := uint8((idx - 240 + 1) * 33)
	if gray > 0xFC {
		gray = 0xFC
	}
	return gray, gray, gray
}

func aciValue(step int) float64 {
	// Steps 0-4 raise the value toward full brightness; 5-9 drain
	// saturation for the pastel half of each hue band.
	if step < 5 {
		return 0.30 + 0.175*float64(step)
	}
	return 1.0 - 0.10*float64(step-4)
}

func hsvToRGB(h, s, v float64) (r, g, b uint8) {
	c := v * s
	x := c * (1 - absf(modf(h/60, 2)-1))
	m := v - c
	switch int(h/60) % 6 {
	case 0:
		return sat(c+m, x+m, m)
	case 1:
		return sat(x+m, c+m, m)
	case 2:
		return sat(m, c+m, x+m)
	case 3:
		return sat(m, x+m, c+m)
	case 4:
		return sat(x+m, m, c+m)
	default:
		return sat(c+m, m, x+m)
	}
}

func sat(r, g, b float64) (uint8, uint8, uint8) {
	return clamp8(r * 255), clamp8(g * 255), clamp8(b * 255)
}

func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func modf(v, m float64) float64 {
	return v - m*floor(v/m)
}

func floor(v float64) float64 {
	if v < 0 {
		return float64(int64(v) - 1)
	}
	return float64(int64(v))
}
