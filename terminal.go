// Library to interact with ansi/v100 style terminals.
package terminal // import "fortio.org/terminal"

import (
	"io"
	"os"

	"fortio.org/log"
	"golang.org/x/term"
)

type Terminal struct {
	fd       int
	oldState *term.State
	term     *term.Terminal
	Out      io.Writer
}

// CRWriter is a writer that adds \r before each \n.
// Needed for raw mode terminals (and I guess also if you want something DOS or http headers like).
type CRWriter struct {
	buf []byte
	Out io.Writer
}

// In case you want to ensure the memory used by the buffer is released.
func (c *CRWriter) Reset() {
	c.buf = nil
}

// Optimized to avoid many small writes by buffering and only writing \r when needed.
// No extra syscall, relies on append() to efficiently reallocate the buffer.
func (c *CRWriter) Write(orig []byte) (n int, err error) {
	l := len(orig)
	if l == 0 {
		return 0, nil
	}
	if l == 1 {
		if orig[0] != '\n' {
			return c.Out.Write(orig)
		}
		_, err = c.Out.Write([]byte("\r\n"))
		return 1, err
	}
	lastEmitted := 0
	for i, b := range orig {
		if b != '\n' { // IndexByte is probably faster than this.
			continue
		}
		// leave the \n for next append. I wish I could write
		//   c.buf = append(c.buf, orig[lastEmitted:i]..., `\r`)
		// instead of the 2 lines.
		c.buf = append(c.buf, orig[lastEmitted:i]...)
		c.buf = append(c.buf, '\r')
		lastEmitted = i
	}
	if lastEmitted == 0 {
		return c.Out.Write(orig)
	}
	c.buf = append(c.buf, orig[lastEmitted:]...)
	_, err = c.Out.Write(c.buf)
	return len(orig), err // in case caller checks... but we might have written "more".
}

// Open opens stdin as a terminal, do `defer terminal.Close()`
// to restore the terminal to its original state upon exit.
func Open() (*Terminal, error) {
	rw := struct {
		io.Reader
		io.Writer
	}{os.Stdin, os.Stderr}
	t := &Terminal{
		fd: int(os.Stdin.Fd()),
	}
	t.term = term.NewTerminal(rw, "")
	t.Out = t.term
	if !t.IsTerminal() {
		t.Out = os.Stderr // no need to add \r for non raw mode.
		return t, nil
	}
	var err error
	t.oldState, err = term.MakeRaw(t.fd)
	if err != nil {
		return nil, err
	}
	t.term.SetBracketedPasteMode(true) // Seems useful to have it on by default.
	return t, nil
}

func (t *Terminal) IsTerminal() bool {
	return term.IsTerminal(t.fd)
}

// Setups fortio logger to write to the terminal as needed to preserve prompt.
func (t *Terminal) LoggerSetup() {
	isTerm := t.IsTerminal()
	// t.Out will add the needed \r for each \n when term is in raw mode
	log.SetOutput(t.Out)
	log.Config.ForceColor = isTerm
	log.SetColorMode()
}

func (t *Terminal) Close() error {
	if t.oldState == nil {
		return nil
	}
	err := term.Restore(t.fd, t.oldState)
	t.oldState = nil
	t.Out = os.Stderr
	return err
}

func (t *Terminal) ReadLine() (string, error) {
	return t.term.ReadLine()
}

func (t *Terminal) SetPrompt(s string) {
	t.term.SetPrompt(s)
}

type AutoCompleteCallback func(line string, pos int, key rune) (newLine string, newPos int, ok bool)

func (t *Terminal) SetAutoCompleteCallback(f AutoCompleteCallback) {
	t.term.AutoCompleteCallback = f
}
