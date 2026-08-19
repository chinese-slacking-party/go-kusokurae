# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build/Run/Test

```bash
go build ./...                           # Build all packages (requires C compiler for cgo)
go test ./...                            # Run all tests
go test ./sm/                            # Run engine tests only
go run ./experimental/selfplay/          # Terminal self-play client (one human plays all hands)
go run ./experimental/online/            # HTTP/WebSocket game server (port 8080)
go build -o server ./experimental/online/
```

## Architecture

Three layers, bottom-up:

**`sm/` — Core game engine (cgo wrapper around C).** The card game "Kusokurae" is implemented in `sm.c` (deck, dealing, trick-taking, scoring, busting). `sm.go` wraps it via cgo using Go structs with identical memory layout to the C structs — `unsafe.Pointer` casts convert between them without serialization. Go replaces the C PRNG with `math/rand/v2` via a callback. State transition callbacks bridge from C back to Go through `callbackMap`. Key types: `GameConfig`, `GameState`, `Player`, `Card`, `RoundState`.

**`gameserver/` — Multiplayer session management.** Channel-based async architecture: each `Player` has `NoticeCh` (outbound) and `OperatorCh` (inbound). `Room` manages a player roster and ready status. `Game` wraps `sm.GameState` with per-player channels (game loop in `GameFn` is a stub — not yet implemented). `Session` runs Input/Output goroutines for WebSocket I/O, forwarding between the raw connection and the player's channels.

**`experimental/` — Entry points (main packages).**
- `experimental/online/` — Gin HTTP server: `POST /api/v1/room/new`, `POST /api/v1/room/join`, `GET /api/v1/communication/:room_id/:player_id` (WebSocket upgrade)
- `experimental/selfplay/` — CLI where one human plays every seat, driven by `sm.GameState.Play()`
- `experimental/sm/` — Prints `unsafe.Sizeof` for the cgo structs (memory layout verification)


