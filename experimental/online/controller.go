package main

import (
	"fmt"
	"log"
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
	PlayerId string `uri:"player_id" binding:"required"`
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

	// 处理连接
	for {
		var msg gameserver.Message
		// 读取消息
		if err := conn.ReadJSON(&msg); err != nil {
			log.Println("Warning: 写入消息失败: ", err)
			break
		}

		// 打印接收到的消息
		log.Println("Received:", &msg)

		// 回传消息
		if err := conn.WriteJSON(&msg); err != nil {
			log.Println("Warning: 写入消息失败: ", err)
			break
		}
	}
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
