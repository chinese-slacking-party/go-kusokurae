package main

import (
	"log"
	"time"

	"github.com/gin-contrib/pprof"
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
	if gin.Mode() != gin.ReleaseMode {
		// 逐条注册，不能用 /debug/pprof/*any 兜底：gin 的路由树不允许通配符
		// 与同前缀下的具体路径共存，两者并存会在启动时 panic。
		pprof.Register(r)
	}
	r.POST("/api/v1/room/new", CreateRoom)
	r.POST("/api/v1/room/join", JoinRoom)
	r.GET("/api/v1/communication/:room_id/:player_id", handleWebSocket)
	r.Run(cfg.Address())
}
