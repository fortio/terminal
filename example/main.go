package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal"
	"golang.org/x/term"
)

func main() {
	os.Exit(Main())
}

const (
	promptCmd = "prompt "
	afterCmd  = "after "
	exitCmd   = "exit"
	helpCmd   = "help"
	testMLCmd = "multiline"
)

var commands = []string{promptCmd, afterCmd, exitCmd, helpCmd, testMLCmd}

// func(line string, pos int, key rune) (newLine string, newPos int, ok bool)

func autoCompleteCallback(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
	log.LogVf("AutoCompleteCallback: %q %d %q", line, pos, key)
	if key != '\t' {
		return // only tab for now
	}
	if len(line) == 0 {
		log.Infof("Available commands: %v", commands)
		return
	}
	if pos != len(line) {
		return // end only (for now)
	}
	for _, c := range commands { // for now all have unique prefixes
		if strings.HasPrefix(c, line) {
			if c == testMLCmd {
				ret := "multiline {\r\n\tline1\r\n\tline2"
				return ret, len(ret), true
			}
			return c, len(c), true
		}
	}
	return
}

func Main() int {
	// Pending https://github.com/golang/go/issues/68780
	flagHistory := flag.String("history", "/tmp/terminal_history", "History `file` to use")
	cli.Main()
	t, err := terminal.Open()
	if err != nil {
		return log.FErrf("Error opening terminal: %v", err)
	}
	defer t.Close()
	t.SetPrompt("Terminal demo> ")
	t.LoggerSetup()
	t.SetHistoryFile(*flagHistory)
	fmt.Fprintf(t.Out, "Terminal is open\nis valid %t\nuse exit or ^D or ^C to exit\n", t.IsTerminal())
	fmt.Fprintf(t.Out, "Use 'prompt <new prompt>' to change the prompt\n")
	fmt.Fprintf(t.Out, "Try 'after duration text...' to see text showing in the middle of edits after said duration\n")
	fmt.Fprintf(t.Out, "Try <tab> for auto completion\n")
	t.SetAutoCompleteCallback(autoCompleteCallback)
	for {
		l, err := t.ReadLine()
		switch {
		case err == nil:
			// no error is good, nothing in this switch.
		case errors.Is(err, io.EOF):
			log.Infof("EOF received, exiting.")
			return 0
		case errors.Is(err, term.ErrPasteIndicator):
			log.Infof("Paste indicator received, which is fine.")
		default:
			return log.FErrf("Error reading line: %v", err)
		}
		log.Infof("Read line got: %q", l)
		switch {
		case l == exitCmd:
			return 0
		case l == helpCmd:
			fmt.Fprintf(t.Out, "Available commands: %v\n", commands)
		case strings.HasPrefix(l, afterCmd):
			parts := strings.SplitN(l, " ", 3)
			if len(parts) < 3 {
				fmt.Fprintf(t.Out, "Usage: %s <duration> <text...>\n", afterCmd)
				continue
			}
			dur, err := time.ParseDuration(parts[1])
			if err != nil {
				fmt.Fprintf(t.Out, "Invalid duration %q: %v\n", parts[1], err)
				continue
			}
			log.Infof("Will show %q after %v", parts[2], dur)
			go func() {
				time.Sleep(dur)
				fmt.Fprintf(t.Out, "%s\n", parts[2])
			}()
		case strings.HasPrefix(l, promptCmd):
			t.SetPrompt(l[len(promptCmd):])
		default:
			fmt.Fprintf(t.Out, "Unknown command %q\n", l)
		}
	}
}
