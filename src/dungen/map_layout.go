package dungen

import (
	"fmt"
	"io/fs"
	"math/rand"

	"rpg/src/dungeon"
	"rpg/src/hhm"
	"rpg/src/recfile"
)

func init() { Layouts["map"] = mapLayout{} }

// MapConfig lays one of the mod's maps down whole (levels.rec's "template:"): a hand-drawn level with its
// entities, its open layout (<template>_open) and all, for a population to fill. D1's quest set levels are this.
type MapConfig struct{ Template string }

type mapLayout struct{}

func (mapLayout) Parse(r recfile.Record) (dungeon.Config, error) {
	t, ok := dungeon.Field(r, "template")
	if !ok || t == "" {
		return nil, fmt.Errorf("levels.rec: map needs a template (the map's name)")
	}
	return MapConfig{Template: t}, nil
}

func (c MapConfig) Build(_ *rand.Rand, fsys fs.FS) (*DungeonMap, error) {
	proj, err := loadTemplates(fsys, hhm.Dir)
	if err != nil {
		return nil, err
	}
	lv := proj.Level(c.Template)
	if lv == nil {
		return nil, fmt.Errorf("map: the maps have no %s", c.Template)
	}
	dm := NewDungeonMap(lv.Width, lv.Height)
	return dm, stampPieces(dm, []placed{{Stamped: Stamped{Map: lv, Open: proj.Level(c.Template + "_open")}}}, fsys, 1)
}
