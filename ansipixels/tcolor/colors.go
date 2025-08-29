// Package tcolor provides ANSI color codes and utilities for terminal colors.
// You can see a good demo/use of it the tcolor CLI at
// https://github.com/fortio/tcolor or by running
//
//	go install fortio.org/tcolor@latest
//	tcolor
//
// Initially partially from images.go and tclock and generalized.
package tcolor // import "fortio.org/terminal/ansipixels/tcolor"

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"fortio.org/safecast"
)

type BasicColor uint8

const (
	None         BasicColor = 0 // no color, default
	Black        BasicColor = 30
	Red          BasicColor = 31
	Green        BasicColor = 32
	Yellow       BasicColor = 33
	Orange       BasicColor = 99 // not in basic colors, but in 256 colors
	Blue         BasicColor = 34
	Purple       BasicColor = 35
	Cyan         BasicColor = 36
	Gray         BasicColor = 37
	DarkGray     BasicColor = 90
	BrightRed    BasicColor = 91
	BrightGreen  BasicColor = 92
	BrightYellow BasicColor = 93
	BrightBlue   BasicColor = 94
	BrightPurple BasicColor = 95
	BrightCyan   BasicColor = 96
	White        BasicColor = 97
	color256     BasicColor = 255 // marker for 256 colors mode.

	// Misc useful sequences.
	Bold       = "\x1b[1m"
	Dim        = "\x1b[2m"
	Underlined = "\x1b[4m"
	Blink      = "\x1b[5m"

	Inverse = "\033[7m"

	Reset = "\033[0m"
)

//go:generate stringer -type=BasicColor
var _ = White.String() // force compile error if go generate is missing.

// Terminal foreground color string for the BasicColor.
func (c BasicColor) Foreground() string {
	switch c {
	case None:
		return ""
	case Orange:
		return "\033[38;5;214m" // Orange is not in the basic colors, but in 256 colors
	default:
		return fmt.Sprintf("\033[%dm", c)
	}
}

// Terminal background color string for the BasicColor.
func (c BasicColor) Background() string {
	switch c {
	case None:
		return ""
	case Orange:
		return "\033[48;5;214m" // Orange is not in the basic colors, but in 256 colors
	default:
		return fmt.Sprintf("\033[%dm", c+10)
	}
}

func (c BasicColor) Color() Color {
	return Color(uint32(ColorTypeBasic)<<30 | uint32(c))
}

type Color256 uint8

// Creates indexed (terminal 256) color: 16 basic colors, 216 color cube, grayscale.
// Stored as 2 bytes, low byte is 0xff to not conflict with basic 16 colors and high byte is the index.
func (idx Color256) Color() Color {
	// Piggy back on the ColorBasic bit/type with low byte == 0xFF
	return Color(uint32(ColorTypeBasic)<<30 | uint32(idx)<<8 | uint32(color256))
}

func (idx Color256) String() string {
	return fmt.Sprintf("c%03d", idx)
}

func (idx Color256) Foreground() string {
	return fmt.Sprintf("\033[38;5;%dm", idx)
}

func (idx Color256) Background() string {
	return fmt.Sprintf("\033[48;5;%dm", idx)
}

type RGBColor struct {
	R, G, B uint8
}

// Terminal foreground color string for RGBColor.
func (c RGBColor) Foreground() string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

// Terminal background color string for RGBColor.
func (c RGBColor) Background() string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

// Color: high 2 bits is type, rest is RGB or HSL or BasicColor.
// HSL is 12 bits for Hue, 8 bits for Saturation, 10 bits for Lightness (lowest error rate when going to/from RGB).
// RGB is 8 bits for each component (R, G, B).
type Color uint32

// ColorType is the type of the color, used in the high 2 bits of the Color.
type ColorType uint8 // 1 for RGB, 2 for HSL, 3 for BasicColor

const (
	ColorTypeRGB ColorType = 1 // RGBColor
	ColorTypeHSL ColorType = 2 // HSLColor
	// Terminal Basic 16 Colors.
	ColorTypeBasic ColorType = 3
	// 256 Colors (216 colorspace cube + grey scale + basic 16).
	// Virtual type, split from ColorTypeBasic when the low byte is 0xFF.
	ColorType256 ColorType = 99
)

