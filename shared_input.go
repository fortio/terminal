package terminal

import (
	"os"
	"time"

	"fortio.org/log"
	"golang.org/x/term"
)

// DefaultReaderTimeout is the default timeout for the timeout reader.
var DefaultReaderTimeout = 250 * time.Millisecond

// InterruptReader is global input that can be shared by multiple callers wanting to interact with the terminal in raw mode.
// For instance both Terminal.ReadLine and AnsiPixels.ReadOrResizeOrSignal can share it.
// InterruptReader is an InterruptReader (for terminal.ReadLine, only the TimeoutReader part is used by ansipixels).
var sharedInput = NewInterruptReader(os.Stdin, 256, DefaultReaderTimeout)

// GetSharedInput returns a shared input that can be used by multiple callers wanting to interact with the terminal in raw mode.
// For instance both Terminal.ReadLine and AnsiPixels.ReadOrResizeOrSignal can share it.
// It also changes the timeout for the timeout reader to the maxRead duration if different than before.
func GetSharedInput(maxRead time.Duration) *InterruptReader {
	sharedInput.mu.Lock()
	if sharedInput.TR == nil {
		sharedInput.TR = NewTimeoutReader(sharedInput.In, maxRead) // same buffer as the internal x/term buffer size.
	} else {
		sharedInput.TR.ChangeTimeout(maxRead)
	}
	sharedInput.mu.Unlock()
	return sharedInput
}

// RawMode sets the terminal to raw mode.
// It's a no-op if the terminal is already in raw mode.
// Must typically defer ir.NormalMode() after calling this to
// avoid leaving the terminal in raw mode upon exit/panic.
func (ir *InterruptReader) RawMode() error {
	ir.mu.Lock()
	if ir.st != nil {
		ir.mu.Unlock() // unlock before logging/IOs.
		log.Debugf("RawMode already set - noop")
		return nil
	}
	fd := ir.In.Fd()
	var err error
	ir.st, err = term.MakeRaw(int(fd))
	ir.mu.Unlock()
	if err != nil {
		log.Errf("Failed to set raw mode: %v", err)
	}
	return err
}

// NormalMode sets the terminal to normal mode.
// It's a no-op if the terminal is already in normal mode.
func (ir *InterruptReader) NormalMode() error {
	ir.mu.Lock()
	if ir.st == nil {
		ir.mu.Unlock()
		log.Debugf("NormalMode already set - noop")
		return nil
	}
	err := term.Restore(int(ir.In.Fd()), ir.st)
	ir.st = nil
	ir.mu.Unlock()
	return err
}

// Raw returns true if the terminal is currentlyin raw mode.
func (ir *InterruptReader) Raw() bool {
	ir.mu.Lock()
	defer ir.mu.Unlock()
	return ir.st != nil
}

// Close restores the terminal to its original state and closes the underlying timeout reader.
// That ends the polling goroutine in non select OSes/mode but doesn't impact the underlying stream (os.Stdin).
// It also returns the terminal to normal mode.
func (ir *InterruptReader) Close() error {
	err := ir.NormalMode()
	ir.TR.Close()
	return err
}
