package sm

// This file exposes the C side of the PRNG callback to Go tests and
// benchmarks. It is not a _test.go file because cgo is not supported in test
// files ("use of cgo in test ... not supported"), so the bridge has to live in
// a regular package file. Nothing here is used by the library itself.

/*
#include "sm_internal.h"

extern void goRandom(int16_t *);

// Mirrors the bridge sm.go installs in init(): the C engine calls back into Go
// for every random number it needs.
static int16_t test_cgo_random(void *state) {
	int16_t ret;
	goRandom(&ret);
	return ret;
}

// Drives that bridge from the C side, i.e. the exact Go -> C -> Go round trip
// the engine performs per dice roll.
static int16_t call_bridge_from_c(void) {
	return test_cgo_random(NULL);
}

static void use_native_prng(void) {
	kusokurae_set_prng(&urand);
}

// Installs a bridge equivalent to the one sm.go's init() installs.
static void use_cgo_prng(void) {
	kusokurae_set_prng(&test_cgo_random);
}
*/
import "C"

import "unsafe"

// cPRNGMax is MS_RAND_MAX, the inclusive upper bound of the engine's own PRNG
// and therefore of any replacement installed through kusokurae_set_prng.
const cPRNGMax = C.MS_RAND_MAX

// goRandomValue calls the Go half of the bridge directly, without crossing any
// language boundary.
func goRandomValue() int16 {
	var out C.int16_t
	goRandom(&out)
	return int16(out)
}

// nativeRandom calls the engine's own PRNG (one Go -> C crossing).
func nativeRandom(state *int32) int16 {
	return int16(C.urand(unsafe.Pointer(state)))
}

// randomViaC goes Go -> C -> Go, the full round trip the engine pays for.
func randomViaC() int16 {
	return int16(C.call_bridge_from_c())
}

// useNativePRNG makes the engine use its built-in PRNG.
func useNativePRNG() { C.use_native_prng() }

// useCgoPRNG restores the Go PRNG bridge.
func useCgoPRNG() { C.use_cgo_prng() }
