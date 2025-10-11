package ansipixels

// Tests/exercising for this code is mostly in `fps`: https://github.com/fortio/fps

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	_ "image/jpeg" // Import JPEG decoder
	_ "image/png"  // Import PNG decoder
	"io"
	"os"
	"time"

	"fortio.org/log"
	"fortio.org/safecast"
	"fortio.org/terminal/ansipixels/tcolor"
	_ "github.com/jbuchbinder/gopnm" // ppm/pnm decoder
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff" // Import tiff decoder
	_ "golang.org/x/image/vp8"  // Import VP8 decoder
	_ "golang.org/x/image/vp8l" // Import VP8L decoder
	_ "golang.org/x/image/webp" // Import WebP decoder
)

func (ap *AnsiPixels) IsBackgroundColor(c color.RGBA) bool {
	return c.A == 0 || (c.R == ap.Background.R && c.G == ap.Background.G && c.B == ap.Background.B)
}

// DrawTrueColorImageTransparent draws a true color image with transparency support, removing pixels of 0 alpha or
// of same color as the Background terminal color. The blending function between background color and
// foreground color is provided as a parameter. [ShowImageScaled] uses [BlendSRGB].
func (ap *AnsiPixels) DrawTrueColorImageTransparent( //nolint:gocognit // yeah...
	sx, sy int,
	img *image.RGBA,
	blendFunc ColorBlendingFunction,
) error {
	ap.MoveCursor(sx, sy)
	var err error
	prevBg := color.RGBA{}
	prevFg := color.RGBA{}
	ap.WriteString(ap.Background.Foreground()) // so initial half pixels only on bg don't show up as white.
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		needPosition := true
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			topPixel := img.RGBAAt(x, y)
			blendedTop := tcolor.RGBColor{R: topPixel.R, G: topPixel.G, B: topPixel.B}
			topIsBg := ap.IsBackgroundColor(topPixel)
			if !topIsBg && topPixel.A < 255 {
				blendedTop = blendFunc(ap.Background, tcolor.RGBColor{R: topPixel.R, G: topPixel.G, B: topPixel.B},
					float64(topPixel.A)/255.0)
			}
			bottomPixel := img.RGBAAt(x, y+1)
			blendedBottom := tcolor.RGBColor{R: bottomPixel.R, G: bottomPixel.G, B: bottomPixel.B}
			bottomIsBg := ap.IsBackgroundColor(bottomPixel)
			if !bottomIsBg && bottomPixel.A < 255 {
				blendedBottom = blendFunc(ap.Background, tcolor.RGBColor{R: bottomPixel.R, G: bottomPixel.G, B: bottomPixel.B},
					float64(bottomPixel.A)/255.0)
			}
			switch {
			case topIsBg && bottomIsBg:
				// In a gap/fully transparent area.
				needPosition = true
				continue
			case topPixel == bottomPixel:
				if topPixel == prevBg && !needPosition {
					ap.WriteRune(' ')
					continue // we haven't changed color
				}
				if needPosition {
					ap.MoveCursor(sx+x, sy)
					needPosition = false
				}
				ap.Printf("%s ", blendedTop.Background())
				prevBg = topPixel
				continue
			case topPixel == prevBg && bottomPixel == prevFg:
				if needPosition {
					ap.MoveCursor(sx+x, sy)
					needPosition = false
				}
				ap.WriteRune('▄')
			default:
				if needPosition {
					ap.MoveCursor(sx+x, sy)
					needPosition = false
				}
				if topIsBg {
					ap.WriteString(ap.Background.Background())
				} else {
					ap.WriteString(blendedTop.Background())
				}
				if bottomIsBg {
					ap.WriteString(ap.Background.Foreground())
				} else {
					ap.WriteString(blendedBottom.Foreground())
				}
				ap.WriteRune('▄')
			}
			prevBg = topPixel
			prevFg = bottomPixel
		}
		sy++
		ap.MoveCursor(sx, sy)
	}
	ap.WriteString(Reset) // reset color
	return err
}

