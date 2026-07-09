# 多人房间对战功能设计

## 概述

实现完整的在线多人游戏循环，连接 `sm` 游戏引擎与 `gameserver` 的 Player channel 架构。玩家创建/加入房间 → 房主开始游戏 → 轮流出牌 → 结算。

## 文件结构

所有改动在现有文件中完成，不新增文件：

```
gameserver/
├── player.go         # 扩展：ready 状态等
├── room.go           # 扩展：房主管理、开始游戏触发、RoomFn
├── game.go           # 核心重写：GameFn 游戏主循环
├── gateway.go        # 基本不变
└── message.go        # 大幅扩展：游戏消息类型和 body struct

experimental/online/
├── main.go           # 路由注册（不变）
├── basic.go          # 不变
└── controller.go     # 适配新的 CreateRoom 返回值
```

## 房主机制

- 第一个加入房间的玩家自动成为房主（`Room.HostPlayerIdx`）
- `CreateRoom` API 同时创建房间和房主玩家，返回 `{RoomID, PlayerID}`
- 房主通过 WebSocket 发送 `START_GAME` 开始游戏
- 不需要准备机制，人数够（达到 `NumPlayers`）房主即可开始

## API

### POST /api/v1/room/new

Body: `{NumPlayers: int}`

Response: `{RoomID: string, PlayerID: string}` — 创建房间 + 房主玩家

### POST /api/v1/room/join

Query: `roomID=xxx`

Response: `{RoomID: string, PlayerID: string}` — 不变

### GET /api/v1/communication/:room_id/:player_id

WebSocket 升级，逻辑不变。

## 消息协议

复用现有 `Message{MsgType, MsgBody}` JSON 格式，新增类型。

### Client → Server

| MsgType | Body | 说明 |
|---|---|---|
| `READY` | — | 玩家准备完毕 |
| `START_GAME` | — | 房主开始游戏 |
| `PLAY_CARD` | `{CardIndex: int}` | 从手牌选出 index 指向的牌打出 |

### Server → Client

| MsgType | Body | 说明 |
|---|---|---|
| `ROOM_STATE` | `{Players, HostIdx}` | 玩家加入时广播房间状态 |
| `GAME_START` | `{HandCards, FirstPlayerIdx}` | 发牌结果，附先手玩家 |
| `YOUR_TURN` | `{PlayableIndices, RoundInfo}` | 轮到你，附可选牌索引和当前回合信息 |
| `MOVE_MADE` | `{PlayerIdx, Card, RoundMoves}` | 有人出牌后广播 |
| `ROUND_END` | `{WinnerIdx, Score, CardsTaken}` | 回合结束 |
| `GAME_OVER` | `{FinalScores, WinnerIdx}` | 游戏结束 |
| `PLAYER_JOINED` | `{PlayerID, Position}` | 新玩家加入 |
| `PLAYER_LEFT` | `{PlayerID, Position}` | 玩家离开 |

## GameFn 游戏主循环

单 goroutine 事件循环：

```
func (g *Game) GameFn(ctx context.Context) {
    // 1. 创建 sm.GameState, Start() 发牌
    // 2. 广播 GAME_START 给所有玩家（各发手牌 + 先手信息）

    for 游戏未结束 {
        activePlayer := g.State.GetActivePlayer()
        发送 YOUR_TURN

        select {
        case msg := <-activePlayer.OperatorCh:
            // 出牌操作
            card := activePlayer.GetHandCards()[msg.CardIndex]
            g.State.Play(card)
            广播 MOVE_MADE

            if 回合结束 {
                广播 ROUND_END
                if 游戏结束 {
                    广播 GAME_OVER
                    return
                }
            }

        case <-activePlayer.disconnected:
            // 自动托管：随机选一张可出牌 Play()
        }
    }
}
```

## 断线托管与重连

- `Session` 断开时向 `Player.disconnected` channel 发信号
- 断线玩家正好是活跃玩家时，`GameFn` 从其手牌中随机选合法牌自动打出
- 非活跃玩家断线不影响当前回合，仅在轮到其时托管
- 重连：新 WebSocket 替换旧 `Session`，共用同一组 `NoticeCh`/`OperatorCh`，重连时补发一次当前游戏状态

## Room 职责

```go
type Room struct {
    ID             string
    HostPlayerIdx  int32
    GameConfig     *sm.GameConfig
    Game           *Game
    Players        []*Player
    // ...
}
```

- `RoomFn(ctx)` — 房间生命周期 goroutine，处理非游戏内的房间级 WebSocket 消息（READY、START_GAME 等）
- `StartGame()` — 检查人数（已达到 NumPlayers）、创建 `Game` 实例、将各 Player channel 绑定到 Game、`go Game.GameFn(ctx)`
- `Broadcast(msg)` — 向所有玩家广播
- 玩家加入时广播 `PLAYER_JOINED` / `ROOM_STATE`

## Interaction Flow

```
CreateRoom → 返回 (RoomID, PlayerID_房主)
JoinRoom   → 返回 (RoomID, PlayerID) → 广播 PLAYER_JOINED
...
房主 WS 发 START_GAME
  → Room.StartGame()
  → NewGame(config)，绑定各 Player channel
  → go Game.GameFn(ctx)
  → GameFn 发 GAME_START 给所有玩家
  → 进入出牌循环
  → GAME_OVER → return
```
