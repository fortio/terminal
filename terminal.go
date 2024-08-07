// Library to interact with ansi terminal
package terminal // import "grol.io/terminal"

import (
	"golang.org/x/term"
)

func IsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}
