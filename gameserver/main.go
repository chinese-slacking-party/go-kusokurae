package main

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func main() {
	InitRoomRepository()
	r := gin.Default()
	r.POST("/api/v1/room/new", CreateRoom)
	r.Run()
}
