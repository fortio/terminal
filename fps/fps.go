package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"fortio.org/cli"
	"fortio.org/fortio/periodic"
	"fortio.org/fortio/stats"
	"fortio.org/log"
	"fortio.org/safecast"
	"fortio.org/terminal"
	"fortio.org/terminal/ansipixels"
	"github.com/loov/hrtime" // To test hrtime correctness: hrtime "time".
)

const defaultMonoImageColor = ansipixels.Blue // ansi blue-ish

func jsonOutput(jsonFileName string, data any) {
	var j []byte
	j, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Unable to json serialize result: %v", err)
	}
	var f *os.File
	f, err = os.Create(jsonFileName)
	if err != nil {
		log.Fatalf("Unable to create %s: %v", jsonFileName, err)
	}
	n, err := f.Write(append(j, '\n'))
	if err != nil {
		log.Fatalf("Unable to write json to %s: %v", jsonFileName, err)
	}
	err = f.Close()
	if err != nil {
		log.Fatalf("Close error for %s: %v", jsonFileName, err)
	}
	fmt.Printf("Successfully wrote %d bytes of Json data (visualize with %sfortio report%s):\n%s\n",
		n, ansipixels.Cyan, ansipixels.Reset, jsonFileName)
}

var (
	perfResults = &Results{
		QPSLabel:      "FPS",
		ResponseLabel: "Frame duration",
		RetCodes:      make(map[string]int64),
	}
	noJSON = flag.Bool("nojson", false,
		"Don't output json file with results that otherwise get produced and can be visualized with fortio report")
)

type Results struct {
	periodic.RunnerResults
	RetCodes      map[string]int64
	Destination   string // shows up in fortio graph title
	StartTime     time.Time
	QPSLabel      string
	ResponseLabel string
	hist          *stats.Histogram
}

func main() {
	ret := Main()
	if !*noJSON && perfResults.hist != nil && perfResults.hist.Count > 0 {
		perfResults.DurationHistogram = perfResults.hist.Export().CalcPercentiles([]float64{50, 75, 90, 99, 99.9})
		perfResults.RetCodes["OK"] = perfResults.hist.Count
		perfResults.RunType = "FPS"
		ro := &periodic.RunnerOptions{
			Labels:  perfResults.Labels,
			RunType: perfResults.RunType,
		}
		ro.GenID()
		perfResults.ID = ro.ID
		fname := ro.ID + ".json"
		jsonOutput(fname, perfResults)
	}
	os.Exit(ret)
}

func isStopKey(ap *ansipixels.AnsiPixels) bool {
	// q, ^C, ^D to exit.
	cleaned, _ := ansipixels.AnsiClean(ap.Data)
	for _, key := range cleaned {
		if key == 'q' || key == 'Q' || key == 3 || key == 4 {
			log.Debugf("Exiting on key %q from %q / %q", key, cleaned, ap.Data)
			return true
		}
	}
	return false
}

