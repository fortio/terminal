package main

import (
	"flag"
	"image"
	"image/png"
	"math"
	"os"

	"fortio.org/terminal/ansipixels"
)

func main() {
	aaFlag := flag.Bool("aa", false, "Use anti-aliasing")
	flag.Parse()
	width := 1024
	height := 1024
	radius := min(width, height)/2 - 10

	// Create a new RGBA image
	img := image.NewNRGBA(image.Rect(0, 0, width, height))

	// Draw lines from the bottom-left corner to the right edge, spaced by 10 pixels in height

	// Draw lines radiating from the center of the image
	centerX := float64(width) / 2  // + 0.5
	centerY := float64(height) / 2 // + 0.5

	// Number of lines and angle between them
	numLines := 360
	for i := 0; i < numLines; i += 2 {
		lineColor := ansipixels.HSLToRGB(float64(i)/float64(numLines), 1, 0.5)

		angle := float64(i) * (2 * math.Pi / float64(numLines))
		// Compute the endpoint using angle
		x := centerX + math.Cos(angle)*float64(radius)
		y := centerY + math.Sin(angle)*float64(radius)
		// Draw the line from the center to the edge
		if *aaFlag {
			ansipixels.DrawAALine(img, centerX, centerY, x, y, lineColor)
		} else {
			ansipixels.DrawLine(img, centerX, centerY, x, y, lineColor)
		}
	}

	// Save the result to a PNG file
	fname := "color_lines.png"
	if *aaFlag {
		fname = "aa_" + fname
	}
	file, _ := os.Create(fname)
	defer file.Close()
	_ = png.Encode(file, img)
}
