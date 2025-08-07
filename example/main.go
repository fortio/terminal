/*
 * A more interesting/real example is https://github.com/grol-io/grol
 * but this demonstrates most of the features of the terminal package.
 */
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal"
)

func main() {
	os.Exit(Main())
}

const (
	promptCmd = "prompt "
	afterCmd  = "after "
	sleepCmd  = "sleep "
	cancelCmd = "cancel " // simulate an external interrupt
	runCmd    = "run "    // run a command after suspending the terminal
	exitCmd   = "exit"
	helpCmd   = "help"
	testMLCmd = "multiline"
	panicCmd  = "panic " // test panic handling
)

var commands = []string{promptCmd, afterCmd, sleepCmd, cancelCmd, exitCmd, helpCmd, testMLCmd, panicCmd}

// func(line string, pos int, key rune) (newLine string, newPos int, ok bool)

func autoCompleteCallback(t *terminal.Terminal, line string, pos int, key rune) (newLine string, newPos int, ok bool) {
	log.LogVf("AutoCompleteCallback: %q %d %q", line, pos, key)
	if key != '\t' {
		return // only tab for now
	}
	if len(line) == 0 {
		fmt.Fprintf(t.Out, "Available commands: %v\n", commands)
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

func AddOrReplaceHistory(t *terminal.Terminal, replace bool, l string) {
	// if in default auto mode, we don't manage history
	// we also don't add empty commands. (at start of program)
	if t.AutoHistory() || l == "" {
		return
	}
	log.LogVf("Adding to history %q replace %t", l, replace)
	if replace {
		t.ReplaceLatest(l)
	} else {
		t.AddToHistory(l)
	}
}

func Main() int { //nolint:funlen // long but simple (and some amount of copy pasta because of flow control).
	flagHistory := flag.String("history", ".history", "History `file` to use")
	flagMaxHistory := flag.Int("max-history", 10, "Max number of history lines to keep")
	flagOnlyValid := flag.Bool("only-valid", false, "Demonstrates filtering of history, only adding valid commands to it")
	cli.Main()
	t, err := terminal.Open(context.Background())
	if err != nil {
		return log.FErrf("Error opening terminal: %v", err)
	}
	defer func() {
		t.Close()
		log.Infof("Terminal closed/restored")
	}()
	onlyValid := *flagOnlyValid
	if onlyValid {
		t.SetAutoHistory(false)
	}
	t.SetPrompt("Terminal demo> ")
	t.NewHistory(*flagMaxHistory)
	if err = t.SetHistoryFile(*flagHistory); err != nil {
		// error already logged
		return 1
	}
	fmt.Fprintf(t.Out, "Terminal is open\nis valid %t\nuse exit or ^D or 3 ^C to exit\n", t.IsTerminal())
	fmt.Fprintf(t.Out, "Use 'prompt <new prompt>' to change the prompt\n")
	fmt.Fprintf(t.Out, "Try 'after duration text...' to see text showing in the middle of edits after said duration\n")
	fmt.Fprintf(t.Out, "Try <tab> for auto completion\n")
	t.SetAutoCompleteCallback(autoCompleteCallback)
	previousCommandWasValid := true // won't be used because `line` is empty at start
	isValidCommand := true
	var cmd string
	interrupts := 0
	ctx := t.Context
	cancel := t.Cancel
	var terr terminal.InterruptedError
	for {
		// Replace unless the previous command was valid.
		AddOrReplaceHistory(t, !previousCommandWasValid, cmd)
		cmd, err = t.ReadLine()
		log.LogVf("Read line got: %q %v - paste %t", cmd, err, t.LastWasPaste())
		switch {
		case err == nil:
			// no error is good, nothing in this switch.
		case errors.Is(err, io.EOF):
			log.Infof("EOF received, exiting.")
			return 0
		case errors.As(err, &terr):
			interrupts++
			if interrupts >= 3 {
				log.Infof("Triple %v, exiting.", terr)
				return 0
			}
			log.Infof("Interrupted (%d, %v), resetting, use exit or ^D. to exit.", interrupts, terr)
			ctx, cancel = t.ResetInterrupts(context.Background()) //nolint:fatcontext // this is only upon interrupt.
		default:
			return log.FErrf("Error reading line: %v", err)
		}
		// Save previous command validity to know whether this one should replace it in history or not.
		previousCommandWasValid = isValidCommand
		isValidCommand = false // not valid unless proven otherwise (reaches the end validations etc)
		if cmd == "" {
			// we do get empty reads on interrupt
			log.LogVf("Empty command read")
			continue
		}
		log.Infof("Read line got: %q", cmd)
		switch {
		case cmd == exitCmd:
			log.Infof("Exit command received, exiting.")
			return 0
		case cmd == helpCmd:
			fmt.Fprintf(t.Out, "Available commands: %v\n", commands)
			isValidCommand = true
		case strings.HasPrefix(cmd, sleepCmd):
			dur, _, ok := parseWithDur(t, cmd, 2, sleepCmd+"<duration>")
			if !ok {
				continue
			}
			isValidCommand = true
			log.Infof("Sleeping for %v (^C to interrupt)", dur)
			err = terminal.SleepWithContext(t.Context, dur)
			if err != nil {
				log.Infof("Sleep interrupted: %v", err)
				interrupts = 0
			}
		case strings.HasPrefix(cmd, cancelCmd):
			dur, _, ok := parseWithDur(t, cmd, 2, cancelCmd+"<duration>")
			if !ok {
				continue
			}
			isValidCommand = true
			log.Infof("Will generate cancel() after %v", dur)
			interrupts = 0
			go func(ctx context.Context, cancel context.CancelFunc) {
				time.Sleep(dur)
				if ctx.Err() != nil {
					log.Infof("Already interrupted, not canceling again: %v", ctx.Err())
					return
				}
				log.Infof("Canceling")
				cancel()
			}(ctx, cancel)
		case strings.HasPrefix(cmd, afterCmd):
			dur, rest, ok := parseWithDur(t, cmd, 3, afterCmd+"<duration> <text>")
			if !ok {
				continue
			}
			isValidCommand = true
			log.Infof("Will show %q after %v", rest, dur)
			go func() {
				time.Sleep(dur)
				fmt.Fprintf(t.Out, "%s\n", rest)
			}()
		case strings.HasPrefix(cmd, promptCmd):
			if onlyValid {
				t.AddToHistory(cmd)
			}
			t.SetPrompt(cmd[len(promptCmd):])
			isValidCommand = true
		case strings.HasPrefix(cmd, runCmd):
			if onlyValid {
				t.AddToHistory(cmd)
			}
			// suspend the terminal, run the command, resume the terminal
			t.Suspend()
			args := strings.Fields(cmd[len(runCmd):])
			// First element is the command, rest are arguments
			// Note: new context.Background() is needed to avoid using the terminal's context which we suspended.
			cmd := exec.CommandContext(context.Background(), args[0], args[1:]...) //nolint:gosec // this is a demo
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
			ctx, cancel = t.Resume(context.Background())
			if err != nil {
				log.Errf("Error running command %v: %v", args, err)
			}
			isValidCommand = true
		case strings.HasPrefix(cmd, panicCmd):
			if onlyValid {
				t.AddToHistory(cmd)
			}
			// panic to test recovery
			// this is a demo, not a real program
			panic("test panic: " + cmd[len(panicCmd):])
		default:
			fmt.Fprintf(t.Out, "Unknown command %q\n", cmd)
		}
	}
}

func parseWithDur(t *terminal.Terminal, cmd string, expected int, usage string) (time.Duration, string, bool) {
	parts, ok := splitN(t, cmd, expected, usage)
	if !ok {
		return 0, "", false
	}
	dur, ok := getDuration(t, parts[1])
	if !ok {
		return 0, "", false
	}
	rest := ""
	if expected > 2 {
		rest = parts[2]
	}
	return dur, rest, true
}

func splitN(t *terminal.Terminal, inp string, expected int, usage string) ([]string, bool) {
	parts := strings.SplitN(inp, " ", expected)
	if len(parts) < expected {
		fmt.Fprintf(t.Out, "Usage: %s\n", usage)
		return parts, false
	}
	return parts, true
}

func getDuration(t *terminal.Terminal, s string) (time.Duration, bool) {
	dur, err := time.ParseDuration(s)
	if err != nil {
		fmt.Fprintf(t.Out, "Invalid duration %q: %v\n", s, err)
		return 0, false
	}
	return dur, true
}
