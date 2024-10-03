// ansipixel provides terminal drawing and key reading abilities. fps/fps.go and life/life.go are examples/demos of how to use it.
package ansipixels

import (
	"bufio"
	"bytes"
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
	"github.com/rivo/uniseg"
)

const BUFSIZE = 1024

type AnsiPixels struct {
	FdIn          int
	fdOut         int
	Out           *bufio.Writer
	In            *os.File
	InWithTimeout *terminal.TimeoutReader
	state         *term.State
	buf           [BUFSIZE]byte
	Data          []byte
	W, H          int  // Width and Height
	x, y          int  // Cursor last set position
	Mouse         bool // Mouse event received
	Mx, My        int  // Mouse last known position
	Mbuttons      int  // Mouse buttons and modifier state
	C             chan os.Signal
	// Should image be monochrome, 256 or true color
	TrueColor bool
	Color     bool         // 256 (216) color mode
	Gray      bool         // grayscale mode
	Margin    int          // Margin around the image (image is smaller by 2*margin)
	FPS       float64      // (Target) Frames per second used for Reading with timeout
	OnResize  func() error // Callback when terminal is resized
}

func NewAnsiPixels(fps float64) *AnsiPixels {
	ap := &AnsiPixels{
		FdIn:          safecast.MustConvert[int](os.Stdin.Fd()),
		fdOut:         safecast.MustConvert[int](os.Stdout.Fd()),
		Out:           bufio.NewWriter(os.Stdout),
		In:            os.Stdin,
		FPS:           fps,
		InWithTimeout: terminal.NewTimeoutReader(os.Stdin, time.Duration(1e9/fps)),
		C:             make(chan os.Signal, 1),
	}
	signal.Notify(ap.C, signalList...)
	return ap
}

func (ap *AnsiPixels) ChangeFPS(fps float64) {
	ap.InWithTimeout.ChangeTimeout(1 * time.Second / time.Duration(fps))
}

func (ap *AnsiPixels) Open() (err error) {
	ap.state, err = term.MakeRaw(ap.FdIn)
	if err == nil {
		err = ap.GetSize()
	}
	return
}

// So this handles both outgoing and incoming escape sequences, but maybe we should split them
// to keep the outgoing (for string width etc) and the incoming (find key pressed without being
// confused by a "q" in the middle of the mouse coordinates) separate.
// This extra complexity (M mode) is now not needed as we run MouseDecode() and that removes/parses the mouse data.
var CleanAnsiRE = regexp.MustCompile("\x1b\\[(M.(.(.|$)|$)|[^@-~]*([@-~]|$))")

// Remove all Ansi code from a given string. Useful among other things to get the correct string width.
func AnsiCleanRE(str string) string {
	return CleanAnsiRE.ReplaceAllString(str, "")
}

var startSequence = []byte("\x1b[")

func AnsiClean(str []byte) []byte {
	// note strings.Index also uses IndexBytes and SIMD when available internally
	// a string version would be fast too without the need to convert to bytes.
	// yet this version is a bit faster and fewer allocations and we need byte based
	// for the stopKey check anyway.
	idx := bytes.Index(str, startSequence)
	if idx == -1 {
		return str
	}
	l := len(str)
	if idx == l-2 {
		return str[:idx] // last 2 bytes are a truncated ESC[
	}
	buf := make([]byte, 0, l-3) // remaining worst case is ESC[m so -3 at least.
	for {
		buf = append(buf, str[:idx]...)
		idx += 2
		/*
			if str[idx] == 'M' {
				// Mouse is fixed size
				idx += 3
				if idx >= l-1 {
					return buf
				}
			} else {
		*/
		// Normal escape: skip until end of escape sequence
		for ; str[idx] < 64; idx++ {
			if idx == l-1 {
				return buf
			}
		}
		// }
		str = str[idx+1:]
		idx = bytes.Index(str, startSequence)
		if idx == -1 {
			break
		}
	}
	buf = append(buf, str...)
	return buf
}

func (ap *AnsiPixels) HandleSignal(s os.Signal) error {
	if !ap.IsResizeSignal(s) {
		return terminal.ErrSignal
	}
	err := ap.GetSize()
	if err != nil {
		return err
	}
	if ap.OnResize != nil {
		err := ap.OnResize()
		ap.EndSyncMode()
		return err
	}
	return nil
}

// Read something or return terminal.ErrSignal if signal is received (normal exit requested case),
// will automatically call OnResize if set and if a resize signal is received and continue trying
// to read.
func (ap *AnsiPixels) ReadOrResizeOrSignal() error {
	ap.EndSyncMode()
	for {
		n, err := ap.ReadOrResizeOrSignalOnce()
		if err != nil {
			return err
		}
		if n != 0 {
			return nil
		}
	}
}

// This will return either because of signal, or something read or the timeout (fps) passed.
// ap.Data is (re)set to the read data.
func (ap *AnsiPixels) ReadOrResizeOrSignalOnce() (int, error) {
	select {
	case s := <-ap.C:
		err := ap.HandleSignal(s)
		if err != nil {
			return 0, err
		}
	default:
		n, err := ap.InWithTimeout.Read(ap.buf[0:BUFSIZE])
		ap.Data = ap.buf[0:n]
		ap.MouseDecode()
		return n, err
	}
	return 0, nil
}

func (ap *AnsiPixels) StartSyncMode() {
	ap.WriteString("\033[?2026h")
}

// End sync (and flush).
func (ap *AnsiPixels) EndSyncMode() {
	ap.WriteString("\033[?2026l")
	_ = ap.Out.Flush()
}

