package sm

import (
	"testing"
	"unsafe"
)

// TestStructLayout compares Go's idea of the shared structs against the C
// compiler's. A disagreement is not a wrong answer, it is C writing through
// Go's memory at the wrong offsets.
func TestStructLayout(t *testing.T) {
	var (
		card   Card
		player Player
		cfg    GameConfig
		cbs    GameCallbacks
		st     GameState
	)

	sizes := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"sizeof(Card)", unsafe.Sizeof(card), cLayout.SizeofCard},
		{"sizeof(Player)", unsafe.Sizeof(player), cLayout.SizeofPlayer},
		{"sizeof(GameConfig)", unsafe.Sizeof(cfg), cLayout.SizeofConfig},
		{"sizeof(GameCallbacks)", unsafe.Sizeof(cbs), cLayout.SizeofCallbacks},
	}
	for _, c := range sizes {
		if c.got != c.want {
			t.Errorf("%s = %d, C says %d", c.name, c.got, c.want)
		}
	}

	offsets := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"Player.index", unsafe.Offsetof(player.index), cLayout.PlayerIndex},
		{"Player.active", unsafe.Offsetof(player.active), cLayout.PlayerActive},
		{"Player.allCards", unsafe.Offsetof(player.allCards), cLayout.PlayerCards},
		{"Player.numCards", unsafe.Offsetof(player.numCards), cLayout.PlayerNCards},
		{"Player.cardsTaken", unsafe.Offsetof(player.cardsTaken), cLayout.PlayerCardsTaken},
		{"Player.score", unsafe.Offsetof(player.score), cLayout.PlayerScore},
		{"Player.busted", unsafe.Offsetof(player.busted), cLayout.PlayerBusted},

		{"GameState.cfg", unsafe.Offsetof(st.cfg), cLayout.StateCfg},
		{"GameState.status", unsafe.Offsetof(st.status), cLayout.StateStatus},
		{"GameState.players", unsafe.Offsetof(st.players), cLayout.StatePlayers},
		{"GameState.numRound", unsafe.Offsetof(st.numRound), cLayout.StateNRound},
		{"GameState.ghostHolder", unsafe.Offsetof(st.ghostHolder), cLayout.StateGhost},
		{"GameState.highRanker", unsafe.Offsetof(st.highRanker), cLayout.StateHighRanker},
		{"GameState.curRound", unsafe.Offsetof(st.curRound), cLayout.StateCurRound},
		{"GameState.rngState", unsafe.Offsetof(st.rngState), cLayout.StateRNGState},
		{"GameState.cbs", unsafe.Offsetof(st.cbs), cLayout.StateCbs},
	}
	for _, c := range offsets {
		if c.got != c.want {
			t.Errorf("offsetof(%s) = %d, C says %d", c.name, c.got, c.want)
		}
	}

	// GameState carries goStateCallbackNo past the end of the C struct, so it
	// may be larger -- never smaller, or C would write past the allocation.
	if unsafe.Sizeof(st) < cLayout.SizeofState {
		t.Errorf("sizeof(GameState) = %d, smaller than C's %d",
			unsafe.Sizeof(st), cLayout.SizeofState)
	}
	if unsafe.Offsetof(st.goStateCallbackNo) < cLayout.SizeofState {
		t.Errorf("goStateCallbackNo sits at %d, inside C's %d-byte struct",
			unsafe.Offsetof(st.goStateCallbackNo), cLayout.SizeofState)
	}
}

// TestSaveLayoutIsBuildIndependent is the Go mirror of the static assertions in
// sm.h: the properties that let the bytes before cbs be written on one build
// and read back on another.
func TestSaveLayoutIsBuildIndependent(t *testing.T) {
	var st GameState

	// The one member wanting 8-byte alignment has to land on a multiple of 8,
	// or a build that aligns uint64_t to 4 (i386 does) pads differently.
	if off := unsafe.Offsetof(st.rngState); off%8 != 0 {
		t.Errorf("rngState is at %d, not a multiple of 8", off)
	}
	// Every repeated element too, or the run of them walks what follows off
	// an 8-byte boundary.
	if n := unsafe.Sizeof(st.cfg); n%8 != 0 {
		t.Errorf("sizeof(GameConfig) = %d, not a multiple of 8", n)
	}
	if n := unsafe.Sizeof(st.players[0]); n%8 != 0 {
		t.Errorf("sizeof(Player) = %d, not a multiple of 8", n)
	}
	if n := unsafe.Sizeof(st.curRound[0]); n%8 != 0 {
		t.Errorf("sizeof(Card) = %d, not a multiple of 8", n)
	}
	if cLayout.SaveBytes != cLayout.StateCbs {
		t.Errorf("KUSOKURAE_SAVE_BYTES = %d, want offsetof(cbs) = %d",
			cLayout.SaveBytes, cLayout.StateCbs)
	}
	if cLayout.SaveBytes%8 != 0 {
		t.Errorf("KUSOKURAE_SAVE_BYTES = %d, not a multiple of 8", cLayout.SaveBytes)
	}
}
