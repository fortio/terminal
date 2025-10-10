// Package ansipixels provides terminal drawing and key reading abilities.
// [fortio.org/terminal/fps] and life/life.go are examples/demos of how to use it.
package ansipixels

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
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
	"fortio.org/terminal/ansipixels/tcolor"
	"github.com/rivo/uniseg"
	"golang.org/x/term"
)

const (
	TermNotSet = "TERM not set"
	bufSize    = 1024
)

// ColorMode determines if images be monochrome, 256 or true color.
// Additionally there is the option to convert to grayscale.
type ColorMode struct {
	TrueColor bool
	Color256  bool // 256 (216) color mode
	Gray      bool // grayscale mode
	// Fallback color to use for image display in mono/when Color or TrueColor are both false.
	// Defaults to [tcolor.Blue] unless NO_COLOR is found in the environment.
	MonoColor tcolor.BasicColor
	// Name of the terminal, from TERM env.
	TermEnv string
}

type AnsiPixels struct {
	ColorMode
	// [tcolor.Color] converter to output the correct color codes when TrueColor is not supported.
	ColorOutput tcolor.ColorOutput
	fdOut       int
	Out         terminal.Bufio // typically a bufio.Writer wrapping os.Stdout but can be swapped for testing or other uses.
	SharedInput *terminal.InterruptReader
	buf         [bufSize]byte
	Data        []byte
	W, H        int  // Width and Height
	x, y        int  // Cursor last set position
	Mouse       bool // Mouse event received
	Mx, My      int  // Mouse last known position, in 1,1 coordinate system
	Mbuttons    int  // Mouse buttons and modifier state, use the accessor methods, e.g [LeftClick] instead of this directly.
	Mrelease    bool // Mouse button release state
	C           chan os.Signal
	Margin      int          // Margin around the image (image is smaller by 2*margin)
	FPS         float64      // (Target) Frames per second used for Reading with timeout
	OnResize    func() error // Callback when terminal is resized
	OnMouse     func()       // Callback when mouse event is received
	// First time we clear the screen, we use 2J to push old content to scrollback buffer, otherwise we use H+0J
	firstClear bool
	restored   bool
	// In NoDecode mode the mouse decode and the end of sync are not done automatically.
	// used for fortio.org/tev raw event dump.
	NoDecode bool
	// Background color of the terminal if detected by [OSCDecode] after issuing [RequestBackgroundColor]
	Background          tcolor.RGBColor
	backgroundRequested bool
	GotBackground       bool // Whether we got a background color from the terminal (after RequestBackgroundColor and OSCDecode)
	// Whether to use transparency when drawing truecolor images ([SyncBackgroundColor] needed)
	Transparency bool
	// Whether FPSTicks automatically calls StartSyncMode and EndSyncMode around the callback.
	// Note that this prevents the cursor from blinking at FPS above 4. Default is true.
	AutoSync bool
	// Concurrent safe sink for output to the terminal while drawing (eg. logger output).
	// it's content if any will be dumped to the terminal when [EndSyncMode] is called.
	// LF are converted to CRLF automatically.
	Logger    *terminal.SyncWriter
	logbuffer terminal.FlushableBytesBuffer
	// Setup fortio logger unless this is set to false (defaults to true in NewAnsiPixels, clear before Open if desired).
	AutoLoggerSetup bool
}

// NewAnsiPixels creates a new ansipixels object (to be [Open] post customization if any).
// A 0 fps means bypassing the interrupt reader and using the underlying os.Stdin directly.
// Otherwise a non blocking reader is setup with 1/fps timeout. Reader is / can be shared
// with Terminal.
func NewAnsiPixels(fps float64) *AnsiPixels {
	var d time.Duration
	if fps > 0 {
		d = time.Duration(1e9 / fps)
	}
	ap := &AnsiPixels{
		fdOut:           safecast.MustConv[int](os.Stdout.Fd()),
		Out:             bufio.NewWriter(os.Stdout),
		FPS:             fps,
		SharedInput:     terminal.GetSharedInput(d),
		C:               make(chan os.Signal, 1),
		AutoSync:        true,
		AutoLoggerSetup: true,
	}
	ap.Logger = &terminal.SyncWriter{Out: &ap.logbuffer}
	ap.ColorMode = DetectColorMode()
	ap.ColorOutput = tcolor.ColorOutput{TrueColor: ap.TrueColor}
	signal.Notify(ap.C, signalList...)
	return ap
}

