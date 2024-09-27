package main

import (
	"flag"
	"math/rand/v2"
	"os"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal/ansipixels"
)

func main() {
	os.Exit(Main())
}

type Conway struct {
	Width   int
	Height  int
	Cells   [2][]Cell
	Current int
}

type Cell uint8 // same as bool but can be counted directly.

func NewConway(width, height int) *Conway {
	return &Conway{
		Width:  width,
		Height: height,
		Cells:  [2][]Cell{make([]Cell, width*height), make([]Cell, width*height)},
	}
}

func (c *Conway) At(x, y int) Cell {
	log.Debugf("Reading At %d, %d (%d)", x, y, c.Current)
	x = (x + c.Width) % c.Width
	y = (y + c.Height) % c.Height
	return c.Cells[c.Current][y*c.Width+x]
}

func (c *Conway) Set(x, y int) {
	log.Debugf("Setting %d, %d (%d on %d)", x, y, y*c.Width+x, 1-c.Current)
	c.Cells[1-c.Current][y*c.Width+x] = 1
}

func (c *Conway) Clear(x, y int) {
	c.Cells[1-c.Current][y*c.Width+x] = 0
}

func (c *Conway) Copy(x, y int) {
	idx := y*c.Width + x
	c.Cells[1-c.Current][idx] = c.Cells[c.Current][idx]
}

func (c *Conway) Count(x, y int) Cell {
	return c.At(x-1, y-1) + c.At(x, y-1) + c.At(x+1, y-1) +
		c.At(x-1, y) + c.At(x+1, y) +
		c.At(x-1, y+1) + c.At(x, y+1) + c.At(x+1, y+1)
}

func (c *Conway) Next() {
	for y := range c.Height {
		for x := range c.Width {
			count := c.Count(x, y)
			switch {
			case count < 2 || count > 3:
				c.Clear(x, y)
			case count == 3:
				c.Set(x, y)
			default:
				c.Copy(x, y)
			}
		}
	}
	c.Current = 1 - c.Current
}

func (c *Conway) Randomize(fillFactor float32) {
	for y := range c.Height {
		for x := range c.Width {
			if rand.Float32() < fillFactor { //nolint:gosec // this is game of life, not crypto.
				c.Set(x, y)
			}
		}
	}
}

func Draw(ap *ansipixels.AnsiPixels, c *Conway) {
	ap.ClearScreen()
	for y := 0; y < c.Height; y += 2 {
		ap.MoveCursor(0, y/2)
		adjacent := true // we just moved.
		for x := range c.Width {
			p1 := c.At(x, y)
			p2 := c.At(x, y+1)
			if p1+p2 == 0 {
				adjacent = false
				continue
			}
			if !adjacent {
				ap.MoveCursor(x, y/2)
			}
			switch {
			case p1 == p2:
				_, _ = ap.Out.WriteRune(ansipixels.FullPixel)
			case p1 == 1:
				_, _ = ap.Out.WriteRune(ansipixels.TopHalfPixel)
			default:
				_, _ = ap.Out.WriteRune(ansipixels.BottomHalfPixel)
			}
			adjacent = true
		}
	}
}

func Main() int {
	fpsFlag := flag.Float64("fps", 60, "Frames per second")
	flagRandomFill := flag.Float64("fill", 0.1, "Random fill factor (0 to 1)")
	flagGlider := flag.Bool("glider", false, "Start with a glider (default is random)")
	cli.Main()
	ap := ansipixels.NewAnsiPixels(*fpsFlag)
	err := ap.Open()
	if err != nil {
		return log.FErrf("Error opening AnsiPixels: %v", err)
	}
	defer ap.Restore()
	ap.HideCursor()
	var generation uint64
	var c *Conway
	fillFactor := float32(*flagRandomFill)
	ap.OnResize = func() error {
		c = NewConway(ap.W, 2*ap.H)
		centerX := ap.W / 2
		centerY := ap.H
		if *flagGlider {
			// Glider
			c.Set(centerX, centerY-1)   // Middle cell of the top row
			c.Set(centerX+1, centerY)   // Right cell of the middle row
			c.Set(centerX-1, centerY+1) // Left cell of the bottom row
			c.Set(centerX, centerY+1)   // Middle cell of the bottom row
			c.Set(centerX+1, centerY+1) // Right cell of the bottom row
		} else {
			// Random
			c.Randomize(fillFactor)
		}
		c.Current = 1 - c.Current
		generation = 0
		return nil
	}
	_ = ap.OnResize()
	for {
		log.Debugf("Initial Cells: %d, %d, %d", c.At(ap.W/2-1, ap.H), c.At(ap.W/2, ap.H), c.At(ap.W/2+1, ap.H))
		ap.StartSyncMode()
		Draw(ap, c)
		generation++
		ap.WriteRight(ap.H-1, "FPS %.0f Generation: %d ", ap.FPS, generation)
		ap.EndSyncMode()
		n, err := ap.ReadOrResizeOrSignalOnce()
		if err != nil {
			return log.FErrf("Error reading: %v", err)
		}
		if n > 0 && (ap.Data[0] == 'q' || ap.Data[0] == 'Q' || ap.Data[0] == 3) {
			return 0
		}
		c.Next()
	}
}
