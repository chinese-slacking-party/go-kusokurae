package sm

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
)

// prngMax is the inclusive upper bound the engine expects from the PRNG.
const prngMax = 0x7FFF

// randSamples is large enough that missing either endpoint of the range by
// chance has probability ~1e-14 ((1 - 1/0x8000)^(1<<20)).
const randSamples = 1 << 20

// bucketWidth is 1/64 of the PRNG range.
const bucketWidth = (prngMax + 1) / 64

func TestPRNGMaxMatchesC(t *testing.T) {
	// Guards against the Go and C sides drifting apart.
	assert.Equal(t, prngMax, cPRNGMax)
}

// TestCPRNGRange documents the contract the Go replacement has to honour: the
// engine's own PRNG yields the closed range [0, KUSOKURAE_RAND_MAX].
func TestCPRNGRange(t *testing.T) {
	state := int32(1)
	lo, hi := prngMax, 0
	for i := 0; i < randSamples; i++ {
		v := int(nativeRandom(&state))
		if v < 0 || v > prngMax {
			t.Fatalf("ms_rand() returned %d, want [0, %d]", v, prngMax)
		}
		lo, hi = min(lo, v), max(hi, v)
	}
	assert.Equal(t, 0, lo)
	assert.Equal(t, prngMax, hi)
}

func TestGoRandomRange(t *testing.T) {
	lo, hi := prngMax, 0
	for i := 0; i < randSamples; i++ {
		v := int(goRandomValue())
		if v < 0 || v > prngMax {
			t.Fatalf("goRandom() returned %d, want [0, %d]", v, prngMax)
		}
		lo, hi = min(lo, v), max(hi, v)
	}
	// Both endpoints must be reachable: rand.IntN(prngMax) would never produce
	// prngMax itself, which is the off-by-one this test exists to catch.
	assert.Equal(t, 0, lo)
	assert.Equal(t, prngMax, hi)
}

func TestGoRandomUniform(t *testing.T) {
	var counts [(prngMax + 1) / bucketWidth]int
	for i := 0; i < randSamples; i++ {
		counts[int(goRandomValue())/bucketWidth]++
	}
	// Expected 16384 samples per bucket with a standard deviation of ~127, so
	// the 5% window below sits more than 6 sigma out.
	expected := randSamples / len(counts)
	tolerance := expected / 20
	for i, c := range counts {
		if c < expected-tolerance || c > expected+tolerance {
			t.Errorf("bucket %d holds %d samples, want %d +/- %d", i, c, expected, tolerance)
		}
	}
}

// TestCgoBridgeRange exercises the full Go -> C -> Go path instead of calling
// goRandom directly, so a broken bridge (a truncating conversion, say) cannot
// hide behind the Go-only tests above.
func TestCgoBridgeRange(t *testing.T) {
	const n = 1 << 17
	lo, hi := prngMax, 0
	for i := 0; i < n; i++ {
		v := int(randomViaC())
		if v < 0 || v > prngMax {
			t.Fatalf("bridge returned %d, want [0, %d]", v, prngMax)
		}
		lo, hi = min(lo, v), max(hi, v)
	}
	// Fewer samples here, so assert on the extremes loosely; TestGoRandomRange
	// pins the exact endpoints.
	assert.Less(t, lo, bucketWidth)
	assert.Greater(t, hi, prngMax-bucketWidth)
}

// TestDealUsesPRNG checks the PRNG end to end: every deal must be a partition
// of the deck, and over many deals the ghost card must reach every seat.
func TestDealUsesPRNG(t *testing.T) {
	const deals = 3000
	const players = 3
	const handSize = 33 / players

	g, err := NewGame(GameConfig{NumPlayers: players}, nil)
	assert.NoError(t, err)

	var ghostCount [players]int
	for d := 0; d < deals; d++ {
		assert.NoError(t, g.Start())
		seen := make(map[uint32]bool, 33)
		for i := 0; i < players; i++ {
			for j := 0; j < handSize; j++ {
				order := g.players[i].allCards[j].displayOrder
				if seen[order] {
					t.Fatalf("deal %d: duplicate card with display order %d", d, order)
				}
				seen[order] = true
			}
		}
		if len(seen) != 33 {
			t.Fatalf("deal %d: dealt %d distinct cards, want 33", d, len(seen))
		}
		ghostCount[g.ghostHolder[0]]++
	}

	// Expected 1000 per seat with a standard deviation of ~26, so the +/-20%
	// window below is far outside the noise.
	for i, c := range ghostCount {
		if c < deals/players*4/5 || c > deals/players*6/5 {
			t.Errorf("seat %d held the ghost %d times out of %d, want roughly %d",
				i, c, deals, deals/players)
		}
	}
}

