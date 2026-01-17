package cli

import (
	"fmt"
	"math/rand/v2"

	"fortio.org/terminal/ansipixels"
)

// Card represents a playing card with a suit and value.
type Card struct {
	Suit  string
	Value string
}

// Deck represents a deck of cards.
type Deck struct {
	Cards []Card
	Decks int // Number of decks in play
}

// GameState represents the current state of the game.
type GameState int

const (
	StatePlayerTurn GameState = iota
	StateDealerTurn
	StateGameOver
)

// Game represents the blackjack game state.
type Game struct {
	AP          *ansipixels.AnsiPixels
	Playing     bool
	State       GameState
	Balance     int
	Bet         int
	BorderColor string
	BorderBG    string
	WideBorder  bool
	first       bool
	deck        *Deck
	player      []Card
	dealer      []Card
	message     string
}

// Heart symbol used in the blackjack game used to be configurable because
// (only) Ghostty is broken and maybe will get fixed some day...
// https://github.com/ghostty-org/ghostty/discussions/7204
// Removing this idiotic workaround (before we switched
// ♥ to ❤ (the wrong one) for ghostty on macos, not anymore).

// InitDeck initializes a new shuffled deck.
func (g *Game) InitDeck(numDecks int) {
	suits := []string{"♠", "♥", "♦", "♣"}
	values := []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "1 0", "J", "Q", "K"}

	g.deck = &Deck{
		Cards: make([]Card, 0, 52*numDecks),
		Decks: numDecks,
	}

	// Create multiple decks
	for range numDecks {
		for _, suit := range suits {
			for _, value := range values {
				g.deck.Cards = append(g.deck.Cards, Card{Suit: suit, Value: value})
			}
		}
	}

	// Shuffle the deck
	rand.Shuffle(len(g.deck.Cards), func(i, j int) {
		g.deck.Cards[i], g.deck.Cards[j] = g.deck.Cards[j], g.deck.Cards[i]
	})
}

// drawCard draws a card from the deck.
func (g *Game) drawCard() Card {
	card := g.deck.Cards[0]
	g.deck.Cards = g.deck.Cards[1:]
	if len(g.deck.Cards) == 0 {
		g.InitDeck(g.deck.Decks)
		g.message = "New deck shuffled!"
	}
	return card
}

const (
	cardBack   = "░░░░░"
	cardWidth  = 6 // including the space in between cards/on the right of a card
	cardHeight = 4 // not including the border
)

// drawCardOnScreen draws a card on the screen at the specified position.
func (g *Game) drawCardOnScreen(x, y int, card Card, hidden bool) {
	// Draw card border: mostly redundant with the outer one except for the middle in between cards
	if g.BorderColor != "" {
		g.AP.DrawColoredBox(x-1, y-1, cardWidth+1, cardHeight+1, g.BorderColor, false)
	}
	// Draw card content
	g.AP.MoveCursor(x, y)
	if hidden {
		g.AP.WriteString(ansipixels.WhiteBG + ansipixels.Black + cardBack)
		g.AP.MoveCursor(x, y+1)
		g.AP.WriteString(cardBack)
		g.AP.MoveCursor(x, y+2)
		g.AP.WriteString(cardBack + ansipixels.Reset)
		return
	}
	// Top suit
	var cardContent string
	if card.Suit == "♥" || card.Suit == "♦" {
		cardContent = fmt.Sprintf("%s%s    ", ansipixels.WhiteBG+ansipixels.Red, card.Suit)
	} else {
		cardContent = fmt.Sprintf("%s%s    ", ansipixels.WhiteBG+ansipixels.Black, card.Suit)
	}
	g.AP.WriteString(cardContent)
	// Center value
	g.AP.MoveCursor(x, y+1)
	if len(card.Value) == 1 {
		cardContent = fmt.Sprintf("  %s  ", card.Value)
	} else { // "1 0"
		cardContent = fmt.Sprintf(" %s ", card.Value)
	}
	g.AP.WriteString(cardContent)
	// Bottom suit
	g.AP.MoveCursor(x, y+2)
	cardContent = fmt.Sprintf("    %s%s", card.Suit, ansipixels.Reset)
	g.AP.WriteString(cardContent)
}

