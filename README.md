# go-kusokurae

一个在线多人卡牌游戏服务器，实现了一款中国大学宿舍民间卡牌游戏。

## 游戏规则

"Kusokurae"是日语"くそくらえ"（吃屎吧）的罗马字，中文俗称"喂你吃翔"。这是一款 3-4 人的吃墩（trick-taking）卡牌游戏。使用 33 张牌，包含三种花色：

- **包子**（值 +1）
- **油条**（值 0）
- **翔 / 屎**（值 -1）
- 外加一张 **鬼牌**（值 2，赢墩时该轮分数翻倍）

每种花色内有 0-10 的等级。玩家轮流领出，被迫打出等级 0 的牌作为领出者时"自爆"（bust）。打完所有手牌后，累计分数最高者获胜。

## 技术栈

- **Go 1.23** — HTTP/WebSocket 服务端
- **C** — 核心游戏引擎，通过 cgo 桥接
- **Gin** — HTTP 路由
- **Gorilla WebSocket** — WebSocket 通信
- **google/uuid** — 房间/玩家 ID 生成

## 架构（三层，自底向上）

### `sm/` — 核心游戏引擎

C 语言实现的完整游戏逻辑：洗牌发牌、出牌校验、吃墩计分、自爆判定。Go 侧通过 cgo 调用，使用 `unsafe.Pointer` 零拷贝传递结构体。Go 的 `math/rand` 替换了 C 的随机数生成器。

关键类型：`GameConfig`、`GameState`、`Player`、`Card`、`RoundState`

### `gameserver/` — 多人会话管理

基于 channel 的异步架构：

- **Player** — `NoticeCh`（下行）/ `OperatorCh`（上行）两个 channel，每个玩家一对
- **Room** — 管理玩家列表和准备状态
- **Game** — 封装 `sm.GameState`，游戏主循环 `GameFn` 负责轮流通知、等待出牌、广播结果；断线玩家自动托管（autoPlay）
- **Session** — Input/Output goroutine，在 WebSocket 连接和 Player channel 之间双向转发

### `experimental/` — 入口程序

| 程序 | 说明 |
|------|------|
| `experimental/online/` | Gin HTTP + WebSocket 服务器（端口 8080） |
| `experimental/selfplay/` | 终端单人自玩客户端，一人操作所有座位 |
| `experimental/sm/` | 打印 `unsafe.Sizeof`，验证 Go/C 结构体内存布局一致 |

## 数据流

```
客户端  <--WebSocket-->  Gin HTTP Server
                                |
                         Session (Input/Output goroutines)
                                |
                         Room → Game (GameFn 游戏主循环)
                                |
                         sm.GameState (cgo → C 引擎)
```

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/room/new` | 创建房间 |
| POST | `/api/v1/room/join` | 加入房间 |
| GET | `/api/v1/communication/:room_id/:player_id` | WebSocket 升级，支持断线重连和状态重同步 |

## 构建/运行

```bash
go build ./...                           # 编译所有包（需要 C 编译器）
go test ./...                            # 运行所有测试
go test ./sm/                            # 仅运行引擎测试
go run ./experimental/selfplay/          # 终端自玩客户端
go run ./experimental/online/            # 启动在线游戏服务器（端口 8080）
go build -o server ./experimental/online/
```

## TODO

1.内存监控，room注销已完成，还需测试是否有协程、channel等泄露问题
