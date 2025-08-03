// Package tcolor provides ANSI color codes and utilities for terminal colors.
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

type Color struct {
	Basic bool // Selector between BasicColor and RGBColor
	BasicColor
	RGBColor
}

func (c Color) String() string {
	if c.Basic {
		return c.BasicColor.String()
	}
	return fmt.Sprintf("%02x%02x%02x", c.R, c.G, c.B)
}

func (c Color) Foreground() string {
	if c.Basic {
		return c.BasicColor.Foreground()
	}
	return c.RGBColor.Foreground()
}

func (c Color) Background() string {
	if c.Basic {
		return c.BasicColor.Background()
	}
	return c.RGBColor.Background()
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
func FromString(color string) (Color, error) {
	toRemove := " \t\r\n_-#" // can't remove . because of hsl
	color = strings.ToLower(strings.Map(func(r rune) rune {
		if strings.ContainsRune(toRemove, r) {
			return -1
		}
		return r
	}, color))
	if c, ok := ColorMap[color]; ok {
		return Color{Basic: true, BasicColor: c}, nil
	}
	if strings.IndexByte(color, ',') != -1 {
		return FromHSLString(color)
	}
	if len(color) == 6 {
		rgbColor, err := RGBFromString(color)
		if err != nil {
			return Color{}, err
		}
		return Color{Basic: false, RGBColor: rgbColor}, nil
	}
	return Color{}, fmt.Errorf("invalid color '%s', must be RRGGBB or h,s,l or one of: %s", color, ColorHelp)
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
	if co.TrueColor || c.Basic {
		return c.Foreground()
	}
	return fmt.Sprintf("\033[38;5;%dm", RGBATo216(c.RGBColor))
}

func (co ColorOutput) Background(c Color) string {
	if co.TrueColor || c.Basic {
		return c.Background()
	}
	return fmt.Sprintf("\033[48;5;%dm", RGBATo216(c.RGBColor))
}

// HSL colors.

// FromHSLString converts a string in the format "h,s,l" [0,1] each, to a (rgb) Color.
func FromHSLString(color string) (Color, error) {
	parts := strings.SplitN(color, ",", 3)
	if len(parts) != 3 {
		return Color{}, fmt.Errorf("invalid HSL color '%s', must be h,s,l", color)
	}
	// H,S,L format
	h, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return Color{}, fmt.Errorf("invalid hue '%s': %w", parts[0], err)
	}
	if h < 0 || h > 1 {
		return Color{}, fmt.Errorf("hue must be in [0,1], got %f", h)
	}
	s, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return Color{}, fmt.Errorf("invalid saturation '%s': %w", parts[1], err)
	}
	if s < 0 || s > 1 {
		return Color{}, fmt.Errorf("saturation must be in [0,1], got %f", s)
	}
	v, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return Color{}, fmt.Errorf("invalid value '%s': %w", parts[2], err)
	}
	if v < 0 || v > 1 {
		return Color{}, fmt.Errorf("brightness must be in [0,1], got %f", v)
	}
	return Color{RGBColor: HSLToRGB(h, s, v)}, nil
}

// HSLToRGB converts HSL values to RGB. h, s and l in [0,1].
// Initially from grol's image extension.
func HSLToRGB(h, s, l float64) RGBColor {
	var r, g, b float64
	// h = math.Mod(h, 360.) / 360.
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
