package common

import (
	"io"
	"path"
	"regexp"
	"strings"
	"testing"

	"rpg/files"

	"rpg/src/dungeon"
	"rpg/src/recfile"

	"rpg/src/geometry"
)

// Deep accretion levels hide some loop doors behind a wall; a found one is a door, and stays one when the level is
// rebuilt from its snapshot. No level declares accretion now (and the passes drop loop doors), so the test swaps
// the Doom levels' layout for a bare accretion one.
func TestSecretDoorsAreFoundAndSaved(t *testing.T) {
	withMod(t, "heavenandhell")
	f, err := files.Open(path.Join("assets", "data", "levels.rec"))
	if err != nil {
		t.Fatal(err)
	}
	text, _ := io.ReadAll(f)
	f.Close()
	doom := regexp.MustCompile(`(?m)^generator: rogue\n(?:(?:width|height|max_gone_rooms|extra_passages|scale|population|decoration): .*\n)*`) // the Doom levels' rogue layout
	src := doom.ReplaceAllString(string(text), "generator: accretion\ntemplate_prefix: Dung\n")
	src = strings.ReplaceAll(src, "set_piece: piece_hh_rogue room\n", "") // any pass rebuilds the map without its loop doors
	levels, err := dungeon.LoadLevels(recfile.Read(strings.NewReader(src)), lookupStrategy)
	if err != nil {
		t.Fatal(err)
	}
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
