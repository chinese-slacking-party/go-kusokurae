package main

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Room struct {
	ID               string
	LasestActiveTime int64
}

var roomRepository map[string]*Room

func InitRoomRepository() {
	roomRepository = make(map[string]*Room)
}

// 创建游戏房间
func CreateRoom(ctx *gin.Context) {
	u, err := uuid.NewRandom()
	if err != nil {
		ctx.JSON(200, NewErrorRes(COMMON_ERR_CODE, err.Error()))
		return
	}
	var roomID string = u.String()
	// TODO 创建房间并仓库
	ctx.JSON(200, NewSuccessRes(roomID))
}

// 加入游戏房间并生成userToken
func JoinRoom(ctx *gin.Context) {

}

// 清除不活动的游戏房间
func CleanUnActiveRoom() {
}
