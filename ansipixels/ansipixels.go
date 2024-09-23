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

	"fortio.org/log"
	"fortio.org/safecast"
	"fortio.org/term"
)

type AnsiPixels struct {
	fd    int
	fdOut int
	Out   *bufio.Writer
	In    io.Reader
	state *term.State
	buf   [256]byte
	Data  []byte
	W, H  int // Width and Height
	x, y  int // Cursor position
	C     chan os.Signal
}

func NewAnsiPixels() *AnsiPixels {
	return &AnsiPixels{
		fd:    safecast.MustConvert[int](os.Stdin.Fd()),
		fdOut: safecast.MustConvert[int](os.Stdout.Fd()),
		Out:   bufio.NewWriter(os.Stdout),
		In:    os.Stdin,
	}
}

func (ap *AnsiPixels) Open() (err error) {
	ap.state, err = term.MakeRaw(ap.fd)
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
	err := term.Restore(ap.fd, ap.state)
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
		n, err = ap.In.Read(ap.buf[i:256])
		// log.Infof("Last buffer read: %q", buf[0:n])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return x, y, err
		}
		if n == 0 {
			return x, y, errors.New("no data read from cursor position")
		}
		res := cursPosRegexp.FindSubmatch(ap.buf[i:n])
		if res == nil {
			ap.Data = append(ap.Data, ap.buf[i:n]...)
			i = 0
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
