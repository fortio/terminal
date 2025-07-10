// ansipixels provides terminal drawing and key reading abilities.
// [fortio.org/terminal/fps] and life/life.go are examples/demos of how to use it.
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
	"fortio.org/terminal"
	"github.com/rivo/uniseg"
	"golang.org/x/term"
)

const bufSize = 1024

type AnsiPixels struct {
	fdOut       int
	Out         *bufio.Writer
	SharedInput *terminal.InterruptReader
	buf         [bufSize]byte
	Data        []byte
	W, H        int  // Width and Height
	x, y        int  // Cursor last set position
	Mouse       bool // Mouse event received
	Mx, My      int  // Mouse last known position
	Mbuttons    int  // Mouse buttons and modifier state
	C           chan os.Signal
	// Should image be monochrome, 256 or true color
	TrueColor bool
	Color     bool         // 256 (216) color mode
	Gray      bool         // grayscale mode
	Margin    int          // Margin around the image (image is smaller by 2*margin)
	FPS       float64      // (Target) Frames per second used for Reading with timeout
	OnResize  func() error // Callback when terminal is resized
	// First time we clear the screen, we use 2J to push old content to scrollback buffer, otherwise we use H+0J
	firstClear bool
	restored   bool
	// In NoDecode mode the mouse decode and the end of sync are not done automatically.
	// used for fortio.org/tev raw event dump.
	NoDecode bool
}

// A 0 fps means bypassing the interrupt reader and using the underlying os.StdIn directly.
// Otherwise a non blocking reader is setup with 1/fps timeout. Reader is / can be shared
// with Terminal.
func NewAnsiPixels(fps float64) *AnsiPixels {
	var d time.Duration
	if fps > 0 {
		d = time.Duration(1e9 / fps)
	}
	ap := &AnsiPixels{
		fdOut:       safecast.MustConvert[int](os.Stdout.Fd()),
		Out:         bufio.NewWriter(os.Stdout),
		FPS:         fps,
		SharedInput: terminal.GetSharedInput(d),
		C:           make(chan os.Signal, 1),
	}
	signal.Notify(ap.C, signalList...)
	return ap
}

func (ap *AnsiPixels) ChangeFPS(fps float64) {
	ap.SharedInput.ChangeTimeout(1 * time.Second / time.Duration(fps))
}

func (ap *AnsiPixels) Open() error {
	ap.firstClear = true
	ap.restored = false
	err := ap.SharedInput.RawMode()
	if err == nil {
		err = ap.GetSize()
	}
	return err
}

// So this handles both outgoing and incoming escape sequences, but maybe we should split them
// to keep the outgoing (for string width etc) and the incoming (find key pressed without being
// confused by a "q" in the middle of the mouse coordinates) separate.
// This extra complexity (M mode) is now not needed as we run MouseDecode() and that removes/parses the mouse data.
var cleanAnsiRE = regexp.MustCompile("\x1b\\[(M.(.(.|$)|$)|[^@-~]*([@-~]|$))")

// Remove all Ansi code from a given string. Useful among other things to get the correct string width.
func ansiCleanRE(str string) string {
	return cleanAnsiRE.ReplaceAllString(str, "")
}

var startSequence = []byte("\x1b[")

// AnsiClean removes all Ansi code from a given byte slice.
// Useful among other things to get the correct string to pass to uniseq.StringWidth
// (ap.StringWidth does it for you).
// Returns the length of the processed input - unterminated sequences can be reprocessed
// using data from str[returnedVal:] plus additional data (see tests and nocolor for usage).
func AnsiClean(str []byte) ([]byte, int) {
	// note strings.Index also uses IndexBytes and SIMD when available internally
	// a string version would be fast too without the need to convert to bytes.
	// yet this version is a bit faster and fewer allocations and we need byte based
	// for the stopKey check anyway.
	idx := bytes.Index(str, startSequence)
	l := len(str)
	if idx == -1 {
		if l > 0 && str[l-1] == 27 {
			return str[:l-1], l - 1
		}
		return str, l
	}
	if idx == l-2 {
		return str[:idx], idx // last 2 bytes are a truncated ESC[
	}
	buf := make([]byte, 0, l-3) // remaining worst case is ESC[m so -3 at least.
	end := l
	oIdx := idx // value we use in case it's not terminated
	for {
		buf = append(buf, str[:idx]...)
		cur := idx
		idx += 2
		for {
			if idx >= l {
				return buf, oIdx
			}
			if str[idx] >= 64 {
				break
			}
			idx++
		}
		idx++
		oIdx += idx - cur
		l -= idx
		str = str[idx:]
		idx = bytes.Index(str, startSequence)
		if idx == -1 {
			break
		}
		oIdx += idx
	}
	nl := len(str)
	if nl > 0 && str[nl-1] == 27 {
		end--
		str = str[:nl-1]
	}
	buf = append(buf, str...)
	return buf, end
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
	if !ap.NoDecode {
		ap.EndSyncMode()
	}
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
		n, err := ap.SharedInput.TR.Read(ap.buf[0:bufSize])
		ap.Data = ap.buf[0:n]
		if !ap.NoDecode {
			ap.MouseDecode()
		}
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
	if ap.restored {
		return
	}
	ap.ShowCursor()
	ap.EndSyncMode()
	_ = ap.SharedInput.NormalMode()
	ap.restored = true
}

