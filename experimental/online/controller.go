package main

import (
	"net/http"

	"github.com/bs-iron-trio/go-kusokurae/gameserver"
	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type CommunicationParams struct {
	RoomID   string `uri:"room_id" binding:"required"`
	PlayerID string `uri:"player_id" binding:"required"`
}

func handleWebSocket(c *gin.Context) {
	var params CommunicationParams
	if err := c.BindUri(&params); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	room, err := gameserver.GetRoomByID(params.RoomID)
	if err != nil {
		conn.WriteJSON(&gameserver.Message{
			MsgType: gameserver.MSG_TYPE_ERROR,
			MsgBody: &gameserver.ErrorBody{Message: err.Error()},
		})
		conn.Close()
		return
	}

	player, err := room.FindPlayerByID(params.PlayerID)
	if err != nil {
		conn.WriteJSON(&gameserver.Message{
			MsgType: gameserver.MSG_TYPE_ERROR,
			MsgBody: &gameserver.ErrorBody{Message: err.Error()},
		})
		conn.Close()
		return
	}

	// Handle reconnect: close old session if exists
	if player.Session != nil {
		player.Session.Conn.Close()
		<-player.Session.ClosedCh
	}

	// Replace Disconnected chan for fresh connection
	player.Disconnected = make(chan struct{})

	s := gameserver.NewSession(conn, player)

	// Notify room goroutine to monitor the new Disconnected channel
	room.UpdateDiscChannel(int(player.RoomPosition), player.Disconnected)

	go s.SessionControl(c.Request.Context())
	go s.Input(c.Request.Context())
	go s.Output(c.Request.Context())

	// Send current room roster on connect (ADR-0003)
	s.Player.NoticeCh <- room.BuildRoomStateMessage()

	// If game is in progress, re-sync state to the reconnected player
	if g := room.Game(); g != nil && g.State != nil {
		g.StateMutex.Lock()
		state := g.State
		status := state.GetStatus()
		var reSyncMsg *gameserver.Message
		if status == sm.StatusPlay {
			activePlayer := state.GetActivePlayer()
			if activePlayer != nil && activePlayer.GetIndex()-1 == int(player.RoomPosition) {
				idx := int(player.RoomPosition)
				p := state.GetPlayer(int32(idx))
				handCards := p.GetHandCards()
				playableIndices := make([]int, 0)
				for i, c := range handCards {
					if c.Playable() {
						playableIndices = append(playableIndices, i)
					}
				}
				rs := state.GetRoundState()
				cardInfos := make([]gameserver.CardInfo, len(rs.Moves))
				for i, c := range rs.Moves {
					cardInfos[i] = gameserver.CardInfo{
						Suit: int32(c.GetSuit()), Rank: int32(c.GetRank()),
					}
				}
				reSyncMsg = &gameserver.Message{
					MsgType: gameserver.MSG_TYPE_YOUR_TURN,
					MsgBody: &gameserver.YourTurnBody{
						PlayableIndices: playableIndices,
						RoundSeq:        rs.Seq,
						RoundMoves:      cardInfos,
					},
				}
			}
		}
		g.StateMutex.Unlock()
		if reSyncMsg != nil {
			s.Player.NoticeCh <- *reSyncMsg
		}
	}

	<-s.ClosedCh
}

func CreateRoom(ctx *gin.Context) {
	var req CreateRoomReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}
	if req.NumPlayers != 3 && req.NumPlayers != 4 {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, "Invalid number of players"))
		return
	}
	if err := gameserver.ValidateTurnTimeoutSec(req.TurnTimeoutSeconds); err != nil {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}
	if err := gameserver.ValidateTurnSyncIntervalSec(req.TurnSyncIntervalSeconds, req.TurnTimeoutSeconds); err != nil {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	host, err := gameserver.NewPlayer(req.Nickname)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	u, err := uuid.NewRandom()
	if err != nil {
		ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	room := gameserver.NewRoom(u.String(), host, &sm.GameConfig{NumPlayers: req.NumPlayers})
	room.TurnTimeoutSec = req.TurnTimeoutSeconds
	room.TurnSyncIntervalSec = req.TurnSyncIntervalSeconds

	ctx.JSON(200, NewSuccessRes(&JoinRoomRet{
		RoomID:   room.ID,
		PlayerID: host.ID,
	}))
}

type CreateRoomReq struct {
	NumPlayers              int32  `json:"num_players"`
	Nickname                string `json:"nickname"`
	TurnTimeoutSeconds      int32  `json:"turn_timeout_seconds"`
	TurnSyncIntervalSeconds int32  `json:"turn_sync_interval_seconds"`
}

type JoinRoomRet struct {
	RoomID   string `json:"room_id"`
	PlayerID string `json:"player_id"`
}

func JoinRoom(ctx *gin.Context) {
	roomID := ctx.Query("room_id")
	if len(roomID) == 0 {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, "Invalid room_id"))
		return
	}

	room, err := gameserver.GetRoomByID(roomID)
	if err != nil {
		if err == gameserver.ErrRoomNotFound {
			ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, "room not found"))
		} else {
			ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		}
		return
	}

	player, err := gameserver.NewPlayer(ctx.Query("nickname"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}
	if err := room.AddPlayer(player); err != nil {
		ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	ctx.JSON(200, NewSuccessRes(&JoinRoomRet{
		RoomID:   player.RoomID,
		PlayerID: player.ID,
	}))
}
