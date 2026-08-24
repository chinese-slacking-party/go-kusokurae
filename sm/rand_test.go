package sm

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests for the Go side of the PRNG bridge. The engine's own generator and its
// seeding are covered by tests/nativeprng.c, and the behaviour of a generator
// that breaks the contract by tests/prngcontract.c.

// prngMax is the inclusive upper bound the engine expects from the PRNG.
const prngMax = 0x7FFF

// randSamples is large enough that missing either endpoint of the range by
// chance has probability ~1e-14 ((1 - 1/0x8000)^(1<<20)).
const randSamples = 1 << 20

// bucketWidth is 1/64 of the PRNG range.
const bucketWidth = (prngMax + 1) / 64

func TestPRNGMaxMatchesC(t *testing.T) {
	assert.Equal(t, prngMax, cPRNGMax)
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
