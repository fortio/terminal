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
			if !parsedColor.Basic {
				t.Errorf("Expected basic color for %q, got %#v", test.input, parsedColor)
				return
			}
			if parsedColor.BasicColor != test.expected {
				t.Errorf("Parsed %q as %d, expected %d", test.input, parsedColor.BasicColor, test.expected)
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
		{"0.1,1,0.5", tcolor.RGBColor{R: 255, G: 153, B: 0}},
		{"0.1,1,0.75", tcolor.RGBColor{R: 255, G: 204, B: 128}},
		{"0.1,1,0.25", tcolor.RGBColor{R: 128, G: 77, B: 0}},
		{"0.7,1,0.5", tcolor.RGBColor{R: 51, G: 0, B: 255}},
		{"0.7,0.5,0.5", tcolor.RGBColor{R: 89, G: 64, B: 191}},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			parsedColor, err := tcolor.FromString(test.input)
			if err != nil {
				t.Errorf("Failed to parse %q: %v", test.input, err)
				return
			}
			if parsedColor.Basic {
				t.Errorf("Expected advanced color for %q, got %#v", test.input, parsedColor)
				return
			}
			if parsedColor.RGBColor != test.expected {
				t.Errorf("Parsed %q as %v, expected %v", test.input, parsedColor.RGBColor, test.expected)
			}
		})
	}
}
