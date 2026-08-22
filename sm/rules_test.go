package sm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// maskPlayable mirrors MASK_PLAYABLE in sm_internal.h.
const maskPlayable = 0x80

func angelCard() Card { return Card{displayOrder: 33, suit: SuitBaozi, rank: 10, flags: maskPlayable} }
func ghostCard() Card { return Card{displayOrder: 31, suit: SuitOther, rank: 11, flags: maskPlayable} }
func baozi(rank int32) Card {
	return Card{displayOrder: uint32(30 - (9 - rank)), suit: SuitBaozi, rank: rank, flags: maskPlayable}
}
func xiang(rank int32) Card {
	return Card{displayOrder: uint32(10 - (9 - rank)), suit: SuitXiang, rank: rank, flags: maskPlayable}
}

// playOneRound deals each player the single card they are about to play, then
// plays them in seat order. The engine deals at random, so the hands are
// written directly instead — the test needs a specific trick, not a random one.
func playOneRound(t *testing.T, hand ...Card) *GameState {
	t.Helper()
	g, err := NewGame(GameConfig{NumPlayers: int32(len(hand))}, nil)
	assert.NoError(t, err)
	assert.NoError(t, g.Start())

	for i, c := range hand {
		g.players[i].allCards = [22]Card{0: c}
		g.players[i].numCards = 1
		g.players[i].score = 0
		g.players[i].cardsTaken = 0
	}
	g.curRound = [4]Card{}
	g.highRanker = -1
	g.numRound = 0

	for _, c := range hand {
		assert.NoError(t, g.Play(c))
	}
	return g
}

func scores(g *GameState, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = int(g.players[i].score)
	}
	return out
}

// TestGhostDoublesRoundScore covers rule 6: the Ghost carries no value of its
// own, but doubles the round score of whoever played it — negative included.
func TestGhostDoublesRoundScore(t *testing.T) {
	t.Run("负分翻倍", func(t *testing.T) {
		// Ghost leads and wins the trick; the other two play Shit (-1 each).
		g := playOneRound(t, ghostCard(), xiang(5), xiang(3))
		assert.Equal(t, []int{-4, 0, 0}, scores(g, 3), "(0-1-1)*2 = -4")
	})

	t.Run("正分翻倍", func(t *testing.T) {
		// Angel counts as a Bun, so the trick is worth +2 before doubling.
		g := playOneRound(t, ghostCard(), angelCard(), baozi(5))
		assert.Equal(t, []int{4, 0, 0}, scores(g, 3), "(0+1+1)*2 = 4")
	})

	t.Run("无鬼牌不翻倍", func(t *testing.T) {
		g := playOneRound(t, baozi(9), xiang(5), xiang(3))
		assert.Equal(t, []int{-1, 0, 0}, scores(g, 3), "1-1-1 = -1, 未翻倍")
	})
}

// TestNativePRNGGoldenSequence pins the engine's own PRNG to the sequence
// Microsoft's rand() produces from seed 1, which is what sm.c set out to
// replicate. It guards the unsigned rewrite of ms_rand() against any change in
// the values it yields.
func TestNativePRNGGoldenSequence(t *testing.T) {
	want := []int{41, 18467, 6334, 26500, 19169, 15724, 11478, 29358, 26962, 24464, 5705, 28145}
	state := int32(1)
	got := make([]int, len(want))
	for i := range got {
		got[i] = nativeRandom(&state)
	}
	assert.Equal(t, want, got)
}
