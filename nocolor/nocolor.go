// nocolor is a simple utility to filter out all Ansi
// (color, cursor movement, etc...) sequences from stdin to stdout.
package main

import (
	"errors"
	"flag"
	"io"
	"os"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal/ansipixels"
)

func main() {
	os.Exit(Main())
}

// NoColorSetup sets up the logger and cli to not use color despite being on a console.
func NoColorSetup() {
	cli.ServerMode = true                 // trick to avoid color mode.
	log.LoggerStaticFlagSetup("loglevel") // have to do that which is usually done in the cli mode of cli.Main().
	log.Config.ConsoleColor = false
	log.Config.JSON = false
	log.SetColorMode()
}

func Main() int {
	NoColorSetup()
	sizeFlag := flag.Int("s", 1024, "Buffer size, in bytes, to use for reading/writing")
	cli.ArgsHelp = "\nReads from stdin, writes to stdout, filters out all Ansi code (color, cursor movement, etc...) sequences\n"
	cli.Main()
	bufSize := *sizeFlag
	// We don't want so small that it's enough to re-assemble escape sequences (and gets inifinite loop with left over).
	if bufSize < 8 {
		return log.FErrf("Buffer size too small: %d", bufSize)
	}
	return Filter(bufSize, os.Stdin, os.Stdout)
}

func Filter(bufSize int, in io.Reader, out io.Writer) int {
	buf := make([]byte, bufSize)
	var totalR int64
	var totalW int64
	var numFiltered int64
	var numReads int
	leftOverIndex := 0
	for {
		rn, rerr := in.Read(buf[leftOverIndex:])
		if errors.Is(rerr, io.EOF) { // rn guaranteed to be 0 in this case.
			break
		}
		numReads++
		totalR += int64(rn)
		// Write/salvage whatever was read even if there is a read error.
		l := leftOverIndex + rn
		filtered, endIdx := ansipixels.AnsiClean(buf[:l])
		log.Debugf("Buf %q -> %q (left %d)", buf[:l], filtered, l-endIdx)
		wn, werr := out.Write(filtered) // write before we might overwrite the buffer, filtered could be a slice of the original.
		if endIdx == l {
			leftOverIndex = 0
		} else {
			leftOverIndex = l - endIdx
			copy(buf, buf[endIdx:l])
		}
		numFiltered += int64(rn - len(filtered))
		totalW += int64(wn)
		if werr != nil {
			return log.FErrf("Error writing: %v", werr)
		}
		if rerr != nil {
			return log.FErrf("Error reading: %v", rerr)
		}
	}
	log.LogVf("Filtered %d bytes (Total bytes read: %d, written: %d in %d read/write)",
		numFiltered, totalR, totalW, numReads)
	return 0
}
