# 多人房间对战功能设计

## 概述

实现完整的在线多人游戏循环，连接 `sm` 游戏引擎与 `gameserver` 的 Player channel 架构。玩家创建/加入房间 → 房主开始游戏 → 轮流出牌 → 结算。

## 文件结构

所有改动在现有文件中完成，不新增文件：

```
gameserver/
├── player.go         # 扩展：Disconnected chan 等
├── room.go           # 扩展：房主管理、开始游戏触发、RoomFn
├── game.go           # 核心重写：GameFn 游戏主循环
├── gateway.go        # 改造：断线检测不再 panic，支持重连时替换 Session
└── message.go        # 大幅扩展：游戏消息类型和 body struct

experimental/online/
├── main.go           # 路由注册（不变）
├── basic.go          # 不变
└── controller.go     # 适配 CreateRoom 返回值 + handleWebSocket 重连逻辑
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

WebSocket 升级。支持重连：若 PlayerID 对应的 Player 已存在且已有 Session，关闭旧连接、创建新 Session 绑定同一 Player。

## Player 与 Session 的关系

**Player 是持久的逻辑实体，Session 是临时连接。** Player 只有一份，Session 随 WebSocket 连接更替。

```
Player (持久)
├── ID, RoomID, RoomPosition (身份)
├── NoticeCh, OperatorCh (持久 channel，重连不重建)
├── Disconnected chan struct{} (断线信号，重连时替换新 chan)
└── Session *Session (当前活跃连接，可为 nil)

