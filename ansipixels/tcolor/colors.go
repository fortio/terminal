// Package tcolor provides ANSI color codes and utilities for terminal colors.
// You can see a good demo/use of it the tcolor CLI at
// [github.com/fortio/tcolor](https://github.com/fortio/tcolor) or by running
// ```shell
// go install fortio.org/tcolor@latest
// tcolor
// ```
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

type Color uint32 // high byte is type, rest is RGB or HSL or BasicColor.

type ColorType uint8 // 0 for RGB, 1 for HSL, 2 for BasicColor
const (
	ColorTypeRGB   ColorType = 1 // RGBColor
	ColorTypeHSL   ColorType = 2 // HSLColor
	ColorTypeBasic ColorType = 3 // BasicColor
)

//nolint:gosec // no overflow possible
func (c Color) Decode() (ColorType, [3]uint8) {
	u := uint32(c)
	switch u & 0xFF000000 {
	case uint32(ColorTypeRGB) << 24:
		return ColorTypeRGB, [3]uint8{uint8(u >> 16 & 0xFF), uint8(u >> 8 & 0xFF), uint8(u & 0xFF)} // RGB
	case uint32(ColorTypeHSL) << 24:
		return ColorTypeHSL, [3]uint8{uint8(u >> 16 & 0xFF), uint8(u >> 8 & 0xFF), uint8(u & 0xFF)} // HSL
	case uint32(ColorTypeBasic) << 24:
		return ColorTypeBasic, [3]uint8{uint8(u & 0xFF)}
	default:
		panic(fmt.Sprintf("Invalid color type %d", c&0xFF000000))
	}
}

func Basic(c BasicColor) Color {
	return Color(uint32(ColorTypeBasic)<<24 | uint32(c))
}

func RGB(c RGBColor) Color {
	return Color(uint32(ColorTypeRGB)<<24 | uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B))
}

// HSLf creates a Color from HSL float values in [0,1] range.
func HSLf(h, s, l float64) Color {
	return Color(uint32(ColorTypeHSL)<<24 |
		uint32(math.Round(h*255))<<16 | // h in [0,1]
		uint32(math.Round(s*255))<<8 | // s in [0,1]
		uint32(math.Round(l*255))) // l in [0,1]
}

// HSL creates a Color from HSLColor.
func HSL(hsl HSLColor) Color {
	return Color(uint32(ColorTypeHSL)<<24 |
		uint32(hsl.H)<<16 | // h in [0,255]
		uint32(hsl.S)<<8 | // s in [0,255]
		uint32(hsl.L)) // l in [0,255]
}

func (c Color) String() string {
	t, components := c.Decode()
	switch t {
	case ColorTypeRGB:
		return fmt.Sprintf("#%02X%02X%02X", components[0], components[1], components[2])
	case ColorTypeHSL:
		return fmt.Sprintf("HSL_%02X%02X%02X", components[0], components[1], components[2])
	case ColorTypeBasic:
		return BasicColor(components[0]).String()
	default:
		panic(fmt.Sprintf("Invalid color type %d", t))
	}
}

func (c Color) BasicColor() (BasicColor, bool) {
	t, components := c.Decode()
	if t == ColorTypeBasic {
		return BasicColor(components[0]), true
	}
	return None, false
}

func ToRGB(t ColorType, components [3]uint8) RGBColor {
	switch t {
	case ColorTypeRGB:
		return RGBColor{components[0], components[1], components[2]}
	case ColorTypeHSL:
		return HSLColor{components[0], components[1], components[2]}.RGB()
	default:
		panic(fmt.Sprintf("ToRGB on invalid color type %d", t))
	}
}

func ToHSL(t ColorType, components [3]uint8) HSLColor {
	switch t {
	case ColorTypeRGB:
		return RGBColor{components[0], components[1], components[2]}.HSL()
	case ColorTypeHSL:
		return HSLColor{components[0], components[1], components[2]}
	default:
		panic(fmt.Sprintf("ToHSL on invalid color type %d", t))
	}
}

func (c Color) Foreground() string {
	t, components := c.Decode()
	switch t {
	case ColorTypeBasic:
		return BasicColor(components[0]).Foreground()
	case ColorTypeRGB, ColorTypeHSL:
		return ToRGB(t, components).Foreground()
	default:
		panic(fmt.Sprintf("Invalid color type %d", t))
	}
}

