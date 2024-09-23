package ansipixels

import (
	"image"
	"image/color"
	_ "image/gif"  // Import GIF decoder
	_ "image/jpeg" // Import JPEG decoder
	_ "image/png"  // Import PNG decoder
	"io"
	"os"

	"fortio.org/log"
	"golang.org/x/image/draw"
)

type Image struct {
	Width  int
	Height int
	Data   []byte
}

const (
	FullPixel       = '█'
	TopHalfPixel    = '▀'
	BottomHalfPixel = '▄'
)

func (ap *AnsiPixels) DrawImage(sx, sy int, img *image.Gray, color string) error {
	ap.WriteAtStr(sx, sy, color)
	var err error
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			pixel1 := img.GrayAt(x, y).Y > 127
			pixel2 := img.GrayAt(x, y+1).Y > 127
			switch {
			case pixel1 && pixel2:
				_, _ = ap.Out.WriteRune(FullPixel)
			case pixel1 && !pixel2:
				_, _ = ap.Out.WriteRune(TopHalfPixel)
			case !pixel1 && pixel2:
				_, _ = ap.Out.WriteRune(BottomHalfPixel)
			case !pixel1 && !pixel2:
				_ = ap.Out.WriteByte(' ')
			}
		}
		sy++
		ap.MoveCursor(sx, sy)
	}
	_, err = ap.Out.WriteString("\033[0m") // reset color
	return err
}

func grayScaleImage(rgbaImg *image.RGBA) *image.Gray {
	grayImg := image.NewGray(rgbaImg.Bounds())
	// Iterate through the pixels of the NRGBA image and convert to grayscale
	for y := rgbaImg.Bounds().Min.Y; y < rgbaImg.Bounds().Max.Y; y++ {
		for x := rgbaImg.Bounds().Min.X; x < rgbaImg.Bounds().Max.X; x++ {
			rgbaColor := rgbaImg.RGBAAt(x, y)

			// Convert to grayscale using the luminance formula
			grayValue := uint8(0.299*float64(rgbaColor.R) + 0.587*float64(rgbaColor.G) + 0.114*float64(rgbaColor.B))

			// Set the gray value in the destination Gray image
			grayImg.SetGray(x, y, color.Gray{Y: grayValue})
		}
	}
	return grayImg
}

func resizeAndCenter(img *image.Gray, maxW, maxH int) *image.Gray {
	// Get original image dimensions
	origBounds := img.Bounds()
	origW := origBounds.Dx()
	origH := origBounds.Dy()

	// Calculate aspect ratio scaling
	scaleW := float64(maxW) / float64(origW)
	scaleH := float64(maxH) / float64(origH)
	scale := min(scaleW, scaleH) // Choose the smallest scale to fit within bounds

	// Calculate new dimensions while preserving aspect ratio
	newW := int(float64(origW) * scale)
	newH := int(float64(origH) * scale)

	canvas := image.NewGray(image.Rect(0, 0, maxW, maxH)) // transparent background (aka black for ANSI)

	// Calculate the offset to center the image
	offsetX := (maxW - newW) / 2
	offsetY := (maxH - newH) / 2

	// Resize the image
	resized := image.NewGray(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(resized, resized.Bounds(), img, origBounds, draw.Over, nil)
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

func (ap *AnsiPixels) ReadImage(path string) (*image.RGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ap.DecodeImage(file)
}

func (ap *AnsiPixels) DecodeImage(data io.Reader) (*image.RGBA, error) {
	// Automatically detect and decode the image format
	img, format, err := image.Decode(data)
	if err != nil {
		return nil, err
	}
	log.Debugf("Image format: %s", format)
	return convertToRGBA(img), nil
}

func (ap *AnsiPixels) ShowImage(imgRGBA *image.RGBA, colorString string) error {
	err := ap.GetSize()
	if err != nil {
		return err
	}
	return ap.DrawImage(1, 1, resizeAndCenter(grayScaleImage(imgRGBA), ap.W-2, 2*ap.H-2), colorString)
}
