package cli

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
	"fortio.org/terminal/ansipixels/tcolor"
)

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
	NumBricks       int // number of bricks left - 0 == win
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
	Auto            bool
	JustBounced     bool
	MoveRecords     []MoveRecord
	Paused          bool // true if paused, false if running
	rnd             *rand.Rand
	waitForInput    bool // true if waiting for user input (after resize or death)
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

	// Ball     = "⚾" // or "◯" or "⚫" doesn't work well/jerky movement, ⚪ is even worse as double width. so we use 1/2 blocks instead.

	PaddleSpinFactor = 0.7
)

// NewBrick creates a new game with height and width in full height blocks (unlike images/life) for most but the ball.
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
		State:      make([]bool, numW*8),
		BallHeight: 2. * float64(height),
		PaddleY:    paddleY,
		Lives:      numLives,
		CheckLives: checkLives,
		Seed:       seed,
		Frames:     1,
		rnd:        rnd,
	}
	b.ResetBall()
	b.Initial()
	return b
}

func (b *Brick) ResetBall() {
	b.BallX = float64(b.Width) / 2.
	b.BallY = 2 * (8 + 3) // just below the bricks.
	b.BallAngle = 2*math.Pi - math.Pi/2 + (b.rnd.Float64()-0.5)*math.Pi/2.
	b.BallSpeed = .98
	b.PaddlePos = b.Width / 2
	b.PaddleDirection = 0
	b.Paused = false
}

func (b *Brick) Has(x, y int) bool {
	return b.State[y*b.NumW+x]
}