// 30 bits, used for 12,8,10 bits HSL color components in HSLColor.
type Uint30 uint32

// 12 bits for Hue component in HSL.
type Uint12 uint16

// 8 bits for Saturation component in HSL.
type Uint8 uint8

// 10 bits for Lightness component in HSL.
type Uint10 uint16

const (
	MaxHSLHue        = 4095 // 12 bits
	MaxHSLSaturation = 255  // 8 bits
	MaxHSLLightness  = 1023 // 10 bits
)

func (c Color) Decode() (ColorType, Uint30) {
	u := uint32(c)
	switch u & 0xC0000000 {
	case uint32(ColorTypeRGB) << 30:
		return ColorTypeRGB, Uint30(0x3FFFFFFF & u) // RGB
	case uint32(ColorTypeHSL) << 30:
		return ColorTypeHSL, Uint30(0x3FFFFFFF & u) // HSL
	case uint32(ColorTypeBasic) << 30:
		if u&0xFF == uint32(color256) {
			return ColorType256, Uint30((u & 0xFFFF) >> 8)
		}
		// safecast here shouldn't be necessary once gosec gets smarter
		return ColorTypeBasic, Uint30(u & 0xFF)
	default:
		panic(fmt.Sprintf("Invalid color type %x (%x)", u&0xC0000000, u))
	}
}

func Basic(c BasicColor) Color {
	return c.Color()
}

func RGB(c RGBColor) Color {
	return Color(uint32(ColorTypeRGB)<<30 | uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B))
}

// HSLf creates a Color from HSL float values in [0,1] range.
func HSLf(h, s, l float64) Color {
	return Color(uint32(ColorTypeHSL)<<30 |
		uint32(math.Round(h*MaxHSLHue))<<18 | // h in [0,1]
		uint32(math.Round(s*MaxHSLSaturation))<<10 | // s in [0,1]
		uint32(math.Round(l*MaxHSLLightness))) // l in [0,1]
}

// HSL creates a Color from HSLColor.
func HSL(hsl HSLColor) Color {
	return Color(uint32(ColorTypeHSL)<<30 |
		uint32(hsl.H)<<18 | // h in [0,4095]
		uint32(hsl.S)<<10 | // s in [0,255]
		uint32(hsl.L)) // l in [0,1023]
}

func Int30ToHSL(val Uint30) (Uint12, Uint8, Uint10) {
	return safecast.MustConv[Uint12]((val >> 18) & 0xFFF),
		safecast.MustConv[Uint8]((val >> 10) & 0xFF),
		safecast.MustConv[Uint10](val & 0x3FF)
}

func Int30To8bits(val Uint30) (uint8, uint8, uint8) {
	return safecast.MustConv[uint8]((val >> 16) & 0xFF),
		safecast.MustConv[uint8]((val >> 8) & 0xFF),
		safecast.MustConv[uint8](val & 0xFF)
}

func (c Color) String() string {
	str, _, _ := c.Extra()
	return str
}

// Extra returns 2 arguments, first one is the same as String() the color string that can
// be used in tcolor.FromString to obtain back the same color, including type.
// Second one is extra information that maps HSL to RGB, and BasicNamed color to their id.
func (c Color) Extra() (string, string, ColorType) {
	t, val := c.Decode()
	switch t {
	case ColorTypeRGB:
		r, g, b := Int30To8bits(val)
		return RGBColor{R: r, G: g, B: b}.String(), "", ColorTypeRGB
	case ColorTypeHSL:
		h, s, l := Int30ToHSL(val)
		hsl := HSLColor{H: h, S: s, L: l}
		return hsl.String(), hsl.RGB().String(), ColorTypeHSL
	case ColorType256:
		return safecast.MustConv[Color256](val).String(), "", ColorType256
	case ColorTypeBasic:
		if val == Uint30(Orange) {
			return "Orange", "c214", ColorTypeBasic
		}
		return safecast.MustConv[BasicColor](val).String(), fmt.Sprintf("%d", val), ColorTypeBasic
	default:
		panic(fmt.Sprintf("Invalid color type %d", t))
	}
}

