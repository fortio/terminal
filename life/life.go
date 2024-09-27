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

// Handles negative and out of bounds coordinates.
func (c *Conway) SafeSet(x, y int) {
	c.Set((x+c.Width)%c.Width, (y+c.Height)%c.Height)
}

func (c *Conway) Glider(x, y int) {
	// Glider
	c.SafeSet(x, y-1)   // Middle cell of the top row
	c.SafeSet(x+1, y)   // Right cell of the middle row
	c.SafeSet(x-1, y+1) // Left cell of the bottom row
	c.SafeSet(x, y+1)   // Middle cell of the bottom row
	c.SafeSet(x+1, y+1) // Right cell of the bottom row
}

func Draw(ap *ansipixels.AnsiPixels, c *Conway) {
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
		c = NewConway(ap.W, 2*ap.H) // half pixels vertically.
		if *flagGlider {
			c.Glider(ap.W/3, 2*ap.H/3) // first third of the screen
		} else {
			// Random
			c.Randomize(fillFactor)
		}
		c.Current = 1 - c.Current
		generation = 0
		return nil
	}
	_ = ap.OnResize()
	showInfo := true
	for {
		ap.StartSyncMode()
		ap.ClearScreen()
		if showInfo {
			ap.WriteRight(ap.H-1, "FPS %.0f Generation: %d ", ap.FPS, generation)
		}
		Draw(ap, c)
		generation++
		ap.EndSyncMode()
		n, err := ap.ReadOrResizeOrSignalOnce()
		if err != nil {
			return log.FErrf("Error reading: %v", err)
		}
		if n > 0 {
			switch ap.Data[0] {
			case 'q', 'Q', 3:
				ap.MoveCursor(0, 0)
				return 0
			case 'i', 'I':
				showInfo = !showInfo
			}
		}
		c.Next()
	}
}
