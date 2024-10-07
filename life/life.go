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

func (c *Conway) SetCurrent(x, y int) {
	c.Cells[c.Current][y*c.Width+x] = 1
}

func (c *Conway) Clear(x, y int) {
	c.Cells[1-c.Current][y*c.Width+x] = 0
}

func (c *Conway) ClearCurrent(x, y int) {
	c.Cells[c.Current][y*c.Width+x] = 0
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
				ap.WriteRune(ansipixels.FullPixel)
			case p1 == 1:
				ap.WriteRune(ansipixels.TopHalfPixel)
			default:
				ap.WriteRune(ansipixels.BottomHalfPixel)
			}
			adjacent = true
		}
	}
}

type GameState string

const (
	Paused  = "Paused"
	Running = "Running"
)

type Game struct {
	ap                     *ansipixels.AnsiPixels
	c                      *Conway
	state                  GameState
	showInfo               bool
	showHelp               bool
	generation             uint64
	lastClickX, lastClickY int
	delta                  int // which 1/2 pixel we're targeting with the mouse.
	lastWasClick           bool
	hasMouse               bool
}

func Main() int {
	fpsFlag := flag.Float64("fps", 60, "Frames per second")
	flagRandomFill := flag.Float64("fill", 0.1, "Random fill factor (0 to 1)")
	flagGlider := flag.Bool("glider", false, "Start with a glider (default is random)")
	noMouseFlag := flag.Bool("nomouse", false, "Disable mouse tracking")
	cli.Main()
	game := &Game{hasMouse: !*noMouseFlag}
	ap := ansipixels.NewAnsiPixels(*fpsFlag)
	err := ap.Open()
	if err != nil {
		return log.FErrf("Error opening AnsiPixels: %v", err)
	}
	game.ap = ap
	defer game.End()
	ap.HideCursor()
	if game.hasMouse {
		ap.MouseClickOn() // start with just clicks, we turn on drag after a click.
	}
	fillFactor := float32(*flagRandomFill)
	ap.OnResize = func() error {
		game.c = NewConway(ap.W, 2*ap.H) // half pixels vertically.
		if *flagGlider {
			game.c.Glider(ap.W/3, 2*ap.H/3) // first third of the screen
		} else {
			// Random
			game.c.Randomize(fillFactor)
		}
		game.c.Current = 1 - game.c.Current
		game.generation = 1
		game.showInfo = true
		game.state = Paused
		game.showHelp = true
		game.delta = 0
		game.DrawOne()
		return nil
	}
	_ = ap.OnResize()
	for {
		switch game.state {
		case Running:
			_, err := ap.ReadOrResizeOrSignalOnce()
			if err != nil {
				return log.FErrf("Error reading: %v", err)
			}
		case Paused:
			err := ap.ReadOrResizeOrSignal()
			if err != nil {
				return log.FErrf("Error reading: %v", err)
			}
		}
		if ap.Mouse {
			game.HandleMouse()
			continue
		}
		if len(ap.Data) == 0 {
			game.Next()
			continue
		}
		switch ap.Data[0] {
		case 'q', 'Q', 3:
			return 0
		case 'i', 'I':
			game.showInfo = !game.showInfo
		case '?', 'h', 'H':
			game.showHelp = true
			game.state = Paused
		case ' ':
			game.state = Paused
		default:
			game.state = Running
		}
		game.Next()
	}
}

func (g *Game) DrawOne() {
	g.ap.StartSyncMode()
	g.ap.ClearScreen()
	if g.showInfo {
		g.ap.WriteRight(g.ap.H-1, "%s FPS %.0f Generation: %d ", g.state, g.ap.FPS, g.generation)
	}
	Draw(g.ap, g.c)
	if g.showHelp {
		helpText := "Space to pause, q to quit, i for info, other key to run\n"
		if g.hasMouse {
			helpText += "Left click or hold to set, right click to clear\nClick in same spot for other half pixel"
		} else {
			helpText += "Mouse support disabled, run without -nomouse for mouse support"
		}
		g.ap.WriteBoxed(g.ap.H/2+2, "%s", helpText)
		g.showHelp = false
	}
	g.ap.EndSyncMode()
}

func (g *Game) Next() {
	g.c.Next()
	g.generation++
	g.DrawOne()
}

func (g *Game) End() {
	g.ap.MouseTrackingOff()
	g.ap.MouseClickOff()
	g.ap.ShowCursor()
	g.ap.MoveCursor(0, g.ap.H-2)
	g.ap.Restore()
}

func (g *Game) HandleMouse() {
	// maybe we need a different delta for left and right clicks
	// but for now it's pretty good to cycle a pixels' 2 halves. (2 left clicks, 2 right clicks)
	delta := 0
	sameSpot := g.ap.Mx == g.lastClickX && g.ap.My == g.lastClickY
	prevWasClick := g.lastWasClick
	leftDrag := g.ap.LeftDrag()
	ld := prevWasClick && leftDrag && !sameSpot
	if sameSpot {
		delta = 1 - g.delta
	} else if ld {
		delta = g.delta
	}
	g.lastWasClick = false
	switch {
	case g.ap.LeftClick(), ld:
		log.LogVf("Mouse left (%06b) click (drag %t) at %d, %d", g.ap.Mbuttons, ld, g.ap.Mx, g.ap.My)
		g.c.SetCurrent(g.ap.Mx-1, (g.ap.My-1)*2+delta)
		g.lastWasClick = true
		g.ap.MouseTrackingOn() // needed for drag, other ap.MouseClickOn() is enough.
		if ld {
			g.DrawOne()
			return
		}
	case g.ap.RightClick():
		log.LogVf("Mouse right (%06b) click (drag %t) at %d, %d", g.ap.Mbuttons, leftDrag, g.ap.Mx, g.ap.My)
		g.c.ClearCurrent(g.ap.Mx-1, (g.ap.My-1)*2+delta)
		g.lastWasClick = true
	default:
		log.LogVf("Mouse %06b at %d, %d last was click %t same spot %t left drag %t",
			g.ap.Mbuttons, g.ap.Mx, g.ap.My,
			prevWasClick, sameSpot, leftDrag)
		if prevWasClick {
			g.ap.MouseClickOn() // turns off drag and back to just clicks.
		}
		return
	}
	if sameSpot {
		g.delta = 1 - g.delta
	} else {
		g.delta = 0
	}
	g.lastClickX = g.ap.Mx
	g.lastClickY = g.ap.My
	g.DrawOne()
}