func (ap *AnsiPixels) DrawTrueColorImage(sx, sy int, img *image.RGBA) error {
	ap.MoveCursor(sx, sy)
	var err error
	prevFG := color.RGBA{}
	prevBG := color.RGBA{}
	ap.WriteAtStr(sx, sy, "\033[38;5;0m\033[48;5;0m") // both fg/bg black matching prev1/prev2.
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			topPixel := img.RGBAAt(x, y)
			bottomPixel := img.RGBAAt(x, y+1)
			switch {
			case topPixel == bottomPixel:
				/*
					avoid full pixel because with apple terminal the color bleeds through
					also full pixel is 3 bytes vs 1 for space.
					if topPixel == prevFG {
						ap.WriteRune('█')
						continue // we haven't changed color
					}
				*/
				if bottomPixel == prevBG {
					ap.WriteRune(' ')
					continue // we haven't changed color
				}
				ap.WriteString(fmt.Sprintf("\033[48;2;%d;%d;%dm ", topPixel.R, topPixel.G, topPixel.B))
				prevBG = topPixel // == bottomPixel
				continue
			case bottomPixel == prevFG && topPixel == prevBG:
				ap.WriteRune('▄')
			default:
				ap.WriteString(fmt.Sprintf("\033[48;2;%d;%d;%dm\033[38;2;%d;%d;%dm▄",
					topPixel.R, topPixel.G, topPixel.B,
					bottomPixel.R, bottomPixel.G, bottomPixel.B))
			}
			prevFG = bottomPixel
			prevBG = topPixel
		}
		sy++
		ap.MoveCursor(sx, sy)
	}
	ap.WriteString(Reset) // reset color
	return err
}

func (ap *AnsiPixels) Draw216ColorImage(sx, sy int, img *image.RGBA) error {
	var err error
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		prevFg := uint8(0)
		prevBg := uint8(0)
		ap.WriteAtStr(sx, sy, Reset)
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			topPixel := img.RGBAAt(x, y)
			bottomPixel := img.RGBAAt(x, y+1)
			bgColor := tcolor.RGBATo216(tcolor.RGBColor{R: topPixel.R, G: topPixel.G, B: topPixel.B})
			fgColor := tcolor.RGBATo216(tcolor.RGBColor{R: bottomPixel.R, G: bottomPixel.G, B: bottomPixel.B})
			// Apple's macOS terminal needs lower half pixel or there are gaps where the background shows
			// also, using space instead of full pixel is way better anyway for bytes/pixel.
			switch {
			case fgColor == bgColor:
				if bgColor == prevBg {
					ap.WriteRune(' ')
				} else {
					ap.WriteString(fmt.Sprintf("\033[48;5;%dm ", bgColor))
					prevBg = bgColor
				}
			case fgColor == prevFg && bgColor == prevBg:
				ap.WriteRune('▄')
			case fgColor == prevFg:
				ap.WriteString(fmt.Sprintf("\033[48;5;%dm▄", bgColor))
				prevBg = bgColor
			case bgColor == prevBg:
				ap.WriteString(fmt.Sprintf("\033[38;5;%dm▄", fgColor))
				prevFg = fgColor
			default:
				ap.WriteString(fmt.Sprintf("\033[38;5;%dm\033[48;5;%dm▄", fgColor, bgColor))
				prevFg = fgColor
				prevBg = bgColor
			}
		}
		sy++
	}
	ap.WriteString(Reset) // reset color
	return err
}

func (ap *AnsiPixels) DrawMonoImage(sx, sy int, img *image.Gray, color string) error {
	ap.WriteAtStr(sx, sy, color)
	threshold := uint8(127)
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			pixel1 := img.GrayAt(x, y).Y > threshold
			pixel2 := img.GrayAt(x, y+1).Y > threshold
			switch {
			case pixel1 && pixel2:
				ap.WriteRune(FullPixel)
			case pixel1 && !pixel2:
				ap.WriteRune(TopHalfPixel)
			case !pixel1 && pixel2:
				ap.WriteRune(BottomHalfPixel)
			case !pixel1 && !pixel2:
				_ = ap.Out.WriteByte(' ')
			}
		}
		sy++
		ap.MoveCursor(sx, sy)
	}
	_, err := ap.Out.WriteString(Reset) // reset color
	return err
}

