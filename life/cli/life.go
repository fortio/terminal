package cli

import (
	"flag"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal/ansipixels"
	"fortio.org/terminal/life/conway"
)

func Main() int {
	fpsFlag := flag.Float64("fps", 60, "Frames per second")
	flagRandomFill := flag.Float64("fill", 0.1, "Random fill factor (0 to 1)")
	flagGlider := flag.Bool("glider", false, "Start with a glider (default is random)")
	noMouseFlag := flag.Bool("nomouse", false, "Disable mouse tracking")
	cli.Main()
	game := &conway.Game{HasMouse: !*noMouseFlag}
	return RunGame(game, *fpsFlag, *flagRandomFill, *flagGlider)
}

func RunGame(game *conway.Game, fps float64, randomFill float64, glider bool) int {
	ap := ansipixels.NewAnsiPixels(fps)
	err := ap.Open()
	if err != nil {
		return log.FErrf("Error opening AnsiPixels: %v", err)
	}
	game.AP = ap
	defer game.End()
	ap.HideCursor()
	if game.HasMouse {
		ap.MouseClickOn() // start with just clicks, we turn on drag after a click.
	}
	fillFactor := float32(randomFill)
	ap.OnResize = func() error {
		game.C = conway.NewConway(ap.W, 2*ap.H) // half pixels vertically.
		if glider {
			game.C.Glider(ap.W/3, 2*ap.H/3) // first third of the screen
		} else {
			// Random
			game.C.Randomize(fillFactor)
		}
		game.ShowInfo = true
		game.State = conway.Paused
		game.ShowHelp = true
		game.Start()
		return nil
	}
	_ = ap.OnResize()
	for {
		switch game.State {
		case conway.Running:
			_, err := ap.ReadOrResizeOrSignalOnce()
			if err != nil {
				return log.FErrf("Error reading: %v", err)
			}
		case conway.Paused:
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
			game.ShowInfo = !game.ShowInfo
		case '?', 'h', 'H':
			game.ShowHelp = true
			game.State = conway.Paused
		case ' ':
			game.State = conway.Paused
		default:
			game.State = conway.Running
		}
		game.Next()
	}
}
