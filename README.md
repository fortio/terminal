[![Go Reference](https://pkg.go.dev/badge/fortio.org/terminal.svg)](https://pkg.go.dev/fortio.org/terminal)
# terminal

Fortio's terminal is a `readline` style library.

See [example/main.go](example/main.go) for a rather complete example/demo.

See the godoc above for details.

The [grol](https://github.com/grol-io/grol#grol) command line repl and others use this.

The implementations currently is a wrapper fully encapsulating (our fork of) [x/term](https://github.com/golang/term), i.e. [fortio.org/term](https://github.com/fortio/term)
