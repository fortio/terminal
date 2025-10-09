package terminal

import (
	"os"
	"sync"
	"time"

	"fortio.org/log"
	"golang.org/x/term"
)

// DefaultReaderTimeout is the default timeout for the timeout reader.
var DefaultReaderTimeout = 250 * time.Millisecond

// InterruptReader is global input that can be shared by multiple callers wanting to interact with the terminal in raw mode.
// For instance both Terminal.ReadLine and AnsiPixels.ReadOrResizeOrSignal can share it.
// InterruptReader is an InterruptReader (for terminal.ReadLine, only the TimeoutReader part is used by ansipixels).
// We now delay creating it so a simple blocking mode can be achieved using 0 as the timeout.
var sharedInput *InterruptReader

var mu sync.Mutex // protects sharedInput, so that multiple calls to GetSharedInput() don't create multiple readers.

// GetSharedInput returns a shared input that can be used by multiple callers wanting to interact with the terminal in raw mode.
// For instance both Terminal.ReadLine and AnsiPixels.ReadOrResizeOrSignal can share it.
// It also changes the timeout for the timeout reader to the maxRead duration if different than before.
// If 0 timeout is used, it can't be changed later and the reader will block on reads (raw/simple os.Stdin mode).
func GetSharedInput(maxRead time.Duration) *InterruptReader {
	mu.Lock()
	if sharedInput == nil {
		log.LogVf("Creating first shared input reader with timeout: %v", maxRead)
		sharedInput = NewInterruptReader(os.Stdin, 256, maxRead)
	} else {
		sharedInput.ChangeTimeout(maxRead)
	}
	mu.Unlock()
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

// Raw returns true if the terminal is currently in raw mode.
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
	ir.tr.Close()
	return err
}