// ClearScreen erases the current frame/screen and positions the cursor in the top left.
// First time we clear the screen, we use 2J to push old content to the scrollback buffer, otherwise we use H+0J.
func (ap *AnsiPixels) ClearScreen() {
	ap.x = 0
	ap.y = 0
	what := "\033[H\033[0J"
	if ap.firstClear {
		what = "\033[2J\033[H" // for consistency we also move to 0,0 (in our coordinate system, 1,1 in the terminal)
		ap.firstClear = false
	}
	_, err := ap.Out.WriteString(what)
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
	b, _ := AnsiClean([]byte(str))
	return uniseg.StringWidth(string(b))
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
		panic("TruncateLeftToFit returned a string longer than the width") // would be a bug/should never happen.
	}
	ap.MoveCursor(x, y)
	ap.WriteString(s)
}

func (ap *AnsiPixels) ClearEndOfLine() {
	ap.WriteString("\033[K")
}

var cursPosRegexp = regexp.MustCompile(`^(.*)\033\[(\d+);(\d+)R(.*)$`)

// This also synchronizes the display and ends the syncmode.
func (ap *AnsiPixels) ReadCursorPos() (int, int, error) {
	x := -1
	y := -1
	reqPosStr := "\033[?2026l\033[6n" // also ends sync mode
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
		if i == bufSize {
			return x, y, errors.New("buffer full, no cursor position found")
		}
		n, err = ap.SharedInput.In.Read(ap.buf[i:bufSize])
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
	ap.MouseDecode()
	return x, y, err
}

func (ap *AnsiPixels) HideCursor() {
	ap.WriteString("\033[?25l") // hide cursor
}

func (ap *AnsiPixels) ShowCursor() {
	ap.WriteString("\033[?25h") // show cursor
}

func (ap *AnsiPixels) DrawSquareBox(x, y, w, h int) {
	ap.DrawBox(x, y, w, h, SquareTopLeft, Horizontal, SquareTopRight, Vertical, SquareBottomLeft, Horizontal, SquareBottomRight, false)
}

func (ap *AnsiPixels) DrawRoundBox(x, y, w, h int) {
	ap.DrawBox(x, y, w, h, RoundTopLeft, Horizontal, RoundTopRight, Vertical, RoundBottomLeft, Horizontal, RoundBottomRight, false)
}

// Draw a colored box with the given background color and double width option which means extra bars on the left and right.
func (ap *AnsiPixels) DrawColoredBox(x, y, w, h int, color string, doubleWidth bool) {
	topHorizontal := Inverse + string(TopHalfPixel)
	bottomHorizontal := string(BottomHalfPixel)
	vertical := " "
	if doubleWidth {
		topHorizontal = " "
		bottomHorizontal = " "
		vertical = " "
	}
	ap.DrawBox(x, y, w, h, color+topHorizontal, topHorizontal, topHorizontal, vertical,
		bottomHorizontal, bottomHorizontal, bottomHorizontal+Reset, doubleWidth)
}

func (ap *AnsiPixels) DrawBox(x, y, w, h int,
	topLeft, horizontalTop, topRight,
	vertical, bottomLeft, horizontalBottom, bottomRight string,
	doubleWidth bool,
) {
	if y >= 0 {
		ap.MoveCursor(x, y)
		ap.WriteString(topLeft)
		ap.WriteString(strings.Repeat(horizontalTop, w-2))
		ap.WriteString(topRight)
	}
	if doubleWidth {
		w++
		x--
		vertical += vertical
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
			ap.WriteString(vertical)
			ap.MoveHorizontally(x + w - 1)
			ap.WriteString(vertical)
		}
	}
	if doubleWidth {
		w--
		x++
	}
	ap.MoveCursor(x, y+h-1)
	ap.WriteString(bottomLeft)
	ap.WriteString(strings.Repeat(horizontalBottom, w-3))
	if x+w <= ap.W {
		ap.WriteString(horizontalBottom + bottomRight)
	} else {
		ap.WriteString(topRight) // used by brick in the top right corner.
	}
}

func (ap *AnsiPixels) WriteBoxed(y int, msg string, args ...interface{}) {
	s := fmt.Sprintf(msg, args...)
	lines := strings.Split(s, "\n")
	maxw := 0
	widths := make([]int, 0, len(lines))
	for _, l := range lines {
		w := ap.ScreenWidth(l)
		widths = append(widths, w)
		maxw = max(maxw, w)
	}
	var cursorX int
	var cursorY int
	for i, l := range lines {
		cursorY = y + i
		x := (ap.W - widths[i]) / 2
		ap.MoveCursor(x, cursorY)
		ap.WriteString(l)
		cursorX = x + widths[i]
	}
	ap.DrawRoundBox((ap.W-maxw)/2-1, y-1, maxw+2, len(lines)+2)
	// put back the cursor at the end of the last line (inside the box)
	ap.MoveCursor(cursorX, cursorY)
}

func (ap *AnsiPixels) WriteRightBoxed(y int, msg string, args ...interface{}) {
	s := fmt.Sprintf(msg, args...)
	w := ap.ScreenWidth(s)
	x := ap.W - w // not using margin as we assume we want to join lines in the corner
	// On some terminals, namely apple terminal and allacritty, the ❤️ isn't actually double width,
	// or it doesn't erase the last corner character. So we need to erase it manually.
	ap.MoveCursor(x+w-1, y)
	ap.WriteRune(' ')
	ap.MoveHorizontally(x)
	ap.WriteString(s)
	ap.DrawRoundBox(x-1, y-1, w+2, 3)
}

func FormatDate(d *time.Time) string {
	return fmt.Sprintf("%d-%02d-%02d-%02d%02d%02d", d.Year(), d.Month(), d.Day(),
		d.Hour(), d.Minute(), d.Second())
}
