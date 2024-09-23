// Library to interact with ansi/v100 style terminals.
package terminal // import "fortio.org/terminal"

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strconv"

	"fortio.org/log"
	"fortio.org/safecast"
	"fortio.org/term"
)

type Terminal struct {
	// Use this for any output to the screen/console so the required \r are added in raw mode
	// the prompt and command edit is refresh as needed when input comes in.
	Out io.Writer
	// Cancellable context after Open(). Use it to cancel the terminal reading or check for done.
	Context     context.Context //nolint:containedctx // To avoid Open() returning 4 values.
	Cancel      context.CancelFunc
	fd          int
	fdOut       int
	oldState    *term.State
	term        *term.Terminal
	intrReader  *InterruptReader
	historyFile string
	capacity    int
	autoHistory bool
}

// Open opens stdin as a terminal, do `defer terminal.Close()`
// to restore the terminal to its original state upon exit.
// fortio.org/log (and thus stdlib "log") will be redirected
// to the terminal in a manner that preserves the prompt.
// New cancellable context is returned, use it to cancel the terminal
// reading or check for done for control-c or signal.
func Open(ctx context.Context) (t *Terminal, err error) {
	intrReader := NewInterruptReader(os.Stdin, 256) // same as the internal x/term buffer size.
	rw := struct {
		io.Reader
		io.Writer
	}{intrReader, os.Stderr}
	t = &Terminal{
		fd:         safecast.MustConvert[int](os.Stdin.Fd()),
		fdOut:      safecast.MustConvert[int](os.Stdout.Fd()),
		intrReader: intrReader,
		Context:    ctx,
	}
	t.term = term.NewTerminal(rw, "")
	t.Out = t.term
	if !t.IsTerminal() {
		t.Out = os.Stderr // no need to add \r for non raw mode.
		t.ResetInterrupts(ctx)
		return
	}
	t.oldState, err = term.MakeRaw(t.fd)
	if err != nil {
		return
	}
	t.term.SetBracketedPasteMode(true) // Seems useful to have it on by default.
	t.capacity = term.DefaultHistoryEntries
	t.loggerSetup()
	t.ResetInterrupts(ctx)
	return
}

// If you want to reset and restart after an interrupt, call this.
func (t *Terminal) ResetInterrupts(ctx context.Context) (context.Context, context.CancelFunc) {
	// locking should not be needed as we're (supposed to be) in the main thread.
	t.Context, t.Cancel = t.intrReader.Start(ctx)
	return t.Context, t.Cancel
}

func (t *Terminal) IsTerminal() bool {
	return term.IsTerminal(t.fd)
}

// Setups fortio logger (and thus stdlib "log" too)
// to write to the terminal as needed to preserve prompt.
func (t *Terminal) loggerSetup() {
	// Keep same color logic as fortio logger, so flags like -logger-no-color work.
	colormode := log.ColorMode()
	// t.Out will add the needed \r for each \n when term is in raw mode
	log.SetOutput(t.Out)
	log.Config.ForceColor = colormode
	log.SetColorMode()
}

// Sets up a file to load and save history from/to. File is being read when this is called.
// If no error is returned, the file will also be automatically updated on Close().
func (t *Terminal) SetHistoryFile(f string) error {
	if f == "" {
		log.Infof("No history file specified")
		return nil
	}
	if t.capacity <= 0 {
		log.Infof("No history capacity set, ignoring history file %s", f)
		return nil
	}
	if !t.IsTerminal() {
		log.Infof("Not a terminal, not setting history file")
		return nil
	}
	t.historyFile = f
	entries, err := readOrCreateHistory(f)
	if err != nil {
		t.historyFile = "" // so we don't try to save during defer'ed close if we can't read
		return err
	}
	start := 0
	if len(entries) > t.capacity {
		log.Infof("History file %s has more than %d entries, truncating.", f, t.capacity)
		start = len(entries) - t.capacity
	} else {
		log.Infof("Loaded %d history entries from %s", len(entries), f)
	}
	for _, e := range entries[start:] {
		t.term.AddToHistory(e)
	}
	return nil
}

// Forward the term history API and not just the high level file history api above.

// AddToHistory add commands to the history.
func (t *Terminal) AddToHistory(commands ...string) {
	t.term.AddToHistory(commands...)
}

// History returns the current history state.
func (t *Terminal) History() []string {
	return t.term.History()
}

// DefaultHistoryCapacity is the default number of entries in the history (99).
const DefaultHistoryCapacity = term.DefaultHistoryEntries

// NewHistory creates/resets the history to a new one with the given capacity.
// Using 0 as capacity will disable history reading and writing but not change
// the underlying history state from it's [DefaultHistoryCapacity].
func (t *Terminal) NewHistory(capacity int) {
	if capacity < 0 {
		log.Errf("Invalid history capacity %d, ignoring", capacity)
		return
	}
	t.capacity = capacity
	if capacity == 0 { // leave the underlying history as is, avoids crashing with 0 as well.
		return
	}
	t.term.NewHistory(capacity)
}

