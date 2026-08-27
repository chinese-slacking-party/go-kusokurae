package main

import (
	"log"
	"net/http/pprof"
	_ "net/http/pprof"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/bs-iron-trio/go-kusokurae/config"
	"github.com/bs-iron-trio/go-kusokurae/gameserver"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	gameserver.Configure(
		time.Duration(cfg.TurnTimeoutSeconds)*time.Second,
		time.Duration(cfg.TurnSyncIntervalSeconds)*time.Second,
		cfg.MinTurnTimeoutSec, cfg.MaxTurnTimeoutSec,
		cfg.MinTurnSyncIntervalSec, cfg.MaxTurnSyncIntervalSec,
	)

	gameserver.InitRoomRepository()
	r := gin.Default()
	if gin.Mode() != "release" {
		r.GET("/debug/pprof/*any", gin.WrapF(pprof.Index))
	}
	r.POST("/api/v1/room/new", CreateRoom)
	r.POST("/api/v1/room/join", JoinRoom)
	r.GET("/api/v1/communication/:room_id/:player_id", handleWebSocket)
	r.Run(cfg.Address())
}
