[![Go Reference](https://pkg.go.dev/badge/fortio.org/terminal.svg)](https://pkg.go.dev/fortio.org/terminal)
# terminal

Fortio's terminal is a `readline` style library. It handles prompts, edit (like Ctrl-A for beginning of line etc...), navigating through history using arrow keys, loading and saving history from file, etc... It works on everywhere go does (including macOS, Windows (using Terminal app), Linux).

See [example/main.go](example/main.go) for a rather complete example/demo.

See the godoc above for details.

The [grol](https://github.com/grol-io/grol#grol) command line repl and others use this.

The implementations currently is a wrapper fully encapsulating (our fork of) [x/term](https://github.com/golang/term), i.e. [fortio.org/term](https://github.com/fortio/term) and new features like the interrupts handling (filters Ctrl-C ahead of term' reads)
