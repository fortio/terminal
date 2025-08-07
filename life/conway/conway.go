package conway

import (
	"math/rand/v2"

	"fortio.org/log"
	"fortio.org/terminal/ansipixels"
)

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
			case p2 == 1:
				ap.WriteRune(ansipixels.BottomHalfPixel)
			default:
				ap.WriteRune(ansipixels.TopHalfPixel)
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
	AP                     *ansipixels.AnsiPixels
	C                      *Conway
	State                  GameState
	ShowInfo               bool
	ShowHelp               bool
	Generation             uint64
	lastClickX, lastClickY int
	Delta                  int // which 1/2 pixel we're targeting with the mouse.
	lastWasClick           bool
	lastWasAlt             bool
	HasMouse               bool
	Extra                  func()
}

// Call once after randomize or reset/restart.
func (g *Game) Start() {
	g.C.Current = 1 - g.C.Current
	g.Generation = 1
	g.Delta = 0
	g.DrawOne()
}

func (g *Game) DrawOne() {
	g.AP.StartSyncMode()
	g.AP.ClearScreen()
	if g.ShowInfo {
		if g.AP.FPS > 0 {
			g.AP.WriteRight(g.AP.H-1, "%s FPS %.0f Generation: %d ", g.State, g.AP.FPS, g.Generation)
		} else {
			g.AP.WriteRight(g.AP.H-1, "Generation: %d ", g.Generation)
		}
	}
	Draw(g.AP, g.C)
	if g.ShowHelp {
		helpText := "Space to pause, q to quit, i for info, other key to run\n"
		if g.HasMouse {
			helpText += "Left click or hold to set, right click to clear\nHold a modifier or click in same spot for other half pixel"
		} else {
			helpText += "Mouse support disabled, run without -nomouse for mouse support"
		}
		g.AP.WriteBoxed(g.AP.H/2+2, "%s", helpText)
		g.ShowHelp = false
	}
	if g.Extra != nil {
		g.Extra()
	}
	g.AP.EndSyncMode()
}

func (g *Game) Next() {
	g.C.Next()
	g.Generation++
	g.DrawOne()
}

func (g *Game) End() {
	g.AP.MouseTrackingOff()
	g.AP.MouseClickOff()
	g.AP.ShowCursor()
	g.AP.MoveCursor(0, g.AP.H-2)
	g.AP.Restore()
}

func (g *Game) HandleMouse() {
	// maybe we need a different delta for left and right clicks
	// but for now it's pretty good to cycle a pixels' 2 halves. (2 left clicks, 2 right clicks)
	delta := 0
	sameSpot := g.AP.Mx == g.lastClickX && g.AP.My == g.lastClickY
	prevWasClick := g.lastWasClick
	leftDrag := g.AP.LeftDrag()
	ld := prevWasClick && leftDrag && !sameSpot
	if sameSpot {
		delta = 1 - g.Delta
	} else if ld {
		delta = g.Delta
	}
	g.lastWasClick = false
	modifier := g.AP.AnyModifier()
	if modifier {
		delta = 1
		g.lastWasAlt = true
	} else {
		if g.lastWasAlt {
			delta = 0
		}
		g.lastWasAlt = false
	}
	switch {
	case g.AP.LeftClick(), ld:
		log.LogVf("Mouse left (%06b) alt %t click (drag %t) at %d, %d", g.AP.Mbuttons, modifier, ld, g.AP.Mx, g.AP.My)
		g.C.SetCurrent(g.AP.Mx-1, (g.AP.My-1)*2+delta)
		g.lastWasClick = true
		g.AP.MouseTrackingOn() // needed for drag, other ap.MouseClickOn() is enough.
		if ld {
			g.DrawOne()
			return
		}
	case g.AP.RightClick():
		log.LogVf("Mouse right (%06b) alt %t click (drag %t) at %d, %d", g.AP.Mbuttons, modifier, leftDrag, g.AP.Mx, g.AP.My)
		g.C.ClearCurrent(g.AP.Mx-1, (g.AP.My-1)*2+delta)
		g.lastWasClick = true
	default:
		log.LogVf("Mouse %06b at %d, %d last was click %t same spot %t left drag %t",
			g.AP.Mbuttons, g.AP.Mx, g.AP.My,
			prevWasClick, sameSpot, leftDrag)
		if prevWasClick {
			g.AP.MouseClickOn() // turns off drag and back to just clicks.
		}
		return
	}
	if sameSpot {
		g.Delta = 1 - g.Delta
	} else {
		g.Delta = 0
	}
	g.lastClickX = g.AP.Mx
	g.lastClickY = g.AP.My
	g.DrawOne()
}
