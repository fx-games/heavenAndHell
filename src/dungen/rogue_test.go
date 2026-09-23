package dungen

import (
	"math/rand"
	"testing"

	"rpg/src/recfile"
)

// TestRogueIsWalkable: Rogue's spanning tree joins all nine cells, gone rooms included, so every seed and size a
// mod may declare comes out as one connected level with somewhere to put the stairs.
func TestRogueIsWalkable(t *testing.T) {
	for i, knobs := range []recfile.Record{
		{{Name: "generator", Value: "rogue"}},
		{{Name: "width", Value: "30"}, {Name: "height", Value: "15"}, {Name: "max_gone_rooms", Value: "8"}},
		{{Name: "width", Value: "120"}, {Name: "height", Value: "40"}, {Name: "doors", Value: "false"}, {Name: "extra_passages", Value: "0"}},
	} {
		cfg, err := rogue{}.Parse(knobs)
		if err != nil {
			t.Fatal(err)
		}
		for seed := int64(1); seed <= 200; seed++ {
			m, err := cfg.(Layout).Build(rand.New(rand.NewSource(seed)), nil)
			if err != nil {
				t.Fatal(err)
			}
			assertConnected(t, int(seed), "rogue", m)
			if len(m.AllRooms()) == 0 {
				t.Fatalf("seed %d: no rooms", seed)
			}
			// Rogue's own size always has room for stairs; a cramped mod level falls back to any two cells
			// (addDungenStairs), so only the defaults must.
			if i == 0 && len(m.GetPotentialStairPositions()) < 2 {
				t.Fatalf("seed %d %v: fewer than two stair spots", seed, knobs)
			}
		}
	}
}

func TestRogueRejectsBadKnobs(t *testing.T) {
	for _, bad := range []recfile.Record{
		{{Name: "width", Value: "20"}},
		{{Name: "max_gone_rooms", Value: "9"}},
		{{Name: "extra_passages", Value: "-1"}},
	} {
		if _, err := (rogue{}).Parse(bad); err == nil {
			t.Errorf("%v: accepted", bad)
		}
	}
}