func GrayScaleImage(rgbaImg *image.RGBA) *image.Gray {
	grayImg := image.NewGray(rgbaImg.Bounds())
	ToGray(rgbaImg, grayImg)
	return grayImg
}

func ToGray(rgbaImg *image.RGBA, img image.Image) {
	// Iterate through the pixels of the NRGBA image and convert to grayscale
	for y := rgbaImg.Bounds().Min.Y; y < rgbaImg.Bounds().Max.Y; y++ {
		for x := rgbaImg.Bounds().Min.X; x < rgbaImg.Bounds().Max.X; x++ {
			rgbaColor := rgbaImg.RGBAAt(x, y)

			// Convert to grayscale using the luminance formula
			grayValue := uint8(0.299*float64(rgbaColor.R) + 0.587*float64(rgbaColor.G) + 0.114*float64(rgbaColor.B))

			// Set the gray value in the destination Gray image
			switch grayImg := img.(type) {
			case *image.Gray:
				grayImg.SetGray(x, y, color.Gray{Y: grayValue})
			case *image.RGBA:
				grayImg.Set(x, y, color.RGBA{grayValue, grayValue, grayValue, 255})
			default:
				log.Fatalf("Unsupported image type %T", img)
			}
		}
	}
}

func resizeAndCenter(img *image.RGBA, maxW, maxH int, zoom float64, offsetX, offsetY int) *image.RGBA {
	// Get original image dimensions
	origBounds := img.Bounds()
	origW := origBounds.Dx()
	origH := origBounds.Dy()

	// Calculate aspect ratio scaling
	scaleW := float64(maxW) / float64(origW)
	scaleH := float64(maxH) / float64(origH)
	scale := min(scaleW, scaleH) // Choose the smallest scale to fit within bounds
	scale *= zoom

	// Calculate new dimensions while preserving aspect ratio
	newW := int(float64(origW) * scale)
	newH := int(float64(origH) * scale)

	canvas := image.NewRGBA(image.Rect(0, 0, maxW, maxH))

	// Calculate the offset to center the image
	offsetX += (maxW - newW) / 2
	offsetY += (maxH - newH) / 2

	// Resize the image
	resized := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.BiLinear.Scale(resized, resized.Bounds(), img, origBounds, draw.Over, nil)
	draw.Draw(canvas, image.Rect(offsetX, offsetY, offsetX+newW, offsetY+newH), resized, image.Point{}, draw.Over)
	return canvas
}

func convertToRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
	return dst
}

func (ap *AnsiPixels) ReadImage(path string) (*Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ap.DecodeImage(file)
}

type Image struct {
	Format string
	Width  int
	Height int
	Images []*image.RGBA
	Delays []int
}

func (ap *AnsiPixels) DecodeImage(inp io.Reader) (*Image, error) {
	// Automatically detect and decode the image format
	all, err := io.ReadAll(inp)
	if err != nil {
		return nil, err
	}
	data := bytes.NewReader(all)
	img, format, err := image.Decode(data)
	if err != nil {
		return nil, err
	}
	log.Debugf("Image format: %s", format)
	res := &Image{
		Format: format,
		Width:  img.Bounds().Dx(),
		Height: img.Bounds().Dy(),
	}
	if format != "gif" {
		res.Images = []*image.RGBA{convertToRGBA(img)}
		return res, nil
	}
	data = bytes.NewReader(all)
	gifImages, err := gif.DecodeAll(data)
	if err != nil {
		return nil, err
	}
	res.Images = make([]*image.RGBA, 0, len(gifImages.Image))
	bounds := gifImages.Image[0].Bounds()
	current := image.NewRGBA(bounds)
	for _, frame := range gifImages.Image {
		// TODO use Disposal[i] correctly.
		draw.Draw(current, bounds, frame, image.Point{}, draw.Over) // Composite each frame onto the canvas
		// make a imgCopy of the current frame
		imgCopy := image.NewRGBA(bounds)
		draw.Draw(imgCopy, bounds, current, image.Point{}, draw.Src)
		res.Images = append(res.Images, imgCopy)
	}
	res.Delays = gifImages.Delay
	return res, nil
}

