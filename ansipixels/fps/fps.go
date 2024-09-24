package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"image"
	"os"
	"strconv"
	"time"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/safecast"
	"fortio.org/terminal/ansipixels"
)

func main() {
	os.Exit(Main())
}

func isStopKey(ap *ansipixels.AnsiPixels) bool {
	// q, ^C, ^D to exit.
	for _, key := range ap.Data {
		if key == 12 { // ^L
			_ = ap.GetSize()
			ap.ClearScreen()
			drawBox(ap)
			ap.Data = ap.Data[0:0:cap(ap.Data)] // reset buffer
			return false
		}
		if key == 'q' || key == 3 || key == 4 {
			return true
		}
	}
	return false
}

func drawBox(ap *ansipixels.AnsiPixels) {
	_ = ap.DrawSquareBox(0, 0, ap.W, ap.H)
	ap.WriteBoxed(ap.H/2-3, " Width: %d, Height: %d ", ap.W, ap.H)
}

func posToXY(pos int, w, h int) (int, int) {
	mod := 2*w + 2*h
	for pos < 0 {
		pos += mod
	}
	pos %= mod
	if pos < w {
		return pos, 0
	}
	pos -= w
	if pos < h {
		return w - 1, pos
	}
	pos -= h
	if pos < w {
		return w - 1 - pos, h - 1
	}
	return 0, h - 1 - pos + w
}

func charAt(ap *ansipixels.AnsiPixels, pos, w, h int, what string) {
	x, y := posToXY(pos, w, h)
	ap.WriteAtStr(x+ap.Margin, y+ap.Margin, what)
}

func animate(ap *ansipixels.AnsiPixels, frame uint) {
	w := ap.W
	h := ap.H
	w -= 2 * ap.Margin
	h -= 2 * ap.Margin
	total := 2*w + 2*h
	pos := safecast.MustConvert[int](frame % safecast.MustConvert[uint](total))
	charAt(ap, pos+2, w, h, "\033[31m█") // Red
	charAt(ap, pos+1, w, h, "\033[32m█") // Green
	charAt(ap, pos, w, h, "\033[34m█")   // Blue
	charAt(ap, pos-1, w, h, "\033[0m ")  // erase and reset color
}

//go:embed fps.jpg
var fpsJpg []byte

//go:embed fps_colors.jpg
var fpsColorsJpg []byte

func imagesViewer(ap *ansipixels.AnsiPixels, imageFiles []string) int {
	ap.ClearScreen()
	ap.HideCursor()
	ap.Data = make([]byte, 1)
	for _, imageFile := range imageFiles {
		img, err := ap.ReadImage(imageFile)
		if err != nil {
			return log.FErrf("Error reading image: %v", err)
		}
		if err = ap.ShowImage(img, "\033[34m"); err != nil {
			return log.FErrf("Error showing image: %v", err)
		}
		ap.Out.Flush()
		_, err = ap.In.Read(ap.Data)
		if err != nil {
			return log.FErrf("Error with cursor position request: %v", err)
		}
		if isStopKey(ap) {
			return 0
		}
	}
	return 0
}

