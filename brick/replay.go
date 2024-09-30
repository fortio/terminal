package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/terminal/ansipixels"
)

func ReplayGame(ap *ansipixels.AnsiPixels, fname string) int {
	// Open the file
	f, err := os.Open(fname)
	if err != nil {
		return log.FErrf("Error opening file: %v\r", err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return log.FErrf("Error reading file: %v\r", err)
	}
	gs := GameSave{}
	err = json.Unmarshal(data, &gs)
	if err != nil {
		return log.FErrf("Error unmarshalling JSON: %v\r", err)
	}
	expectedX := gs.Width + 2
	expectedY := gs.Height + 2
	sizeOk := false
	var b *Brick

	ap.OnResize = func() error {
		ap.ClearScreen()
		ap.StartSyncMode()
		xmatch := "✅"
		sizeOk = true
		if ap.W != expectedX {
			sizeOk = false
			xmatch = "❌"
		}
		ymatch := "✅"
		if ap.H != expectedY {
			sizeOk = false
			ymatch = "❌"
		}
		ap.WriteBoxed(ap.H/2-2, "Size %s %d x %s %d (need %dx%d)", xmatch, ap.W, ymatch, ap.H, expectedX, expectedY)
		b = NewBrick(ap.W, ap.H, 0, false, gs.Seed)
		b.ShowInfo = gs.ShowInfo // currently always true because we set it to true on exit and then save.
		b.MoveRecords = gs.MoveRecords
		b.Replay = true
		ap.WriteCentered(ap.H/2+2, "Press a key once matched and ready, ^C to abort\r")
		return nil
	}
	_ = ap.OnResize()
	ap.MoveCursor(0, 0)
	ap.EndSyncMode()
	log.Infof("GameSave: %dx%d from %s (we are %s)\r", gs.Width+2, gs.Height+2, gs.Version, cli.LongVersion)
	for {
		err := ap.ReadOrResizeOrSignal()
		if err != nil || handleKeys(ap, b, true) {
			ap.WriteAt(0, 1, "%sReplay aborted", log.ANSIColors.Reset)
			ap.MoveCursor(0, 2)
			ap.Out.Flush()
			return 0
		}
		if sizeOk {
			break
		}
	}
	return replayGame(ap, b, gs.Frames)
}

func replayGame(ap *ansipixels.AnsiPixels, b *Brick, numFrames uint64) int {
	ap.OnResize = func() error {
		return errors.New("resize during replay")
	}
	for {
		ap.StartSyncMode()
		ap.ClearScreen()
		Draw(ap, b)
		showInfo(ap, b)
		ap.EndSyncMode()
		_, err := ap.ReadOrResizeOrSignalOnce()
		if err != nil {
			ap.MoveCursor(0, 1)
			ap.Out.Flush()
			return log.FErrf("Error: %v\r", err)
		}
		if handleKeys(ap, b, false /* pauses ok */) {
			return 0
		}
		if b.Frames <= numFrames {
			b.Next()
		} else {
			ap.WriteCentered(ap.H/2, "%s🔂 Replay done... any key to exit...", log.ANSIColors.Reset)
			ap.MoveCursor(0, 1)
			_ = ap.ReadOrResizeOrSignal()
			return 0
		}
	}
}