func drawBox(ap *ansipixels.AnsiPixels, withText bool) {
	if ap.Margin != 0 {
		ap.DrawSquareBox(0, 0, ap.W, ap.H)
	}
	if withText {
		ap.WriteBoxed(ap.H/2-3, " Width: %d, Height: %d ", ap.W, ap.H)
	}
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

func animate(ap *ansipixels.AnsiPixels, frame int64) {
	w := ap.W
	h := ap.H
	w -= 2 * ap.Margin
	h -= 2 * ap.Margin
	total := 2*w + 2*h
	pos := safecast.MustConvert[int](frame % safecast.MustConvert[int64](total))
	charAt(ap, pos+2, w, h, ansipixels.RedPixel)   // Red
	charAt(ap, pos+1, w, h, ansipixels.GreenPixel) // ansipixels.Green
	charAt(ap, pos, w, h, ansipixels.BluePixel)    // Blue
	charAt(ap, pos-1, w, h, ansipixels.ResetClear) // erase and reset color
}

//go:embed fps.jpg
var fpsJpg []byte

//go:embed fps_colors.jpg
var fpsColorsJpg []byte

func mouseOffets(ap *ansipixels.AnsiPixels, offsetX, offsetY *int) {
	dx := float64(ap.Mx - ap.W/2)
	dy := float64(2 * (ap.My - ap.H/2))
	*offsetX -= safecast.MustRound[int](dx)
	*offsetY -= safecast.MustRound[int](dy)
}

func imagesViewer(ap *ansipixels.AnsiPixels, imageFiles []string) int { //nolint:funlen,gocyclo // yeah well...
	ap.Data = make([]byte, 3)
	i := 0
	l := len(imageFiles)
	showInfo := l > 1
	changedInfo := false
	for {
		zoom := 1.0
		offsetX := 0
		offsetY := 0
		if i >= l {
			i = 0
		}
		imageFile := imageFiles[i]
		img, err := ap.ReadImage(imageFile)
		if err != nil {
			return log.FErrf("Error reading image %s: %v", imageFile, err)
		}
		extra := ""
		if l > 1 {
			extra = fmt.Sprintf(", %d/%d", i+1, l)
		}
		info := fmt.Sprintf("%s (%dx%d %s%s)", imageFile, img.Width, img.Height, img.Format, extra)
		ap.OnResize = func() error {
			ap.StartSyncMode()
			ap.ClearScreen()
			/* todo: prevent image from going off screen
			offsetX = max(-ap.W+1, offsetX)
			offsetX = min(2*ap.W-1, offsetX)
			offsetY = max(-ap.H+1, offsetY)
			offsetY = min(2*ap.H-1, offsetY)
			*/
			e := ap.ShowImage(img, zoom, offsetX, offsetY, defaultMonoImageColor)
			if showInfo {
				ap.WriteRight(ap.H-1, "%s", info)
			}
			ap.EndSyncMode()
			return e
		}
	redraw:
		if err = ap.OnResize(); err != nil {
			return log.FErrf("Error showing image: %v", err)
		}
	wait:
		// read a key or resize signal or stop signal
		err = ap.ReadOrResizeOrSignal()
		if errors.Is(err, terminal.ErrSignal) {
			return 0
		}
		if err != nil {
			return log.FErrf("Error reading key: %v", err)
		}
		switch {
		case ap.MouseWheelUp():
			zoom *= 1.25
			goto redraw
		case ap.MouseWheelDown():
			zoom /= 1.25
			goto redraw
		case ap.LeftClick():
			mouseOffets(ap, &offsetX, &offsetY)
			goto redraw
		case len(ap.Data) == 0:
			goto redraw
		}
		// ap.Data is set by ReadOrResizeOrSignal and ~~can't be empty~~ (mouse clicks can have it empty) if no error was returned
		largeSteps := int(10. * zoom)
		c := ap.Data[0]
		justRedraw := true
		switch c {
		case '?', 'h', 'H':
			ap.WriteCentered(ap.H/2-1, "Showing %d out of %d images, hit any key to continue, up/down for zoom,", i+1, l)
			ap.WriteCentered(ap.H/2, "WSAD to pan, 'q' to exit, left arrow to go back, 'i' to toggle image information")
			ap.WriteCentered(ap.H/2+1, "or Mouse wheel to zoom, Mouse click center; 'c' to reset to center of the image.")
			ap.Out.Flush()
			goto wait
		case 'i', 'I':
			changedInfo = true
			showInfo = !showInfo
		case 'W':
			offsetY -= largeSteps
		case 'S':
			offsetY += largeSteps
		case 'A':
			offsetX -= largeSteps
		case 'D':
			offsetX += largeSteps
		case 'w':
			offsetY--
		case 's':
			offsetY++
		case 'a':
			offsetX--
		case 'd':
			offsetX++
		case 'c', 'C':
			offsetX = 0
			offsetY = 0
		case 12: // ^L, refresh
		default:
			justRedraw = false
		}
		// check for left arrow to go to next/previous image
		if len(ap.Data) >= 3 && c == 27 && ap.Data[1] == '[' {
			// Arrow key
			switch ap.Data[2] {
			case 'D': // left arrow
				i = (i + l - 1) % l
				continue
			case 'A': // up arrow
				zoom *= 1.25
				justRedraw = true
			case 'B': // down arrow
				zoom /= 1.25
				justRedraw = true
			}
		}
		if justRedraw {
			goto redraw
		}
		if l == 1 {
			if !changedInfo {
				ap.WriteRight(0, "%s", info)
			}
			return 0
		}
		if isStopKey(ap) {
			return 0
		}
		i++
	}
}

func setLabels(labels ...string) {
	perfResults.Labels = strings.Join(labels, ", ")
}

func Main() int { //nolint:funlen,gocognit,gocyclo,maintidx // color and mode if/else are a bit long.
	defaultTrueColor := false
	if os.Getenv("COLORTERM") != "" {
		defaultTrueColor = true
	}
	defaultColor := false
	tenv := os.Getenv("TERM")
	switch tenv {
	case "xterm-256color":
		defaultColor = true
	case "":
		tenv = "TERM not set"
	}
	perfResults.Destination = tenv
	imgFlag := flag.String("image", "", "Image file to display in monochrome in the background instead of the default one")
	colorFlag := flag.Bool("color", defaultColor,
		"If your terminal supports color, this will load image in (216) colors instead of monochrome")
	trueColorFlag := flag.Bool("truecolor", defaultTrueColor,
		"If your terminal supports truecolor, this will load image in truecolor (24bits) instead of monochrome")
	grayFlag := flag.Bool("gray", false, "Convert the image to grayscale")
	noboxFlag := flag.Bool("nobox", false,
		"Don't draw the box around the image, make the image full screen instead of 1 pixel less on all sides")
	imagesOnlyFlag := flag.Bool("i", false, "Arguments are now images files to show, no FPS test (hit any key to continue)")
	exactlyFlag := flag.Int64("n", 0, "Start immediately an FPS test with the specified `number of frames` (default is interactive)")
	noMouseFlag := flag.Bool("nomouse", false, "Disable mouse tracking")
	fireFlag := flag.Bool("fire", false, "Show fire animation instead of RGB around the image")
	cli.MinArgs = 0
	cli.MaxArgs = -1
	cli.ArgsHelp = "[maxfps] or fps -i imagefiles..."
	cli.Main()
	fireMode := *fireFlag
	fireStr := "no_fire"
	if fireMode {
		fireStr = "fire"
	}
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
		perfResults.hist = stats.NewHistogram(0, .01/fpsLimit)
	}
	perfResults.Exactly = *exactlyFlag
	perfResults.RequestedQPS = fpsStr
	perfResults.Version = "fps " + cli.LongVersion
	ap := ansipixels.NewAnsiPixels(max(25, fpsLimit)) // initial fps for the start screen and/or the image viewer.
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
		ap.MoveCursor(0, ap.H-1)
		ap.MouseTrackingOff()
		ap.MouseClickOff()
		ap.Restore() // flushes and shows cursor and resets terminal back to original state.
	}()
	// GetSize done in Open (and resize signal handler).
	ap.HideCursor()
	ap.ClearScreen()
	if imagesOnly && len(flag.Args()) > 0 {
		if !*noMouseFlag {
			ap.MouseClickOn()
		}
		return imagesViewer(ap, flag.Args())
	}
	var background *ansipixels.Image
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
	ap.OnResize = func() error {
		ap.HideCursor()
		ap.StartSyncMode()
		ap.ClearScreen()
		e := ap.ShowImage(background, 1.0, 0, 0, defaultMonoImageColor)
		if !imagesOnly {
			drawBox(ap, true)
			if fireMode {
				ShowPalette(ap)
			}
			ap.WriteCentered(ap.H/2+4, "In -fire mode, space bar to toggle on/off; i to hide text")
			ap.WriteCentered(ap.H/2+3, "FPS %s test... any key to start; q, ^C, or ^D to exit... %s",
				fpsStr, ansipixels.MoveLeft)
			ap.ShowCursor()
		}
		ap.EndSyncMode()
		return e
	}
	if err = ap.OnResize(); err != nil {
		return log.FErrf("Error showing image: %v", err)
	}
	if imagesOnly {
		ap.Out.Flush()
		err = ap.ReadOrResizeOrSignal()
		if err != nil && !errors.Is(err, terminal.ErrSignal) {
			return log.FErrf("Error reading key: %v", err)
		}
		return 0
	}
	// FPS test
	fps := 0.0
	// sleep := 1 * time.Second / time.Duration(fps)
	if perfResults.Exactly <= 0 {
		err = ap.ReadOrResizeOrSignal()
	}
	if err != nil {
		return log.FErrf("Error reading initial key: %v", err)
	}
	ap.HideCursor()
	if !*noMouseFlag {
		ap.MouseTrackingOn()
	}
	var frames int64
	var hideText bool
	ap.OnResize = func() error {
		ap.StartSyncMode()
		ap.ClearScreen()
		e := ap.ShowImage(background, 1.0, 0, 0, defaultMonoImageColor)
		if !imagesOnly {
			drawBox(ap, false) // no boxed Width x Height in pure fps mode, keeping it simple.
		}
		ap.EndSyncMode()
		// with max fps expect values in the tens of usec range with usec precision (at max fps for fast terminals)
		perfResults.hist = stats.NewHistogram(0, 0.0000001)
		frames = 0
		setLabels("fps "+strings.TrimSuffix(fpsStr, ".0"), tenv, fmt.Sprintf("%dx%d", ap.W, ap.H), fireStr)
		hideText = false
		return e
	}
	if err = ap.OnResize(); err != nil {
		return log.FErrf("Error showing image: %v", err)
	}
	var elapsed time.Duration
	var entry []byte
	sendableTickerChan := make(chan time.Time, 1)
	var tickerChan <-chan time.Time
	perfResults.StartTime = time.Now()
	startTime := hrtime.Now()
	now := startTime
	if hasFPSLimit {
		ticker := time.NewTicker(time.Duration(float64(time.Second) / fpsLimit))
		tickerChan = ticker.C
	} else {
		tickerChan = sendableTickerChan
		sendableTickerChan <- perfResults.StartTime
	}
	for {
		select {
		case s := <-ap.C:
			err = ap.HandleSignal(s)
			if errors.Is(err, terminal.ErrSignal) {
				return 0
			}
			if err != nil {
				return log.FErrf("Error handling signal/resize: %v", err)
			}
			continue // was a resize without error, get back to the fps loop.
		case v := <-tickerChan:
			elapsed = hrtime.Since(now)
			sec := elapsed.Seconds()
			fps = 1. / sec
			now = hrtime.Now()
			// perfResults.ActualDuration = now.Sub(startTime)
			perfResults.ActualDuration = now - startTime
			perfResults.ActualQPS = float64(frames) / perfResults.ActualDuration.Seconds()
			if frames > 0 {
				perfResults.hist.Record(sec) // record in milliseconds
			}
			if fireMode {
				ap.StartSyncMode()
				AnimateFire(ap, frames)
			}
			// stats.Record("fps", fps)
			if !hideText {
				ap.WriteAt(ap.W/2-20, ap.H/2+2, "%s Last frame %s%v%s FPS: %s%.0f%s Avg %s%.2f%s ",
					ansipixels.Reset, ansipixels.Green, elapsed.Round(10*time.Microsecond), ansipixels.Reset,
					ansipixels.BrightRed, fps, ansipixels.Reset,
					ansipixels.Cyan, perfResults.ActualQPS, ansipixels.Reset)
				ap.WriteAt(ap.W/2-20, ap.H/2+3, " Best %.1f Worst %.1f: %.1f +/- %.1f ",
					1/perfResults.hist.Min, 1/perfResults.hist.Max, 1/perfResults.hist.Avg(), 1/perfResults.hist.StdDev())
			}
			if perfResults.Exactly > 0 && frames >= perfResults.Exactly {
				return 0
			}
			if !fireMode {
				animate(ap, frames)
			}
			if !hideText {
				invert := ""
				if ap.Mouse {
					invert = ansipixels.Reverse
				}
				ap.WriteRight(ap.H-1-ap.Margin, " Target %sFPS %s%s%s, %dx%d, typed so far: %s[%s%q%s]%s %sMouse %d,%d (%06b)%s",
					ansipixels.Cyan, ansipixels.Green, fpsStr, ansipixels.Reset, ap.W, ap.H,
					ansipixels.DarkGray, ansipixels.Reset, entry, ansipixels.DarkGray, ansipixels.Reset,
					invert, ap.Mx, ap.My, ap.Mbuttons, ansipixels.Reset)
			}
			// Request cursor position (note that FPS is about the same without it, the Flush seems to be enough)
			_, _, err = ap.ReadCursorPos()
			if err != nil {
				return log.FErrf("Error with cursor position request: %v", err)
			}
			// q, ^C, ^D to exit.
			if isStopKey(ap) {
				return 0
			}
			if len(ap.Data) > 0 {
				switch {
				case fireMode && ap.Data[0] == ' ':
					ToggleFire()
				case ap.Data[0] == 'i':
					hideText = !hideText
				}
			}
			entry = append(entry, ap.Data...)
			ap.Data = ap.Data[0:0:cap(ap.Data)] // reset buffer
			frames++
			if !hasFPSLimit {
				sendableTickerChan <- v // shove it back (to always be readable channel in max FPS mode)
			}
		}
	}
}
