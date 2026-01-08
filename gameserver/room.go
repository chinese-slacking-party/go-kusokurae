package main

import (
	"errors"
	"sync"

	"github.com/bs-iron-trio/go-kusokurae/sm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var ErrRoomFull = errors.New("room is full!!!")

type Room struct {
	ID                string
	Mutex             sync.Mutex // protects the room state
	LasestActiveTime  int64
	RoomGameConfig    *sm.GameConfig
	CurrentGame       *Game
	CurrentPlayers    int32
	Players           []*Player
	PlayerReadyStatus []*int32
}

func NewRoom(id string, config *sm.GameConfig) *Room {
	return &Room{
		ID:                id,
		RoomGameConfig:    config,
		CurrentPlayers:    0,
		Players:           make([]*Player, config.NumPlayers),
		PlayerReadyStatus: make([]*int32, config.NumPlayers),
	}
}

func (r *Room) AddPlayer(player *Player) error {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	if r.CurrentPlayers >= r.RoomGameConfig.NumPlayers {
		return ErrRoomFull
	}
	r.Players[r.CurrentPlayers] = player
	r.CurrentPlayers++
	player.Sit(r.ID, r.CurrentPlayers-1)
	return nil
}

var roomRepository map[string]*Room

func InitRoomRepository() {
	roomRepository = make(map[string]*Room)
}

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

	var room = NewRoom(u.String(), &gameConfig)
	var masterPlayer = NewPlayer()
	room.AddPlayer(masterPlayer)
	roomRepository[room.ID] = room
	ctx.JSON(200, NewSuccessRes(room.ID))
}

// 加入游戏房间并生成userToken
func JoinRoom(ctx *gin.Context) {

}

// 清除不活动的游戏房间
func CleanUnActiveRoom() {
}
