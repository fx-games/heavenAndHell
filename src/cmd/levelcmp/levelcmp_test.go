package main

import (
	"os"
	"testing"
)

// TestAltConnected: every -alt level, every type, seeds 1-200, has all its floor reachable from all its floor, and
// every door sits in a wall (solid on two opposite sides) between floor (on the other two).
func TestAltConnected(t *testing.T) {
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

// TestMetrics prints D1's and the stand-ins' averages over seeds 1-20 side by side (go test -v -run Metrics);
// D1's half needs the d1 mod's data, run from the repo root, and is skipped without it.
func TestMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	var d *d1
	if err := os.Chdir("../../.."); err == nil {
		if _, err := os.Stat("files/mods/d1"); err == nil {
			d = loadD1()
		}
	}
	for ltype := 1; ltype <= 4; ltype++ {
		for _, side := range []struct {
			name  string
			build func(int, int64) (*grid, error)
		}{{"d1", func(l int, s int64) (*grid, error) {
			if d == nil {
				return nil, nil
			}
			return d.build(l, s)
		}}, {"alt", buildAlt}} {
			var floor float64
			var posts, loops, doors, water, n int
			for seed := int64(1); seed <= 20; seed++ {
				g, err := side.build(ltype, seed)
				if err != nil || g == nil {
					continue
				}
				n++
				floor += g.floorPct()
				posts += g.posts()
				loops += g.loops()
				for _, c := range g.c {
					doors += b2i(c == cDoor)
					water += b2i(c == cWater)
				}
			}
			if n > 0 {
				t.Logf("%-9s %-3s floor %4.1f%%  posts %5.1f  loops %4.1f  doors %4.1f  lava %5.1f", typeNames[ltype],
					side.name, floor/float64(n), float64(posts)/float64(n), float64(loops)/float64(n),
					float64(doors)/float64(n), float64(water)/float64(n))
			}
		}
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
