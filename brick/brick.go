package main

import (
	"flag"
	"math"
	"math/rand/v2"
	"os"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/safecast"
	"fortio.org/terminal/ansipixels"
)

func main() {
	os.Exit(Main())
}

type Brick struct {
	Width           int
	Height          int
	NumW            int
	Padding         int
	PaddlePos       int
	PaddleDirection int
	PaddleY         int
	Score           int
	Lives           int
	State           []bool
	BallX           float64
	BallY           float64
	BallAngle       float64
	BallHeight      float64
	BallSpeed       float64
	CheckLives      bool
}

/*
grol -c 'print((("\u2586" * 6 + " ") * 5 + "\n") * 3)'
▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆
▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆
▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆
or 2585:
▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅
▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅
▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅
*/

const (
	OneBrick     = "▅▅▅▅▅▅" // 6 \u2585 (3/4 height blocks)
	Empty        = "      " // 6 spaces
	BrickWidth   = 6
	PaddleYDelta = 2
	Paddle       = "▀▀▀▀▀▀▀" // 7 \u2580 (1/2 height top locks).
	PaddleWidth  = 7
	// Ball     = "⚾" // or "◯" or "⚫" doesn't work well/jerky movement - let's use 1/2 blocks instead.
)

func NewBrick(width, height, numLives int, checkLives bool) *Brick { // height and width in full height blocks (unlike images/life)
	if numLives == 0 || !checkLives {
		numLives = -1
	}
	paddleY := height - PaddleYDelta
	numW := (width - 1) / (BrickWidth + 1) // 6 + 1 space but only in between bricks so -1 in numerator despite -2 border.
	spaceNeeded := numW*BrickWidth + (numW - 1)
	width -= 2 // border
	padding := (width - spaceNeeded) / 2
	height -= 2 // border
	log.Debugf("Width %d Height %d NumW %d Padding %d, SpaceNeeded %d", width, height, numW, padding, spaceNeeded)
	b := &Brick{
		Width:  width,
		Height: height,
		// border on each side plus spaces in between bricks
		NumW:       numW,
		Padding:    padding,
		PaddlePos:  width / 2,
		State:      make([]bool, numW*8),
		BallX:      float64(width) / 2.,
		BallHeight: 2. * float64(height),
		BallY:      2 * (8 + 3), // just below the bricks.
		PaddleY:    paddleY,
		BallAngle:  -math.Pi/2 + (rand.Float64()-0.5)*math.Pi/2., //nolint:gosec // not crypto, starting in a cone up.
		BallSpeed:  1,
		Lives:      numLives,
		CheckLives: checkLives,
	}
	b.Initial()
	return b
}

func (b *Brick) Has(x, y int) bool {
	return b.State[y*b.NumW+x]
}

func (b *Brick) Clear(x, y int) {
	b.State[y*b.NumW+x] = false
	switch y {
	case 0, 1:
		b.Score += 7
	case 2, 3:
		b.Score += 5
	case 4, 5:
		b.Score += 3
	case 6, 7:
		b.Score++
	}
}

func (b *Brick) Next() {
	// move paddle
	b.PaddlePos += b.PaddleDirection
	if b.PaddlePos-3 <= 0 {
		b.PaddlePos = 3
		b.PaddleDirection = 0
	}
	if b.PaddlePos+4 >= b.Width {
		b.PaddlePos = b.Width - 4
		b.PaddleDirection = 0
	}
	b.BallX += b.BallSpeed * math.Cos(b.BallAngle)
	b.BallY -= b.BallSpeed * math.Sin(b.BallAngle)

	by := safecast.MustRound[int](b.BallY)
	by2 := by / 2

	switch {
	// bounce on paddle
	case (1+by2 == b.PaddleY) && (by%2 == 0) && math.Abs(b.BallX-float64(b.PaddlePos)) <= 3:
		b.BallAngle = -b.BallAngle
	// bounce on walls
	case b.BallX <= 0 || b.BallX >= float64(b.Width)-1:
		b.BallAngle = math.Pi - b.BallAngle
	case b.BallY < 0 || b.BallY >= b.BallHeight-1:
		b.BallAngle = -b.BallAngle
	default:
		return
	}
	// avoid vertical or horizontal movement
	dx := math.Cos(b.BallAngle)
	if math.Abs(dx) < 0.2 {
		b.BallAngle += (rand.Float64() - 0.5) * math.Pi / 7 //nolint:gosec // not crypto
	}
	dy := math.Sin(b.BallAngle)
	if math.Abs(dy) < 0.2 {
		b.BallAngle += (rand.Float64() - 0.5) * math.Pi / 7 //nolint:gosec // not crypto
	}
	b.BallX += b.BallSpeed * dx
	b.BallY -= b.BallSpeed * dy
}

func (b *Brick) Set(x, y int) {
	b.State[y*b.NumW+x] = true
}

