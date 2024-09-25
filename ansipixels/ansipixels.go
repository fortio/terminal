// ansipixel provides terminal drawing and key reading abilities. fps/fps.go is an example of how to use it.
package ansipixels

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"

	"fortio.org/log"
	"fortio.org/safecast"
	"fortio.org/term"
	"fortio.org/terminal"
)

const BUFSIZE = 1024

type AnsiPixels struct {
	FdIn          int
	fdOut         int
	Out           *bufio.Writer
	In            *os.File
	InWithTimeout io.Reader // TimeoutReader
	state         *term.State
	buf           [BUFSIZE]byte
	Data          []byte
	W, H          int // Width and Height
	x, y          int // Cursor last set position
	C             chan os.Signal
	// Should image be monochrome, 256 or true color
	TrueColor bool
	Color     bool    // 256 (216) color mode
	Gray      bool    // grayscale mode
	Margin    int     // Margin around the image (image is smaller by 2*margin)
	FPS       float64 // (Target) Frames per second used for Reading with timeout
}

func NewAnsiPixels(fps float64) *AnsiPixels {
	return &AnsiPixels{
		FdIn:          safecast.MustConvert[int](os.Stdin.Fd()),
		fdOut:         safecast.MustConvert[int](os.Stdout.Fd()),
		Out:           bufio.NewWriter(os.Stdout),
		In:            os.Stdin,
		FPS:           fps,
		InWithTimeout: terminal.NewTimeoutReader(os.Stdin, time.Duration(1e9/fps)),
	}
}

func (ap *AnsiPixels) Open() (err error) {
	ap.state, err = term.MakeRaw(ap.FdIn)
	return
}

func (ap *AnsiPixels) GetSize() (err error) {
	ap.W, ap.H, err = term.GetSize(ap.fdOut)
	return
}

func (ap *AnsiPixels) Restore() {
	ap.Out.Flush()
	if ap.state == nil {
		return
	}
	err := term.Restore(ap.FdIn, ap.state)
	if err != nil {
		log.Fatalf("Error restoring terminal: %v", err)
	}
}

func (ap *AnsiPixels) ClearScreen() {
	_, err := ap.Out.WriteString("\033[2J")
	if err != nil {
		log.Errf("Error clearing screen: %v", err)
	}
}

func (ap *AnsiPixels) MoveCursor(x, y int) {
	ap.x, ap.y = x, y
	_, err := ap.Out.WriteString("\033[" + strconv.Itoa(y+1) + ";" + strconv.Itoa(x+1) + "H")
	if err != nil {
		log.Errf("Error moving cursor: %v", err)
	}
}

func (ap *AnsiPixels) MoveHorizontally(x int) {
	ap.x = x
	_, err := ap.Out.WriteString("\033[" + strconv.Itoa(x+1) + "G")
	if err != nil {
		log.Errf("Error moving cursor horizontally: %v", err)
	}
}

func (ap *AnsiPixels) WriteAtStr(x, y int, msg string) {
	ap.MoveCursor(x, y)
	_, _ = ap.Out.WriteString(msg)
}

func (ap *AnsiPixels) WriteAt(x, y int, msg string, args ...interface{}) {
	ap.MoveCursor(x, y)
	_, _ = fmt.Fprintf(ap.Out, msg, args...)
}

func (ap *AnsiPixels) WriteCentered(y int, msg string, args ...interface{}) {
	s := fmt.Sprintf(msg, args...)
	x := (ap.W - len(s)) / 2
	ap.MoveCursor(x, y)
	_, _ = ap.Out.WriteString(s)
}

func (ap *AnsiPixels) WriteRight(y int, msg string, args ...interface{}) {
	s := fmt.Sprintf(msg, args...)
	x := ap.W - len(s)
	ap.MoveCursor(x, y)
	_, _ = ap.Out.WriteString(s)
}

func (ap *AnsiPixels) ClearEndOfLine() {
	_, _ = ap.Out.WriteString("\033[K")
}

