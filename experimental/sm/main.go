package main

import (
	"log"
	"unsafe"

	"github.com/bs-iron-trio/go-kusokurae/sm"
)

func main() {
	log.Println(unsafe.Sizeof(sm.GameConfig{}))
	log.Println(unsafe.Sizeof(sm.Card{}))
	log.Println(unsafe.Sizeof(sm.Player{}))
	log.Println(unsafe.Sizeof(sm.GameState{}))
}