// drawHand draws a hand of cards at the specified position.
func (g *Game) drawHand(x, y int, cards []Card, hideFirst bool) {
	for i, card := range cards {
		hidden := hideFirst && i == 0
		pos := x + i*cardWidth
		g.drawCardOnScreen(pos, y, card, hidden)
	}
	// For wide mode: erase top/bottom thin border and add extra bars
	// vertically so space around cards is even on height vs width (as pixels are 2x tall than wide)
	if g.BorderColor != "" && g.WideBorder {
		g.AP.DrawColoredBox(x-1, y-1, cardWidth*len(cards)+1, cardHeight+1, g.BorderBG, true)
	}
}

// calculateHand calculates the value of a hand.
func (g *Game) calculateHand(hand []Card) int {
	value := 0
	aces := 0

	for _, card := range hand {
		switch card.Value {
		case "A":
			aces++
			value += 11
		case "K", "Q", "J":
			value += 10
		case "1 0":
			value += 10
		default:
			value += int(card.Value[0] - '0')
		}
	}

	// Adjust for aces
	for aces > 0 && value > 21 {
		value -= 10
		aces--
	}

	return value
}

// isBlackjack checks if a hand is a blackjack (21 with first two cards).
func (g *Game) isBlackjack(hand []Card) bool {
	if len(hand) != 2 {
		return false
	}
	return g.calculateHand(hand) == 21
}

// Run starts the game loop.
func (g *Game) Run() {
	defer func() {
		g.AP.MoveCursor(0, g.AP.H-1)
		g.AP.Restore()
	}()

	// Initial deal
	g.resetGame()
	g.first = true

	for g.Playing {
		g.draw()
		if g.first {
			g.first = false
			helpText := `Blackjack! Get closest to 21 without going over.
Aces are either 1pt or 11pts. Blackjack (21 in 2 cards) pays 3:2.
Dealer always hits (gets another card) on 16pts or less,
stands (no more card) on 17pts or more. Space to continue.`
			g.AP.WriteBoxed(g.AP.H/2-1, "%s", helpText)
		}
		if g.State == StatePlayerTurn && g.calculateHand(g.player) == 21 {
			g.State = StateDealerTurn
			g.dealerTurn()
			g.draw()
		}
		// Handle input
		err := g.AP.ReadOrResizeOrSignal()
		if err != nil {
			break
		}

		// Process input based on game state
		if len(g.AP.Data) > 0 {
			switch g.AP.Data[0] {
			case 'q', 'Q':
				g.Playing = false
			default:
				switch g.State {
				case StatePlayerTurn:
					switch g.AP.Data[0] {
					case 'h', 'H':
						g.player = append(g.player, g.drawCard())
						playerScore := g.calculateHand(g.player)
						if playerScore > 21 {
							g.State = StateGameOver
							g.message = fmt.Sprintf("Bust! You lose $%d!", g.Bet)
							g.Balance -= g.Bet
						}
					case 's', 'S':
						g.State = StateDealerTurn
						g.dealerTurn()
					}
				case StateGameOver:
					if g.Balance >= g.Bet {
						g.resetGame()
					}
				case StateDealerTurn:
					panic("shouldn't be reached (played above)")
				}
			}
		}
	}
}

// dealerTurn handles the dealer's turn.
func (g *Game) dealerTurn() {
	// Reveal dealer's hidden card
	dealerScore := g.calculateHand(g.dealer)
	playerScore := g.calculateHand(g.player)

	// Check for player blackjack first
	if g.isBlackjack(g.player) {
		if g.isBlackjack(g.dealer) {
			g.message = "Both have blackjack! Push! Your bet is returned."
		} else {
			// 3:2 payout for blackjack
			winnings := (g.Bet * 3) / 2
			g.message = fmt.Sprintf("Blackjack! You win $%d!", winnings)
			g.Balance += winnings
		}
		g.State = StateGameOver
		return
	}

	// Dealer must hit on 16 and below, stand on 17 and above
	for dealerScore < 17 {
		g.dealer = append(g.dealer, g.drawCard())
		dealerScore = g.calculateHand(g.dealer)
	}

	// Determine winner and update balance
	switch {
	case dealerScore > 21:
		g.message = fmt.Sprintf("Dealer busts! You win $%d!", g.Bet)
		g.Balance += g.Bet
	case dealerScore > playerScore:
		g.message = fmt.Sprintf("Dealer wins! You lose $%d!", g.Bet)
		g.Balance -= g.Bet
	case dealerScore < playerScore:
		g.message = fmt.Sprintf("You win $%d!", g.Bet)
		g.Balance += g.Bet
	default:
		g.message = "Push! Your bet is returned."
	}

	g.State = StateGameOver
}

