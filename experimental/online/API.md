# Kusokurae 在线玩法 API 文档

## 通用约定

### 基础地址

```
http://localhost:8080
```

### HTTP 响应格式

所有 HTTP 接口统一返回 JSON：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| code | int | 0=成功，非0=失败 |
| message | string | 状态描述 |
| data | object/null | 业务数据 |

### WebSocket 消息格式

所有 WebSocket 消息使用统一的信封结构：

```json
{
  "type": "消息类型",
  "body": {}
}
```

---

## 一、HTTP 接口

### 1.1 创建房间

```
POST /api/v1/room/new
```

**请求体：**

```json
{
  "numPlayers": 3
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| numPlayers | int | 是 | 玩家人数，仅支持 3 或 4 |

**成功响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "roomID": "550e8400-e29b-41d4-a716-446655440000",
    "playerID": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| roomID | string | 房间 ID（UUID），分享给其他玩家加入 |
| playerID | string | 房主玩家 ID（UUID），用于 WebSocket 连接 |

房主即创建者，自动入座 position 0，拥有启动游戏权限。

---

### 1.2 加入房间

```
POST /api/v1/room/join?roomID={roomID}
```

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| roomID | string | 是 | 要加入的房间 ID |

**成功响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "roomID": "550e8400-e29b-41d4-a716-446655440000",
    "playerID": "bf3d5e7a-1234-5678-9abc-def012345678"
  }
}
```

**错误响应：**

| code | message | 场景 |
|------|---------|------|
| 1 | room not found | roomID 对应的房间不存在 |
| 1 | room is full!!! | 房间已满 |
| 1 | game already started | 游戏已开始，无法加入 |

---

## 二、WebSocket 实时通信

### 连接

```
GET ws://localhost:8080/api/v1/communication/{roomID}/{playerID}
```

连接成功后，客户端与服务端通过 JSON 消息双向通信。

**断线重连：** 使用相同的 URL 重新建立 WebSocket 连接即可。服务端会关闭旧连接、复用已有 Player 状态。如果游戏进行中且轮到该玩家，会立即收到 `YOUR_TURN` 消息恢复状态。

---

## 三、客户端 → 服务端消息

### 3.1 开始游戏

仅房主可发送，房间满员后有效。

```json
{
  "type": "START_GAME"
}
```

服务器收到后开始发牌并广播 `GAME_START`。

**可能错误：**
- `game already started` — 游戏已经开始
- `not enough players` — 人数未满
- `only host can start game` — 非房主发送

---

### 3.2 出牌

轮到自己的回合时发送。

