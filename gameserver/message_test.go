package gameserver

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlayCardBodyJSON(t *testing.T) {
	body := PlayCardBody{CardIndex: 3}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"card_index":3}`, string(b))
}

func TestGameStartBodyJSON(t *testing.T) {
	body := GameStartBody{
		HandCards: []CardInfo{
			{Suit: 0, Rank: 5, Playable: true},
			{Suit: -1, Rank: 3, Playable: false},
		},
		FirstPlayerIdx: 1,
		YourPlayerIdx:  0,
	}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"hand_cards":[{"suit":0,"rank":5,"playable":true},{"suit":-1,"rank":3,"playable":false}],"first_player_idx":1,"your_player_idx":0}`, string(b))
}

func TestYourTurnBodyJSON(t *testing.T) {
	body := YourTurnBody{
		PlayableIndices: []int{0, 2, 5},
		RoundSeq:        3,
		RoundMoves:      []CardInfo{{Suit: 0, Rank: 4, Playable: false}},
	}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"playable_indices":[0,2,5],"round_seq":3,"round_moves":[{"suit":0,"rank":4,"playable":false}]}`, string(b))
}

func TestRoundEndBodyJSON(t *testing.T) {
	body := RoundEndBody{WinnerIdx: 2, Score: 5}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"winner_idx":2,"score":5,"is_doubled":false}`, string(b))
}

func TestGameOverBodyJSON(t *testing.T) {
	body := GameOverBody{
		FinalScores: []PlayerScore{{PlayerIdx: 1, Score: 15}, {PlayerIdx: 2, Score: -3}},
		WinnerIdx:   1,
	}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"final_scores":[{"player_idx":1,"score":15},{"player_idx":2,"score":-3}],"winner_idx":1}`, string(b))
}

func TestErrorBodyJSON(t *testing.T) {
	msg := Message{MsgType: MSG_TYPE_ERROR, MsgBody: &ErrorBody{Message: "not your turn"}}
	b, err := json.Marshal(msg)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"type":"ERROR","body":{"message":"not your turn"}}`, string(b))
}

func TestRoomStateBodyJSON(t *testing.T) {
	body := RoomStateBody{
		Players: []RoomPlayerInfo{
			{PlayerID: "abc", Position: 1, IsHost: true},
			{PlayerID: "def", Position: 2, IsHost: false},
		},
		HostIdx: 0,
	}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"players":[{"player_id":"abc","position":1,"is_host":true},{"player_id":"def","position":2,"is_host":false}],"host_idx":0}`, string(b))
}

func TestMoveMadeBodyJSON(t *testing.T) {
	body := MoveMadeBody{
		PlayerIdx:  2,
		Card:       CardInfo{Suit: 1, Rank: 8, Playable: false},
		RoundMoves: []CardInfo{{Suit: 1, Rank: 8, Playable: false}, {Suit: 0, Rank: 3, Playable: false}},
	}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"player_idx":2,"card":{"suit":1,"rank":8,"playable":false},"round_moves":[{"suit":1,"rank":8,"playable":false},{"suit":0,"rank":3,"playable":false}]}`, string(b))
}
