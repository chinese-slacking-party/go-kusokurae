package sm

// Exposes the C side of the PRNG callback to the tests and benchmarks.
// Nothing here is used by the library itself; the test_ prefix says so, since
// cgo is not allowed in a _test.go file.

/*
#include "sm_internal.h"

extern void goRandom(int *);

// Mirrors the bridge sm.go installs in init(): the C engine calls back into Go
// for every random number it needs.
static int test_cgo_random(void *state) {
	int ret;
	goRandom(&ret);
	return ret;
}

// Drives that bridge from the C side, i.e. the exact Go -> C -> Go round trip
// the engine performs per dice roll.
static int call_bridge_from_c(void) {
	return test_cgo_random(NULL);
}

static void use_native_prng(void) {
	kusokurae_set_prng(&ms_rand);
}

// Installs a bridge equivalent to the one sm.go's init() installs.
static void use_cgo_prng(void) {
	kusokurae_set_prng(&test_cgo_random);
}
*/
import "C"

import "unsafe"

// cPRNGMax is KUSOKURAE_RAND_MAX, the inclusive upper bound every generator
// installed through kusokurae_set_prng must honour.
const cPRNGMax = C.KUSOKURAE_RAND_MAX

// goRandomValue calls the Go half of the bridge directly, without crossing any
// language boundary.
func goRandomValue() int {
	var out C.int
	goRandom(&out)
	return int(out)
}

// nativeRandom calls the engine's own PRNG (one Go -> C crossing).
func nativeRandom(state *int32) int {
	return int(C.ms_rand(unsafe.Pointer(state)))
}

// randomViaC goes Go -> C -> Go, the full round trip the engine pays for.
func randomViaC() int {
	return int(C.call_bridge_from_c())
}

// useNativePRNG makes the engine use its built-in PRNG.
func useNativePRNG() { C.use_native_prng() }

// useCgoPRNG restores the Go PRNG bridge.
func useCgoPRNG() { C.use_cgo_prng() }
