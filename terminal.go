// Library to interact with ansi terminal
package terminal // import "grol.io/terminal"

import (
	"os"

	"golang.org/x/term"
)

type Terminal struct {
	fd       int
	oldState *term.State
}

// Open opens stdin as a terminal, do `defer terminal.Close()`
// to restore the terminal to its original state upon exit.
func Open() (*Terminal, error) {
	t := &Terminal{fd: int(os.Stdin.Fd())}
	if !t.IsTerminal() {
		return t, nil
	}
	var err error
	t.oldState, err = term.MakeRaw(t.fd)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Terminal) IsTerminal() bool {
	return term.IsTerminal(t.fd)
}

func (t *Terminal) Close() error {
	if t.oldState == nil {
		return nil
	}
	err := term.Restore(t.fd, t.oldState)
	t.oldState = nil
	return err
}
