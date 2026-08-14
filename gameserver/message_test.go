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
		PlayableIndices:  []int{0, 2, 5},
		RoundSeq:         3,
		RoundMoves:       []CardInfo{{Suit: 0, Rank: 4, Playable: false}},
		TimeoutSeconds:   30,
		RemainingSeconds: 28,
	}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"playable_indices":[0,2,5],"round_seq":3,"round_moves":[{"suit":0,"rank":4,"playable":false}],"timeout_seconds":30,"remaining_seconds":28}`, string(b))
}

func TestTurnTimeSyncBodyJSON(t *testing.T) {
	body := TurnTimeSyncBody{RemainingSeconds: 8, RoundSeq: 3}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"remaining_seconds":8,"round_seq":3}`, string(b))
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
			{PlayerID: "abc", Nickname: "房主", Position: 1, IsHost: true},
			{PlayerID: "def", Nickname: "Alice", Position: 2, IsHost: false},
		},
		HostIdx: 0,
	}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"players":[{"player_id":"abc","nickname":"房主","position":1,"is_host":true},{"player_id":"def","nickname":"Alice","position":2,"is_host":false}],"host_idx":0}`, string(b))
}

func TestPlayerJoinedBodyJSON(t *testing.T) {
	body := PlayerJoinedBody{PlayerID: "abc", Nickname: "小明", Position: 2}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"player_id":"abc","nickname":"小明","position":2}`, string(b))
}

func TestGameFatalBodyJSON(t *testing.T) {
	msg := Message{MsgType: MSG_TYPE_GAME_FATAL, MsgBody: &GameFatalBody{Message: "boom"}}
	b, err := json.Marshal(msg)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"type":"GAME_FATAL","body":{"message":"boom"}}`, string(b))
}

func TestMoveMadeBodyJSON(t *testing.T) {
	body := MoveMadeBody{
		PlayerIdx:  2,
		CardIdx:    1,
		Card:       CardInfo{Suit: 1, Rank: 8, Playable: false},
		RoundMoves: []CardInfo{{Suit: 1, Rank: 8, Playable: false}, {Suit: 0, Rank: 3, Playable: false}},
		AutoPlay:   true,
	}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"player_idx":2,"card_idx":1,"card":{"suit":1,"rank":8,"playable":false},"round_moves":[{"suit":1,"rank":8,"playable":false},{"suit":0,"rank":3,"playable":false}],"auto_play":true}`, string(b))
}

func TestResyncStateBodyJSON_NoGame(t *testing.T) {
	body := ResyncStateBody{
		Room: RoomResyncBody{
			Players: []RoomPlayerInfo{{PlayerID: "abc", Nickname: "小明", Position: 0, IsHost: true}},
			HostIdx: 0,
			Game:    nil,
		},
	}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"room":{"players":[{"player_id":"abc","nickname":"小明","position":0,"is_host":true}],"host_idx":0,"game":null}}`, string(b))
}

func TestResyncStateBodyJSON_WithGame(t *testing.T) {
	body := ResyncStateBody{
		Room: RoomResyncBody{
			Players: []RoomPlayerInfo{{PlayerID: "abc", Nickname: "小明", Position: 0, IsHost: true}},
			HostIdx: 0,
			Game: &GameResyncBody{
				Status:           2,
				HandCards:        []CardInfo{{Suit: 0, Rank: 5, Playable: true}},
				RoundSeq:         1,
				RoundMoves:       []CardInfo{{Suit: 1, Rank: 8, Playable: false}},
				Scores:           []PlayerScore{{PlayerIdx: 0, Score: 2}, {PlayerIdx: 1, Score: 0}},
				ActivePlayerIdx:  0,
				PlayableIndices:  []int{0, 1},
				RemainingSeconds: 28,
			},
		},
	}
	b, err := json.Marshal(body)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"room":{"players":[{"player_id":"abc","nickname":"小明","position":0,"is_host":true}],"host_idx":0,"game":{"status":2,"hand_cards":[{"suit":0,"rank":5,"playable":true}],"round_seq":1,"round_moves":[{"suit":1,"rank":8,"playable":false}],"scores":[{"player_idx":0,"score":2},{"player_idx":1,"score":0}],"active_player_idx":0,"playable_indices":[0,1],"remaining_seconds":28}}}`, string(b))
}

func TestRoomClosedBodyJSON(t *testing.T) {
	msg := Message{MsgType: MSG_TYPE_ROOM_CLOSED, MsgBody: &RoomClosedBody{Reason: "host_left"}}
	b, err := json.Marshal(msg)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"type":"ROOM_CLOSED","body":{"reason":"host_left"}}`, string(b))
}
