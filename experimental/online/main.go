package main

import (
	"github.com/gin-gonic/gin"

	"github.com/bs-iron-trio/go-kusokurae/gameserver"
)

func main() {
	gameserver.InitRoomRepository()
	r := gin.Default()
	r.POST("/api/v1/room/new", CreateRoom)
	r.POST("/api/v1/room/join", JoinRoom)
	r.GET("/api/v1/communication/:room_id/:player_id", handleWebSocket)
	r.Run()
}
