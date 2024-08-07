package main

import (
	"fmt"
	"os"

	"fortio.org/cli"
	"fortio.org/log"
	"grol.io/terminal"
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
	fmt.Printf("Terminal is open - is valid %t\n", t.IsTerminal())
	return 0
}
