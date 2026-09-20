package d1

import (
	"os"
	"testing"
)

// A monster must never start the level inside a door or another solid object (moveMonstersOffObjects).
func TestNoMonsterStartsOnASolidObject(t *testing.T) {
	d, err := Load(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	for game := int32(1); game <= 3; game++ {
		seeds := LevelSeeds(game * 7919)
		q := InitQuests(seeds)
		gnr := NewGenerator(d, game)
		for dlvl := 1; dlvl <= 16; dlvl++ {
			lvl, err := gnr.CreateLevel(&q, seeds, dlvl, SLNone, EntryMain)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range lvl.Monsters {
				if m == nil {
					continue
				}
				for _, o := range lvl.Objects {
					if o.X == m.X && o.Y == m.Y && (o.Solid || o.DoorFlag) {
						t.Errorf("game %d level %d: monster at (%d,%d) stands in object kind %d", game, dlvl, m.X, m.Y, o.Kind)
					}
				}
			}
		}
	}
}
