package ansipixels

import (
	"math"

	"fortio.org/terminal/ansipixels/tcolor"
)

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func intensity(x, y, radius int, aliasing float64) float64 {
	r := float64(radius * radius)
	fx := float64(abs(x)) + 0.5
	fy := float64(abs(y)) + 0.5
	d := fx*fx + fy*fy
	if d > r {
		return 0
	}
	edgeDistance := math.Sqrt(r - d)
	if edgeDistance > aliasing*float64(radius) {
		return 1 // full intensity
	}
	return edgeDistance / float64(radius) / aliasing
}

// Draws disc/sphere. aliasing is 0.0 to 1.0 fraction of the disc which is anti-aliased.
// Smaller aliasing the sharper the edge. Larger aliasing the more sphere like effect.
func (ap *AnsiPixels) Disc(x, y, radius int, hsl tcolor.HSLColor, aliasing float64) {
	tcolOut := tcolor.ColorOutput{TrueColor: ap.TrueColor}
	for j := -radius; j <= radius; j += 2 {
		first := true
		inside := false
		for i := -radius; i <= radius; i++ {
			xx := x + i
			yy := y + j/2
			if xx < 0 || yy < 0 || xx >= ap.W || yy >= ap.H {
				continue // skip out of bounds
			}
			intTop := intensity(i, j, radius, aliasing)
			intBottom := intensity(i, j+1, radius, aliasing)
			if intTop == 0 && intBottom == 0 {
				continue // skip if not in the disc
			}
			if first {
				ap.MoveCursor(xx, yy)
				first = false
			}
			if intTop == 1 && intBottom == 1 {
				if !inside {
					ap.WriteString(tcolOut.Background(hsl.Color()))
					inside = true
				}
				ap.WriteRune(' ')
				continue
			}
			newTopL := float64(hsl.L) * intTop
			newBottomL := float64(hsl.L) * intBottom
			ncTop := hsl
			ncTop.L = tcolor.Uint10(math.Round(newTopL))
			ncBottom := hsl
			ncBottom.L = tcolor.Uint10(math.Round(newBottomL))
			ap.WriteString(tcolOut.Background(ncTop.Color()))
			ap.WriteString(tcolOut.Foreground(ncBottom.Color()))
			ap.WriteRune(BottomHalfPixel)
		}
	}
	ap.WriteString(tcolor.Reset)
}
