package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/safecast"
	"fortio.org/term"
)

type AnsiPixels struct {
	fd    int
	Out   *bufio.Writer
	In    io.Reader
	state *term.State
	buf   [256]byte
	W, H  int // Width and Height
	x, y  int // Cursor position
}

func NewAnsiPixels() *AnsiPixels {
	return &AnsiPixels{fd: safecast.MustConvert[int](os.Stdin.Fd()), Out: bufio.NewWriter(os.Stdout), In: os.Stdin}
}

func (ap *AnsiPixels) Open() (err error) {
	ap.state, err = term.MakeRaw(ap.fd)
	return
}

func (ap *AnsiPixels) GetSize() (err error) {
	ap.W, ap.H, err = term.GetSize(ap.fd)
	return
}

func (ap *AnsiPixels) Restore() {
	ap.Out.Flush()
	if ap.state == nil {
		return
	}
	err := term.Restore(ap.fd, ap.state)
	if err != nil {
		log.Fatalf("Error restoring terminal: %v", err)
	}
}

func (ap *AnsiPixels) ClearScreen() {
	_, err := ap.Out.WriteString("\033[2J")
	if err != nil {
		log.Fatalf("Error clearing screen: %v", err)
	}
}

func (ap *AnsiPixels) MoveCursor(x, y int) {
	ap.x, ap.y = x, y
	_, err := ap.Out.WriteString("\033[" + strconv.Itoa(y+1) + ";" + strconv.Itoa(x+1) + "H")
	if err != nil {
		log.Fatalf("Error moving cursor: %v", err)
	}
}

func (ap *AnsiPixels) WriteAt(x, y int, msg string, args ...interface{}) {
	ap.MoveCursor(x, y)
	fmt.Fprintf(ap.Out, msg, args...)
}

func (ap *AnsiPixels) WriteCentered(y int, msg string, args ...interface{}) {
	s := fmt.Sprintf(msg, args...)
	x := (ap.W - len(s)) / 2
	ap.MoveCursor(x, y)
	ap.Out.WriteString(s)
}

// This also synchronizes the the display.
func (ap *AnsiPixels) ReadCursorPos() (x, y int, err error) {
	return
}

func main() {
	os.Exit(Main())
}

func isStopKey(key byte) bool {
	return key == 'q' || key == 3 || key == 4
}

func Main() int {
	cli.Main()
	ap := NewAnsiPixels()
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
	_, _ = ap.Out.WriteString("\033[?25l") // hide cursor
	// _, _ = ap.Out.WriteString("\033[?2026h") // sync mode
	frames := 0
	startTime := time.Now()
	var elapsed time.Duration
	for {
		now := time.Now()
		ap.WriteAt(w/2-20, h/2+1, "Last frame %v FPS: %.0f Avg %.2f ", elapsed, fps, float64(frames)/now.Sub(startTime).Seconds())
		_, err := ap.Out.WriteString("\033[6n") // request cursor position
		if err != nil {
			return log.FErrf("Error writing cursor position request: %v", err)
		}
		ap.Out.Flush()
		n := 0
		key := byte(0)
		for {
			n, err = ap.In.Read(buf[:])
			// log.Infof("Last buffer read: %q", buf[0:n])
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return log.FErrf("Error reading cursor position: %v", err)
			}
			if n == 0 {
				return log.FErrf("No data read from cursor position")
			}
			for i := 0; !isStopKey(key) && i < n; i++ {
				key = buf[i]
				if isStopKey(key) {
					break
				}
			}
			break
			/*
				if bytes.IndexByte(buf[:n], 'R') >= 0 {
					break
				}
			*/
		}
		// q, ^C, ^D to exit.
		if isStopKey(key) {
			break
		}
		elapsed = time.Since(now)
		fps = 1. / elapsed.Seconds()
		frames++
	}
	_, _ = ap.Out.WriteString("\033[?25h") // show cursor
	ap.MoveCursor(0, h-2)
	ap.Out.Flush()
	return 0
}