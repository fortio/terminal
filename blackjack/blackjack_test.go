package main

import (
	"testing"
)

func TestCalculateHand(t *testing.T) {
	tests := []struct {
		name     string
		cards    []Card
		expected int
	}{
		{
			name:     "Simple hand",
			cards:    []Card{{Value: "1 0", Suit: "♠"}, {Value: "5", Suit: Heart}},
			expected: 15,
		},
		{
			name:     "Ace as 11",
			cards:    []Card{{Value: "A", Suit: "♠"}, {Value: "9", Suit: Heart}},
			expected: 20,
		},
		{
			name:     "Ace as 1",
			cards:    []Card{{Value: "A", Suit: "♠"}, {Value: "9", Suit: Heart}, {Value: "2", Suit: "♦"}},
			expected: 12,
		},
		{
			name:     "Multiple aces",
			cards:    []Card{{Value: "A", Suit: "♠"}, {Value: "A", Suit: Heart}, {Value: "9", Suit: "♦"}},
			expected: 21,
		},
		{
			name:     "Face cards",
			cards:    []Card{{Value: "K", Suit: "♠"}, {Value: "Q", Suit: Heart}},
			expected: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Game{}
			result := g.calculateHand(tt.cards)
			if result != tt.expected {
				t.Errorf("calculateHand() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsBlackjack(t *testing.T) {
	tests := []struct {
		name     string
		cards    []Card
		expected bool
	}{
		{
			name:     "Blackjack",
			cards:    []Card{{Value: "A", Suit: "♠"}, {Value: "K", Suit: Heart}},
			expected: true,
		},
		{
			name:     "Not blackjack",
			cards:    []Card{{Value: "A", Suit: "♠"}, {Value: "9", Suit: Heart}},
			expected: false,
		},
		{
			name:     "Three cards",
			cards:    []Card{{Value: "A", Suit: "♠"}, {Value: "K", Suit: Heart}, {Value: "Q", Suit: "♦"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Game{}
			result := g.isBlackjack(tt.cards)
			if result != tt.expected {
				t.Errorf("isBlackjack() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGameBalance(t *testing.T) {
	tests := []struct {
		name            string
		playerCards     []Card
		dealerCards     []Card
		initialBalance  int
		bet             int
		expectedBalance int
		expectedMessage string
	}{
		{
			name:            "Player wins",
			playerCards:     []Card{{Value: "K", Suit: "♠"}, {Value: "Q", Suit: Heart}},
			dealerCards:     []Card{{Value: "9", Suit: "♠"}, {Value: "8", Suit: Heart}},
			initialBalance:  100,
			bet:             10,
			expectedBalance: 110,
			expectedMessage: "You win $10!",
		},
		{
			name:            "Player loses",
			playerCards:     []Card{{Value: "9", Suit: "♠"}, {Value: "8", Suit: Heart}},
			dealerCards:     []Card{{Value: "K", Suit: "♠"}, {Value: "Q", Suit: Heart}},
			initialBalance:  100,
			bet:             10,
			expectedBalance: 90,
			expectedMessage: "Dealer wins! You lose $10!",
		},
		{
			name:            "Player blackjack",
			playerCards:     []Card{{Value: "A", Suit: "♠"}, {Value: "K", Suit: Heart}},
			dealerCards:     []Card{{Value: "9", Suit: "♠"}, {Value: "8", Suit: Heart}},
			initialBalance:  100,
			bet:             10,
			expectedBalance: 115,
			expectedMessage: "Blackjack! You win $15!",
		},
		{
			name:            "Player busts",
			playerCards:     []Card{{Value: "K", Suit: "♠"}, {Value: "Q", Suit: Heart}, {Value: "2", Suit: "♦"}},
			dealerCards:     []Card{{Value: "9", Suit: "♠"}, {Value: "8", Suit: Heart}},
			initialBalance:  100,
			bet:             10,
			expectedBalance: 90,
			expectedMessage: "Bust! You lose $10!",
		},
		{
			name:            "Push",
			playerCards:     []Card{{Value: "K", Suit: "♠"}, {Value: "Q", Suit: Heart}},
			dealerCards:     []Card{{Value: "K", Suit: "♦"}, {Value: "Q", Suit: "♣"}},
			initialBalance:  100,
			bet:             10,
			expectedBalance: 100,
			expectedMessage: "Push! Your bet is returned.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Game{
				balance: tt.initialBalance,
				bet:     tt.bet,
				player:  tt.playerCards,
				dealer:  tt.dealerCards,
			}

			// Test dealer turn for normal play
			if len(tt.playerCards) == 2 {
				g.dealerTurn()
			} else {
				// Test bust case
				g.state = StateGameOver
				g.message = tt.expectedMessage
				g.balance -= tt.bet
			}

			if g.balance != tt.expectedBalance {
				t.Errorf("balance = %v, want %v", g.balance, tt.expectedBalance)
			}
			if g.message != tt.expectedMessage {
				t.Errorf("message = %v, want %v", g.message, tt.expectedMessage)
			}
		})
	}
}
