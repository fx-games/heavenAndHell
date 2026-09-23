package dungen

import (
	"slices"
	"testing"

	"rpg/src/dungeon"
)

// A room one level requires grows nowhere else: H&H's level 1 once grew Meat Jack's room (and him) because its
// hand-kept forbid list missed it.
func TestReserveRequiredRooms(t *testing.T) {
	levels := []dungeon.Level{
		{Map: "l1", Config: AccretionConfig{Prefix: "Cath"}},
		{Map: "l2", Config: AccretionConfig{Prefix: "Cath", Require: []string{"Cath_template_for_meat_jack"}}},
	}
	ReserveRequiredRooms(levels)
	if f := levels[0].Config.(AccretionConfig).Forbid; !slices.Contains(f, "Cath_template_for_meat_jack") {
		t.Fatalf("level 1 may still grow the room: forbid %v", f)
	}
	if f := levels[1].Config.(AccretionConfig).Forbid; len(f) != 0 {
		t.Fatalf("level 2 forbids its own room: %v", f)
	}
}
