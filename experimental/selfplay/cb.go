package main

import (
	"fmt"
	"log"

	"github.com/bs-iron-trio/go-kusokurae/sm"
)

func roundOrStateChange(state sm.GameStatus) {
	if g.GetStatus() != state {
		log.Printf("GameStatus %v -> %v", g.GetStatus(), state)
		return
	}
	rs := g.GetRoundState()
	fmt.Printf("回合 %d", rs.Seq)
	if rs.IsDoubled {
		fmt.Printf("，已翻倍")
	}
	fmt.Println()
	fmt.Printf("玩家 %d 大，得分 %d\n", rs.RoundWinner.GetIndex(), rs.ScoreOnBoard)
	fmt.Print(g.GetPlayer(0).GetScore())
	for i := int32(1); i < g.GetConfig().NumPlayers; i++ {
		fmt.Printf(":%d", g.GetPlayer(i).GetScore())
	}
	fmt.Println()
}
