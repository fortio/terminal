// Package terminal provides a library to interact with ansi/v100 style terminals, readline style.
// It wraps golang.org/x/term and adds history, prompt, auto-completion and
// interrupt handling (control-C and signals).
// It also provides a way to suspend the terminal (restore normal mode) to run sub commands
// and resume it back to raw mode.
// It also provides a way to share a single InterruptReader between multiple users of the terminal
// (for instance both Terminal.ReadLine and AnsiPixels.ReadOrResizeOrSignal can share it).
// See example/main.go for an example of usage.
package terminal // import "fortio.org/terminal"

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"time"

	"fortio.org/log"
	"fortio.org/safecast"
	"golang.org/x/term"
)

type Terminal struct {
	// Use this for any output to the screen/console so the required \r are added in raw mode
	// the prompt and command edit is refresh as needed when input comes in.
	Out io.Writer
	// Cancellable context after Open(). Use it to cancel the terminal reading or check for done.
	Context context.Context //nolint:containedctx // To avoid Open() returning 4 values.
	Cancel  context.CancelFunc
	fd      int
	fdOut   int
	term    *term.Terminal
	// [InterruptReader.Read] can read from the underlying terminal instead of a full line with history and prompt.
	// Can be used to read single keystrokes or events still in raw mode, without waiting for a new line.
	// [InterruptReader.ReadNonBlocking] reads from the underlying terminal if there is data available.
	// Can be used to check for single keystrokes or events still in raw mode,
	// while refreshing the output (ie without blocking).
	IntrReader  *InterruptReader
	historyFile string
	capacity    int
	history     *TermHistory // original implementation of new History + exposed constructor etc.
	// Terminal (last updated) width.
	Width int
	// Terminal (last updated) height.
	Height     int
	lastPrompt []byte
	// lastWasPaste is true if the last ReadLine was a paste.
	lastWasPaste bool
}

// Open opens stdin as a terminal, do `defer terminal.Close()`
// to restore the terminal to its original state upon exit.
// fortio.org/log (and thus stdlib "log") will be redirected
// to the terminal in a manner that preserves the prompt.
// A default 250ms InterruptReader is used if not set already.
// New cancellable context is set, use it to cancel the terminal
// reading or check for done for control-c or signal.
// You can also call [Setup] to customize the terminal with a custom InterruptReader
// and/or a custom history.
func Open(ctx context.Context) (*Terminal, error) {
	t := &Terminal{
		IntrReader: GetSharedInput(250 * time.Millisecond),
		history:    NewHistory(DefaultHistoryCapacity),
	}
	err := t.Setup(ctx)
	return t, err
}

func (t *Terminal) Setup(ctx context.Context) error {
	t.Context = ctx
	t.fd = safecast.MustConv[int](os.Stdin.Fd())
	t.fdOut = safecast.MustConv[int](os.Stdout.Fd())
	rw := struct {
		io.Reader
		io.Writer
	}{t.IntrReader, os.Stdout}
	t.term = term.NewTerminal(rw, "")
	t.term.History = t.history
	t.Out = t.term
	if !t.IsTerminal() {
		t.Out = os.Stdout // no need to add \r for non raw mode.
		t.ResetInterrupts(ctx)
		return nil
	}
	var err error
	err = t.IntrReader.RawMode()
	if err != nil {
		return err
	}
	t.term.SetBracketedPasteMode(true) // Seems useful to have it on by default.
	t.capacity = DefaultHistoryCapacity
	LoggerSetup(t.Out)
	err = t.UpdateSize() // error already logged - tbd to return or not / not fatal
	t.ResetInterrupts(ctx)
	return err
}

// UpdateSize refreshes the terminal size to current size (so wrapping works).
// This is called automatically when the terminal is opened, but can be called
// again if the terminal size changes (e.g. when resizing the window).
func (t *Terminal) UpdateSize() error {
	w, h, err := platformGetSize(t.fdOut)
	if err != nil {
		log.Errf("Error getting terminal size: %v", err)
		return err
	}
	log.Debugf("Terminal size: %d x %d", w, h)
	t.Width, t.Height = w, h
	err = t.term.SetSize(w, h)
	if err != nil {
		log.Errf("Error setting terminal size (%d x %d): %v", w, h, err)
		return err
	}
	return nil
}

