package main

import (
	"os"
	"os/signal"
	"syscall"
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
			drawCorners(ap)
			ap.Data = ap.Data[0:0:cap(ap.Data)] // reset buffer
			return false
		}
		if key == 'q' || key == 3 || key == 4 {
			return true
		}
	}
	return false
}

func drawCorners(ap *ansipixels.AnsiPixels) {
	w := ap.W
	h := ap.H
	ap.WriteAt(0, 0, "┌")
	ap.WriteAt(1, 0, "─")
	ap.WriteAt(w-2, 0, "─")
	ap.WriteAt(w-1, 0, "┐")
	ap.WriteAt(0, 1, "|")
	ap.WriteAt(w-1, 1, "|")
	ap.WriteAt(0, h-2, "|")
	ap.WriteAt(w-1, h-2, "|")
	ap.WriteAt(0, h-1, "└")
	ap.WriteAt(1, h-1, "─")
	ap.WriteAt(w-2, h-1, "─")
	ap.WriteAt(w-1, h-1, "┘")
	ap.WriteCentered(h/2, "Width: %d, Height: %d", ap.W, ap.H)
}

func posToXY(pos int, w, h int) (int, int) {
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

func animate(ap *ansipixels.AnsiPixels, frame uint) {
	w := ap.W
	h := ap.H
	w -= 4
	h -= 4
	total := 2*w + 2*h
	pos := safecast.MustConvert[int](frame % safecast.MustConvert[uint](total))
	x, y := posToXY(pos, w, h)
	ap.WriteAt(x+2, y+2, " ")
	x, y = posToXY((pos+1)%total, w, h)
	ap.WriteAt(x+2, y+2, "█")
}

func Main() int {
	cli.Main()
	ap := ansipixels.NewAnsiPixels()
	if err := ap.Open(); err != nil {
		log.Fatalf("Not a terminal: %v", err)
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
	drawCorners(ap)
	// FPS test
	fps := 0.0
	buf := [256]byte{}
	// sleep := 1 * time.Second / time.Duration(fps)
	ap.WriteCentered(ap.H/2+3, "FPS test... any key to start; q, ^C, or ^D to exit... ")
	ap.Out.Flush()
	_, err := ap.In.Read(buf[:])
	if err != nil {
		return log.FErrf("Error reading key: %v", err)
	}
	ap.HideCursor()
	// _, _ = ap.Out.WriteString("\033[?2026h") // sync mode // doesn't seem to do anything
	frames := uint(0)
	startTime := time.Now()
	var elapsed time.Duration
	var entry []byte
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGWINCH)
	for {
		select {
		case <-sigchan:
			_ = ap.GetSize()
			ap.ClearScreen()
			drawCorners(ap)
		default:
			now := time.Now()
			animate(ap, frames)
			ap.WriteAt(ap.W/2-20, ap.H/2+1, "Last frame %v FPS: %.0f Avg %.2f",
				elapsed.Round(10*time.Microsecond), fps, float64(frames)/now.Sub(startTime).Seconds())
			// Request cursor position (note that FPS is about the same without it, the Flush seems to be enough)
			ap.ClearEndOfLine()
			_, _, err = ap.ReadCursorPos()
			if err != nil {
				return log.FErrf("Error with cursor position request: %v", err)
			}
			// q, ^C, ^D to exit.
			if isStopKey(ap) {
				return 0
			}
			entry = append(entry, ap.Data...)
			ap.WriteCentered(ap.H/2+5, "Entry so far: [%s]", entry)
			ap.Data = ap.Data[0:0:cap(ap.Data)] // reset buffer
			elapsed = time.Since(now)
			fps = 1. / elapsed.Seconds()
			frames++
		}
	}
}