func (c Color) BasicColor() (BasicColor, bool) {
	t, v := c.Decode()
	if t == ColorTypeBasic {
		return safecast.MustConv[BasicColor](v), true
	}
	return None, false
}

func ToRGB(t ColorType, v Uint30) RGBColor {
	switch t {
	case ColorTypeRGB:
		r, g, b := Int30To8bits(v)
		return RGBColor{R: r, G: g, B: b}
	case ColorTypeHSL:
		h, s, l := Int30ToHSL(v)
		hsl := HSLColor{H: h, S: s, L: l}
		rgb := hsl.RGB()
		// log.Printf("Converting HSL(%d,%d,%d) -> %#v to RGB: %x, %x, %x", h, s, l, hsl, rgb.R, rgb.G, rgb.B)
		return rgb
	default:
		panic(fmt.Sprintf("ToRGB on invalid color type %d", t))
	}
}

func ToHSL(t ColorType, v Uint30) HSLColor {
	switch t {
	case ColorTypeRGB:
		r, g, b := Int30To8bits(v)
		return RGBColor{R: r, G: g, B: b}.HSL()
	case ColorTypeHSL:
		h, s, l := Int30ToHSL(v)
		return HSLColor{H: h, S: s, L: l}
	default:
		panic(fmt.Sprintf("ToHSL on invalid color type %d", t))
	}
}

func (c Color) Foreground() string {
	t, v := c.Decode()
	switch t {
	case ColorTypeBasic:
		return safecast.MustConv[BasicColor](v).Foreground()
	case ColorType256:
		return safecast.MustConv[Color256](v).Foreground()
	case ColorTypeRGB, ColorTypeHSL:
		return ToRGB(t, v).Foreground()
	default:
		panic(fmt.Sprintf("Invalid color type %d", t))
	}
}

func (c Color) Background() string {
	t, v := c.Decode()
	switch t {
	case ColorTypeBasic:
		return safecast.MustConv[BasicColor](v).Background()
	case ColorType256:
		return safecast.MustConv[Color256](v).Background()
	case ColorTypeRGB, ColorTypeHSL:
		return ToRGB(t, v).Background()
	default:
		panic(fmt.Sprintf("Invalid color type %d", t))
	}
}

// Ordered list of the basic colors.
var BasicColorList []BasicColor

// Map from color name to BasicColor.
var ColorMap map[string]BasicColor

// Help string for the basic color choices.
var ColorHelp string

func init() {
	BasicColorList = append(BasicColorList, None)
	for i := Black; i <= Gray; i++ {
		BasicColorList = append(BasicColorList, i)
		if i == Yellow {
			BasicColorList = append(BasicColorList, Orange)
		}
	}
	for i := DarkGray; i <= White; i++ {
		BasicColorList = append(BasicColorList, i)
	}
	ColorMap = make(map[string]BasicColor, len(BasicColorList))
	buf := strings.Builder{}
	for i, c := range BasicColorList {
		lower := strings.ToLower(c.String())
		ColorMap[lower] = c
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(lower)
	}
	ColorHelp = buf.String()
}

// Extract 24 bit values from a hex color string (RRGGBB / HHSSLL) or error.
func Hex24bitFromString(label, color string) (RGBColor, error) {
	var i int
	_, err := fmt.Sscanf(color, "%x", &i)
	if err != nil {
		return RGBColor{}, fmt.Errorf("invalid hex color '%s', must be hex %s: %w", color, label, err)
	}
	// safecast won't be necessary once gosec gets smarter.
	r := safecast.MustConv[uint8]((i >> 16) & 0xFF)
	g := safecast.MustConv[uint8]((i >> 8) & 0xFF)
	b := safecast.MustConv[uint8](i & 0xFF)
	return RGBColor{R: r, G: g, B: b}, nil
}

func From256(color string) (Color, error) {
	if len(color) != 4 || color[0] != 'c' {
		return 0, fmt.Errorf("invalid 256 color '%s', must be c000-255", color)
	}
	var i int
	_, err := fmt.Sscanf(color[1:], "%d", &i)
	if err != nil {
		return 0, fmt.Errorf("invalid 256 color '%s', must be c000-255: %w", color, err)
	}
	if i < 0 || i > 255 {
		return 0, fmt.Errorf("invalid 256 color '%s', must be c000-255 (got %d)", color, i)
	}
	return Color256(i).Color(), nil
}

