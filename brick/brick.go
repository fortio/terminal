package main

import (
	"encoding/json"
	"flag"
	"math"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/safecast"
	"fortio.org/terminal/ansipixels"
)

func main() {
	os.Exit(Main())
}

type MoveRecord struct {
	Frame     uint64
	Direction int8
}

type Brick struct {
	Width           int
	Height          int
	NumW            int
	Padding         int
	PaddlePos       int
	PaddleY         int
	Score           int
	Lives           int
	State           []bool
	BallX           float64
	BallY           float64
	BallAngle       float64
	BallHeight      float64
	BallSpeed       float64
	Seed            uint64
	Frames          uint64
	PaddleDirection int8
	CheckLives      bool
	ShowInfo        bool
	Replay          bool
	MoveRecords     []MoveRecord
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
	PaddleYDelta = 3
	Paddle       = "▀▀▀▀▀▀▀" // 7 \u2580 (1/2 height top locks).
	PaddleWidth  = 7
	// Ball     = "⚾" // or "◯" or "⚫" doesn't work well/jerky movement - let's use 1/2 blocks instead.
	PaddleSpinFactor = 0.7
)

// height and width in full height blocks (unlike images/life) for most but the ball.
func NewBrick(width, height, numLives int, checkLives bool, seed uint64) *Brick {
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
	rnd := rand.New(rand.NewPCG(0, seed)) //nolint:gosec // not crypto, starting in a cone up.
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
		BallAngle:  -math.Pi/2 + (rnd.Float64()-0.5)*math.Pi/2.,
		BallSpeed:  1.1,
		Lives:      numLives,
		CheckLives: checkLives,
		Seed:       seed,
		Frames:     1,
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
	if b.Replay && (len(b.MoveRecords) > 0) && (b.MoveRecords[0].Frame == b.Frames) {
		b.PaddleDirection = b.MoveRecords[0].Direction
		b.MoveRecords = b.MoveRecords[1:]
	}
	b.Frames++
	// move paddle
	b.PaddlePos += int(b.PaddleDirection)
	halfWidth := PaddleWidth / 2 // 7 so 3.5
	if b.PaddlePos-halfWidth <= 0 {
		b.PaddlePos = halfWidth
		b.PaddleDirection = 0
	}
	if b.PaddlePos+halfWidth >= b.Width-1 {
		b.PaddlePos = b.Width - 1 - halfWidth
		b.PaddleDirection = 0
	}
	vx := b.BallSpeed * math.Cos(b.BallAngle)
	vy := b.BallSpeed * math.Sin(b.BallAngle)
	b.BallX += vx
	b.BallY -= vy

	by := safecast.MustRound[int](b.BallY)
	by2 := by / 2

	paddleXdistance := math.Abs(b.BallX - float64(b.PaddlePos))
	switch {
	// bounce on paddle
	case (1+by2 == b.PaddleY) && (by%2 == 0) && paddleXdistance <= float64(PaddleWidth)/2.:
		vx += PaddleSpinFactor * float64(b.PaddleDirection)
		vy = -vy
		b.BallAngle = math.Atan2(vy, vx)
		b.BallSpeed = min(1.5, max(0.5, math.Sqrt(vx*vx+vy*vy)))
		return
	// bounce on walls
	case b.BallX <= 0 || b.BallX >= float64(b.Width)-1:
		b.BallAngle = math.Pi - b.BallAngle
	case b.BallY < 0 || b.BallY >= b.BallHeight:
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

func (b *Brick) recordMove(direction int8) {
	if b.Replay {
		return
	}
	b.MoveRecords = append(b.MoveRecords, MoveRecord{Frame: b.Frames, Direction: direction})
	b.PaddleDirection = direction
}

func (b *Brick) Left() {
	b.recordMove(-1)
}

func (b *Brick) Center() {
	b.recordMove(0)
}

func (b *Brick) Right() {
	b.recordMove(1)
}

func Draw(ap *ansipixels.AnsiPixels, b *Brick) {
	ap.WriteString(log.ANSIColors.Reset)
	ap.DrawRoundBox(0, 0, ap.W, ap.H)
	ap.WriteBoxed(0, "Score %d", b.Score)
	switch b.Lives {
	case 0:
		ap.WriteRightBoxed(0, "☠️")
	case -1:
		if b.Replay {
			ap.WriteRightBoxed(0, "🔂")
		} else {
			ap.WriteRightBoxed(0, "∞❤️")
		}
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
	// https://www.youtube.com/watch?v=f3Rs20k-hcI
	if by%2 == 0 {
		ap.WriteRune(ansipixels.TopHalfPixel)
	} else {
		ap.WriteRune(ansipixels.BottomHalfPixel)
	}
}

type GameSave struct {
	Version string
	*Brick
}

func (b *Brick) SaveGame() int {
	now := time.Now()
	fname := "brick_" + ansipixels.FormatDate(&now) + ".json"
	f, err := os.Create(fname)
	if err != nil {
		return log.FErrf("Error creating %s: %v", fname, err)
	}
	buf, err := json.MarshalIndent(GameSave{Version: "brick " + cli.LongVersion, Brick: b}, "", "  ")
	if err != nil {
		return log.FErrf("Error marshaling %s: %v", fname, err)
	}
	_, err = f.Write(buf)
	if err != nil {
		return log.FErrf("Error writing %s: %v", fname, err)
	}
	err = f.Close()
	if err != nil {
		return log.FErrf("Error closing %s: %v", fname, err)
	}
	log.Infof("Saved to %s\r", fname)
	return 0
}

func Main() int {
	fpsFlag := flag.Float64("fps", 30, "Frames per second")
	numLives := flag.Int("lives", 3, "Number of lives - 0 is infinite")
	noDeath := flag.Bool("nodeath", false, "No death mode")
	noSave := flag.Bool("nosave", false, "Don't save the game as JSON (default is to save)")
	seed := flag.Uint64("seed", 0, "Random number generator `seed`, default (0) is time based")
	replay := flag.String("replay", "", "Replay a `game` from a JSON file")
	cli.Main()
	ap := ansipixels.NewAnsiPixels(*fpsFlag)
	err := ap.Open()
	if err != nil {
		return log.FErrf("Error opening AnsiPixels: %v", err)
	}
	defer ap.Restore()
	ap.HideCursor()
	ap.Margin = 1
	if *replay != "" {
		return ReplayGame(ap, *replay)
	}
	seedV := *seed
	if seedV == 0 {
		seedV = safecast.MustConvert[uint64](time.Now().UnixNano() % (1<<16 - 1))
	}
	var b *Brick
	restarted := false
	ap.OnResize = func() error {
		ap.ClearScreen()
		ap.StartSyncMode()
		prevInfo := false
		if b != nil {
			prevInfo = b.ShowInfo
		}
		b = NewBrick(ap.W, ap.H, *numLives, !*noDeath, seedV) // half pixels vertically.
		b.ShowInfo = prevInfo
		Draw(ap, b)
		ap.WriteCentered(ap.H/2+1, "Any key to start... 🕹️ controls:")
		ap.WriteCentered(ap.H/2+2, "Left A, Stop: S, Right: D - Quit: ^C or Q")
		showInfo(ap, b)
		ap.EndSyncMode()
		restarted = true
		return nil
	}
	_ = ap.OnResize()
	err = ap.ReadOrResizeOrSignal()
	if err != nil {
		return log.FErrf("Error reading: %v", err)
	}
	if handleKeys(ap, b, true /* no pauses just at the start */) {
		return 0
	}
	for {
		restarted = false
		ap.StartSyncMode()
		ap.ClearScreen()
		Draw(ap, b)
		showInfo(ap, b)
		ap.EndSyncMode()
		_, err := ap.ReadOrResizeOrSignalOnce()
		if err != nil {
			return log.FErrf("Error reading: %v", err)
		}
		if restarted {
			_ = ap.ReadOrResizeOrSignal()
		}
		if handleKeys(ap, b, restarted /* handle pauses */) {
			if !*noSave {
				return b.SaveGame()
			}
			return 0
		}
		b.Next()
	}
}

func showInfo(ap *ansipixels.AnsiPixels, b *Brick) {
	if !b.ShowInfo {
		return
	}
	ap.WriteRight(ap.H-2, "Ball speed %.2f angle %.1f Target FPS %.0f Frame %d Seed %d",
		b.BallSpeed, b.BallAngle*180/math.Pi, ap.FPS, b.Frames, b.Seed)
}

// returns true if should exit.
func handleKeys(ap *ansipixels.AnsiPixels, b *Brick, noPause bool) bool {
	if len(ap.Data) == 0 {
		return false
	}
	switch ap.Data[0] {
	case 'a':
		b.Left()
	case 's':
		b.Center()
	case 'd':
		b.Right()
	case 'i':
		b.ShowInfo = !b.ShowInfo
	case 3 /* ^C */, 'Q': // Not lower case q, too near the A,S,D keys
		b.ShowInfo = true
		showInfo(ap, b)
		ap.MoveCursor(0, 1)
		ap.Out.Flush()
		return true
	case ' ':
		if noPause {
			return false
		}
		n := 0
		for {
			msg := "⏱️ Paused, any key to resume... ⏱️"
			mlen := ap.ScreenWidth(msg)
			erase := strings.Repeat(" ", mlen)
			x := (ap.W - mlen) / 2
			y := ap.H/2 - 1
			switch n % 20 {
			case 0:
				ap.MoveCursor(x, y)
				ap.WriteString(msg)
			case 10:
				ap.MoveCursor(x, y)
				ap.WriteString(erase)
			}
			ap.EndSyncMode()
			r, _ := ap.ReadOrResizeOrSignalOnce()
			if r != 0 {
				return handleKeys(ap, b, true) // no pause loop.
			}
			n++
		}
	}
	return false
}