// TestNativePRNGSeedVaries covers the seeding of the engine's own PRNG, which
// the Go bridge normally hides. Two games created back to back must not be
// dealt identically: before the seed was taken from a nanosecond clock, games
// created within the same second shared a seed, and on big-endian machines the
// seed was always zero.
func TestNativePRNGSeedVaries(t *testing.T) {
	useNativePRNG()
	defer useCgoPRNG()

	const games = 8
	deals := make(map[[33]uint32]bool, games)
	for i := 0; i < games; i++ {
		g, err := NewGame(GameConfig{NumPlayers: 3}, nil)
		assert.NoError(t, err)
		assert.NoError(t, g.Start())

		// The engine deals in deck order, so the hands alone identify the deal.
		var deal [33]uint32
		for s := 0; s < 3; s++ {
			for j := 0; j < 11; j++ {
				deal[s*11+j] = g.players[s].allCards[j].displayOrder
			}
		}
		deals[deal] = true
	}
	assert.Equal(t, games, len(deals), "games created in the same second were dealt identically")
}

// The benchmarks below peel the PRNG callback apart layer by layer to show what
// a cgo boundary crossing costs. Run them with:
//
//	go test ./sm/ -run '^$' -bench PRNG -benchmem

// BenchmarkPRNGGoOnly is the baseline: the Go PRNG with no cgo in the picture.
func BenchmarkPRNGGoOnly(b *testing.B) {
	var sink int
	for i := 0; i < b.N; i++ {
		sink = rand.IntN(prngMax + 1)
	}
	_ = sink
}

// BenchmarkPRNGGoRandom measures the Go half of the bridge called from Go, so
// no language boundary is crossed. The delta against GoOnly is the cost of the
// callback signature itself (C type conversion plus the pointer write).
func BenchmarkPRNGGoRandom(b *testing.B) {
	var sink int
	for i := 0; i < b.N; i++ {
		sink = goRandomValue()
	}
	_ = sink
}

// BenchmarkPRNGNativeInC measures the engine's own PRNG called from Go: one
// Go -> C crossing plus three arithmetic instructions. Against GoOnly this is
// essentially the price of a single Go -> C call.
func BenchmarkPRNGNativeInC(b *testing.B) {
	state := int32(1)
	var sink int
	for i := 0; i < b.N; i++ {
		sink = nativeRandom(&state)
	}
	_ = sink
}

// BenchmarkPRNGCgoRoundTrip measures Go -> C -> Go, which is what the engine
// pays for every dice roll: a call into C plus a callback back into Go, the
// latter being the expensive direction.
func BenchmarkPRNGCgoRoundTrip(b *testing.B) {
	var sink int
	for i := 0; i < b.N; i++ {
		sink = randomViaC()
	}
	_ = sink
}

// BenchmarkPRNGDealNative and BenchmarkPRNGDealCgo show the aggregate effect on
// real work: dealing 33 cards to 3 players draws 55 random numbers, so the
// per-call overhead is multiplied by 55.
func BenchmarkPRNGDealNative(b *testing.B) {
	useNativePRNG()
	defer useCgoPRNG()
	benchmarkDeal(b)
}

func BenchmarkPRNGDealCgo(b *testing.B) {
	useCgoPRNG()
	benchmarkDeal(b)
}

func benchmarkDeal(b *testing.B) {
	b.Helper()
	g, err := NewGame(GameConfig{NumPlayers: 3}, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := g.Start(); err != nil {
			b.Fatal(err)
		}
	}
}