// FromString converts user input color string to a terminal color.
// Supports basic color names, RGB hex format (RRGGBB),
// HSL float format (h,s,l in [0,1]), and HSL 30 bits hex format (HSL#HHHSSSLLL).
func FromString(color string) (Color, error) {
	toRemove := "\t\r\n_-#" // can't remove . because of hsl
	hasParen := false
	color = strings.ToLower(strings.Map(func(r rune) rune {
		// keep spaces only inside (); for `hsl(deg psat plight)` format.
		if r == '(' {
			hasParen = true
			return r
		}
		if r == ')' {
			hasParen = false
			return r
		}
		if r == ' ' && !hasParen {
			return -1
		}
		if strings.ContainsRune(toRemove, r) {
			return -1
		}
		return r
	}, color))
	if c, ok := ColorMap[color]; ok {
		return Basic(c), nil
	}
	if strings.IndexByte(color, ',') != -1 {
		return From3floatHSLString(color)
	}
	if hex, ok := strings.CutPrefix(color, "hsl"); ok {
		return FromHSLString(hex)
	}
	if len(color) == 4 && color[0] == 'c' {
		return From256(color)
	}
	if len(color) == 6 {
		rgbColor, err := Hex24bitFromString("RRGGBB", color)
		if err != nil {
			return 0, err
		}
		return RGB(rgbColor), nil
	}
	return 0, fmt.Errorf("invalid color '%s', must be RRGGBB or h,s,l or one of: %s", color, ColorHelp)
}

// WebHSL returns a CSS HSL string for the given color (empty string
// for basic and 256 colors).
// Uses specified number of digits rounding (default is full precision
// (2 for hue and lightness, 1 for saturation) when passing rounding < 0).
// Used/Demonstrated in the fortio.org/tcolor TUI.
func WebHSL(c Color, rounding int) string {
	t, v := c.Decode()
	if t != ColorTypeHSL && t != ColorTypeRGB {
		return ""
	}
	hsl := ToHSL(t, v)
	deg := float64(hsl.H) * 360.0 / 4095.0 // Convert to degrees
	sat := float64(hsl.S) * 100.0 / 255.0  // Convert to percentage
	lum := float64(hsl.L) * 100.0 / 1023.0 // Convert to percentage
	satRound := 1
	otherRound := 2
	if rounding >= 0 {
		satRound = rounding
		otherRound = rounding
	}
	return fmt.Sprintf("hsl(%.*f %.*f %.*f)", otherRound, deg, satRound, sat, otherRound, lum)
}

func RGBATo216(pixel RGBColor) uint8 {
	// Check if grayscale
	shift := 4
	if (pixel.R>>shift) == (pixel.G>>shift) && (pixel.G>>shift) == (pixel.B>>shift) {
		// Bugged:
		// lum := safecast.MustConv[uint8](max(255, math.Round(0.299*float64(pixel.R)+
		// 0.587*float64(pixel.G)+0.114*float64(pixel.B))))
		lum := (uint16(pixel.R) + uint16(pixel.G) + uint16(pixel.B)) / 3
		if lum < 9 { // 0-9.8 but ... 0-8 9 levels
			return 16 // -> black
		}
		if lum > 247 { // 248-255 (incl) 8 levels
			return 231 // -> white
		}
		return safecast.MustConv[uint8](min(255, 232+((lum-9)*(256-232))/(247-9)))
	}
	// 6x6x6 color cube
	col := 16 + 36*(pixel.R/51) + 6*(pixel.G/51) + pixel.B/51
	return col
}

// User specified color (obtained from FromString) to terminal color output including
// conversion to 216 colors if TrueColor is false.
type ColorOutput struct {
	TrueColor bool // true if the output supports true color, false for 256 colors
}

