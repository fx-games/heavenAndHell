package d1

import (
	"os"
	"testing"
)

// A wall torch belongs on a wall: D1 hangs some of them off the floor cell in front, which top-down leaves a
// torch standing in the open (addL2Torches).
func TestWallTorchesSitOnWalls(t *testing.T) {
	d, err := Load(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	torches := 0
	for game := int32(1); game <= 3; game++ {
		seeds := LevelSeeds(game * 7919)
		q := InitQuests(seeds)
		gnr := NewGenerator(d, game)
		for dlvl := 5; dlvl <= 8; dlvl++ {
			lvl, err := gnr.CreateLevel(&q, seeds, dlvl, SLNone, EntryMain)
			if err != nil {
				t.Fatal(err)
			}
			for _, o := range lvl.Objects {
				switch o.Kind {
				case ObjTORCHL, ObjTORCHR, ObjTORCHL2, ObjTORCHR2:
					torches++
					if !lvl.Solid(o.X, o.Y) {
						t.Errorf("game %d level %d: torch kind %d at (%d,%d) is not on a wall", game, dlvl, o.Kind, o.X, o.Y)
					}
				}
			}
		}
	}
	if torches == 0 {
		t.Fatal("no wall torches placed at all")
	}
}
