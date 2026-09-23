package dungen

import (
	"math/rand"
	"testing"

	"rpg/src/geometry"
)

// Two open halls joined by one corridor: a lake may flood either hall but never the only way between them, and
// every lake is ringed by shallows, so no dry floor touches deep water.
func TestAddLakesKeepsTheLevelConnected(t *testing.T) {
	for seed := int64(1); seed <= 20; seed++ {
		m := NewDungeonMap(60, 30)
		for y := 1; y < 29; y++ {
			for x := 1; x < 59; x++ {
				if x < 28 || x > 31 {
					m.tiles[x+y*60] = Room
				}
			}
		}
		for x := 28; x <= 31; x++ {
			m.tiles[x+15*60] = Corridor
		}
		m.addLakes(rand.New(rand.NewSource(seed)), 3)

		lakes := 0
		for y := 0; y < m.height; y++ {
			for x := 0; x < m.width; x++ {
				p := geometry.Point{X: x, Y: y}
				if m.GetTileAt(p) != Lake {
					continue
				}
				lakes++
				for _, d := range dirs8 {
					if n := p.Add(d); m.Contains(n) && m.GetTileAt(n) == Room {
						t.Fatalf("seed %d: dry floor %v touches deep water at %v", seed, n, p)
					}
				}
			}
		}
		if lakes == 0 {
			t.Errorf("seed %d: no lake on two open halls", seed)
		}
		if !m.connectedWithout(nil) {
			t.Fatalf("seed %d: a lake cut the level in two", seed)
		}
	}
}