func (co ColorOutput) Foreground(c Color) string {
	if co.TrueColor {
		return c.Foreground()
	}
	t, v := c.Decode()
	switch t {
	case ColorTypeBasic, ColorType256:
		return c.Foreground()
	case ColorTypeRGB, ColorTypeHSL:
		rgb := ToRGB(t, v)
		return fmt.Sprintf("\033[38;5;%dm", RGBATo216(rgb))
	default:
		panic(fmt.Sprintf("Foreground on invalid color type %d", t))
	}
}

func (co ColorOutput) Background(c Color) string {
	if co.TrueColor {
		return c.Background()
	}
	t, v := c.Decode()
	switch t {
	case ColorTypeBasic, ColorType256:
		return c.Background()
	case ColorTypeRGB, ColorTypeHSL:
		rgb := ToRGB(t, v)
		return fmt.Sprintf("\033[48;5;%dm", RGBATo216(rgb))
	default:
		panic(fmt.Sprintf("Background on invalid color type %d", t))
	}
}

// HSL colors.

// HSLColor is the hex version of h,s,l, each in [0,1023] (10 bits).
type HSLColor struct {
	H Uint12
	S Uint8
	L Uint10
}

// From3floatHSLString converts a string in the format "h,s,l" [0,1] each, to a 3 bytes Color.
func From3floatHSLString(color string) (Color, error) {
	parts := strings.SplitN(color, ",", 4)
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid HSL color '%s', must be h,s,l", color)
	}
	// H,S,L format
	h, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hue '%s': %w", parts[0], err)
	}
	if h < 0 || h > 1 {
		return 0, fmt.Errorf("hue must be in [0,1], got %f", h)
	}
	s, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid saturation '%s': %w", parts[1], err)
	}
	if s < 0 || s > 1 {
		return 0, fmt.Errorf("saturation must be in [0,1], got %f", s)
	}
	v, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid lightness '%s': %w", parts[2], err)
	}
	if v < 0 || v > 1 {
		return 0, fmt.Errorf("lightness must be in [0,1], got %f", v)
	}
	return HSLf(h, s, v), nil
}

func HexStrToUint12(hex string) (Uint12, error) {
	var i uint32
	_, err := fmt.Sscanf(hex, "%x", &i)
	if err != nil {
		return 0, fmt.Errorf("invalid 12 bits hex '%s', must be hex 000-FFF: %w", hex, err)
	}
	if i > 0xFFF {
		return 0, fmt.Errorf("invalid 12 bits hex '%s', must be hex 000-FFF: %w", hex, err)
	}
	return Uint12(i), nil
}

func HexStrToUint8(hex string) (Uint8, error) {
	var i uint32
	_, err := fmt.Sscanf(hex, "%x", &i)
	if err != nil {
		return 0, fmt.Errorf("invalid 8 bits hex '%s', must be hex 00-FF: %w", hex, err)
	}
	if i > 0xFF {
		return 0, fmt.Errorf("invalid 8 bits hex '%s', must be hex 00-FF: %w", hex, err)
	}
	return Uint8(i), nil
}

func HexStrToUint10(hex string) (Uint10, error) {
	var i uint32
	_, err := fmt.Sscanf(hex, "%x", &i)
	if err != nil {
		return 0, fmt.Errorf("invalid 10 bits hex '%s', must be hex 000-3FF: %w", hex, err)
	}
	if i > 0x3FF {
		return 0, fmt.Errorf("invalid 10 bits hex '%s', must be hex 000-3FF: %w", hex, err)
	}
	return Uint10(i), nil
}

// Extract RGB values from a hex color string (HHH_SS_LLL or HHSSLL) or error.
// (Same as RGBFromString but bytes being HSL instead of RGB).
func FromHexHSLString(color string) (Color, error) {
	if len(color) == 6 {
		// reuse 6 digit RGB parsing
		rgbColor, err := Hex24bitFromString("HHSSLL", color)
		if err != nil {
			return 0, err
		}
		// Convert from 8,8,8 extracted above to 12,8,10 bits.
		hsl := HSLColor{H: Uint12(rgbColor.R) << 4, S: Uint8(rgbColor.G), L: Uint10(rgbColor.B) << 2}
		return HSL(hsl), nil
	}
	if len(color) != 8 {
		return 0, fmt.Errorf("invalid HSL hex color '%s', must be hex HHH_SS_LLL or HHSSLL", color)
	}
	h, err := HexStrToUint12(color[:3])
	if err != nil {
		return 0, err
	}
	s, err := HexStrToUint8(color[3:5])
	if err != nil {
		return 0, err
	}
	l, err := HexStrToUint10(color[5:])
	if err != nil {
		return 0, err
	}
	return HSL(HSLColor{H: h, S: s, L: l}), nil
}

