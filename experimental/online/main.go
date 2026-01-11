package main

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/bs-iron-trio/go-kusokurae/gameserver"
)

var upgrader = websocket.Upgrader{}

func main() {
	gameserver.InitRoomRepository()
	r := gin.Default()
	r.POST("/api/v1/room/new", CreateRoom)
	r.POST("/api/v1/room/join", JoinRoom)
	r.Run()
}
