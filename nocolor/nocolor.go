// nocolor is a simple utility to filter out all Ansi
// (color, cursor movement, etc...) sequences from stdin to stdout.
package main

import (
	"errors"
	"io"
	"os"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal/ansipixels"
)

func main() {
	os.Exit(Main())
}

func Main() int {
	cli.ServerMode = true // trick to avoid color mode.
	log.Config.ConsoleColor = false
	log.Config.JSON = false
	log.SetColorMode()
	cli.ArgsHelp = "\nReads from stdin, writes to stdout, filters out all Ansi code (color, cursor movement, etc...) sequences\n"
	cli.Main()
	var buf [1024]byte
	var totalR int64
	var totalW int64
	var numFiltered int64
	for {
		rn, rerr := os.Stdin.Read(buf[:])
		if rn > 0 {
			totalR += int64(rn)
			filtered := ansipixels.AnsiClean(buf[:rn])
			numFiltered += int64(rn - len(filtered))
			wn, werr := os.Stdout.Write(filtered)
			totalW += int64(wn)
			if werr != nil {
				return log.FErrf("Error writing: %v", werr)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
	}
	log.LogVf("Filtered %d bytes (Total bytes read: %d, written: %d)", numFiltered, totalR, totalW)
	return 0
}