func FromHSLString(color string) (Color, error) {
	if len(color) <= 2 {
		return 0, fmt.Errorf("invalid too short HSL color 'hsl%s'", color)
	}
	if color[0] != '(' {
		return FromHexHSLString(color)
	}
	// Web HSL: `hsl(degree percentsat percentlight)`
	if color[len(color)-1] != ')' {
		return 0, fmt.Errorf("invalid HSL color 'hsl%s' should end with ')'", color)
	}
	color = color[1 : len(color)-1]
	parts := strings.SplitN(color, " ", 4)
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid HSL color 'hsl(%s)', must be hsl(h s l)", color)
	}
	h, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid hue '%s': %w", parts[0], err)
	}
	if h < 0 || h > 360 {
		return 0, fmt.Errorf("hue degrees must be in [0,360], got %f", h)
	}
	s, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid saturation '%s': %w", parts[1], err)
	}
	if s < 0 || s > 100 {
		return 0, fmt.Errorf("saturation %% must be in [0,100], got %f", s)
	}
	v, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid lightness '%s': %w", parts[2], err)
	}
	if v < 0 || v > 100 {
		return 0, fmt.Errorf("lightness %% must be in [0,100], got %f", v)
	}
	return HSLf(h/360.0, s/100.0, v/100.0), nil
}

func (hsl HSLColor) String() string {
	return fmt.Sprintf("HSL#%03X_%02X_%03X", hsl.H, hsl.S, hsl.L)
}

func (hsl HSLColor) RGB() RGBColor {
	return HSLToRGB(float64(hsl.H)/MaxHSLHue, float64(hsl.S)/MaxHSLSaturation, float64(hsl.L)/MaxHSLLightness)
}

func (hsl HSLColor) Color() Color {
	return HSL(hsl)
}

// HSLToRGB converts HSL values to RGB. h, s and l in [0,1].
// Initially from grol's image extension.
func HSLToRGB(h, s, l float64) RGBColor {
	var r, g, b float64
	// h = math.Mod(h, 360.) / 360. if we wanted in degrees.
	if s == 0 {
		r, g, b = l, l, l
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1. + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		r = hueToRGB(p, q, h+1/3.)
		g = hueToRGB(p, q, h)
		b = hueToRGB(p, q, h-1/3.)
	}
	return RGBColor{
		R: uint8(math.Round(r * 255)),
		G: uint8(math.Round(g * 255)),
		B: uint8(math.Round(b * 255)),
	}
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1.
	}
	if t > 1 {
		t -= 1.
	}
	if t < 1/6. {
		return p + (q-p)*6*t
	}
	if t < 0.5 {
		return q
	}
	if t < 2/3. {
		return p + (q-p)*(2/3.-t)*6
	}
	return p
}

func (c RGBColor) HSL() HSLColor {
	h, s, l := RGBToHSL(c)
	return HSLColor{
		H: Uint12(math.Round(h * MaxHSLHue)),
		S: Uint8(math.Round(s * MaxHSLSaturation)),
		L: Uint10(math.Round(l * MaxHSLLightness)),
	}
}

func (c RGBColor) Color() Color {
	return RGB(c)
}

func (c RGBColor) String() string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

func RGBToHSL(c RGBColor) (h, s, l float64) {
	r := float64(c.R) / 255.
	g := float64(c.G) / 255.
	b := float64(c.B) / 255.

	maxv := max(r, g, b)
	minv := min(r, g, b)
	l = (maxv + minv) / 2

	if maxv == minv {
		h, s = 0, 0 // achromatic
		return h, s, l
	}
	d := maxv - minv
	if l > 0.5 {
		s = d / (2 - maxv - minv)
	} else {
		s = d / (maxv + minv)
	}

	switch maxv {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	case b:
		h = (r-g)/d + 4
	}
	h /= 6
	return h, s, l
}
