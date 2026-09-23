package rules

// Rng is D1's Borland LCG (engine.cpp SetRndSeed/GetRndSeed/random_). Levels built from the same seed with the
// same calls in the same order come out as in D1.
type Rng struct{ seed int32 }

func (r *Rng) SetSeed(s int32) { r.seed = s }

// Seed is GetRndSeed: advance, then abs (abs(INT_MIN) stays negative, as in C).
func (r *Rng) Seed() int32 {
	r.seed = 0x015A4E35*r.seed + 1
	if r.seed < 0 {
		return -r.seed
	}
	return r.seed
}

// Random is random_(idx, v): 0..v-1.
func (r *Rng) Random(v int) int {
	if v <= 0 {
		return 0
	}
	if v < 0xFFFF {
		return int((r.Seed() >> 16) % int32(v))
	}
	return int(r.Seed() % int32(v))
}
