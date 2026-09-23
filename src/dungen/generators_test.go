package dungen

import (
	"fmt"
	"math/rand"
	"testing"

	"rpg/src/dungeon"
	"rpg/src/recfile"
)

// TestGeneratorsAreWalkable: every seed of each generator's defaults (and a few edge knobs) is one connected level.
func TestGeneratorsAreWalkable(t *testing.T) {
	for _, tc := range []struct {
		s     dungeon.Parser
		knobs recfile.Record
	}{
		{cave{}, nil},
		{cave{}, recfile.Record{{Name: "fill", Value: "50"}, {Name: "min_floor", Value: "0.2"}}},
		{drunkard{}, nil},
		{drunkard{}, recfile.Record{{Name: "dla", Value: "true"}}},
		{bsp{}, nil},
		{bsp{}, recfile.Record{{Name: "width", Value: "12"}, {Name: "height", Value: "12"}, {Name: "min_leaf", Value: "6"}}},
		{roomGraph{}, nil},
		{roomGraph{}, recfile.Record{{Name: "loops", Value: "1"}}},
		{noise{}, nil},
		{noise{}, recfile.Record{{Name: "rivers", Value: "3"}, {Name: "river_width", Value: "3"}, {Name: "river_meander", Value: "0.5"}}},
		{megadungeon{}, recfile.Record{{Name: "inner_room_chance", Value: "1"}}},
	} {
		cfg, err := tc.s.Parse(tc.knobs)
		if err != nil {
			t.Fatal(err)
		}
		for seed := int64(1); seed <= 50; seed++ {
			m, err := cfg.(Layout).Build(rand.New(rand.NewSource(seed)), nil)
			if err != nil {
				t.Fatalf("%T %v: %v", tc.s, tc.knobs, err)
			}
			assertConnected(t, int(seed), fmtT(tc.s), m)
		}
	}
}

func fmtT(v any) string { return fmt.Sprintf("%T", v) }
