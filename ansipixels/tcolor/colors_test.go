package tcolor_test

import (
	"testing"

	"fortio.org/terminal/ansipixels/tcolor"
)

func TestHelpString(t *testing.T) {
	// Check Orange is there now
	expected := "none, black, red, green, yellow, orange, blue, purple, cyan, gray, darkgray, " +
		"brightred, brightgreen, brightyellow, brightblue, brightpurple, brightcyan, white"
	if tcolor.ColorHelp != expected {
		t.Errorf("Expected %q, got %q", expected, tcolor.ColorHelp)
	}
}

func TestParsingBasicColors(t *testing.T) {
	tests := []struct {
		input    string
		expected tcolor.BasicColor
	}{
		{"none", tcolor.None},
		{"white", tcolor.White},
		{"orange", tcolor.Orange},
		{" bRig_ht - BLue ", tcolor.BrightBlue},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			parsedColor, err := tcolor.FromString(test.input)
			if err != nil {
				t.Errorf("Failed to parse %q: %v", test.input, err)
				return
			}
			bc, ok := parsedColor.BasicColor()
			if !ok {
				t.Errorf("Expected basic color for %q, got %#v", test.input, parsedColor)
				return
			}
			if bc != test.expected {
				t.Errorf("Parsed %q as %d, expected %d", test.input, bc, test.expected)
			}
		})
	}
}

func TestParsingAdvancedColor(t *testing.T) {
	tests := []struct {
		input    string
		expected tcolor.RGBColor
	}{
		{"#000000", tcolor.RGBColor{R: 0, G: 0, B: 0}},
		{"#FFFFFF", tcolor.RGBColor{R: 255, G: 255, B: 255}},
		{"#FF5733", tcolor.RGBColor{R: 255, G: 87, B: 51}},
		{"#33FF57", tcolor.RGBColor{R: 51, G: 255, B: 87}},
		{"#3357FF", tcolor.RGBColor{R: 51, G: 87, B: 255}},
		// HSL are not really verified but seem to make sense (matched what was returned)
		{"0.5,0.5,0.5", tcolor.RGBColor{R: 64, G: 191, B: 191}},
		{"0.1,1,0.5", tcolor.RGBColor{R: 255, G: 153, B: 0}},
		{"0.1,1,0.75", tcolor.RGBColor{R: 255, G: 204, B: 127}},
		{"0.1,1,0.25", tcolor.RGBColor{R: 128, G: 76, B: 0}},
		{"0.7,1,0.5", tcolor.RGBColor{R: 51, G: 0, B: 255}},
		{"0.7,0.5,0.5", tcolor.RGBColor{R: 89, G: 64, B: 191}},
		{"1.0,1,0.75", tcolor.RGBColor{R: 255, G: 127, B: 127}},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			parsedColor, err := tcolor.FromString(test.input)
			if err != nil {
				t.Errorf("Failed to parse %q: %v", test.input, err)
				return
			}
			ct, v := parsedColor.Decode()
			if ct == tcolor.ColorTypeBasic {
				t.Errorf("Expected advanced color for %q, got %s", test.input, parsedColor.String())
				return
			}
			rgb := tcolor.ToRGB(ct, v)
			if rgb != test.expected {
				t.Errorf("Parsed %q as %s - %v %X -> %v, expected %v", test.input, parsedColor.String(), ct, v, rgb, test.expected)
			}
		})
	}
}

func TestParsingHSLHex(t *testing.T) {
	tests := []struct {
		input    string
		expected tcolor.HSLColor
	}{
		{"HSL#001002003", tcolor.HSLColor{H: 1, S: 2, L: 3}},
		{"HSL#010203", tcolor.HSLColor{H: 4, S: 8, L: 12}},
		{"HSL#3F03F13F2", tcolor.HSLColor{H: 0x3F0, S: 0x3F1, L: 0x3F2}},
		{"HSL#FF5733", tcolor.HSLColor{H: 0x3FC, S: 0x15C, L: 0xCC}},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			parsedColor, err := tcolor.FromString(test.input)
			if err != nil {
				t.Errorf("Failed to parse %q: %v", test.input, err)
				return
			}
			ct, v := parsedColor.Decode()
			if ct != tcolor.ColorTypeHSL {
				t.Errorf("Expected advanced color for %q, got %s", test.input, parsedColor.String())
				return
			}
			hsl := tcolor.ToHSL(ct, v)
			if hsl != test.expected {
				t.Errorf("Parsed %q as %v, expected %v", test.input, hsl, test.expected)
			}
		})
	}
}

func TestHSLRGBExactRoundTripFloats(t *testing.T) {
	var mismatches int
	for r := range 256 {
		for g := range 256 {
			for b := range 256 {
				in := tcolor.RGBColor{uint8(r), uint8(g), uint8(b)}
				h, s, l := tcolor.RGBToHSL(in)
				out := tcolor.HSLToRGB(h, s, l)

				if out.R != in.R || out.G != in.G || out.B != in.B {
					mismatches++
					if mismatches <= 10 { // log only first few
						t.Errorf("Mismatch: in=%v hsl=(%.10f,%.10f,%.10f) out=%v",
							in, h, s, l, out)
					}
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("Total mismatches: %d", mismatches)
	}
}

func dist(a, b uint8) uint32 {
	if a < b {
		return uint32(b - a)
	}
	return uint32(a - b)
}

func rgbDistance(a, b tcolor.RGBColor) uint32 {
	return dist(a.R, b.R) + dist(a.G, b.G) + dist(a.B, b.B)
}

func TestHSLRGBExactRoundTrip3Bytes(t *testing.T) {
	var mismatches int
	for r := range 256 {
		for g := range 256 {
			for b := range 256 {
				in := tcolor.RGBColor{uint8(r), uint8(g), uint8(b)}
				hsl := in.HSL()
				out := hsl.RGB()
				dist := rgbDistance(in, out)
				if dist > 0 {
					if mismatches%997 == 0 { // log random few
						t.Errorf("Mismatch: %d in=%v hsl=%s out=%v",
							dist, in, hsl.String(), out)
					}
					mismatches++
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("Total mismatches: %d", mismatches)
	}
}