// ShowImages a series of images (eg from decoding animated gif), scaling them to fit given the zoom and offsets.
// Color used depends on TrueColor, Color or Gray settings. MonoColor is the default if no color is desired.
// See the `fps` image viewer for example of use.
func (ap *AnsiPixels) ShowImages(imagesRGBA *Image, zoom float64, offsetX, offsetY int) error {
	// GetSize done in Open and Resize handler.
	for i, imgRGBA := range imagesRGBA.Images {
		img := resizeAndCenter(imgRGBA, ap.W-2*ap.Margin, 2*ap.H-4*ap.Margin, zoom, offsetX, offsetY)
		if i > 0 {
			ap.StartSyncMode()
		}
		err := ap.ShowScaledImage(img)
		if err != nil {
			return err
		}
		if i < len(imagesRGBA.Delays)-1 { // maybe read keyboard/signal for stop request in case this is longish.
			if i > 0 {
				ap.EndSyncMode()
			}
			delay := imagesRGBA.Delays[i]
			log.Debugf("Delay %d", delay)
			if delay > 0 {
				time.Sleep(time.Duration(delay) * 10 * time.Millisecond)
			}
		}
	}
	return nil
}

// ShowScaledImage writes an image to the terminal.
// It must already have the right size to fit exactly in width/height within margins.
func (ap *AnsiPixels) ShowScaledImage(img *image.RGBA) error {
	if ap.Gray {
		ToGray(img, img)
	}
	var err error
	switch {
	case ap.TrueColor:
		if ap.Transparency {
			err = ap.DrawTrueColorImageTransparent(ap.Margin, ap.Margin, img, BlendSRGB)
		} else {
			err = ap.DrawTrueColorImage(ap.Margin, ap.Margin, img)
		}
	case ap.Color256:
		err = ap.Draw216ColorImage(ap.Margin, ap.Margin, img)
	default:
		err = ap.DrawMonoImage(ap.Margin, ap.Margin, GrayScaleImage(img), ap.MonoColor.Foreground())
	}
	return err
}

// NRGBA to RGBA (from grol images extension initially)

// NRGBAtoRGBA converts a non-premultiplied alpha color to a premultiplied alpha color.
func NRGBAtoRGBA(c color.NRGBA) color.RGBA {
	if c.A == 0xFF {
		return color.RGBA(c)
	}
	if c.A == 0 {
		return color.RGBA{0, 0, 0, 0}
	}
	// Convert non-premultiplied alpha to premultiplied alpha
	// RGBA = (R * A/255, G * A/255, B * A/255, A)
	return color.RGBA{
		R: safecast.MustConv[uint8](uint16(c.R) * uint16(c.A) / 255),
		G: safecast.MustConv[uint8](uint16(c.G) * uint16(c.A) / 255),
		B: safecast.MustConv[uint8](uint16(c.B) * uint16(c.A) / 255),
		A: c.A,
	}
}

func AddPixel(img *image.RGBA, x, y int, c color.RGBA) {
	p1 := img.RGBAAt(x, y)
	if p1.R == 0 && p1.G == 0 && p1.B == 0 { // black is no change
		img.SetRGBA(x, y, c)
		return
	}
	if c.R == 0 && c.G == 0 && c.B == 0 { // black is no change
		return
	}
	// gosec not smart enough to see the range checks with min - https://github.com/securego/gosec/issues/1212
	// when it does we can remove the MustConv(s).
	p1.R = safecast.MustConv[uint8](min(255, uint16(p1.R)+uint16(c.R)))
	p1.G = safecast.MustConv[uint8](min(255, uint16(p1.G)+uint16(c.G)))
	p1.B = safecast.MustConv[uint8](min(255, uint16(p1.B)+uint16(c.B)))
	// p1.A = uint8(min(255, uint16(p1.A)+uint16(p2.A))) // summing transparency yield non transparent quickly
	p1.A = max(p1.A, c.A)
	img.SetRGBA(x, y, p1)
}