func (ap *AnsiPixels) ChangeFPS(fps float64) {
	ap.SharedInput.ChangeTimeout(1 * time.Second / time.Duration(fps))
}

// DetectColorMode uses environment variables COLORTERM,
// TERM and NO_COLOR to guess good starting values for Color and TrueColor.
// Use flags to give the option to user to override these settings.
// This is called by [NewAnsiPixels] but values can be changed based on flags
// before [Open]. Can also be called on a temporary empty AnsiPixels to
// extract flag default values (see usage in fps and tcolor demos for instance).
// Example of use:
//
//	defaultTrueColor := ansipixels.DetectColorMode().TrueColor
func DetectColorMode() (cm ColorMode) {
	cm.MonoColor = tcolor.Blue // default mono (16) color
	if os.Getenv("NO_COLOR") != "" {
		cm.Color256 = false
		cm.TrueColor = false
		cm.MonoColor = tcolor.White
		return cm
	}
	if os.Getenv("COLORTERM") != "" {
		cm.TrueColor = true
	}
	cm.TermEnv = os.Getenv("TERM")
	switch cm.TermEnv {
	case "xterm-256color":
		cm.Color256 = true
	case "xterm-truecolor", "xterm-kitty", "alacritty", "wezterm", "xterm-ghostty", "ghostty":
		cm.TrueColor = true
	case "":
		cm.TermEnv = TermNotSet
	}
	// TODO: how to find out we're inside windows terminal which also supports true color
	// but doesn't advertise it.
	if cm.TrueColor {
		cm.Color256 = true // if we have true color, we also have 256 color.
	}
	return cm
}

// Open sets the terminal in raw mode, gets the size and starts the shared input reader using
// default background context. Use [OpenWithContext] to pass a specific context for that underlying
// reader.
func (ap *AnsiPixels) Open() error {
	ap.firstClear = true
	ap.restored = false
	ap.ColorOutput.TrueColor = ap.TrueColor // sync, in case it was changed by flags from auto detect.
	err := ap.SharedInput.RawMode()
	if err == nil {
		err = ap.GetSize()
	}
	if ap.AutoLoggerSetup {
		ap.LoggerSetup()
	}
	ap.SharedInput.StartDirect()
	return err
}

// WriteFg writes the tcolor code as foreground color, including down converting from truecolor to closest 256 color.
func (ap *AnsiPixels) WriteFg(c tcolor.Color) {
	ap.WriteString(ap.ColorOutput.Foreground(c))
}

