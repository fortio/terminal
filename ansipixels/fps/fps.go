package main

import (
	"os"
	"time"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal/ansipixels"
)

func main() {
	os.Exit(Main())
}

func isStopKey(buf []byte) bool {
	// q, ^C, ^D to exit.
	for _, key := range buf {
		if key == 'q' || key == 3 || key == 4 {
			return true
		}
	}
	return false
}

func Main() int {
	cli.Main()
	ap := ansipixels.NewAnsiPixels()
	if err := ap.Open(); err != nil {
		log.Fatalf("Not a terminal: %v", err)
	}
	defer ap.Restore()
	if err := ap.GetSize(); err != nil {
		return log.FErrf("Error getting terminal size: %v", err)
	}
	ap.ClearScreen()
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
	// FPS test
	fps := 0.0
	buf := [256]byte{}
	// sleep := 1 * time.Second / time.Duration(fps)
	ap.WriteCentered(h/2+3, "FPS test... any key to start; q, ^C, or ^D to exit... ")
	ap.Out.Flush()
	_, err := ap.In.Read(buf[:])
	if err != nil {
		return log.FErrf("Error reading key: %v", err)
	}
	ap.HideCursor()
	// _, _ = ap.Out.WriteString("\033[?2026h") // sync mode // doesn't seem to do anything
	frames := 0
	startTime := time.Now()
	var elapsed time.Duration
	var entry []byte
	for {
		now := time.Now()
		ap.WriteAt(w/2-20, h/2+1, "Last frame %v FPS: %.0f Avg %.2f",
			elapsed, fps, float64(frames)/now.Sub(startTime).Seconds())
		// Request cursor position (note that FPS is about the same without it, the Flush seems to be enough)
		ap.ClearEndOfLine()
		_, _, err = ap.ReadCursorPos()
		if err != nil {
			return log.FErrf("Error with cursor position request: %v", err)
		}
		// q, ^C, ^D to exit.
		if isStopKey(ap.Data) {
			break
		}
		entry = append(entry, ap.Data...)
		ap.WriteCentered(h/2+5, "Entry so far: [%s]", entry)
		ap.Data = ap.Data[0:0:cap(ap.Data)] // reset buffer
		elapsed = time.Since(now)
		fps = 1. / elapsed.Seconds()
		frames++
	}
	ap.ShowCursor()
	ap.MoveCursor(0, h-2)
	ap.Out.Flush()
	return 0
}
