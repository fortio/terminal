package main

import (
	"flag"
	"os"

	"fortio.org/cli"
	"fortio.org/log"
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
	State           []bool
	BallX           int
	BallY           int
}

/*
grol -c 'print((("\u2586" * 6 + " ") * 5 + "\n") * 5)'
▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆
▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆
▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆
▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆
▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆ ▆▆▆▆▆▆
or 2585:
▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅
▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅
▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅
▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅
▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅ ▅▅▅▅▅▅
*/

const (
	OneBrick = "▅▅▅▅▅▅" // 6 \u2585 (3/4 height blocks)
	Empty    = "      " // 6 spaces
	BrickLen = 6
	Ball     = "⬤" // or "◯" or "⚫"
)

func NewBrick(width, height int) *Brick { // height and width in full height blocks (unlike images/life)
	numW := (width - 1) / (BrickLen + 1) // 6 + 1 space but only in between bricks so -1 in numerator despite -2 border.
	spaceNeeded := numW*BrickLen + (numW - 1)
	width -= 2 // border
	padding := (width - spaceNeeded) / 2
	log.Debugf("Width %d Height %d NumW %d Padding %d, SpaceNeeded %d", width, height, numW, padding, spaceNeeded)
	b := &Brick{
		Width:  width,
		Height: height - 2,
		// border on each side plus spaces in between bricks
		NumW:      numW,
		Padding:   padding,
		PaddlePos: width / 2,
		State:     make([]bool, numW*8),
		BallX:     width / 2,
		BallY:     2 * height / 3,
	}
	b.Initial()
	return b
}

func (b *Brick) Has(x, y int) bool {
	return b.State[y*b.NumW+x]
}

func (b *Brick) Next() {
	b.PaddlePos += b.PaddleDirection
	if b.PaddlePos-3 <= 0 {
		b.PaddlePos = 3
		b.PaddleDirection = 0
	}
	if b.PaddlePos+3 >= b.Width {
		b.PaddlePos = b.Width - 3
		b.PaddleDirection = 0
	}
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
	_, _ = ap.Out.WriteString(log.ANSIColors.Reset)
	_ = ap.DrawRoundBox(0, 0, ap.W, ap.H)
	ap.WriteAtStr(1+b.BallX, 1+b.BallY, Ball)
	for y := range 8 {
		switch y {
		case 0, 1:
			ap.WriteAtStr(b.Padding+1, 3+y, log.ANSIColors.BrightRed)
		case 2, 3:
			ap.WriteAtStr(b.Padding+1, 3+y, "\033[38;5;214m") // orange
		case 4, 5:
			ap.WriteAtStr(b.Padding+1, 3+y, log.ANSIColors.Green)
		case 6, 7:
			ap.WriteAtStr(b.Padding+1, 3+y, log.ANSIColors.Yellow)
		}
		for n := range b.NumW {
			if n > 0 {
				_, _ = ap.Out.WriteRune(' ')
			}
			if b.Has(n, y) {
				_, _ = ap.Out.WriteString(OneBrick)
			} else {
				_, _ = ap.Out.WriteString(Empty)
			}
		}
	}
	_, _ = ap.Out.WriteString(log.ANSIColors.Cyan)
	ap.WriteAtStr(1+b.PaddlePos-3, ap.H-4, OneBrick)
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
