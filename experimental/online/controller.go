package main

import (
	"fmt"
	"net/http"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/bs-iron-trio/go-kusokurae/gameserver"
)

var upgrader = websocket.Upgrader{}

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

	// 升级 HTTP 连接到 WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}
	defer conn.Close()

	conn.WriteJSON(&gameserver.Message{
		MsgType: gameserver.MSG_TYPE_SMS,
		MsgBody: &gameserver.SMSMesssageBody{
			Data: "Hello Welcome Join the Room",
		},
	})

	room, err := gameserver.GetRoomByID(params.RoomID)
	if err != nil {
		conn.WriteJSON(&gameserver.Message{
			MsgType: gameserver.MSG_TYPE_FATAL,
			MsgBody: &gameserver.SMSMesssageBody{
				Data: err.Error(),
			},
		})
		return
	}

	player, err := room.FindPlayerByID(params.PlayerID)
	if err != nil {
		conn.WriteJSON(&gameserver.Message{
			MsgType: gameserver.MSG_TYPE_FATAL,
			MsgBody: &gameserver.SMSMesssageBody{
				Data: err.Error(),
			},
		})
		return
	}

	s := gameserver.NewSession(conn, player)

	go s.SessionControl(c.Request.Context())
	go s.Input(c.Request.Context())
	go s.Output(c.Request.Context())

	<-s.ClosedCh

}

// 创建游戏房间
func CreateRoom(ctx *gin.Context) {
	var gameConfig sm.GameConfig
	err := ctx.BindJSON(&gameConfig)
	if gameConfig.NumPlayers != 3 && gameConfig.NumPlayers != 4 {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, "Invalid number of players"))
		return
	}

	u, err := uuid.NewRandom()
	if err != nil {
		ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}

	var room = gameserver.NewRoom(u.String(), &gameConfig)
	ctx.JSON(200, NewSuccessRes(room.ID))
}

type JoinRoomRet struct {
	RoomID   string `json:"roomID"`
	PlayerID string `json:"playerID"`
}

func JoinRoom(ctx *gin.Context) {
	var roomID = ctx.Query("roomID")
	if len(roomID) == 0 {
		ctx.JSON(http.StatusBadRequest, NewErrorRes(COMMON_ERR_CODE, "Invalid roomID"))
		return
	}
	var player = gameserver.NewPlayer()
	room, err := gameserver.GetRoomByID(roomID)
	if err != nil {
		if err == gameserver.ErrRoomNotFound {
			ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, fmt.Sprintf("room %s 未找到", roomID)))
		} else {
			ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		}
		return
	}

	room.AddPlayer(player)
	ctx.JSON(200, NewSuccessRes(&JoinRoomRet{
		RoomID:   player.RoomID,
		PlayerID: player.ID,
	}))

}
