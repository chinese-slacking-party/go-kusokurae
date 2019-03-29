package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/bs-iron-trio/go-kusokurae/sm"
)

var (
	cardsFirst = flag.Bool("cards-first", false, "Output every player's cards immediately after deal")
)

var (
	g *sm.GameState
)

func init() {
	flag.Parse()
}

/*
go-kusokurae
正　解：Go 语言写的喂你吃翔
歪解一：走，吃翔去！
歪解二：请用御翔（go=日语“御”）
歪解三：不敢说，啊不敢说……
*/

func main() {
	var err error
	var cfg sm.GameConfig

	fmt.Println("为你吃翔")
	fmt.Print("请输入游戏人数：")
	_, err = fmt.Scanf("%d", &cfg.NumPlayers)
	if err != nil {
		log.Fatal(err)
	}
	g, err = sm.NewGame(cfg, roundOrStateChange)
	if err != nil {
		log.Fatal(err)
	}
	g.Start()
	if *cardsFirst {
		inspectAllHandCards(g)
	}
	game(g)
	for i := int32(0); i < cfg.NumPlayers; i++ {
		fmt.Print(g.GetPlayer(i))
	}
}

func game(G *sm.GameState) {
	var err error
	var P *sm.Player
	var move int
	var cards []sm.Card
	for G.GetStatus() == sm.StatusPlay {
		P = G.GetActivePlayer()
		cards = P.GetHandCards()
		fmt.Printf("该你了，你是玩家 %d，你手上的牌有：\n", P.GetIndex())
		printCards(cards)
	prompt:
		if len(cards) < 2 {
			move = 0
		} else {
			fmt.Printf("前面人出的牌：%v\n", g.GetRoundState().Moves)
			fmt.Printf("要出哪张牌？[0-%d] ", len(cards)-1)
			_, err = fmt.Scanf("%d", &move)
			if err != nil {
				if err == io.EOF {
					os.Exit(0)
				}
				log.Println(err)
				goto prompt
			}
			if move < 0 || move >= len(cards) {
				goto prompt
			}
		}
		fmt.Printf("玩家 %d 出了：", P.GetIndex())
		printCard(cards[move])
		err = G.Play(cards[move])
		if err != nil {
			panic(err)
		}
	}
}

var suitNamesCHS = map[sm.Suit]string{
	sm.SuitXiang:   "翔",
	sm.SuitYoutiao: "油条",
	sm.SuitBaozi:   "包子",
	sm.SuitOther:   "鬼",
}

func inspectAllHandCards(g *sm.GameState) {
	numPlayers := g.GetConfig().NumPlayers
	for i := int32(0); i < numPlayers; i++ {
		fmt.Printf("玩家 %d 的牌：\n", i+1)
		printCardsPretty(g.GetPlayer(i).GetCards())
	}
}

func printCard(in sm.Card) {
	fmt.Printf("%s%d\n", suitNamesCHS[in.GetSuit()], in.GetRank())
}

func printCards(in []sm.Card) {
	for i := range in {
		fmt.Printf("%-4d", i)
		printCard(in[i])
	}
}

func printCardsPretty(in []sm.Card) {
	curSuit := sm.SuitUnknown
	first := true
	if (len(in) > 0 && in[0].GetSuit() == sm.SuitOther) ||
		(len(in) > 1 && in[1].GetSuit() == sm.SuitOther) ||
		(len(in) > 2 && in[2].GetSuit() == sm.SuitOther) {
		fmt.Printf("鬼\n")
	}
	for i := range in {
		if in[i].GetSuit() == sm.SuitOther {
			continue
		}
		if suit := in[i].GetSuit(); suit != sm.SuitOther && suit != curSuit {
			if !first {
				fmt.Println()
			} else {
				first = false
			}
			curSuit = suit
			fmt.Printf("%s：", suitNamesCHS[curSuit])
		}
		fmt.Printf("%d ", in[i].GetRank())
	}
	fmt.Println()
}
