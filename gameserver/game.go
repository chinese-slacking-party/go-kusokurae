package main

import "github.com/bs-iron-trio/go-kusokurae/sm"

type Game struct {
	ID     string
	Config *sm.GameConfig
	State  *sm.GameState
}