func (ap *AnsiPixels) GetSize() (err error) {
	ap.W, ap.H, err = term.GetSize(ap.fdOut)
	return
}

func (ap *AnsiPixels) Restore() {
	if ap.state == nil {
		return
	}
	ap.ShowCursor()
	ap.EndSyncMode()
	err := term.Restore(ap.FdIn, ap.state)
	if err != nil {
		log.Fatalf("Error restoring terminal: %v", err)
	}
	ap.state = nil
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

func (ap *AnsiPixels) WriteString(msg string) {
	_, _ = ap.Out.WriteString(msg)
}

func (ap *AnsiPixels) WriteRune(r rune) {
	_, _ = ap.Out.WriteRune(r)
}

func (ap *AnsiPixels) WriteAtStr(x, y int, msg string) {
	ap.MoveCursor(x, y)
	ap.WriteString(msg)
}

func (ap *AnsiPixels) WriteAt(x, y int, msg string, args ...interface{}) {
	ap.MoveCursor(x, y)
	_, _ = fmt.Fprintf(ap.Out, msg, args...)
}

func (ap *AnsiPixels) ScreenWidth(str string) int {
	// Hopefully the compiler will optimize this alloc wise for string<->[]byte.
	return uniseg.StringWidth(string(AnsiClean([]byte(str))))
}

func (ap *AnsiPixels) WriteCentered(y int, msg string, args ...interface{}) {
	s := fmt.Sprintf(msg, args...)
	x := (ap.W - ap.ScreenWidth(s)) / 2
	ap.MoveCursor(x, y)
	ap.WriteString(s)
}

func (ap *AnsiPixels) TruncateLeftToFit(msg string, maxWidth int) (string, int) {
	w := ap.ScreenWidth(msg)
	if w < maxWidth {
		return msg, w
	}
	// slow path.
	str := "…"
	runes := []rune(msg)
	// This isn't optimized and also because of AnsiClean behind the scene we might remove codes we should keep.
	for i := range runes {
		w = ap.ScreenWidth(string(runes[i:]))
		if w < maxWidth {
			return str + string(runes[i:]), w + 1
		}
	}
	return str, 1
}

func (ap *AnsiPixels) WriteRight(y int, msg string, args ...interface{}) {
	s := fmt.Sprintf(msg, args...)
	s, l := ap.TruncateLeftToFit(s, ap.W-2*ap.Margin)
	x := ap.W - l - ap.Margin
	if x < 0 {
		panic("TruncateLeftToFit returned a string longer than the width")
	}
	ap.MoveCursor(x, y)
	ap.WriteString(s)
}

func (ap *AnsiPixels) ClearEndOfLine() {
	ap.WriteString("\033[K")
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
	ap.Data = nil
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
		ap.MouseDecode()
		break
	}
	return x, y, err
}

func (ap *AnsiPixels) HideCursor() {
	ap.WriteString("\033[?25l") // hide cursor
}

func (ap *AnsiPixels) ShowCursor() {
	ap.WriteString("\033[?25h") // show cursor
}

func (ap *AnsiPixels) DrawSquareBox(x, y, w, h int) {
	ap.DrawBox(x, y, w, h, SquareTopLeft, SquareTopRight, SquareBottomLeft, SquareBottomRight)
}

func (ap *AnsiPixels) DrawRoundBox(x, y, w, h int) {
	ap.DrawBox(x, y, w, h, RoundTopLeft, RoundTopRight, RoundBottomLeft, RoundBottomRight)
}

func (ap *AnsiPixels) DrawBox(x, y, w, h int, topLeft, topRight, bottomLeft, bottomRight string) {
	if y >= 0 {
		ap.MoveCursor(x, y)
		ap.WriteString(topLeft)
		ap.WriteString(strings.Repeat(Horizontal, w-2))
		ap.WriteString(topRight)
	}
	for i := 1; i < h-1; i++ {
		ap.MoveCursor(x, y+i)
		if y+i == 0 {
			ap.WriteString(topRight)
			if x+w < ap.W {
				ap.MoveHorizontally(x + w - 1)
				ap.WriteString(topLeft)
			}
		} else {
			ap.WriteString(Vertical)
			ap.MoveHorizontally(x + w - 1)
			ap.WriteString(Vertical)
		}
	}
	ap.MoveCursor(x, y+h-1)
	ap.WriteString(bottomLeft)
	ap.WriteString(strings.Repeat(Horizontal, w-3))
	if x+w <= ap.W {
		ap.WriteString(Horizontal + bottomRight)
	} else {
		ap.WriteString(topRight)
	}
}

func (ap *AnsiPixels) WriteBoxed(y int, msg string, args ...interface{}) {
	s := fmt.Sprintf(msg, args...)
	w := ap.ScreenWidth(s)
	x := (ap.W - w) / 2
	ap.MoveCursor(x, y)
	ap.WriteString(s)
	ap.DrawRoundBox(x-1, y-1, w+2, 3)
}

func (ap *AnsiPixels) WriteRightBoxed(y int, msg string, args ...interface{}) {
	s := fmt.Sprintf(msg, args...)
	w := ap.ScreenWidth(s)
	x := ap.W - w // not using margin as we assume we want to join lines in the corner
	ap.MoveCursor(x, y)
	ap.WriteString(s)
	ap.DrawRoundBox(x-1, y-1, w+2, 3)
}

func FormatDate(d *time.Time) string {
	return fmt.Sprintf("%d-%02d-%02d-%02d%02d%02d", d.Year(), d.Month(), d.Day(),
		d.Hour(), d.Minute(), d.Second())
}
