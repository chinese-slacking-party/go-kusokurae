package sm

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGame(t *testing.T) {
	_, err := NewGame(GameConfig{
		NumPlayers: 2,
	}, nil)
	assert.Equal(t, ErrBadNPlayers, err)

	correctCfg := GameConfig{
		NumPlayers: 3,
	}
	state, err := NewGame(correctCfg, nil)
	assert.NoError(t, err)
	assert.Equal(t, correctCfg, state.cfg)
	assert.Equal(t, 1, int(state.players[0].index))
	assert.Equal(t, 2, int(state.players[1].index))
	assert.Equal(t, 3, int(state.players[2].index))
	assert.Equal(t, 0, int(state.players[3].index))
}

func TestActivePlayerNil(t *testing.T) {
	state, err := NewGame(GameConfig{
		NumPlayers: 3,
	}, nil)
	assert.NoError(t, err)

	// bs@bs-newnb-w10:~/go/src/github.com/bs-iron-trio/go-kusokurae/sm$ go test
	// --- FAIL: TestActivePlayerNil (0.00s)
	// 		sm_test.go:34:
	// 						Error Trace:    sm_test.go:34
	// 						Error:          Not equal:
	// 										expected: <nil>(<nil>)
	// 										actual  : *sm.Player((*sm.Player)(nil))
	// 						Test:           TestActivePlayerNil
	// FAIL
	// exit status 1
	// FAIL    github.com/bs-iron-trio/go-kusokurae/sm 0.034s
	//assert.Equal(t, nil, state.GetActivePlayer())
	//assert.EqualValues(t, nil, state.GetActivePlayer())
	assert.Nil(t, state.GetActivePlayer())
}

func TestStateCB(t *testing.T) {
	var calls int
	var recordedNewState GameStatus
	state, err := NewGame(GameConfig{
		NumPlayers: 4,
	}, func(newState GameStatus) {
		calls++
		recordedNewState = newState
	})
	assert.NoError(t, err)

	err = state.Start()
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, StatusPlay, recordedNewState)
}

func TestGameStart(t *testing.T) {
	state, err := NewGame(GameConfig{
		NumPlayers: 3,
	}, nil)
	assert.NoError(t, err)

	err = state.Start()
	assert.NoError(t, err)
	assert.Equal(t, RoundActive, state.players[0].active)
	assert.Equal(t, RoundWaiting, state.players[1].active)
	assert.Equal(t, RoundWaiting, state.players[2].active)
	assert.Equal(t, StatusPlay, state.status)
	assert.Equal(t, &state.players[0], state.GetActivePlayer())

	t.Log(state.players[0].allCards)
	t.Log(state.players[1].allCards)
	t.Log(state.players[2].allCards)
	// Verify dealing: ensure no duplicate card
	cards := make(map[uint32]bool)
	var i, j int
	var order uint32
	for i = 0; i < 3; i++ {
		for j = 0; j < 11; j++ {
			order = state.players[i].allCards[j].displayOrder
			if cards[order] {
				t.Errorf("Duplicate card %+v", state.players[i].allCards[j])
			}
			cards[order] = true
		}
	}
}

func TestCardString(t *testing.T) {
	assert.Equal(t, "8(-1)", fmt.Sprintf("%v", Card{
		suit: SuitXiang,
		rank: 8,
	}))
	assert.Equal(t, "10(x2)", fmt.Sprintf("%v", Card{
		displayOrder: 3,
		suit:         SuitOther,
		rank:         10,
	}))
	assert.Equal(t, "1(1),played=1", Card{
		suit:  SuitBaozi,
		rank:  1,
		flags: 1,
	}.String())
	assert.Equal(t, "[2(0) 3(0)]", fmt.Sprint([]Card{
		{0, SuitYoutiao, 2, 128},
		{0, SuitYoutiao, 3, 128},
	}))
}

func TestPickLargestPlayable(t *testing.T) {
	playable := func(rank int32, suit Suit) Card {
		return Card{displayOrder: 1, suit: suit, rank: rank, flags: 0x80}
	}
	unplayable := func(rank int32, suit Suit) Card {
		return Card{displayOrder: 1, suit: suit, rank: rank}
	}

	t.Run("picks highest rank among playable", func(t *testing.T) {
		hand := []Card{
			playable(3, SuitBaozi),
			unplayable(9, SuitBaozi),
			playable(7, SuitYoutiao),
			playable(5, SuitXiang),
		}
		assert.Equal(t, 2, PickLargestPlayable(hand))
	})

	t.Run("tie breaks to first in hand", func(t *testing.T) {
		hand := []Card{
			playable(10, SuitBaozi),
			playable(10, SuitOther),
		}
		assert.Equal(t, 0, PickLargestPlayable(hand))
	})

	t.Run("no playable returns -1", func(t *testing.T) {
		hand := []Card{unplayable(9, SuitBaozi), unplayable(3, SuitXiang)}
		assert.Equal(t, -1, PickLargestPlayable(hand))
	})

	t.Run("empty hand returns -1", func(t *testing.T) {
		assert.Equal(t, -1, PickLargestPlayable(nil))
	})
}