```json
{
  "type": "PLAY_CARD",
  "body": {
    "cardIndex": 2
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| cardIndex | int | 手牌索引，对应 `YOUR_TURN` 中 `playableIndices` 之一 |

**可能错误：**
- `not your turn` — 不是你的回合
- `card index out of range` — 手牌索引越界
- `KUSOKURAE_ERROR_FORBIDDEN_MOVE` — 规则不允许出此牌

---

## 四、服务端 → 客户端消息

### 4.1 房间状态 `ROOM_STATE`

玩家加入/离开时广播全房间状态。

```json
{
  "type": "ROOM_STATE",
  "body": {
    "players": [
      { "playerID": "a1b2...", "position": 0, "isHost": true },
      { "playerID": "bf3d...", "position": 1, "isHost": false }
    ],
    "hostIdx": 0
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| players[].playerID | string | 玩家 ID |
| players[].position | int | 座位号 (0-based) |
| players[].isHost | bool | 是否为房主 |
| hostIdx | int | 房主座位号 |

---

### 4.2 玩家加入 `PLAYER_JOINED`

有新玩家加入时广播。

```json
{
  "type": "PLAYER_JOINED",
  "body": {
    "playerID": "bf3d...",
    "position": 1
  }
}
```

---

### 4.3 游戏开始 `GAME_START`

房间满员且房主开始游戏后，单独发给每位玩家。

```json
{
  "type": "GAME_START",
  "body": {
    "handCards": [
      { "suit": 0, "rank": 3, "playable": false },
      { "suit": 1, "rank": 5, "playable": false }
    ],
    "firstPlayerIdx": 0
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| handCards | CardInfo[] | 该玩家的初始手牌 |
| firstPlayerIdx | int | 首位行动玩家的座位号 |

---

### 4.4 轮到你了 `YOUR_TURN`

发送给当前行动玩家，告知可出的手牌。

```json
{
  "type": "YOUR_TURN",
  "body": {
    "playableIndices": [0, 2],
    "roundSeq": 1,
    "roundMoves": [
      { "suit": 0, "rank": 3, "playable": false }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| playableIndices | int[] | 当前可出的手牌索引列表 |
| roundSeq | int | 当前轮次序号 |
| roundMoves | CardInfo[] | 本轮已出的牌 |

---

### 4.5 出牌广播 `MOVE_MADE`

某位玩家出牌后向全体广播。

```json
{
  "type": "MOVE_MADE",
  "body": {
    "playerIdx": 0,
    "card": { "suit": 1, "rank": 5, "playable": false },
    "roundMoves": [
      { "suit": 1, "rank": 5, "playable": false }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| playerIdx | int | 出牌玩家座位号 |
| card | CardInfo | 打出的牌 |
| roundMoves | CardInfo[] | 本轮截至目前所有已出的牌 |

---

### 4.6 回合结束 `ROUND_END`

一轮结束时广播。

```json
{
  "type": "ROUND_END",
  "body": {
    "winnerIdx": 1,
    "score": 2
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| winnerIdx | int | 本轮赢家座位号 |
| score | int | 本轮牌面总分 |

---

### 4.7 游戏结束 `GAME_OVER`

所有牌打完时广播，宣布最终胜负。

```json
{
  "type": "GAME_OVER",
  "body": {
    "finalScores": [
      { "playerIdx": 0, "score": 5 },
      { "playerIdx": 1, "score": 8 },
      { "playerIdx": 2, "score": 3 }
    ],
    "winnerIdx": 1
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| finalScores[].playerIdx | int | 玩家座位号 |
| finalScores[].score | int | 最终得分（吃进的总分数） |
| winnerIdx | int | 赢家座位号（得分最高者） |

---

### 4.8 错误消息 `ERROR`

操作失败时单独发送给对应玩家。

```json
{
  "type": "ERROR",
  "body": {
    "message": "not your turn"
  }
}
```

---

## 五、数据结构

### CardInfo

```json
{
  "suit": 0,
  "rank": 5,
  "playable": true
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| suit | int | 花色：-1=翔(Xiang), 0=油条(Youtiao), 1=包子(Baozi), 2=其他(Other) |
| rank | int | 牌面点数 |
| playable | bool | 当前是否可出 |

---

## 六、完整流程示例

```
1. 玩家A: POST /api/v1/room/new  { "numPlayers": 3 }
   → 获得 roomID + playerID (房主，position=0)

2. 玩家A: WebSocket /api/v1/communication/{roomID}/{playerA_ID}
   → 连接建立

3. 玩家B: POST /api/v1/room/join?roomID={roomID}
   → 获得 playerID (position=1)
   → 玩家A 收到 PLAYER_JOINED + ROOM_STATE

4. 玩家B: WebSocket /api/v1/communication/{roomID}/{playerB_ID}
   → 连接建立

5. 玩家C: POST /api/v1/room/join → WebSocket 连接
   → 全员收到 ROOM_STATE，3人满员

6. 玩家A 发送: { "type": "START_GAME" }
   → 全员收到 GAME_START（含各自手牌）
   → firstPlayerIdx 对应玩家收到 YOUR_TURN

7. 当前行动玩家发送: { "type": "PLAY_CARD", "body": { "cardIndex": 0 } }
   → 全员收到 MOVE_MADE
   → 下一位收到 YOUR_TURN
   ...（回合循环）

8. 所有牌出完后，全员收到 GAME_OVER
```