func Main() int { //nolint:funlen,gocognit // color and mode if/else are a bit long.
	defaultTrueColor := false
	if os.Getenv("COLORTERM") != "" {
		defaultTrueColor = true
	}
	defaultColor := false
	if os.Getenv("TERM") == "xterm-256color" {
		defaultColor = true
	}
	imgFlag := flag.String("image", "", "Image file to display in monochrome in the background instead of the default one")
	colorFlag := flag.Bool("color", defaultColor,
		"If your terminal supports color, this will load image in (216) colors instead of monochrome")
	trueColorFlag := flag.Bool("truecolor", defaultTrueColor,
		"If your terminal supports truecolor, this will load image in truecolor (24bits) instead of monochrome")
	grayFlag := flag.Bool("gray", false, "Convert the image to grayscale")
	noboxFlag := flag.Bool("nobox", false,
		"Don't draw the box around the image, make the image full screen instead of 1 pixel less on all sides")
	imagesOnlyFlag := flag.Bool("i", false, "Arguments are now images files to show, no FPS test (hit any key to continue)")
	cli.MinArgs = 0
	cli.MaxArgs = -1
	cli.ArgsHelp = "[maxfps] or fps -i imagefiles..."
	cli.Main()
	imagesOnly := *imagesOnlyFlag
	fpsLimit := -1.0
	fpsStr := "unlimited"
	hasFPSLimit := false
	if !imagesOnly && len(flag.Args()) > 0 {
		// parse as float64
		var err error
		fpsLimit, err = strconv.ParseFloat(flag.Arg(0), 64)
		if err != nil {
			return log.FErrf("Invalid maxfps: %v", err)
		}
		if fpsLimit < 0 {
			return log.FErrf("Invalid maxfps: %v", fpsLimit)
		}
		fpsStr = fmt.Sprintf("%.1f", fpsLimit)
		hasFPSLimit = true
	}
	ap := ansipixels.NewAnsiPixels()
	if err := ap.Open(); err != nil {
		log.Fatalf("Not a terminal: %v", err)
	}
	ap.TrueColor = *trueColorFlag
	ap.Color = *colorFlag
	ap.Gray = *grayFlag
	ap.Margin = 1
	if *noboxFlag || imagesOnly {
		ap.Margin = 0
	}
	defer func() {
		ap.ShowCursor()
		ap.MoveCursor(0, ap.H-2)
		ap.Out.Flush()
		ap.Restore()
	}()
	if err := ap.GetSize(); err != nil {
		return log.FErrf("Error getting terminal size: %v", err)
	}
	ap.ClearScreen()
	if imagesOnly && len(flag.Args()) > 0 {
		return imagesViewer(ap, flag.Args())
	}
	var background *image.RGBA
	var err error
	if *imgFlag == "" {
		if *trueColorFlag || *colorFlag {
			background, err = ap.DecodeImage(bytes.NewReader(fpsColorsJpg))
		} else {
			background, err = ap.DecodeImage(bytes.NewReader(fpsJpg))
		}
	} else {
		background, err = ap.ReadImage(*imgFlag)
	}
	if err != nil {
		return log.FErrf("Error reading image: %v", err)
	}
	if err = ap.ShowImage(background, "\033[34m"); err != nil {
		return log.FErrf("Error showing image: %v", err)
	}
	buf := [256]byte{}
	if imagesOnly {
		ap.HideCursor()
		ap.Out.Flush()
		_, _ = ap.In.Read(buf[:])
		return 0
	}
	if !*noboxFlag {
		drawBox(ap)
	}
	// FPS test
	fps := 0.0
	// sleep := 1 * time.Second / time.Duration(fps)
	ap.WriteCentered(ap.H/2+3, "FPS %s test... any key to start; q, ^C, or ^D to exit... \033[1D", fpsStr)
	ap.Out.Flush()
	_, err = ap.In.Read(buf[:])
	if err != nil {
		return log.FErrf("Error reading key: %v", err)
	}
	ap.HideCursor()
	// _, _ = ap.Out.WriteString("\033[?2026h") // sync mode // doesn't seem to do anything
	frames := uint(0)
	var elapsed time.Duration
	var entry []byte
	ap.SignalChannel()
	sendableTickerChan := make(chan time.Time, 1)
	var tickerChan <-chan time.Time
	startTime := time.Now()
	now := startTime
	if hasFPSLimit {
		ticker := time.NewTicker(time.Second / time.Duration(fpsLimit))
		tickerChan = ticker.C
	} else {
		tickerChan = sendableTickerChan
		sendableTickerChan <- now
	}
	for {
		select {
		case s := <-ap.C:
			if ap.IsResizeSignal(s) {
				_ = ap.GetSize()
				ap.ClearScreen()
				_ = ap.ShowImage(background, "\033[34m")
				if !*noboxFlag {
					drawBox(ap)
				}
				continue
			}
			return 0
		case <-tickerChan:
			elapsed = time.Since(now)
			fps = 1. / elapsed.Seconds()
			now = time.Now()
			ap.WriteAt(ap.W/2-20, ap.H/2, " Last frame %v FPS: %.0f Avg %.2f ",
				elapsed.Round(10*time.Microsecond), fps, float64(frames)/now.Sub(startTime).Seconds())
			/*
				ap.ClearEndOfLine()
				ap.MoveHorizontally(ap.W - 1)
				_, _ = ap.Out.WriteString(ansipixels.Vertical)
			*/
			animate(ap, frames)
			// Request cursor position (note that FPS is about the same without it, the Flush seems to be enough)
			_, _, err = ap.ReadCursorPos()
			if err != nil {
				return log.FErrf("Error with cursor position request: %v", err)
			}
			// q, ^C, ^D to exit.
			if isStopKey(ap) {
				return 0
			}
			entry = append(entry, ap.Data...)
			ap.WriteCentered(ap.H/2+5, "Entry so far: [%q]", entry)
			ap.Data = ap.Data[0:0:cap(ap.Data)] // reset buffer
			frames++
			if !hasFPSLimit {
				sendableTickerChan <- now
			}
		}
	}
}
