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

func TestParsingErrors(t *testing.T) {
	tests := []string{
		"invalidcolor",
		"#GGGGGG", // invalid hex
		"hsl#1234567",
		"c256", // invalid 256 color
		"cabc",
		"hsl()",
		"hsl(360 100 100",
		"hsl(360,100,100)",
		"hsl(361 100 100)",
		"hsl(360 100.1 100)",
		"hsl(360 100 100.1)",
		"hsl(abc 10 10)",
		"hsl(10 abc 10 10)",
		"hsl(10 10 def)",
	}
	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			_, err := tcolor.FromString(test)
			if err == nil {
				t.Errorf("Expected error for %q, got none", test)
			}
		})
	}
}

func TestParsingBasicColors(t *testing.T) {
	tests := []struct {
		input    string
		expected tcolor.Color
	}{
		{"none", tcolor.None.Color()},
		{"white", tcolor.White.Color()},
		{"orange", tcolor.Orange.Color()},
		{" bRig_ht - BLue ", tcolor.BrightBlue.Color()},
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
			if parsedColor != test.expected {
				t.Errorf("Parsed %q as %x %x, expected %x", test.input, parsedColor, bc, test.expected)
			}
		})
	}
}

func TestParsing256Colors(t *testing.T) {
	tests := []struct {
		input    string
		expected tcolor.Color
	}{
		{" c123 ", tcolor.Color256(123).Color()},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			parsedColor, err := tcolor.FromString(test.input)
			if err != nil {
				t.Errorf("Failed to parse %q: %v", test.input, err)
				return
			}
			if parsedColor != test.expected {
				t.Errorf("Parsed %q as %x, expected %x", test.input, parsedColor, test.expected)
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
		{"0.5,0.5,0.5", tcolor.RGBColor{R: 64, G: 191, B: 192}},
		{"0.1,1,0.5", tcolor.RGBColor{R: 255, G: 153, B: 0}},
		{"0.1,1,0.75", tcolor.RGBColor{R: 255, G: 204, B: 127}},
		{"0.1,1,0.25", tcolor.RGBColor{R: 128, G: 77, B: 0}},
		{"0.70,0.5,0.5", tcolor.RGBColor{R: 89, G: 64, B: 192}},
		{"0.75,1,0.5", tcolor.RGBColor{R: 128, G: 0, B: 255}},
		{"0.75,0.5,0.5", tcolor.RGBColor{R: 128, G: 64, B: 192}},
		{"1.0,1,0.75", tcolor.RGBColor{R: 255, G: 127, B: 127}},
		{"hsl(192.88 57.3 50.05)", tcolor.RGBColor{R: 0x37, G: 0xA9, B: 0xC9}}, // #37A9C9
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
		{"HSL#00102003", tcolor.HSLColor{H: 1, S: 2, L: 3}},
		{"HSL#010203", tcolor.HSLColor{H: 0x10, S: 2, L: 12}},
		{"HSL#FE1_BB_3F2", tcolor.HSLColor{H: 0xFE1, S: 0xBB, L: 0x3F2}},
		{"HSL#FF5733", tcolor.HSLColor{H: 0xFF0, S: 0x57, L: 0xCC}},
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
				if out != in {
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
	var total int
	for r := range 256 {
		for g := range 256 {
			for b := range 256 {
				total++ // 256^3 at the end.
				in := tcolor.RGBColor{uint8(r), uint8(g), uint8(b)}
				hsl := in.HSL()
				out := hsl.RGB()
				dist := rgbDistance(in, out)
				if dist > 0 {
					if mismatches%97 == 0 { // log random few
						t.Logf("Sample mismatch #%d: dist %d in=%v hsl=%s out=%v",
							mismatches+1, dist, in, hsl.String(), out)
					}
					mismatches++
				}
			}
		}
	}
	errorPercent := float64(mismatches) / float64(total) * 100
	t.Logf("Total RGB to HSL roundtrip mismatches: %d out of %d (%.4f%%)",
		mismatches, total, errorPercent)
	if errorPercent > 0.2 { // 0.2% is about what we get
		t.Fatalf("Total mismatches: %d (%.4f%%)", mismatches, errorPercent)
	}
}
