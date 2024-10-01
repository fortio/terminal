package ansipixels

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
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff" // Import tiff decoder
	_ "golang.org/x/image/vp8"  // Import VP8 decoder
	_ "golang.org/x/image/vp8l" // Import VP8L decoder
	_ "golang.org/x/image/webp" // Import WebP decoder
)

func (ap *AnsiPixels) DrawTrueColorImage(sx, sy int, img *image.RGBA) error {
	ap.MoveCursor(sx, sy)
	var err error
	prev1 := color.RGBA{}
	prev2 := color.RGBA{}
	ap.WriteAt(sx, sy, "\033[38;5;%dm\033[48;5;%dm", 0, 0)
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			pixel1 := img.RGBAAt(x, y)
			pixel2 := img.RGBAAt(x, y+1)
			switch {
			case pixel1 == pixel2:
				if pixel1 == prev1 {
					ap.WriteRune('█')
					continue // we haven't changed color
				}
				if pixel2 == prev2 {
					ap.WriteRune(' ')
					continue // we haven't changed color
				}
				ap.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm█", pixel1.R, pixel1.G, pixel1.B))
				prev1 = pixel1
				continue
			case pixel1 == prev1 && pixel2 == prev2:
				ap.WriteRune('▀')
			default:
				ap.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀",
					pixel1.R, pixel1.G, pixel1.B,
					pixel2.R, pixel2.G, pixel2.B))
			}
			prev1 = pixel1
			prev2 = pixel2
		}
		sy++
		ap.MoveCursor(sx, sy)
	}
	ap.WriteString("\033[0m") // reset color
	return err
}

func convertColorTo216(pixel color.RGBA) uint8 {
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

func (ap *AnsiPixels) Draw216ColorImage(sx, sy int, img *image.RGBA) error {
	var err error
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y += 2 {
		prevFg := uint8(0)
		prevBg := uint8(0)
		ap.WriteAtStr(sx, sy, "\033[0m")
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			pixel1 := img.RGBAAt(x, y)
			pixel2 := img.RGBAAt(x, y+1)
			fgColor := convertColorTo216(pixel1)
			bgColor := convertColorTo216(pixel2)
			switch {
			case fgColor == prevFg && bgColor == prevBg:
				ap.WriteRune('▄')
			case fgColor == prevFg:
				ap.WriteString(fmt.Sprintf("\033[38;5;%dm▄", bgColor))
			case bgColor == prevBg:
				ap.WriteString(fmt.Sprintf("\033[48;5;%dm▄", fgColor))
			default:
				// Apple's macOS terminal needs lower half pixel or there are gaps where the background shows.
				ap.WriteString(fmt.Sprintf("\033[38;5;%dm\033[48;5;%dm▄", bgColor, fgColor))
			}
			prevFg = fgColor
			prevBg = bgColor
		}
		sy++
	}
	ap.WriteString("\033[0m") // reset color
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
	_, err := ap.Out.WriteString("\033[0m") // reset color
	return err
}

func grayScaleImage(rgbaImg *image.RGBA) *image.Gray {
	grayImg := image.NewGray(rgbaImg.Bounds())
	toGrey(rgbaImg, grayImg)
	return grayImg
}

func toGrey(rgbaImg *image.RGBA, img image.Image) {
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

// Color string is the fallback mono color to use when AnsiPixels.TrueColor is false.
func (ap *AnsiPixels) ShowImage(imagesRGBA *Image, zoom float64, offsetX, offsetY int, colorString string) error {
	// GetSize done in Open and Resize handler.
	for i, imgRGBA := range imagesRGBA.Images {
		img := resizeAndCenter(imgRGBA, ap.W-2*ap.Margin, 2*ap.H-4*ap.Margin, zoom, offsetX, offsetY)
		if ap.Gray {
			toGrey(img, img)
		}
		var err error
		switch {
		case ap.TrueColor:
			err = ap.DrawTrueColorImage(ap.Margin, ap.Margin, img)
		case ap.Color:
			err = ap.Draw216ColorImage(ap.Margin, ap.Margin, img)
		default:
			err = ap.DrawMonoImage(ap.Margin, ap.Margin, grayScaleImage(img), colorString)
		}
		if err != nil {
			return err
		}
		ap.Out.Flush()
		if i < len(imagesRGBA.Delays)-1 { // maybe read keyboard/signal for stop request in case this is longish.
			delay := imagesRGBA.Delays[i]
			log.Debugf("Delay %d", delay)
			if delay > 0 {
				time.Sleep(time.Duration(delay) * 10 * time.Millisecond)
			}
		}
	}
	return nil
}
