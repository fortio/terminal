package ansipixels

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // Import GIF decoder
	_ "image/jpeg" // Import JPEG decoder
	_ "image/png"  // Import PNG decoder
	"io"
	"os"

	"fortio.org/log"
	"fortio.org/safecast"
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

func (ap *AnsiPixels) DrawTrueColorImage(sx, sy int, img *image.RGBA) error {
	ap.MoveCursor(sx, sy)
	var err error
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		prev1 := color.RGBA{}
		prev2 := color.RGBA{}
		ap.WriteAt(sx, sy, "\033[38;5;%dm\033[48;5;%dm", 0, 0)
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			pixel1 := img.RGBAAt(x, y)
			pixel2 := img.RGBAAt(x, y+1)
			switch {
			case pixel1 == pixel2:
				if pixel1 == prev1 {
					_, _ = ap.Out.WriteRune('█')
					continue // we haven't changed color
				}
				if pixel2 == prev2 {
					_, _ = ap.Out.WriteRune(' ')
					continue // we haven't changed color
				}
				_, _ = ap.Out.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm█", pixel1.R, pixel1.G, pixel1.B))
				prev1 = pixel1
				continue
			case pixel1 == prev1 && pixel2 == prev2:
				_, _ = ap.Out.WriteRune('▀')
			default:
				_, _ = ap.Out.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀",
					pixel1.R, pixel1.G, pixel1.B,
					pixel2.R, pixel2.G, pixel2.B))
			}
			prev1 = pixel1
			prev2 = pixel2
		}
		sy++
		ap.MoveCursor(sx, sy)
	}
	_, err = ap.Out.WriteString("\033[0m") // reset color
	return err
}

func convertColorTo216(pixel color.RGBA) uint8 {
	// Check if grayscale
	shift := 2
	if (pixel.R>>shift) == (pixel.G>>shift) && (pixel.G>>shift) == (pixel.B>>shift) {
		// Bugged:
		// lum := safecast.MustConvert[uint8](max(255, math.Round(0.299*float64(pixel.R)+
		// 0.587*float64(pixel.G)+0.114*float64(pixel.B))))
		lum := (uint16(pixel.R) + uint16(pixel.G) + uint16(pixel.B)) / 3
		return 232 + safecast.MustConvert[uint8](lum*23/255)
	}
	// 6x6x6 color cube
	col := 16 + 36*(pixel.R/51) + 6*(pixel.G/51) + pixel.B/51
	return col
}

func (ap *AnsiPixels) Draw216ColorImage(sx, sy int, img *image.RGBA) error {
	var err error
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		prevFg := uint8(0)
		prevBg := uint8(0)
		ap.WriteAt(sx, sy, "\033[38;5;%dm\033[48;5;%dm", 0, 0)
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			pixel1 := img.RGBAAt(x, y)
			pixel2 := img.RGBAAt(x, y+1)
			fgColor := convertColorTo216(pixel1)
			bgColor := convertColorTo216(pixel2)
			switch {
			case fgColor == prevFg && bgColor == prevBg:
				_, _ = ap.Out.WriteRune('▄')
				/*
					case fgColor == bgColor:
						if fgColor == prevFg {
							_, _ = ap.Out.WriteRune('█')
							continue // we haven't changed bg color
						}
						if bgColor == prevBg {
							_, _ = ap.Out.WriteRune(' ')
							continue // we haven't changed fg color
						}
						_, _ = ap.Out.WriteString(fmt.Sprintf("\033[38;5;%dm█", fgColor))
						prevFg = fgColor
						continue
							case fgColor == prevFg:
								_, _ = ap.Out.WriteString(fmt.Sprintf("\033[48;5;%dm▄", bgColor))
							case bgColor == prevBg:
								_, _ = ap.Out.WriteString(fmt.Sprintf("\033[38;5;%dm▄", fgColor))
				*/
			default:
				_, _ = ap.Out.WriteString(fmt.Sprintf("\033[38;5;%dm\033[48;5;%dm▄", bgColor, fgColor))
			}
			prevFg = fgColor
			prevBg = bgColor
		}
		sy++
	}
	_, err = ap.Out.WriteString("\033[0m") // reset color
	return err
}

func (ap *AnsiPixels) DrawMonoImage(sx, sy int, img *image.Gray, color string) error {
	ap.WriteAtStr(sx, sy, color)
	var err error
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			pixel1 := img.GrayAt(x, y).Y > 90
			pixel2 := img.GrayAt(x, y+1).Y > 90
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

func resizeAndCenter(img *image.RGBA, maxW, maxH int) *image.RGBA {
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

	canvas := image.NewRGBA(image.Rect(0, 0, maxW, maxH))

	// Calculate the offset to center the image
	offsetX := (maxW - newW) / 2
	offsetY := (maxH - newH) / 2

	// Resize the image
	resized := image.NewRGBA(image.Rect(0, 0, newW, newH))
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

// Color string is the fallback mono color to use when AnsiPixels.TrueColor is false.
func (ap *AnsiPixels) ShowImage(imgRGBA *image.RGBA, colorString string) error {
	err := ap.GetSize()
	if err != nil {
		return err
	}
	switch {
	case ap.TrueColor:
		return ap.DrawTrueColorImage(1, 1, resizeAndCenter(imgRGBA, ap.W-2, 2*ap.H-2))
	case ap.Color:
		return ap.Draw216ColorImage(1, 1, resizeAndCenter(imgRGBA, ap.W-2, 2*ap.H-2))
	default:
		return ap.DrawMonoImage(1, 1, grayScaleImage(resizeAndCenter(imgRGBA, ap.W-2, 2*ap.H-2)), colorString)
	}
}
