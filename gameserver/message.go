package gameserver

// Message type constants — Client → Server
const MSG_TYPE_START_GAME = "START_GAME"
const MSG_TYPE_PLAY_CARD = "PLAY_CARD"

// Message type constants — Server → Client
const MSG_TYPE_SMS = "SMS"
const MSG_TYPE_EVENT = "EVENT"
const MSG_TYPE_ERROR = "ERROR"
const MSG_TYPE_FATAL = "FATAL"
const MSG_TYPE_NOTICE = "NOTICE"
const MSG_TYPE_ROOM_STATE = "ROOM_STATE"
const MSG_TYPE_GAME_START = "GAME_START"
const MSG_TYPE_YOUR_TURN = "YOUR_TURN"
const MSG_TYPE_MOVE_MADE = "MOVE_MADE"
const MSG_TYPE_ROUND_END = "ROUND_END"
const MSG_TYPE_GAME_OVER = "GAME_OVER"
const MSG_TYPE_PLAYER_JOINED = "PLAYER_JOINED"
const MSG_TYPE_PLAYER_LEFT = "PLAYER_LEFT"

// Message is the universal WebSocket message envelope.
type Message struct {
	MsgType string `json:"type"`
	MsgBody any    `json:"body"`
}

// Legacy body types
type SMSMesssageBody struct {
	Data string `json:"data"`
}

// Client → Server body types
type PlayCardBody struct {
	CardIndex int `json:"card_index"`
}

// Server → Client body types
type CardInfo struct {
	Suit     int32 `json:"suit"`
	Rank     int32 `json:"rank"`
	Playable bool  `json:"playable"`
}

type RoomStateBody struct {
	Players []RoomPlayerInfo `json:"players"`
	HostIdx int32            `json:"host_idx"`
}

type RoomPlayerInfo struct {
	PlayerID string `json:"player_id"`
	Position int32  `json:"position"`
	IsHost   bool   `json:"is_host"`
}

type GameStartBody struct {
	HandCards      []CardInfo `json:"hand_cards"`
	FirstPlayerIdx int32      `json:"first_player_idx"`
	YourPlayerIdx  int32      `json:"your_player_idx"`
}

type YourTurnBody struct {
	PlayableIndices []int      `json:"playable_indices"`
	RoundSeq        int        `json:"round_seq"`
	RoundMoves      []CardInfo `json:"round_moves"`
}

type MoveMadeBody struct {
	PlayerIdx  int32      `json:"player_idx"`
	Card       CardInfo   `json:"card"`
	RoundMoves []CardInfo `json:"round_moves"`
}

type RoundEndBody struct {
	WinnerIdx int32 `json:"winner_idx"`
	Score     int32 `json:"score"`
	IsDoubled bool  `json:"is_doubled"`
}

type GameOverBody struct {
	FinalScores []PlayerScore `json:"final_scores"`
	WinnerIdx   int32         `json:"winner_idx"`
}

type PlayerScore struct {
	PlayerIdx int32 `json:"player_idx"`
	Score     int32 `json:"score"`
}

type PlayerJoinedBody struct {
	PlayerID string `json:"player_id"`
	Position int32  `json:"position"`
}

type PlayerLeftBody struct {
	PlayerID string `json:"player_id"`
	Position int32  `json:"position"`
}

type ErrorBody struct {
	Message string `json:"message"`
}