Session (临时)
├── Conn *websocket.Conn
└── Player *Player (反向引用)
```

## 断线检测

`Session.Input` 中 `ReadJSON` 返回 error 时即判定断线：

```go
func (s *Session) Input(ctx context.Context) {
    defer close(s.inputStreamClosed)

    for {
        if err := s.Conn.ReadJSON(&msg); err != nil {
            close(s.Player.Disconnected) // 通知断线
            s.Player.Session = nil       // 解除关联
            return // 正常退出，不再 panic
        }
        s.Player.OperatorCh <- msg
    }
}
```

## 重连机制

PlayerID 是重连凭据。客户端记住自己的 PlayerID，重连时使用同一 URL：

`GET /api/v1/communication/:room_id/:player_id`

handleWebSocket 处理流程：

1. `room.FindPlayerByID(playerID)` 找到已有 Player
2. 若 `player.Session != nil`：关闭旧 Session 的 Conn，旧 Session 的 Input/Output goroutine 自然退出
3. `player.Disconnected = make(chan struct{})` — 创建新断线 chan
4. `NewSession(conn, player)` — 创建新 Session 绑定同一 Player
5. 启动新 Session 的 Input/Output/SessionControl goroutines
6. 如果游戏进行中，补发一次 `YOUR_TURN` 或 `ROOM_STATE` 让重连玩家恢复状态

## 托管状态

断线时 `close(s.Player.Disconnected)`，GameFn 中对应的 select case 立即触发（已关闭的 chan 永远返回零值），进入托管：

- 轮到断线活跃玩家出牌时：从手牌中随机选一张合法牌自动 `Play()`
- 未轮到该玩家时：不做任何事，等到轮到他时再托管
- 重连后 `Disconnected` 被替换为新 chan，Case 不再触发，托管自动结束

## 消息协议

复用现有 `Message{MsgType, MsgBody}` JSON 格式，新增类型。

### Client → Server

| MsgType | Body | 说明 |
|---|---|---|
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
| `ERROR` | `{Message: string}` | 错误响应（如非自己回合出牌） |

## GameFn 游戏主循环

单 goroutine 事件循环，利用已有的 `PlayerReaderChs[i]`（`game.go:16`）作为中间 bridge channel。

### 启动时的 relay goroutine

每个玩家启动一个 relay goroutine，将其 `OperatorCh` 的消息转发到 `PlayerReaderChs[i]`：

```go
for i, p := range players {
    go func(idx int, player *Player) {
        for msg := range player.OperatorCh {
            g.PlayerReaderChs[idx] <- msg
        }
    }(i, p)
}
```

### 游戏主循环

```go
func (g *Game) GameFn(ctx context.Context) {
    // 1. 创建 sm.GameState, Start() 发牌
    // 2. 广播 GAME_START 给所有玩家

    for g.State.GetStatus() == sm.StatusPlay {
        activePlayer := g.State.GetActivePlayer()
        activeIdx := activePlayer.GetIndex() - 1 // 转为 0-based
        sendYOUR_TURN(activeIdx)

        select {
        case msg := <-g.PlayerReaderChs[0]:
            if !g.isActivePlayer(0) { sendError(0, "not your turn"); continue }
            g.handlePlay(0, msg)
        case msg := <-g.PlayerReaderChs[1]:
            if !g.isActivePlayer(1) { sendError(1, "not your turn"); continue }
            g.handlePlay(1, msg)
        case msg := <-g.PlayerReaderChs[2]:
            if !g.isActivePlayer(2) { sendError(2, "not your turn"); continue }
            g.handlePlay(2, msg)
        case msg := <-g.PlayerReaderChs[3]:
            if !g.isActivePlayer(3) { sendError(3, "not your turn"); continue }
            g.handlePlay(3, msg)

        case <-g.DisconnectedCh(0): // 玩家 0 断线
            if g.isActivePlayer(0) { g.autoPlay(0) }
        case <-g.DisconnectedCh(1):
            if g.isActivePlayer(1) { g.autoPlay(1) }
        case <-g.DisconnectedCh(2):
            if g.isActivePlayer(2) { g.autoPlay(2) }
        case <-g.DisconnectedCh(3):
            if g.isActivePlayer(3) { g.autoPlay(3) }
        }
    }
}
```

`handlePlay(idx, msg)`:
1. 校验 `msg.MsgType == PLAY_CARD`
2. 从 `players[idx]` 手牌中取出 `msg.CardIndex` 指向的牌
3. 调用 `g.State.Play(card)`，若非法则回 error 给该玩家
4. 广播 `MOVE_MADE` 给所有玩家
5. 若回合结束，广播 `ROUND_END`
6. 若游戏结束，广播 `GAME_OVER`

`autoPlay(idx)`:
1. 从 `players[idx].GetHandCards()` 中筛选 `Playable()` 的牌
2. 随机选一张，调用 `g.State.Play(card)`
3. 广播 `MOVE_MADE`

### 非活跃玩家操作过滤

GameFn 的 select 固定监听所有 `PlayerReaderChs[0..N-1]`。当非活跃玩家发来 `PLAY_CARD` 时，`isActivePlayer(idx)` 检查失败，直接回复 `ERROR{"not your turn"}`，消息被丢弃、不进入 `handlePlay`。

### 最大玩家数 hack

`PlayerReaderChs` 长度为 `config.NumPlayers`（3 或 4）。对于 3 人局，`PlayerReaderChs[3]` 是 nil channel，select 中 nil channel 的 case 永不触发，天然跳过。

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

- `RoomFn(ctx)` — 房间生命周期 goroutine，处理非游戏内消息
- `StartGame()` — 检查人数、创建 `Game`、绑定 channel、启动 relay + `go Game.GameFn(ctx)`
- `Broadcast(msg)` — 向所有有活跃 Session 的玩家广播
- 玩家加入时广播 `PLAYER_JOINED` / `ROOM_STATE`

## Interaction Flow

```
CreateRoom → 返回 (RoomID, PlayerID_房主), 房主升级 WS
JoinRoom   → 返回 (RoomID, PlayerID) → 广播 PLAYER_JOINED
...
房主 WS 发 START_GAME
  → Room.StartGame()
  → NewGame(config)，各 Player.OperatorCh → PlayerReaderChs[i] relay
  → go Game.GameFn(ctx)
  → GameFn 发 GAME_START 给所有玩家
  → 进入出牌循环 (select over PlayerReaderChs[i] + Disconnected[i])
  → GAME_OVER → return

断线:
  → Session.Input ReadJSON err → close(Player.Disconnected)
  → GameFn 中对应 Disconnected case 触发 → autoPlay (如轮到)

重连:
  → 同 URL 再次 upgrade → FindPlayerByID → 关闭旧 Session
  → 创建新 Disconnected chan → NewSession → 启动新 Input/Output
  → 补发当前状态 (YOUR_TURN 或 ROOM_STATE)
```
