package common

import (
	"testing"

	"rpg/src/geometry"
)

// Deep dungen levels hide some loop doors behind a wall; a found one is a door, and stays one when the level is
// rebuilt from its snapshot.
func TestSecretDoorsAreFoundAndSaved(t *testing.T) {
	withMod(t, "heavenandhell")
	levels := loadModLevels(t)
	for seed := int64(1); seed <= 20; seed++ {
		g := modGenerator(t, levels, seed)
		m := g.GenerateMap("doom_10")
		var p geometry.Point
		found := false
		for q, hidden := range m.SecretDoors() {
			if hidden {
				p, found = q, true
				break
			}
		}
		if !found {
			continue
		}
		if m.IsTileWalkable(p) {
			t.Fatalf("seed %d: secret door at %v is walkable before it is found", seed, p)
		}
		g.revealSecretDoor(m, p)
		o, _ := m.TryGetObjectAt(p)
		if _, isDoor := o.(*Door); !m.IsTileWalkable(p) || !isDoor {
			t.Fatalf("seed %d: found secret door at %v is no door", seed, p)
		}
		g2 := modGenerator(t, levels, seed)
		g2.snapshots = map[string]*LevelSnapshot{"doom_10": {FoundDoors: []geometry.Point{p}}}
		if m2 := g2.GenerateMap("doom_10"); m2.IsSecretDoorAt(p) || !m2.IsTileWalkable(p) {
			t.Fatalf("seed %d: the rebuilt level hid the found door at %v again", seed, p)
		}
		return
	}
	t.Fatal("no secret door on doom_10 in 20 seeds")
}