// ResetInterrupts resets and restarts if you want to continue after an interrupt.
func (t *Terminal) ResetInterrupts(ctx context.Context) (context.Context, context.CancelFunc) {
	// locking should not be needed as we're (supposed to be) in the main thread.
	t.Context, t.Cancel = t.IntrReader.Start(ctx)
	return t.Context, t.Cancel
}

func (t *Terminal) IsTerminal() bool {
	return platformIsTerminal(t.fd)
}

// LoggerSetup sets up fortio logger (and thus stdlib "log" too)
// to write to the terminal as needed to preserve prompt/work in raw mode (ie add \r before \n).
// If stderr has been redirected, it doesn't do anything (logs to json in said case).
func LoggerSetup(out io.Writer) {
	// If stderr has been redirected, don't change anything.
	if !log.ConsoleLogging() {
		return
	}
	// Keep same color logic as fortio logger, so flags like -logger-no-color work.
	colormode := log.ColorMode()
	// t.Out will add the needed \r for each \n when term is in raw mode
	log.SetOutput(out)
	log.Config.ForceColor = colormode
	log.SetColorMode()
}

// SetHistoryFile sets up a file to load and save history from/to. File is being read when this is called.
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
		t.AddToHistory(e)
	}
	return nil
}

// Implements the rest of the term history API and not just the high level file history api above.

// AddToHistory add commands to the TermHistory.
func (t *Terminal) AddToHistory(commands ...string) {
	for _, c := range commands {
		t.history.UnconditionalAdd(c)
	}
}

// History returns the current history state.
func (t *Terminal) History() []string {
	res := make([]string, 0, t.term.History.Len())
	for i := range t.term.History.Len() {
		res = append(res, t.term.History.At(i))
	}
	return res
}

// DefaultHistoryCapacity is the default number of entries in the history (99).
// History index 1-99 prints using %02d.
const DefaultHistoryCapacity = 99

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
	newHistory := NewHistory(capacity)
	// Copy AutoHistory setting (if any)
	if t.history != nil {
		newHistory.AutoHistory = t.history.AutoHistory
	}
	t.history = newHistory
	t.term.History = t.history
}

// SetAutoHistory enables/disables auto history (default is enabled).
func (t *Terminal) SetAutoHistory(enabled bool) {
	log.Debugf("SetAutoHistory %t", enabled)
	t.history.AutoHistory = enabled
}

// AutoHistory returns the current auto history setting.
func (t *Terminal) AutoHistory() bool {
	return t.history.AutoHistory
}