// SetAutoHistory enables/disables auto history (default is enabled).
func (t *Terminal) SetAutoHistory(enabled bool) {
	t.autoHistory = enabled
	t.term.AutoHistory(enabled)
}

// AutoHistory returns the current auto history setting.
func (t *Terminal) AutoHistory() bool {
	return t.autoHistory
}

// ReplaceLatest replaces the current history with the given commands, returns the previous value.
func (t *Terminal) ReplaceLatest(command string) string {
	return t.term.ReplaceLatest(command)
}

func readOrCreateHistory(f string) ([]string, error) {
	// open file or create it
	h, err := os.OpenFile(f, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		log.Errf("Error opening history file %s: %v", f, err)
		return nil, err
	}
	defer h.Close()
	// read lines separated by \n
	var lines []string
	scanner := bufio.NewScanner(h)
	for scanner.Scan() {
		// unquote to get the actual command
		rl := scanner.Text()
		l, err := strconv.Unquote(rl)
		if err != nil {
			log.Errf("Error unquoting history file %s for %q: %v", f, rl, err)
			return nil, err
		}
		lines = append(lines, l)
	}
	if err := scanner.Err(); err != nil {
		log.Errf("Error reading history file %s: %v", f, err)
		return nil, err
	}
	return lines, nil
}

// We don't return any error because this is ran through a defer at the end of the program.
// So logging errors is the best thing we can do.
func saveHistory(f string, h []string) {
	// open file or create it
	hf, err := os.OpenFile(f, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o600)
	if err != nil {
		log.Errf("Error opening history file %s: %v", f, err)
		return
	}
	defer hf.Close()
	// write lines separated by \n
	for _, l := range h {
		_, err := hf.WriteString(strconv.Quote(l) + "\n")
		if err != nil {
			log.Errf("Error writing history file %s: %v", f, err)
			return
		}
	}
}

// Temporarily suspend/resume of the terminal back to normal (for example to run a sub process).
// use defer t.Resume() after calling Suspend() to put the terminal back in raw mode.
func (t *Terminal) Suspend() {
	if t.oldState == nil {
		return
	}
	t.intrReader.Stop() // stop the interrupt reader
	err := term.Restore(t.fd, t.oldState)
	if err != nil {
		log.Errf("Error restoring terminal for suspend: %v", err)
	}
}

func (t *Terminal) Resume(ctx context.Context) (context.Context, context.CancelFunc) {
	if t.oldState == nil {
		return nil, nil
	}
	_, err := term.MakeRaw(t.fd)
	if err != nil {
		log.Errf("Error for terminal resume: %v", err)
	}
	return t.ResetInterrupts(ctx) // resume the interrupt reader
}

// Close restores the terminal to its original state. Must be called at exit to avoid leaving
// the terminal in raw mode. Safe to call multiple times. Will save the history to the history file
// if one was set using [SetHistoryFile] and the capacity is > 0.
func (t *Terminal) Close() error {
	if t.oldState == nil {
		return nil
	}
	// To avoid prompt being repeated on the last line (shouldn't be necessary but... is
	// consider fixing in term instead)
	t.term.SetPrompt("") // will still reprint the last command on ^C in middle of typing.
	t.Cancel()           // cancel the interrupt reader
	err := term.Restore(t.fd, t.oldState)
	t.oldState = nil
	t.Out = os.Stderr
	// saving history if any
	if t.historyFile == "" || t.capacity <= 0 {
		log.Debugf("No history file %q or capacity %d, not saving history", t.historyFile, t.capacity)
		return nil
	}
	h := t.term.History()
	// log.LogVf("got history %v", h)
	slices.Reverse(h)
	extra := len(h) - t.capacity
	if extra > 0 {
		h = h[extra:] // truncate to max capacity otherwise extra ones will get out of order
	}
	log.Infof("Saving history (%d commands) to %s", len(h), t.historyFile)
	saveHistory(t.historyFile, h)
	return err
}

// ReadLine reads a line from the terminal using the setup prompt and history
// and edit capabilities. Returns the line and an error if any. io.EOF is returned
// when the user presses Control-D. ErrInterrupted is returned when the user presses
// Control-C or a signal is received.
func (t *Terminal) ReadLine() (string, error) {
	c, err := t.term.ReadLine()
	// That error isn't an error that needs to be propagated,
	// it's just to allow copy/paste without autocomplete.
	if errors.Is(err, term.ErrPasteIndicator) {
		return c, nil
	}
	return c, err
}

// Sets or change the prompt.
func (t *Terminal) SetPrompt(s string) {
	t.term.SetPrompt(s)
}

// AutoCompleteCallback is called with "this" terminal as first argument so AutoCompleteCallback
// can use t.Out etc. (compared to the original x/term callback).
type AutoCompleteCallback func(t *Terminal, line string, pos int, key rune) (newLine string, newPos int, ok bool)

// SetAutoCompleteCallback sets the callback called for each key press. Can be used to implement
// auto completion. See example/main.go for an example.
func (t *Terminal) SetAutoCompleteCallback(f AutoCompleteCallback) {
	t.term.AutoCompleteCallback = func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
		return f(t, line, pos, key)
	}
}
