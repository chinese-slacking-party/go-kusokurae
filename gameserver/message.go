package gameserver

// Message type constants — Client → Server
const MSG_TYPE_START_GAME = "START_GAME"
const MSG_TYPE_PLAY_CARD = "PLAY_CARD"

// Message type constants — Server → Client
const MSG_TYPE_SMS = "SMS"
const MSG_TYPE_EVENT = "EVENT"
const MSG_TYPE_ERROR = "ERROR"
const MSG_TYPE_GAME_FATAL = "GAME_FATAL"
const MSG_TYPE_NOTICE = "NOTICE"
const MSG_TYPE_ROOM_STATE = "ROOM_STATE"
const MSG_TYPE_GAME_START = "GAME_START"
const MSG_TYPE_YOUR_TURN = "YOUR_TURN"
const MSG_TYPE_MOVE_MADE = "MOVE_MADE"
const MSG_TYPE_ROUND_END = "ROUND_END"
const MSG_TYPE_GAME_OVER = "GAME_OVER"
const MSG_TYPE_TURN_TIME_SYNC = "TURN_TIME_SYNC"
const MSG_TYPE_RESYNC_STATE = "RESYNC_STATE"
const MSG_TYPE_GAME_RESYNC = "GAME_RESYNC"
const MSG_TYPE_LEAVE_ROOM = "LEAVE_ROOM"
const MSG_TYPE_ROOM_CLOSED = "ROOM_CLOSED"
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
	Nickname string `json:"nickname"`
	Position int32  `json:"position"`
	IsHost   bool   `json:"is_host"`
}

type GameStartBody struct {
	HandCards      []CardInfo `json:"hand_cards"`
	FirstPlayerIdx int32      `json:"first_player_idx"`
	YourPlayerIdx  int32      `json:"your_player_idx"`
}

type YourTurnBody struct {
	PlayableIndices  []int      `json:"playable_indices"`
	RoundSeq         int        `json:"round_seq"`
	RoundMoves       []CardInfo `json:"round_moves"`
	TimeoutSeconds   int        `json:"timeout_seconds"`
	RemainingSeconds int        `json:"remaining_seconds"`
}

type TurnTimeSyncBody struct {
	RemainingSeconds int `json:"remaining_seconds"`
	RoundSeq         int `json:"round_seq"`
}

type MoveMadeBody struct {
	PlayerIdx  int32      `json:"player_idx"`
	CardIdx    int32      `json:"card_idx"`
	Card       CardInfo   `json:"card"`
	RoundMoves []CardInfo `json:"round_moves"`
	AutoPlay   bool       `json:"auto_play"`
}

type RoundEndBody struct {
	WinnerIdx int32 `json:"winner_idx"`
	Score     int32 `json:"score"`
	IsDoubled bool  `json:"is_doubled"`
}

type GameOverBody struct {
	FinalScores []PlayerScore `json:"final_scores"`

	// WinnerIdx is the single winner: the lowest seat among those tied for the
	// highest score. Seat 1 is the human player in a game against bots, so this
	// matches how Windows Hearts resolves a tie.
	WinnerIdx int32 `json:"winner_idx"`

	// WinnerIdxs holds every player tied for the highest score, in seat order.
	// The rules do not define a tie-break, so the full set is reported here and
	// WinnerIdx is left as the convenience answer. Never empty.
	WinnerIdxs []int32 `json:"winner_idxs"`
}

type PlayerScore struct {
	PlayerIdx int32 `json:"player_idx"`
	Score     int32 `json:"score"`
}

// ResyncStateBody is the server→client reply to a RESYNC_STATE request.
// Game is nested inside Room and is null when no game is in progress.
type ResyncStateBody struct {
	Room RoomResyncBody `json:"room"`
}

type RoomResyncBody struct {
	Players []RoomPlayerInfo `json:"players"`
	HostIdx int32            `json:"host_idx"`
	Game    *GameResyncBody  `json:"game"`
}

// GameResyncBody carries the game-layer snapshot for the requesting player.
type GameResyncBody struct {
	Status           int32         `json:"status"` // sm.GameStatus
	HandCards        []CardInfo    `json:"hand_cards"`
	RoundSeq         int           `json:"round_seq"`
	RoundMoves       []CardInfo    `json:"round_moves"`
	Scores           []PlayerScore `json:"scores"`
	ActivePlayerIdx  int32         `json:"active_player_idx"`
	PlayableIndices  []int         `json:"playable_indices"`
	RemainingSeconds int           `json:"remaining_seconds"`
}

type PlayerJoinedBody struct {
	PlayerID string `json:"player_id"`
	Nickname string `json:"nickname"`
	Position int32  `json:"position"`
}

type PlayerLeftBody struct {
	PlayerID string `json:"player_id"`
	Nickname string `json:"nickname"`
	Position int32  `json:"position"`
}

type ErrorBody struct {
	Message string `json:"message"`
}

type GameFatalBody struct {
	Message string `json:"message"`
}

type RoomClosedBody struct {
	Reason string `json:"reason"`
}
