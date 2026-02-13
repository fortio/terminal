//go:build !(js && wasm)

package terminal

import "golang.org/x/term"

// Platform-specific wrappers for terminal operations.
// On native platforms, these delegate directly to golang.org/x/term.

func platformIsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

func platformMakeRaw(fd int) (*term.State, error) {
	return term.MakeRaw(fd)
}

func platformRestore(fd int, st *term.State) error {
	return term.Restore(fd, st)
}

func platformGetSize(fd int) (width, height int, err error) {
	return term.GetSize(fd)
}