func (c Color) Background() string {
	t, components := c.Decode()
	switch t {
	case ColorTypeBasic:
		return BasicColor(components[0]).Background()
	case ColorTypeRGB, ColorTypeHSL:
		return ToRGB(t, components).Background()
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

// Extract RGB values from a hex color string (RRGGBB) or error.
func RGBFromString(color string) (RGBColor, error) {
	var i int
	_, err := fmt.Sscanf(color, "%x", &i)
	if err != nil {
		return RGBColor{}, fmt.Errorf("invalid hex color '%s', must be hex RRGGBB: %w", color, err)
	}
	r := (i >> 16) & 0xFF
	g := (i >> 8) & 0xFF
	b := i & 0xFF
	return RGBColor{R: uint8(r), G: uint8(g), B: uint8(b)}, nil //nolint:gosec // no overflow here
}

// FromString converts user input color string to a terminal color.
// Supports basic color names, RGB hex format (RRGGBB), HSL float format (h,s,l in [0,1]), and HSL hex format (HSL#HHSSLL).
func FromString(color string) (Color, error) {
	toRemove := " \t\r\n_-#" // can't remove . because of hsl
	color = strings.ToLower(strings.Map(func(r rune) rune {
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
		return FromHexHSLString(hex)
	}
	if len(color) == 6 {
		rgbColor, err := RGBFromString(color)
		if err != nil {
			return 0, err
		}
		return RGB(rgbColor), nil
	}
	return 0, fmt.Errorf("invalid color '%s', must be RRGGBB or h,s,l or one of: %s", color, ColorHelp)
}

func RGBATo216(pixel RGBColor) uint8 {
	// Check if grayscale
	shift := 4
	if (pixel.R>>shift) == (pixel.G>>shift) && (pixel.G>>shift) == (pixel.B>>shift) {
		// Bugged:
		// lum := safecast.MustConvert[uint8](max(255, math.Round(0.299*float64(pixel.R)+
		// 0.587*float64(pixel.G)+0.114*float64(pixel.B))))
		lum := (uint16(pixel.R) + uint16(pixel.G) + uint16(pixel.B)) / 3
		if lum < 9 { // 0-9.8 but ... 0-8 9 levels
			return 16 // -> black
		}
		if lum > 247 { // 248-255 (incl) 8 levels
			return 231 // -> white
		}
		return safecast.MustConvert[uint8](min(255, 232+((lum-9)*(256-232))/(247-9)))
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
	t, components := c.Decode()
	switch t {
	case ColorTypeBasic:
		return BasicColor(components[0]).Foreground()
	case ColorTypeRGB, ColorTypeHSL:
		rgb := ToRGB(t, components)
		return fmt.Sprintf("\033[38;5;%dm", RGBATo216(rgb))
	default:
		panic(fmt.Sprintf("Foreground on invalid color type %d", t))
	}
}

func (co ColorOutput) Background(c Color) string {
	if co.TrueColor {
		return c.Background()
	}
	t, components := c.Decode()
	switch t {
	case ColorTypeBasic:
		return BasicColor(components[0]).Background()
	case ColorTypeRGB, ColorTypeHSL:
		rgb := ToRGB(t, components)
		return fmt.Sprintf("\033[48;5;%dm", RGBATo216(rgb))
	default:
		panic(fmt.Sprintf("Background on invalid color type %d", t))
	}
}

// HSL colors.

// HSLColor is the hex version of h,s,l, each in [0,255].
type HSLColor struct {
	H, S, L uint8
}

// FromHSLString converts a string in the format "h,s,l" [0,1] each, to a 3 bytes Color.
func From3floatHSLString(color string) (Color, error) {
	parts := strings.SplitN(color, ",", 3)
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
		return 0, fmt.Errorf("invalid brightness '%s': %w", parts[2], err)
	}
	if v < 0 || v > 1 {
		return 0, fmt.Errorf("brightness must be in [0,1], got %f", v)
	}
	return HSLf(h, s, v), nil
}

// Extract RGB values from a hex color string (HHSSLL) or error.
// (Same as RGBFromString but bytes being HSL instead of RGB).
func FromHexHSLString(color string) (Color, error) {
	var i int
	_, err := fmt.Sscanf(color, "%x", &i)
	if err != nil {
		return 0, fmt.Errorf("invalid HSL hex color '%s', must be hex HHSSLL: %w", color, err)
	}
	h := (i >> 16) & 0xFF
	s := (i >> 8) & 0xFF
	l := i & 0xFF
	return HSL(HSLColor{H: uint8(h), S: uint8(s), L: uint8(l)}), nil //nolint:gosec // no overflow here
}

func (hsl HSLColor) String() string {
	return fmt.Sprintf("HSL#%02X%02X%02X", hsl.H, hsl.S, hsl.L)
}

func (hsl HSLColor) RGB() RGBColor {
	return HSLToRGB(float64(hsl.H)/255., float64(hsl.S)/255., float64(hsl.L)/255.)
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
		H: uint8(math.Round(h * 255)),
		S: uint8(math.Round(s * 255)),
		L: uint8(math.Round(l * 255)),
	}
}

func (c RGBColor) Color() Color {
	return RGB(c)
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
