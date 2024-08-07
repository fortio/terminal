package main

import (
	"errors"
	"fmt"
	"io"
	"os"

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
	t.SetPrompt("Fortio> ")
	isTerm := t.IsTerminal()
	// t.Out will add the needed \r for each \n when term is in raw mode
	log.SetOutput(&terminal.CRWriter{Out: os.Stderr})
	log.Config.ForceColor = isTerm
	log.SetColorMode()
	fmt.Fprintf(t.Out, "Terminal is open\nis valid %t\n", isTerm)
	l, err := t.ReadLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Infof("EOF received, exiting.")
			return 0
		}
		return log.FErrf("Error reading line: %v", err)
	}
	log.Infof("Read line got: %q", l)
	/*
		if isTerm {
			cli.UntilInterrupted()
		}
	*/
	return 0
}
