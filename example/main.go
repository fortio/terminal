package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal"
)

func main() {
	os.Exit(Main())
}

func Main() int {
	cli.Main()
	t, err := terminal.Open()
	if err != nil {
		return log.FErrf("Error opening terminal: %v", err)
	}
	defer t.Close()
	t.SetPrompt("Terminal demo> ")
	isTerm := t.IsTerminal()
	// t.Out will add the needed \r for each \n when term is in raw mode
	log.SetOutput(&terminal.CRWriter{Out: os.Stderr})
	log.Config.ForceColor = isTerm
	log.SetColorMode()
	fmt.Fprintf(t.Out, "Terminal is open\nis valid %t\nuse exit or ^D or ^C to exit\n", isTerm)
	fmt.Fprintf(t.Out, "Use 'prompt <new prompt>' to change the prompt\n")
	fmt.Fprintf(t.Out, "Try 'after duration text...' to see text showing in the middle of edits after said duration\n")
	promptCmd := "prompt "
	afterCmd := "after"
	for {
		l, err := t.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Infof("EOF received, exiting.")
				return 0
			}
			return log.FErrf("Error reading line: %v", err)
		}
		log.Infof("Read line got: %q", l)
		switch {
		case l == "exit":
			return 0
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
	/*
		if isTerm {
			cli.UntilInterrupted()
		}
	*/
	return 0
}