// WriteBg writes the tcolor code as background color, including down converting from truecolor to closest 256 color.
func (ap *AnsiPixels) WriteBg(c tcolor.Color) {
	ap.WriteString(ap.ColorOutput.Background(c))
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

// ReadOrResizeOrSignal reads something or return terminal.ErrSignal if signal is received (normal exit requested case),
// will automatically call OnResize if set and if a resize signal is received and continue trying to read.
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

// FPSTicks is a main program loop for fixed FPS applications: You pass a callback
// which will be called at fixed fps rate (outside of resize events which can call your OnResize callback asap).
// Data available if any (or mouse events decoded) will be set in ap.Data and the callback will be called every tick.
// The callback should return false to stop the loop (typically to exit the program), true to continue it.
// Note this is using and 'starting' ap.SharedInput unlike ReadOrResizeOrSignal which is using the underlying
// InterruptReader directly. Data can be lost if you mix the 2 modes (so don't).
// StartSyncMode and EndSyncMode are called around the callback when AutoSync is set (which is the default)
// to ensure the display is synchronized so you don't have to do it in the callback.
func (ap *AnsiPixels) FPSTicks(callback func() bool) error {
	if ap.FPS <= 0 {
		panic("FPSTicks called with non-positive FPS")
	}
	timer := time.NewTicker(time.Duration(1e9 / ap.FPS))
	ap.SharedInput.Start(context.Background())
	defer func() {
		timer.Stop()
		ap.SharedInput.Stop()
	}()
	for {
		select {
		case s := <-ap.C:
			err := ap.HandleSignal(s)
			if err != nil {
				return err
			}
		case <-timer.C:
			n, err := ap.SharedInput.ReadNonBlocking(ap.buf[0:bufSize])
			if err != nil {
				return err
			}
			ap.Data = ap.buf[0:n]
			if !ap.NoDecode {
				ap.MouseDecodeAll()
			}
			if ap.AutoSync {
				ap.StartSyncMode() // will also flush logger output if any.
			}
			cont := callback()
			if ap.AutoSync {
				ap.EndSyncMode()
			}
			if !cont {
				return nil // exit the loop
			}
		}
	}
}

// ReadOrResizeOrSignalOnce will return either because of signal, or something read or the timeout (fps) passed.
// ap.Data is (re)set to the read data. Note that if there is a lot of data produced (eg. mouse movements)
// this will return more often than the fps. Use [FPSTicks] with a callback for a fixed fps (outside of resize events)
// where data available will be set in ap.Data and the callback will be called every tick.
func (ap *AnsiPixels) ReadOrResizeOrSignalOnce() (int, error) {
	select {
	case s := <-ap.C:
		err := ap.HandleSignal(s)
		if err != nil {
			return 0, err
		}
	default:
		n, err := ap.SharedInput.DirectRead(ap.buf[0:bufSize])
		ap.Data = ap.buf[0:n]
		if !ap.NoDecode {
			ap.MouseDecodeAll()
		}
		return n, err
	}
	return 0, nil
}

// StartSyncMode starts the terminal output transaction. If AutoSync is set (default) logger output will get flushed as well.
func (ap *AnsiPixels) StartSyncMode() {
	if ap.AutoSync { // we could rely on && shortcut but for clarity, inner if:
		if ap.FlushLogger() {
			return // we're done, flushlogger did the start sync mode
		}
	}
	ap.startSyncMode()
}

// Internal unconditional/not dealing with logger start sync mode.
func (ap *AnsiPixels) startSyncMode() {
	ap.WriteString("\033[?2026h")
}

// FlushLogger flushes the logger output if any to the terminal (saving/restoring cursor position).
// Returns true if something was output. if there was output a StartSyncMode is done before that output.
func (ap *AnsiPixels) FlushLogger() bool {
	var extra []byte
	ap.Logger.Lock()
	hasOutput := ap.logbuffer.Len() > 0
	if hasOutput {
		extra = append(extra, ap.logbuffer.Bytes()...) // need to copy as we release the lock
		ap.logbuffer.Reset()
	}
	ap.Logger.Unlock()
	if !hasOutput {
		return false
	}
	ap.startSyncMode() // internal one so we don't infinite loop back here.
	ap.RestoreCursorPos()
	_, _ = terminal.CRLFWrite(ap.Out, extra)
	ap.SaveCursorPos()
	return true
}

// EndSyncMode ends sync (and flushes).
func (ap *AnsiPixels) EndSyncMode() {
	_, _ = ap.Out.WriteString("\033[?2026l")
	_ = ap.Out.Flush()
}

func (ap *AnsiPixels) GetSize() (err error) {
	ap.W, ap.H, err = term.GetSize(ap.fdOut)
	return err
}

func (ap *AnsiPixels) Restore() {
	if ap.restored {
		return
	}
	ap.ShowCursor()
	ap.FlushLogger() // in case there is something pending
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

// MoveCursor moves the cursor to the given x,y position (0,0 is top left).
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

func (ap *AnsiPixels) Printf(msg string, args ...interface{}) {
	_, _ = fmt.Fprintf(ap.Out, msg, args...)
}

// CopyToClipboard copies the given text to the system clipboard.
// Uses OSC 52 (supported by most terminal emulators).
func (ap *AnsiPixels) CopyToClipboard(text string) {
	ap.Printf("\033]52;c;%s\007", base64.StdEncoding.EncodeToString([]byte(text)))
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
	ap.WriteString(ClearLine)
}

func (ap *AnsiPixels) SaveCursorPos() {
	ap.WriteString("\0337")
	ap.Out.Flush()
}

func (ap *AnsiPixels) RestoreCursorPos() {
	ap.WriteString("\0338")
}

var cursPosRegexp = regexp.MustCompile(`^(.*)\033\[(\d+);(\d+)R(.*)$`)

// ReadCursorPosXY returns the current X,Y coordinates of the cursor or insertion point
// using the same coordinate system as MoveCursor, WriteAt, etc. ie 0,0 is the top left corner.
// it also synchronizes the display and ends the syncmode.
// It wraps the original/lower level [ReadCursorPos], which returns 1,1-based coordinates
// and x/y swapped (row, col terminal native coordinates), and converts them to 0,0-based coordinates.
func (ap *AnsiPixels) ReadCursorPosXY() (int, int, error) {
	row, col, err := ap.ReadCursorPos()
	if err != nil {
		return -1, -1, err
	}
	return col - 1, row - 1, nil // convert to 0,0 based coordinates
}

// ReadCursorPos requests and read native coordinates of the cursor and
// also synchronizes the display and ends the syncmode.
//
// It returns row,col (y,x; line and column) and/or an error.
// It's using the native terminal coordinates which start at 1,1 unlike
// the AnsiPixels coordinates which start at 0,0.
// Use [ReadCursorPosXY] to get 0,0 based coordinates usable with MoveCursor,
// WriteAt, etc.
func (ap *AnsiPixels) ReadCursorPos() (row int, col int, err error) {
	col = -1
	row = -1
	reqPosStr := "\033[?2026l\033[6n" // also ends sync mode
	var n int
	n, err = ap.Out.WriteString(reqPosStr)
	if err != nil {
		return row, col, err
	}
	if n != len(reqPosStr) {
		err = errors.New("short write")
		return row, col, err
	}
	err = ap.Out.Flush()
	if err != nil {
		return row, col, err
	}
	log.Debugf("Sent and flushed cursor position request %q", reqPosStr)
	i := 0
	ap.Data = nil
	for {
		if i == bufSize {
			err = errors.New("buffer full, no cursor position found")
			return row, col, err
		}
		n, err = ap.SharedInput.ReadBlocking(ap.buf[i:bufSize])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return row, col, err
		}
		if n == 0 {
			err = errors.New("no data read from cursor position")
			return row, col, err
		}
		log.Debugf("Read %d (response maybe) %q", n, ap.buf[0:i+n])
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
		row, err = strconv.Atoi(string(res[2]))
		if err != nil {
			return row, col, err
		}
		col, err = strconv.Atoi(string(res[3]))
		if err != nil {
			return row, col, err
		}
		ap.Data = append(ap.Data, res[1]...)
		ap.Data = append(ap.Data, res[4]...)
		break
	}
	if !ap.NoDecode {
		ap.MouseDecodeAll()
	}
	return row, col, err
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

// DrawColoredBox draws a colored box with the given background color and double width option
// which means extra bars on the left and right.
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
	leftX := (ap.W - maxw) / 2
	for i, l := range lines {
		cursorY = y + i
		ap.MoveCursor(leftX, cursorY)
		delta := (maxw - widths[i])
		ap.WriteString(strings.Repeat(" ", delta/2))
		ap.WriteString(l)
		ap.WriteString(strings.Repeat(" ", delta/2+delta%2)) // if odd, add 1 more space on the right
		cursorX = leftX + delta/2 + widths[i]
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

func (ap *AnsiPixels) SetBracketedPasteMode(on bool) {
	if on {
		ap.WriteString("\x1b[?2004h")
	} else {
		ap.WriteString("\x1b[?2004l")
	}
}

func FormatDate(d *time.Time) string {
	return fmt.Sprintf("%d-%02d-%02d-%02d%02d%02d", d.Year(), d.Month(), d.Day(),
		d.Hour(), d.Minute(), d.Second())
}

// LoggerSetup sets up the fortio logger to have CRLF and SyncWriter so it can log while we're drawing.
// (will only happen if stderr hasn't been redirected).
// Called automatically by [Open] unless AutoLoggerSetup is false.
func (ap *AnsiPixels) LoggerSetup() {
	terminal.LoggerSetup(ap.Logger)
}
