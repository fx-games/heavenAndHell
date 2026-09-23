package main

import (
	"os"
	"testing"
)

// TestAltConnected: every level type's layout, seeds 1-200, has all its floor reachable from all its floor, and
// every door sits in a wall (solid on two opposite sides) between floor (on the other two).
func TestAltConnected(t *testing.T) {
	if err := os.Chdir("../../.."); err != nil { // levels.rec is read from the repo's files/
		t.Fatal(err)
	}
	for ltype := 1; ltype <= 4; ltype++ {
		for seed := int64(1); seed <= 200; seed++ {
			g, err := buildAlt(ltype, seed)
			if err != nil {
				t.Fatalf("%s seed %d: %v", typeNames[ltype], seed, err)
			}
			if n := g.unreachable(); n > 0 {
				t.Errorf("%s seed %d: %d floor cells cut off\n%s", typeNames[ltype], seed, n, g.render(""))
			}
			for i, c := range g.c {
				x, y := i%g.w, i/g.w
				solid := func(x, y int) bool { return g.at(x, y) == cWall }
				if c == cDoor && !(g.walkable(x-1, y) && g.walkable(x+1, y) && solid(x, y-1) && solid(x, y+1)) &&
					!(g.walkable(x, y-1) && g.walkable(x, y+1) && solid(x-1, y) && solid(x+1, y)) {
					t.Errorf("%s seed %d: door at %d,%d is not set in a wall between two floors", typeNames[ltype], seed, x, y)
				}
			}
		}
	}
}
