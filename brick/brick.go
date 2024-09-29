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
	State           []bool
	BallX           float64
	BallY           float64
	BallAngle       float64
	BallHeight      float64
	BallSpeed       float64
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
	BrickLen     = 6
	PaddleYDelta = 4
	Paddle       = "▀▀▀▀▀▀▀" // 7 \u2580 (1/2 height top locks).
	// Ball     = "⚾" // or "◯" or "⚫" doesn't work well/jerky movement - let's use 1/2 blocks instead.
)

func NewBrick(width, height int) *Brick { // height and width in full height blocks (unlike images/life)
	paddleY := height - PaddleYDelta
	numW := (width - 1) / (BrickLen + 1) // 6 + 1 space but only in between bricks so -1 in numerator despite -2 border.
	spaceNeeded := numW*BrickLen + (numW - 1)
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
		BallY:      2. * float64(height) / 3,
		PaddleY:    paddleY,
		BallAngle:  -math.Pi/2 + (rand.Float64()-0.5)*math.Pi/2., //nolint:gosec // not crypto, starting in a cone up.
		BallSpeed:  1,
	}
	b.Initial()
	return b
}

func (b *Brick) Has(x, y int) bool {
	return b.State[y*b.NumW+x]
}

func (b *Brick) Clear(x, y int) {
	b.State[y*b.NumW+x] = false
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
	b.BallX += b.BallSpeed * math.Cos(b.BallAngle)
	b.BallY -= b.BallSpeed * math.Sin(b.BallAngle)
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
		bxx := max(0, min(b.NumW-1, (bx-b.Padding)/(BrickLen+1)))
		byy := max(0, min(7, by2-1))
		if b.Has(bxx, byy) {
			b.BallAngle = -b.BallAngle
		}
		b.Clear(bxx, byy)
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
	fpsFlag := flag.Float64("fps", 60, "Frames per second")
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
	ap.OnResize = func() error {
		ap.Out.Flush()
		b = NewBrick(ap.W, ap.H) // half pixels vertically.
		generation = 0
		return nil
	}
	_ = ap.OnResize()
	showInfo := true
	for {
		ap.StartSyncMode()
		ap.ClearScreen()
		if showInfo {
			ap.WriteRight(ap.H-2, "Left: a, Stop: s, Right: d - FPS %.0f Frame %d ", ap.FPS, generation)
		}
		Draw(ap, b)
		generation++
		ap.EndSyncMode()
		n, err := ap.ReadOrResizeOrSignalOnce()
		if err != nil {
			return log.FErrf("Error reading: %v", err)
		}
		if n > 0 {
			switch ap.Data[0] {
			case 'a':
				b.PaddleDirection = -1
			case 's':
				b.PaddleDirection = 0
			case 'd':
				b.PaddleDirection = 1
			case 3 /* ^C */, 'Q': // Not lower case q, two near the A,S,D keys
				ap.MoveCursor(0, 0)
				return 0
			case 'i', 'I':
				showInfo = !showInfo
			}
		}
		b.Next()
	}
}
