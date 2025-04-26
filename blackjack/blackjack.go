package main

import (
	"flag"
	"fmt"
	"math/rand/v2"

	"fortio.org/cli"
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
	ap      *ansipixels.AnsiPixels
	deck    *Deck
	player  []Card
	dealer  []Card
	playing bool
	state   GameState
	message string
	balance int
	bet     int
}

// initDeck initializes a new shuffled deck.
func (g *Game) initDeck(numDecks int) {
	suits := []string{"♠", "❤", "♦", "♣"}
	values := []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}

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
		g.initDeck(g.deck.Decks)
		g.message = "New deck shuffled!"
	}
	return card
}

// drawCardOnScreen draws a card on the screen at the specified position.
func (g *Game) drawCardOnScreen(x, y int, card Card, hidden bool) {
	// Draw card border
	g.ap.MoveCursor(x, y)
	g.ap.WriteString("┌─────┐")

	// Draw card content
	g.ap.MoveCursor(x, y+1)
	if hidden {
		g.ap.WriteString("│░░░░░│")
	} else {
		// Add color for hearts and diamonds
		cardContent := fmt.Sprintf("│%s%2s %s %s│", ansipixels.WhiteBG+ansipixels.Black, card.Value, card.Suit, ansipixels.Reset)
		if card.Suit == "❤" || card.Suit == "♦" {
			cardContent = fmt.Sprintf("│%s%2s %s %s│", ansipixels.WhiteBG+ansipixels.Red, card.Value, card.Suit, ansipixels.Reset)
		}
		g.ap.WriteString(cardContent)
	}

	g.ap.MoveCursor(x, y+2)
	g.ap.WriteString("└─────┘")
}

