package terminal_test

import (
	"testing"

	"fortio.org/terminal"
)

// Darth test - your lack of testing is disturbing

func TestSetHistoryFile(t *testing.T) {
	term := terminal.Terminal{}
	err := term.SetHistoryFile("/a/b/c")
	if err != nil {
		t.Errorf("as we run tests without terminal, history file should be ignored/no error: %v", err)
	}
}
