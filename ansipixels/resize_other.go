// This was really `windows` but wasm also has SIGTERM so let's see what breaks.
//go:build !unix

package ansipixels

import "syscall"

const ResizeSignal = syscall.Signal(999) // Virtual signal, for ssh on windows for instance and once we figure out #202.
