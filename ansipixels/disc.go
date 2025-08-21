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

type ColorBlendingFunction func(tcolor.RGBColor, tcolor.RGBColor, float64) tcolor.RGBColor

// Draws disc/sphere. aliasing is 0.0 to 1.0 fraction of the disc which is anti-aliased.
// Smaller aliasing the sharper the edge. Larger aliasing the more sphere like effect.
// This version is older and meant to output over a black background (aliases toward 0 lightness).
// Deprecated: use [DiscSRGB] instead.
func (ap *AnsiPixels) Disc(x, y, radius int, hsl tcolor.HSLColor, aliasing float64) {
	// Initial version was staying in HSL space but to reuse same code, we keep converting back and forth.
	ap.DiscBlendFN(x, y, radius, tcolor.RGBColor{}, hsl.RGB(), aliasing, BlendLuminance)
}

// DiscBlend is like [Disc] but blends to the provided background color instead of black and provided blending function.
func (ap *AnsiPixels) DiscBlendFN(
	x, y, radius int,
	background, foreground tcolor.RGBColor, aliasing float64,
	blendFunc ColorBlendingFunction,
) {
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
					ap.WriteString(tcolOut.Background(foreground.Color()))
					inside = true
				}
				ap.WriteRune(' ')
				continue
			}
			ncTop := blendFunc(background, foreground, intTop)
			ncBottom := blendFunc(background, foreground, intBottom)
			ap.WriteString(tcolOut.Background(ncTop.Color()))
			ap.WriteString(tcolOut.Foreground(ncBottom.Color()))
			ap.WriteRune(BottomHalfPixel) // least bad option for Apple Terminal.
		}
	}
	ap.WriteString(tcolor.Reset)
}

func BlendLuminance(_, foreground tcolor.RGBColor, alpha float64) tcolor.RGBColor {
	hsl := foreground.HSL()
	newL := float64(hsl.L) * alpha
	newHSL := tcolor.HSLColor{
		H: hsl.H,
		S: hsl.S,
		L: tcolor.Uint10(math.Round(newL)),
	}
	return newHSL.RGB()
}

// sRGB <-> linear helpers.
// TODO: Consider just precalculating the srgbToLinear at least as a table.
// Or memoize the Blend*().
func srgbToLinear(c uint8) float64 {
	f := float64(c) / 255.0
	if f <= 0.04045 {
		return f / 12.92
	}
	return math.Pow((f+0.055)/1.055, 2.4)
}

func linearToSrgb(f float64) uint8 {
	if f <= 0.0 {
		return 0
	}
	if f >= 1.0 {
		return 255
	}
	var c float64
	if f <= 0.0031308 {
		c = f * 12.92
	} else {
		c = 1.055*math.Pow(f, 1./2.4) - 0.055
	}
	return uint8(math.Round(c * 255.0))
}

// Gamma aware blending (keeps foreground sharper/closer).
func BlendSRGB(bg, fg tcolor.RGBColor, alpha float64) tcolor.RGBColor {
	if alpha < 0 {
		alpha = 0
	} else if alpha > 1 {
		alpha = 1
	}

	// Convert to linear
	bgR, bgG, bgB := srgbToLinear(bg.R), srgbToLinear(bg.G), srgbToLinear(bg.B)
	fgR, fgG, fgB := srgbToLinear(fg.R), srgbToLinear(fg.G), srgbToLinear(fg.B)

	// Blend in linear space
	r := (1-alpha)*bgR + alpha*fgR
	g := (1-alpha)*bgG + alpha*fgG
	b := (1-alpha)*bgB + alpha*fgB

	// Convert back to sRGB
	return tcolor.RGBColor{
		R: linearToSrgb(r),
		G: linearToSrgb(g),
		B: linearToSrgb(b),
	}
}

// Simple linear blend.
func BlendLinear(background, foreground tcolor.RGBColor, alpha float64) tcolor.RGBColor {
	r := (1-alpha)*float64(background.R) + alpha*float64(foreground.R)
	g := (1-alpha)*float64(background.G) + alpha*float64(foreground.G)
	b := (1-alpha)*float64(background.B) + alpha*float64(foreground.B)
	return tcolor.RGBColor{R: uint8(r + 0.5), G: uint8(g + 0.5), B: uint8(b + 0.5)}
}

// DiscSRGB is like [Disc] but blends to the provided background color instead of black
// using SRGB aware (non linear, perceptual) blending.
func (ap *AnsiPixels) DiscSRGB(x, y, radius int, background, foreground tcolor.RGBColor, aliasing float64) {
	ap.DiscBlendFN(x, y, radius, background, foreground, aliasing, BlendSRGB)
}

// DiscLinear is like [Disc] but blends to the provided background color instead of black
// using simple linear blending.
func (ap *AnsiPixels) DiscLinear(x, y, radius int, background, foreground tcolor.RGBColor, aliasing float64) {
	ap.DiscBlendFN(x, y, radius, background, foreground, aliasing, BlendLinear)
}
