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

	// Attach the new session first (room always sees the player connected),
	// then shut down the previous session and wait for it to fully stop.
	s := gameserver.NewSession(conn, player)
	if old := room.AttachSession(player.RoomPosition, s); old != nil {
		old.Close()
		<-old.ClosedCh
	}

	go s.SessionControl()
	go s.Input()
	go s.Output()

	// Client pulls state by sending RESYNC_STATE; the server pushes nothing.

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
