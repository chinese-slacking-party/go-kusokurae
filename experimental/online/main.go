package main

import (
	"net/http/pprof"
	_ "net/http/pprof"

	"github.com/gin-gonic/gin"

	"github.com/bs-iron-trio/go-kusokurae/gameserver"
)

func main() {
	gameserver.InitRoomRepository()
	r := gin.Default()
	if gin.Mode() != "release" {
		r.GET("/debug/pprof/*any", gin.WrapF(pprof.Index))
	}
	r.POST("/api/v1/room/new", CreateRoom)
	r.POST("/api/v1/room/join", JoinRoom)
	r.GET("/api/v1/communication/:room_id/:player_id", handleWebSocket)
	r.Run()
}