func (b *Brick) Clear(x, y int) {
	b.State[y*b.NumW+x] = false
	b.NumBricks--
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

func (b *Brick) Death() {
	if b.Lives == -1 {
		b.ResetBall()
		return
	}
	b.Lives--
	if b.Lives > 0 {
		b.ResetBall()
	}
}

// Next updates the game state and returns true if there has been a death.
func (b *Brick) Next() bool {
	if b.Replay && (len(b.MoveRecords) > 0) && (b.MoveRecords[0].Frame == b.Frames) {
		b.PaddleDirection = b.MoveRecords[0].Direction
		b.MoveRecords = b.MoveRecords[1:]
	}
	// move ball (before adjustments)
	vx := b.BallSpeed * math.Cos(b.BallAngle)
	vy := b.BallSpeed * math.Sin(b.BallAngle)
	b.BallX += vx
	b.BallY -= vy
	// auto
	if b.Auto {
		b.AutoPlay(vx, vy)
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
	// bounce ball
	by := safecast.MustRound[int](b.BallY)
	by2 := by / 2

	paddleXdistance := math.Abs(b.BallX - float64(b.PaddlePos))
	switch {
	// bounce on paddle
	case (1+by2 == b.PaddleY) && (by%2 == 0) && paddleXdistance <= float64(PaddleWidth)/2.:
		if b.JustBounced {
			b.JustBounced = false
			return false
		}
		vx += PaddleSpinFactor * float64(b.PaddleDirection)
		vy = -vy
		b.BallAngle = math.Atan2(vy, vx)
		b.BallSpeed = min(1.1, max(0.3, math.Sqrt(vx*vx+vy*vy)))
		b.JustBounced = true
	// bounce on walls
	case b.BallY >= b.BallHeight:
		if b.CheckLives {
			b.Death()
			return true
		}
		fallthrough
	case b.BallY < 0:
		b.BallAngle = 2*math.Pi - b.BallAngle
	case b.BallX <= 0 || b.BallX >= float64(b.Width)-1:
		b.BallAngle = math.Mod(3*math.Pi-b.BallAngle, 2*math.Pi)
	default:
		b.JustBounced = false
		return false
	}
	// avoid vertical or horizontal movement
	dx := math.Cos(b.BallAngle)
	dy := math.Sin(b.BallAngle)
	if math.Abs(dx) < 0.2 {
		b.BallAngle += (b.rnd.Float64() - 0.5) * math.Pi / 7
	}
	if math.Abs(dy) < 0.3 {
		incr := .15 + b.rnd.Float64()/10 // add 0.15 to 0.25 of vertical movement.
		if dy < 0 {
			incr = -incr
		}
		b.BallAngle = math.Atan2(dy+incr, dx)
	}
	// Angle might have changed above, recalculate dx, dy
	dx = math.Cos(b.BallAngle)
	dy = math.Sin(b.BallAngle)
	b.BallX += b.BallSpeed * dx
	b.BallY -= b.BallSpeed * dy
	return false
}

func (b *Brick) AutoPlay(vx, vy float64) {
	target := b.BallX + vx
	delta := 0.
	if vy > 0 {
		target = float64(b.Width) / 2.
	}
	target = math.Round(target + delta)
	switch {
	case target < float64(b.PaddlePos):
		b.Left()
	case target > float64(b.PaddlePos):
		b.Right()
	default:
		b.Center()
	}
}

func (b *Brick) Initial() {
	for y := range 8 {
		for x := range b.NumW {
			b.State[y*b.NumW+x] = true
		}
	}
	b.NumBricks = 8 * b.NumW
}

func (b *Brick) recordMove(direction int8) {
	if b.Replay {
		return
	}
	if direction == b.PaddleDirection {
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
	ap.WriteString(ansipixels.Reset)
	ap.DrawRoundBox(0, 0, ap.W, ap.H)
	ap.WriteBoxed(0, "Score %d", b.Score)
	livesSymbol := "❤️"
	if b.Auto {
		livesSymbol = "🤖"
	}
	switch b.Lives {
	case 0:
		ap.WriteRightBoxed(0, "☠️")
	case -1:
		if b.Replay {
			ap.WriteRightBoxed(0, "🔂")
		} else {
			ap.WriteRightBoxed(0, "∞%s", livesSymbol)
		}
	default:
		ap.WriteRightBoxed(0, "%d%s", b.Lives, livesSymbol)
	}
	for y := range 8 {
		ap.MoveCursor(b.Padding+1, 3+y)
		switch y {
		case 0:
			ap.WriteAtStr(b.Padding+1, 3+y, tcolor.BrightRed.Foreground())
		case 2:
			ap.WriteAtStr(b.Padding+1, 3+y, tcolor.Orange.Foreground())
		case 4:
			ap.WriteAtStr(b.Padding+1, 3+y, tcolor.Green.Foreground())
		case 6:
			ap.WriteAtStr(b.Padding+1, 3+y, tcolor.Yellow.Foreground())
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
	ap.WriteString(ansipixels.Cyan)
	ap.WriteAtStr(1+b.PaddlePos-3, ap.H-PaddleYDelta, Paddle)
	ap.WriteString(ansipixels.Reset)
	bx := safecast.MustRound[int](b.BallX)
	by := safecast.MustRound[int](b.BallY)
	by2 := by / 2
	if by2-1 < 8 {
		// probably should calculate this exactly right instead of this anti oob.
		bxx := max(0, min(b.NumW-1, (bx-b.Padding)/(BrickWidth+1)))
		byy := max(0, min(7, by2-1))
		if b.Has(bxx, byy) {
			b.BallAngle = 2*math.Pi - b.BallAngle
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

type BrickConfig struct {
	FPS      float64
	NumLives int
	NoDeath  bool
	NoSave   bool
	Seed     uint64
	Replay   string
	AutoPlay bool
	Ap       *ansipixels.AnsiPixels
}

func Main() int {
	fpsFlag := flag.Float64("fps", 30, "Frames per second")
	numLives := flag.Int("lives", 3, "Number of lives - 0 is infinite")
	noDeath := flag.Bool("nodeath", false, "No death mode")
	noSave := flag.Bool("nosave", false, "Don't save the game as JSON (default is to save)")
	seed := flag.Uint64("seed", 0, "Random number generator `seed`, default (0) is time based")
	replay := flag.String("replay", "", "Replay a `game` from a JSON file")
	autoPlay := flag.Bool("autoplay", false, "Computer plays mode")
	cli.Main()
	ap := ansipixels.NewAnsiPixels(*fpsFlag)
	cfg := BrickConfig{
		NumLives: *numLives,
		NoDeath:  *noDeath,
		NoSave:   *noSave,
		Seed:     *seed,
		Replay:   *replay,
		AutoPlay: *autoPlay,
		Ap:       ap,
	}
	err := ap.Open()
	if err != nil {
		return log.FErrf("Error opening AnsiPixels: %v", err)
	}
	defer ap.Restore()
	return cfg.Run()
}

func (cfg *BrickConfig) Run() int {
	ap := cfg.Ap
	ap.HideCursor()
	ap.Margin = 1
	if cfg.Replay != "" {
		return ReplayGame(ap, cfg.Replay)
	}
	seedV := cfg.Seed
	if seedV == 0 {
		seedV = safecast.MustConv[uint64](time.Now().UnixNano() % (1<<16 - 1))
	}
	var b *Brick
	ap.OnResize = func() error {
		ap.ClearScreen()
		ap.StartSyncMode()
		prevInfo := false
		if b != nil {
			prevInfo = b.ShowInfo
		}
		b = NewBrick(ap.W, ap.H, cfg.NumLives, !cfg.NoDeath, seedV) // half pixels vertically.
		b.Auto = cfg.AutoPlay
		b.ShowInfo = prevInfo
		Draw(ap, b)
		ap.WriteCentered(ap.H/2+1, "Any key to start...")
		ap.WriteCentered(ap.H/2+2, "🕹️ controls: Left A, Stop: S, Right: D")
		ap.WriteCentered(ap.H/2+3, "Quit: ^C or Shift-Q; Pause: Space")
		showInfo(ap, b)
		ap.EndSyncMode()
		b.waitForInput = true
		return nil
	}
	_ = ap.OnResize()
	death := false
	result := 0
	n := 0

	err := ap.FPSTicks(func() bool {
		if b.waitForInput && len(ap.Data) == 0 {
			// Pause mode after resize or death.
			return true
		}
		if handleKeys(ap, b) {
			if !cfg.NoSave {
				result = b.SaveGame()
			}
			return false // exit the loop
		}
		b.waitForInput = false
		if b.Paused {
			// User issued Pause (space bar): make the message blink (without flickering/clearing the screen: in place)
			msg := "⏱️ Paused, any key to resume... ⏱️"
			mlen := ap.ScreenWidth(msg)
			erase := strings.Repeat(" ", mlen)
			x := (ap.W - mlen) / 2
			y := ap.H/2 - 3
			switch n % 20 {
			case 0:
				ap.MoveCursor(x, y)
				ap.WriteString(msg)
			case 10:
				ap.MoveCursor(x, y)
				ap.WriteString(erase)
			}
			n++
			return true // continue the loop
		}
		n = 0 // reset pause counter (for next time we pause if any)
		ap.ClearScreen()
		Draw(ap, b)
		showInfo(ap, b)
		if death {
			DeathInfo(ap, b)
			if b.Lives == 0 {
				atEnd(ap, b)
				return false // exit the loop
			}
			death = false
			b.waitForInput = true
			return true // continue the loop
		}
		if b.NumBricks == 0 {
			handleWin(ap, b)
			return false
		}
		death = b.Next() // process a turn/tick.
		return true      // continue the ticks/loop
	})
	if err != nil {
		return log.FErrf("Error reading: %v", err)
	}
	return result
}

func handleWin(ap *ansipixels.AnsiPixels, b *Brick) {
	ap.WriteBoxed(ap.H/2, " 🏆✨ You won! ✨🏆 ")
	atEnd(ap, b)
}

func DeathInfo(ap *ansipixels.AnsiPixels, b *Brick) {
	if b.Lives == 0 {
		ap.WriteBoxed(ap.H/2, " ☠️ Game Over ☠️ ")
	} else {
		if b.Lives == -1 {
			ap.WriteBoxed(ap.H/2, " ☠️ Lost a life (∞ left 💔) ")
		} else {
			ap.WriteBoxed(ap.H/2, " ☠️ Lost a life, %d left 💔 ", b.Lives)
		}
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
func handleKeys(ap *ansipixels.AnsiPixels, b *Brick) bool {
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
		atEnd(ap, b)
		return true
	case ' ':
		// first space, starts the game, doesn't toggle pause.
		if !b.waitForInput {
			b.Paused = !b.Paused
		}
		return false // continue the loop
	}
	b.Paused = false // unpause if we were paused.
	return false     // continue the loop
}

func atEnd(ap *ansipixels.AnsiPixels, b *Brick) {
	b.ShowInfo = true
	showInfo(ap, b)
	ap.MoveCursor(0, ap.H-PaddleYDelta) // so 0,1 is great for shells that don't clear the bottom of the screen... yet zsh does that.
	ap.Out.Flush()
	_ = ap.ReadOrResizeOrSignal()
}
