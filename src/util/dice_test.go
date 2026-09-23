package util

import (
	"math/rand"
	"testing"
)

// Dice and Interval both read "min-max" and must agree on its bounds; each Roll stays inside them.
func TestDiceAndIntervalAgreeOnRanges(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, s := range []string{"12-24", "3-3", "7"} {
		d, i := ParseDice(s), NewIntervalFromString(s)
		if d.Min() != i.Min() || d.Max() != i.Max() {
			t.Fatalf("%s: dice %d-%d, interval %d-%d", s, d.Min(), d.Max(), i.Min(), i.Max())
		}
		for n := 0; n < 200; n++ {
			if r := d.Roll(rng); r < d.Min() || r > d.Max() {
				t.Fatalf("%s: dice rolled %d", s, r)
			}
			if r := i.Roll(rng); r < i.Min() || r > i.Max() {
				t.Fatalf("%s: interval rolled %d", s, r)
			}
		}
	}
}

// A single negative number is a value, not a range (affixes.rec's "price: -1000").
func TestIntervalReadsNegativeNumbers(t *testing.T) {
	for s, want := range map[string][2]int{"-1000": {-1000, -1000}, "5": {5, 5}, "3-7": {3, 7}, "-5--2": {-5, -2}} {
		if v := NewIntervalFromString(s); v.Min() != want[0] || v.Max() != want[1] {
			t.Errorf("%q = %d-%d, want %d-%d", s, v.Min(), v.Max(), want[0], want[1])
		}
	}
}
