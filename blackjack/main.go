package main

import (
	"flag"
	"log"
	"os"

	clishell "fortio.org/cli"
	"fortio.org/terminal/ansipixels"
	"fortio.org/terminal/blackjack/cli"
)

func main() {
	initialBalance := flag.Int("balance", 100, "Initial balance in `dollars`")
	betAmount := flag.Int("bet", 10, "Bet amount in `dollars`")
	numDecks := flag.Int("decks", 4, "Number of decks to use")
	fps := flag.Float64("fps", 60, "Frames per second (for resize/refreshes/animations)")
	greenFlag := flag.Bool("green", false, "Use green instead of black around the cards")
	noBorder := flag.Bool("no-border", false, "Don't draw the border at all around the cards")
	wideBorder := flag.Bool("wide", false, "Draw a wide border around the cards")
	clishell.Main()
	if *fps < 1 {
		log.Fatalf("Invalid fps (%f) must be at least 1", *fps)
	}
	if *fps > 100000 {
		log.Fatalf("Invalid fps (%f) must be less than 100000", *fps)
	}
	if *numDecks < 1 {
		log.Fatalf("Invalid number of decks (%d) must be at least 1", *numDecks)
	}
	if *betAmount < 1 {
		log.Fatalf("Invalid bet amount (%d) must be at least 1", *betAmount)
	}
	if *initialBalance < *betAmount {
		log.Fatalf("Initial balance (%d) must be at least the bet amount (%d)", *initialBalance, *betAmount)
	}
	ap := ansipixels.NewAnsiPixels(*fps)
	err := ap.Open()
	if err != nil {
		panic(err)
	}

	game := &cli.Game{
		AP:          ap,
		Playing:     true,
		State:       cli.StatePlayerTurn,
		Balance:     *initialBalance,
		Bet:         *betAmount,
		BorderColor: ansipixels.Black,
		BorderBG:    ansipixels.BlackBG,
		WideBorder:  *wideBorder,
	}
	if *wideBorder {
		game.BorderColor = ansipixels.BlackBG
	}
	if *greenFlag {
		game.BorderColor = ansipixels.Green
		game.BorderBG = ansipixels.GreenBG
	}
	if *noBorder {
		game.BorderColor = ""
		game.BorderBG = ""
	}
	game.InitDeck(*numDecks)
	os.Exit(game.RunGame(ap))
}
