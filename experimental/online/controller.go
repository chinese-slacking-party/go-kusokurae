package main

import (
	"fmt"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/bs-iron-trio/go-kusokurae/gameserver"
)

// 创建游戏房间
func CreateRoom(ctx *gin.Context) {
	var gameConfig sm.GameConfig
	err := ctx.BindJSON(&gameConfig)
	if gameConfig.NumPlayers != 3 && gameConfig.NumPlayers != 4 {
		ctx.JSON(400, NewErrorRes(COMMON_ERR_CODE, "Invalid number of players"))
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
		ctx.JSON(400, NewErrorRes(COMMON_ERR_CODE, "Invalid roomID"))
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