func (b *Brick) Initial() {
	for y := range 8 {
		for x := range b.NumW {
			b.Set(x, y)
		}
	}
}

func Draw(ap *ansipixels.AnsiPixels, b *Brick) {
	ap.WriteString(log.ANSIColors.Reset)
	ap.DrawRoundBox(0, 0, ap.W, ap.H)
	ap.WriteBoxed(0, "Score %d", b.Score)
	switch b.Lives {
	case 0:
		ap.WriteRightBoxed(0, "☠️")
	case -1:
		ap.WriteRightBoxed(0, "∞❤️")
	default:
		ap.WriteRightBoxed(0, "%d❤️", b.Lives)
	}
	for y := range 8 {
		ap.MoveCursor(b.Padding+1, 3+y)
		switch y {
		case 0:
			ap.WriteAtStr(b.Padding+1, 3+y, log.ANSIColors.BrightRed)
		case 2:
			ap.WriteAtStr(b.Padding+1, 3+y, "\033[38;5;214m") // orange
		case 4:
			ap.WriteAtStr(b.Padding+1, 3+y, log.ANSIColors.Green)
		case 6:
			ap.WriteAtStr(b.Padding+1, 3+y, log.ANSIColors.Yellow)
		}
		for n := range b.NumW {
			if n > 0 {
				ap.WriteRune(' ')
			}
			if b.Has(n, y) {
				ap.WriteString(OneBrick)
			} else {
				ap.WriteString(Empty)
			}
		}
	}
	ap.WriteString(log.ANSIColors.Cyan)
	ap.WriteAtStr(1+b.PaddlePos-3, ap.H-PaddleYDelta, Paddle)
	ap.WriteString(log.ANSIColors.Reset)
	bx := safecast.MustRound[int](b.BallX)
	by := safecast.MustRound[int](b.BallY)
	by2 := by / 2
	if by2-1 < 8 {
		// probably should calculate this exactly right instead of this anti oob.
		bxx := max(0, min(b.NumW-1, (bx-b.Padding)/(BrickWidth+1)))
		byy := max(0, min(7, by2-1))
		if b.Has(bxx, byy) {
			b.BallAngle = -b.BallAngle
			b.Clear(bxx, byy)
		}
	}
	ap.MoveCursor(1+bx, 1+by2)
	// TODO Antialias http://members.chello.at/easyfilter/bresenham.html
	if by%2 == 0 {
		ap.WriteRune(ansipixels.TopHalfPixel)
	} else {
		ap.WriteRune(ansipixels.BottomHalfPixel)
	}
}

func Main() int {
	fpsFlag := flag.Float64("fps", 30, "Frames per second")
	numLives := flag.Int("lives", 3, "Number of lives - 0 is infinite")
	noDeath := flag.Bool("nodeath", false, "No death mode")
	cli.Main()
	ap := ansipixels.NewAnsiPixels(*fpsFlag)
	err := ap.Open()
	if err != nil {
		return log.FErrf("Error opening AnsiPixels: %v", err)
	}
	defer ap.Restore()
	ap.HideCursor()
	var generation uint64
	var b *Brick
	inStartScreen := true
	ap.OnResize = func() error {
		ap.ClearScreen()
		ap.StartSyncMode()
		b = NewBrick(ap.W, ap.H, *numLives, !*noDeath) // half pixels vertically.
		Draw(ap, b)
		generation = 0
		if inStartScreen {
			ap.WriteCentered(ap.H/2+1, "Any key to start... 🕹️ controls:")
			ap.WriteCentered(ap.H/2+2, "Left A, Stop: S, Right: D - Quit: ^C or Q")
		}
		ap.EndSyncMode()
		return nil
	}
	_ = ap.OnResize()
	err = ap.ReadOrResizeOrSignal()
	if err != nil {
		return log.FErrf("Error reading: %v", err)
	}
	if handleKeys(ap, b) {
		return 0
	}
	for {
		ap.StartSyncMode()
		ap.ClearScreen()
		Draw(ap, b)
		generation++
		ap.EndSyncMode()
		_, err := ap.ReadOrResizeOrSignalOnce()
		if err != nil {
			return log.FErrf("Error reading: %v", err)
		}
		if handleKeys(ap, b) {
			return 0
		}
		b.Next()
	}
}

// returns true if should exit.
func handleKeys(ap *ansipixels.AnsiPixels, b *Brick) bool {
	if len(ap.Data) == 0 {
		return false
	}
	switch ap.Data[0] {
	case 'a':
		b.PaddleDirection = -1
	case 's':
		b.PaddleDirection = 0
	case 'd':
		b.PaddleDirection = 1
	case 3 /* ^C */, 'Q': // Not lower case q, too near the A,S,D keys
		ap.MoveCursor(0, 0)
		return true
	}
	return false
}