// ReplaceLatest replaces the current history with the given commands, returns the previous value.
// Enables to add invalid commands to the history for editing purpose and
// replace them with the corrected version. Returns the replaced entry.
func (t *Terminal) ReplaceLatest(command string) string {
	return t.history.Replace(command)
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

// Suspend temporarily suspends/resumes the terminal back to normal (for example to run a sub process).
// use defer t.Resume() after calling Suspend() to put the terminal back in raw mode.
func (t *Terminal) Suspend() {
	t.IntrReader.Stop() // stop the interrupt reader
	err := t.IntrReader.NormalMode()
	if err != nil {
		log.Errf("Error restoring terminal for suspend: %v", err)
	}
}

func (t *Terminal) Resume(ctx context.Context) (context.Context, context.CancelFunc) {
	err := t.IntrReader.RawMode()
	if err != nil {
		log.Errf("Error for terminal resume: %v", err)
	}
	log.Debugf("Restarting term...")
	return t.ResetInterrupts(ctx) // resume the interrupt reader
}

// Close restores the terminal to its original state. Must be called at exit to avoid leaving
// the terminal in raw mode. Safe to call multiple times. Will save the history to the history file
// if one was set using [SetHistoryFile] and the capacity is > 0.
func (t *Terminal) Close() error {
	if t.IntrReader == nil {
		return nil
	}
	// To avoid prompt being repeated on the last line (shouldn't be necessary but... is
	// consider fixing in term instead)
	t.term.SetPrompt("") // will still reprint the last command on ^C in middle of typing.
	t.Cancel()           // cancel the interrupt reader
	err := t.IntrReader.NormalMode()
	t.IntrReader.Close()
	t.IntrReader = nil
	// t.Out = os.Stdout // races during exit.
	// saving history if any - ok to panic (in a bad History implementation)
	// after this point as we already restored the terminal.
	if t.historyFile == "" || t.capacity <= 0 {
		log.Debugf("No history file %q or capacity %d, not saving history", t.historyFile, t.capacity)
		return nil
	}
	h := t.History()
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
// We forward to term.ReadLine when in raw mode, otherwise we read until \n or \r.
// x/term.ReadLine unfortunately doesn't support \n, so we need to handle that ourselves.
func (t *Terminal) ReadLine() (string, error) {
	t.lastWasPaste = false // reset paste indicator
	if !t.IntrReader.Raw() {
		_, _ = t.Out.Write(t.lastPrompt)
		return t.IntrReader.ReadLine()
	}
	c, err := t.term.ReadLine()
	// If Ctrl-D generated a synthetic EOF, we need to close the interrupt reader.
	if errors.Is(err, io.EOF) && !t.IntrReader.InEOF() {
		log.LogVf("ReadLine got artificial EOF, closing interrupt reader %q", c)
		t.IntrReader.Stop()
		t.IntrReader.tr.Close()
	}
	// That error isn't an error that needs to be propagated,
	// it's just to allow copy/paste without autocomplete.
	if errors.Is(err, term.ErrPasteIndicator) {
		t.lastWasPaste = true // If someone wants to know if this was a paste or not.
		log.Debugf("ReadLine got paste indicator, swallowing that virtual error %v", err)
		return c, nil
	}
	return c, err
}

// LastWasPaste returns true if the last ReadLine was a multiline paste.
func (t *Terminal) LastWasPaste() bool {
	return t.lastWasPaste
}

// SetPrompt sets or change the prompt.
func (t *Terminal) SetPrompt(s string) {
	t.lastPrompt = []byte(s)
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

// -- History ring buffer as in https://github.com/golang/term/pull/15/files
// (ie same but with size configurable and using the History API from
// https://github.com/golang/go/issues/68780#issuecomment-2707041053 )

// TermHistory is a ring buffer of strings.
type TermHistory struct {
	// entries contains max elements.
	entries []string
	max     int
	// head contains the index of the element most recently added to the ring.
	head int
	// size contains the number of elements in the ring.
	size int
	// AutoHistory, if true, causes lines to be automatically added to the history (through term's History.Add()).
	// If false, call AddToHistory to add lines to the history for instance only adding
	// successful commands. Defaults to true. This is controlled through AutoHistory(bool).
	AutoHistory bool
}

// NewHistory creates a new ring buffer of strings with the given capacity.
func NewHistory(capacity int) *TermHistory {
	return &TermHistory{
		entries:     make([]string, capacity),
		max:         capacity,
		AutoHistory: true,
	}
}

// Add is the term.History interface implementation and
// conditionally adds a string to the ring buffer based on the autoHistory flag.
func (th *TermHistory) Add(a string) {
	log.Debugf("Called Add(%q) for history - auto %t", a, th.AutoHistory)
	if !th.AutoHistory {
		return
	}
	th.UnconditionalAdd(a)
}

// UnconditionalAdd unconditionally add a string to the ring buffer.
func (th *TermHistory) UnconditionalAdd(a string) {
	if th.entries[th.head] == a {
		// Already there at the top, so don't add.
		// Also has the nice side effect of ignoring empty strings,
		// no s.size check on purpose.
		return
	}
	th.head = (th.head + 1) % th.max
	th.entries[th.head] = a
	if th.size < th.max {
		th.size++
	}
}

// Replace theoretically could panic on an empty ring buffer but
// it's harmless on strings.
func (th *TermHistory) Replace(a string) string {
	previous := th.entries[th.head]
	th.entries[th.head] = a
	return previous
}

// At returns the value passed to the nth previous call to Add.
// If n is zero then the immediately prior value is returned, if one, then the
// next most recent, and so on. If such an element doesn't exist then ok is
// false.
func (th *TermHistory) At(n int) string {
	log.Debugf("Called At(%d) for history (head %d sz %d max %d)", n, th.head, th.size, th.max)
	if n < 0 || n >= th.size {
		panic(fmt.Sprintf("TermHistory: index [%d] out of range [0,%d)", n, th.size))
	}
	index := th.head - n
	if index < 0 {
		index += th.max
	}
	/* test panic in History:
	res := th.entries[index]
	if strings.HasPrefix(res, "panic ") {
		panic(res[6:])
	}
	*/
	return th.entries[index]
}

// Len returns the current number of elements in the ring.
func (th *TermHistory) Len() int {
	log.Debugf("Called Len() for history (head %d sz %d max %d)", th.head, th.size, th.max)
	return th.size
}