func (ap *AnsiPixels) SignalChannel() {
	ap.C = make(chan os.Signal, 1)
	signal.Notify(ap.C, signalList...)
}

var cursPosRegexp = regexp.MustCompile(`^(.*)\033\[(\d+);(\d+)R(.*)$`)

// This also synchronizes the display.
func (ap *AnsiPixels) ReadCursorPos() (int, int, error) {
	x := -1
	y := -1
	reqPosStr := "\033[6n"
	n, err := ap.Out.WriteString(reqPosStr)
	if err != nil {
		return x, y, err
	}
	if n != len(reqPosStr) {
		return x, y, errors.New("short write")
	}
	err = ap.Out.Flush()
	if err != nil {
		return x, y, err
	}
	i := 0
	for {
		if i == BUFSIZE {
			return x, y, errors.New("buffer full, no cursor position found")
		}
		n, err = ap.In.Read(ap.buf[i:BUFSIZE])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return x, y, err
		}
		if n == 0 {
			return x, y, errors.New("no data read from cursor position")
		}
		res := cursPosRegexp.FindSubmatch(ap.buf[0 : i+n])
		if log.LogVerbose() {
			// use go run . -loglevel verbose 2> /tmp/ansipixels.log to capture this
			log.LogVf("Last buffer read: [%q] -> [%q] regexp match %t", ap.buf[i:i+n], ap.buf[0:i+n], res != nil)
		}
		if res == nil {
			// must get the whole response.
			i += n
			continue
		}
		x, err = strconv.Atoi(string(res[2]))
		if err != nil {
			return x, y, err
		}
		y, err = strconv.Atoi(string(res[3]))
		if err != nil {
			return x, y, err
		}
		ap.Data = append(ap.Data, res[1]...)
		ap.Data = append(ap.Data, res[4]...)
		break
	}
	return x, y, err
}

func (ap *AnsiPixels) HideCursor() {
	_, _ = ap.Out.WriteString("\033[?25l") // hide cursor
}

func (ap *AnsiPixels) ShowCursor() {
	_, _ = ap.Out.WriteString("\033[?25h") // show cursor
}

func (ap *AnsiPixels) DrawSquareBox(x, y, w, h int) error {
	return ap.DrawBox(x, y, w, h, SquareTopLeft, SquareTopRight, SquareBottomLeft, SquareBottomRight)
}

func (ap *AnsiPixels) DrawRoundBox(x, y, w, h int) error {
	return ap.DrawBox(x, y, w, h, RoundTopLeft, RoundTopRight, RoundBottomLeft, RoundBottomRight)
}

func (ap *AnsiPixels) DrawBox(x, y, w, h int, topLeft, topRight, bottomLeft, bottomRight string) error {
	ap.MoveCursor(x, y)
	_, _ = ap.Out.WriteString(topLeft)
	_, _ = ap.Out.WriteString(strings.Repeat(Horizontal, w-2))
	_, _ = ap.Out.WriteString(topRight)
	for i := 1; i < h-1; i++ {
		ap.MoveCursor(x, y+i)
		_, _ = ap.Out.WriteString(Vertical)
		ap.MoveHorizontally(x + w - 1)
		_, _ = ap.Out.WriteString(Vertical)
	}
	ap.MoveCursor(x, y+h-1)
	_, _ = ap.Out.WriteString(bottomLeft)
	_, _ = ap.Out.WriteString(strings.Repeat(Horizontal, w-2))
	_, err := ap.Out.WriteString(bottomRight)
	return err
}

func (ap *AnsiPixels) WriteBoxed(y int, msg string, args ...interface{}) {
	s := fmt.Sprintf(msg, args...)
	x := (ap.W - len(s)) / 2
	ap.MoveCursor(x, y)
	_, _ = ap.Out.WriteString(s)
	_ = ap.DrawRoundBox(x-1, y-1, len(s)+2, 3)
}
