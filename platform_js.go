//go:build js && wasm

package terminal

import (
	"syscall/js"

	"fortio.org/log"
	"golang.org/x/term"
)

// Platform-specific wrappers for terminal operations in WASM/js.
//
// When running in a browser with xterm.js connected, the JS side sets:
//
//	globalThis.TerminalConnected = true
//	globalThis.TerminalCols = <number>
//	globalThis.TerminalRows = <number>
//
// This allows fortio.org/terminal to detect that a real terminal emulator
// is present and operate in raw/interactive mode. Without these globals,
// IsTerminal returns false (e.g. when using the textarea-based interface).

func platformIsTerminal(fd int) bool {
	return js.Global().Get("TerminalConnected").Truthy()
}

func platformMakeRaw(fd int) (*term.State, error) {
	// xterm.js already operates in raw mode — no OS-level terminal state to change.
	// Return a non-nil State so that Raw() returns true.
	log.Debugf("platformMakeRaw: no-op on js/wasm")
	return &term.State{}, nil
}

func platformRestore(fd int, st *term.State) error {
	// No terminal state to restore in WASM.
	log.Debugf("platformRestore: no-op on js/wasm")
	return nil
}

func platformGetSize(fd int) (width, height int, err error) {
	w := js.Global().Get("TerminalCols")
	h := js.Global().Get("TerminalRows")
	if w.Truthy() && h.Truthy() {
		return w.Int(), h.Int(), nil
	}
	return 80, 24, nil // sensible defaults if not set
}