func TestGameStartWithFirstPlayer(t *testing.T) {
	for _, leader := range []int32{0, 1, 2} {
		state, err := NewGame(GameConfig{
			NumPlayers:     3,
			FirstPlayerIdx: leader,
		}, nil)
		assert.NoError(t, err)

		err = state.Start()
		assert.NoError(t, err)
		assert.Equal(t, StatusPlay, state.status)
		for i := int32(0); i < 3; i++ {
			want := RoundWaiting
			if i == leader {
				want = RoundActive
			}
			assert.Equalf(t, want, state.players[i].active, "player %d active state", i)
		}
		assert.Equal(t, &state.players[leader], state.GetActivePlayer())
	}
}

func TestGameStartFirstPlayerOutOfRangeWraps(t *testing.T) {
	// Invalid first_player_idx must wrap mod np instead of panicking
	for _, leader := range []int32{-1, 3, 4, 100} {
		state, err := NewGame(GameConfig{
			NumPlayers:     3,
			FirstPlayerIdx: leader,
		}, nil)
		assert.NoError(t, err)

		err = state.Start()
		assert.NoError(t, err)
		assert.Equal(t, StatusPlay, state.status)
		// Active player must be one of the three seats
		ap := state.GetActivePlayer()
		assert.NotNil(t, ap)
		assert.GreaterOrEqual(t, ap.index, int32(1))
		assert.LessOrEqual(t, ap.index, int32(3))
	}
}

func TestSetConfig(t *testing.T) {
	t.Run("before start", func(t *testing.T) {
		state, err := NewGame(GameConfig{
			NumPlayers: 3,
		}, nil)
		assert.NoError(t, err)

		newCfg := GameConfig{
			NumPlayers:     4,
			FirstPlayerIdx: 2,
		}
		err = state.SetConfig(newCfg)
		assert.NoError(t, err)
		assert.Equal(t, newCfg.NumPlayers, state.GetConfig().NumPlayers)
		assert.Equal(t, newCfg.FirstPlayerIdx, state.GetConfig().FirstPlayerIdx)
		assert.Equal(t, int32(4), state.players[3].index)

		err = state.Start()
		assert.NoError(t, err)
		assert.Equal(t, StatusPlay, state.GetStatus())
		assert.Equal(t, int32(3), state.GetActivePlayer().index)
	})

	t.Run("during play rejected", func(t *testing.T) {
		state, err := NewGame(GameConfig{
			NumPlayers: 3,
		}, nil)
		assert.NoError(t, err)
		err = state.Start()
		assert.NoError(t, err)
		assert.Equal(t, StatusPlay, state.GetStatus())

		err = state.SetConfig(GameConfig{NumPlayers: 4})
		assert.Equal(t, ErrGameInProgress, err)
		assert.Equal(t, int32(3), state.GetConfig().NumPlayers)
	})

	t.Run("invalid player count", func(t *testing.T) {
		state, err := NewGame(GameConfig{
			NumPlayers: 3,
		}, nil)
		assert.NoError(t, err)

		assert.Equal(t, ErrBadNPlayers, state.SetConfig(GameConfig{NumPlayers: 2}))
		assert.Equal(t, ErrBadNPlayers, state.SetConfig(GameConfig{NumPlayers: 5}))
		assert.Equal(t, int32(3), state.GetConfig().NumPlayers)
	})

	t.Run("between finished games", func(t *testing.T) {
		state, err := NewGame(GameConfig{
			NumPlayers: 3,
		}, nil)
		assert.NoError(t, err)
		err = state.Start()
		assert.NoError(t, err)

		// Play out the game to completion
		for state.GetStatus() == StatusPlay {
			p := state.GetActivePlayer()
			assert.NotNil(t, p)
			idx := PickLargestPlayable(p.GetHandCards())
			assert.GreaterOrEqual(t, idx, 0)
			err = state.Play(p.GetHandCards()[idx])
			assert.NoError(t, err)
		}
		assert.Equal(t, StatusFinish, state.GetStatus())

		// Reconfigure to 4 players for next game
		newCfg := GameConfig{
			NumPlayers:     4,
			FirstPlayerIdx: 1,
		}
		err = state.SetConfig(newCfg)
		assert.NoError(t, err)
		assert.Equal(t, newCfg.NumPlayers, state.GetConfig().NumPlayers)

		// Start second game with new configuration
		err = state.Start()
		assert.NoError(t, err)
		assert.Equal(t, StatusPlay, state.GetStatus())
		assert.Equal(t, int32(2), state.GetActivePlayer().index)
	})
}