// drawHand draws a hand of cards at the specified position.
func (g *Game) drawHand(x, y int, cards []Card, hideFirst bool) {
	cardWidth := 7 // Width of a card including borders
	for i, card := range cards {
		hidden := hideFirst && i == 0
		g.drawCardOnScreen(x+i*cardWidth, y, card, hidden)
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
		case "10":
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
		g.ap.MoveCursor(0, g.ap.H-1)
		g.ap.Restore()
	}()

	// Initial deal
	g.player = []Card{g.drawCard(), g.drawCard()}
	g.dealer = []Card{g.drawCard(), g.drawCard()}

	for g.playing {
		g.draw()

		// Handle input
		err := g.ap.ReadOrResizeOrSignal()
		if err != nil {
			break
		}

		// Process input based on game state
		if len(g.ap.Data) > 0 {
			switch g.ap.Data[0] {
			case 'q', 'Q':
				g.playing = false
			default:
				switch g.state {
				case StatePlayerTurn:
					switch g.ap.Data[0] {
					case 'h', 'H':
						g.player = append(g.player, g.drawCard())
						playerScore := g.calculateHand(g.player)
						if playerScore > 21 {
							g.state = StateGameOver
							g.message = fmt.Sprintf("Bust! You lose $%d!", g.bet)
							g.balance -= g.bet
						}
					case 's', 'S':
						g.state = StateDealerTurn
						g.dealerTurn()
					}
				case StateGameOver:
					if g.balance >= g.bet {
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
			winnings := (g.bet * 3) / 2
			g.message = fmt.Sprintf("Blackjack! You win $%d!", winnings)
			g.balance += winnings
		}
		g.state = StateGameOver
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
		g.message = fmt.Sprintf("Dealer busts! You win $%d!", g.bet)
		g.balance += g.bet
	case dealerScore > playerScore:
		g.message = fmt.Sprintf("Dealer wins! You lose $%d!", g.bet)
		g.balance -= g.bet
	case dealerScore < playerScore:
		g.message = fmt.Sprintf("You win $%d!", g.bet)
		g.balance += g.bet
	default:
		g.message = "Push! Your bet is returned."
	}

	g.state = StateGameOver
}

// resetGame resets the game state for a new round.
func (g *Game) resetGame() {
	// Check if player has enough balance
	if g.balance < g.bet {
		g.message = fmt.Sprintf("Not enough balance! You have $%d but need $%d to play.", g.balance, g.bet)
		return
	}

	g.player = []Card{g.drawCard(), g.drawCard()}
	g.dealer = []Card{g.drawCard(), g.drawCard()}
	g.state = StatePlayerTurn
	g.message = ""
}

// draw draws the current game state.
func (g *Game) draw() {
	g.ap.ClearScreen()

	// Draw balance and bet
	g.ap.WriteAt(2, 1, "Balance: $%d", g.balance)
	g.ap.WriteAt(g.ap.W-20, 1, "Bet: $%d", g.bet)

	// Draw dealer's hand
	dealerTitle := "Dealer's Hand"
	g.ap.WriteCentered(2, dealerTitle)
	cardWidth := 7
	dealerOffset := (g.ap.W - cardWidth*len(g.dealer)) / 2
	g.drawHand(dealerOffset, 4, g.dealer, g.state == StatePlayerTurn)

	// Draw player's hand
	playerTitle := "Your Hand"
	g.ap.WriteCentered(g.ap.H-8, playerTitle)
	playerOffset := (g.ap.W - cardWidth*len(g.player)) / 2
	g.drawHand(playerOffset, g.ap.H-6, g.player, false)

	// Draw scores
	dealerScore := g.calculateHand(g.dealer)
	if g.state == StatePlayerTurn {
		dealerScore = g.calculateHand(g.dealer[1:]) // Only show visible cards
	}
	playerScore := g.calculateHand(g.player)

	// Add blackjack indicator if applicable
	scoreText := fmt.Sprintf("Your Score: %d", playerScore)
	if g.isBlackjack(g.player) {
		scoreText += " (Blackjack!)"
	}

	g.ap.WriteAt(2, g.ap.H-2, scoreText)
	g.ap.WriteAt(g.ap.W-20, g.ap.H-2, "Dealer's Score: %d", dealerScore)

	// Draw game message
	if g.message != "" {
		g.ap.WriteCentered(g.ap.H-3, g.message)
	}

	// Number of cards left in the deck:
	cardsLeft := len(g.deck.Cards)
	totalCards := 52 * g.deck.Decks
	g.ap.WriteRight(g.ap.H-1, fmt.Sprintf("%d/%d cards", cardsLeft, totalCards))

	// Draw deck size indicator
	percentage := float64(cardsLeft) / float64(totalCards)
	indicatorHeight := int(2 * float64(g.ap.H-4) * percentage) // -4 to account for scores and instructions

	// Draw the indicator from bottom to top using half-height pixels
	for y := 0; y < indicatorHeight-1; y += 2 {
		g.ap.MoveCursor(g.ap.W-1, g.ap.H-3-y/2)
		g.ap.WriteRune(ansipixels.FullPixel)
	}
	if indicatorHeight%2 == 1 {
		g.ap.MoveCursor(g.ap.W-1, g.ap.H-3-indicatorHeight/2)
		g.ap.WriteRune(ansipixels.BottomHalfPixel)
	}

	// Draw instructions
	if g.state == StatePlayerTurn {
		g.ap.WriteCentered(g.ap.H-1, "Press 'h' to hit, 's' to stand, 'q' to quit")
	} else if g.state == StateGameOver {
		if g.balance >= g.bet {
			g.ap.WriteCentered(g.ap.H-1, "Any key for new game, 'q' to quit")
		} else {
			g.ap.WriteCentered(g.ap.H-1, "Press 'q' to quit")
		}
	}

	g.ap.EndSyncMode()
}

func main() {
	initialBalance := flag.Int("balance", 100, "Initial balance in `dollars`")
	betAmount := flag.Int("bet", 10, "Bet amount in `dollars`")
	numDecks := flag.Int("decks", 4, "Number of decks to use")
	cli.Main()

	ap := ansipixels.NewAnsiPixels(10) // 10 FPS is plenty for this game
	err := ap.Open()
	if err != nil {
		panic(err)
	}

	game := &Game{
		ap:      ap,
		playing: true,
		state:   StatePlayerTurn,
		balance: *initialBalance,
		bet:     *betAmount,
	}
	game.initDeck(*numDecks)

	// Handle terminal resize
	ap.OnResize = func() error {
		ap.ClearScreen()
		ap.StartSyncMode()
		game.draw()
		ap.EndSyncMode()
		return nil
	}

	game.Run()
}