// resetGame resets the game state for a new round.
func (g *Game) resetGame() {
	// Check if player has enough balance
	if g.Balance < g.Bet {
		g.message = fmt.Sprintf("Not enough balance! You have $%d but need $%d to play.", g.Balance, g.Bet)
		return
	}

	g.player = []Card{g.drawCard(), g.drawCard()}
	g.dealer = []Card{g.drawCard(), g.drawCard()}
	g.State = StatePlayerTurn
	g.message = ""
}

func (g *Game) LeftMostCardPos(numCards int) int {
	// Calculate the starting horizontal position for the cards
	width := cardWidth*numCards - 1 // -1 because of right space on last card
	return (g.AP.W - width) / 2
}

// draw draws the current game state.
func (g *Game) draw() {
	g.AP.ClearScreen()

	// Draw balance and bet
	g.AP.WriteAt(2, 1, "Balance: $%d", g.Balance)
	g.AP.WriteRight(1, "Bet: $%d   ", g.Bet)

	// Draw dealer's hand
	g.AP.WriteCentered(2, "Dealer's Hand")
	dealerOffset := g.LeftMostCardPos(len(g.dealer))
	g.drawHand(dealerOffset, 5, g.dealer, g.State == StatePlayerTurn)

	// Draw player's hand
	g.AP.WriteCentered(g.AP.H-12, "Your Hand")
	playerOffset := g.LeftMostCardPos(len(g.player))
	g.drawHand(playerOffset, g.AP.H-9, g.player, false)

	// Draw scores
	dealerScore := g.calculateHand(g.dealer)
	if g.State == StatePlayerTurn {
		dealerScore = g.calculateHand(g.dealer[1:]) // Only show visible cards
	}
	playerScore := g.calculateHand(g.player)

	// Add blackjack indicator if applicable
	extraText := ""
	if g.isBlackjack(g.player) {
		extraText = " (Blackjack!)"
	}

	g.AP.WriteAt(2, g.AP.H-2, "Your Score: %d%s", playerScore, extraText)
	g.AP.WriteRight(g.AP.H-2, "Dealer's Score: %d   ", dealerScore)

	// Draw game message
	if g.message != "" {
		g.AP.WriteCentered(g.AP.H-3, "%s", g.message)
	}

	// Number of cards left in the deck:
	cardsLeft := len(g.deck.Cards)
	totalCards := 52 * g.deck.Decks
	g.AP.WriteRight(g.AP.H-1, "%d cards   ", cardsLeft)

	// Draw deck size indicator
	percentage := float64(cardsLeft) / float64(totalCards)
	indicatorHeight := int(2 * float64(g.AP.H-4) * percentage) // -4 to account for scores and instructions

	// Draw the indicator from bottom to top using half-height pixels
	for y := 0; y < indicatorHeight-1; y += 2 {
		g.AP.MoveCursor(g.AP.W-1, g.AP.H-3-y/2)
		g.AP.WriteRune(ansipixels.FullPixel)
	}
	if indicatorHeight%2 == 1 {
		g.AP.MoveCursor(g.AP.W-1, g.AP.H-3-indicatorHeight/2)
		g.AP.WriteRune(ansipixels.BottomHalfPixel)
	}
	// Draw instructions
	switch g.State {
	case StatePlayerTurn:
		g.AP.WriteCentered(g.AP.H-1, "Press 'h' to hit, 's' to stand, 'q' to quit")
	case StateGameOver:
		if g.Balance >= g.Bet {
			g.AP.WriteCentered(g.AP.H-1, "Any key for new game, 'q' to quit")
		} else {
			g.AP.WriteCentered(g.AP.H-1, "Press 'q' to quit")
		}
	case StateDealerTurn:
		// nothing
	}
	g.AP.EndSyncMode()
}

func (g *Game) RunGame(ap *ansipixels.AnsiPixels) int {
	// Handle terminal resize
	ap.OnResize = func() error {
		ap.ClearScreen()
		ap.StartSyncMode()
		g.draw()
		ap.EndSyncMode()
		return nil
	}
	g.Run()
	return 0
}
