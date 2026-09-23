package rules

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"rpg/src/util"
)

// DropTable is one of the mod's drops.rec tables. Its drops are rolled in order and the first to hit is the
// item; when none hits, nothing drops. A mod without the table drops nothing there.
type DropTable struct {
	Count util.Interval // how many to roll, for the tables that place several (level)
	Drops []Drop
}

// Drop is one drop: line. Item nil is drops.rec's "random": any droppable base item the roll allows.
type Drop struct {
	Item    *ItemData
	Nothing bool          // drops.rec's "nothing": a hit drops no item
	Chance  int           // percent
	Levels  util.Interval // dungeon levels the line applies on; zero = all
}

// readDropTables reads drops.rec, whose drop: lines name items of the mod's own items.rec.
func (d *Data) readDropTables(fsys fs.FS) error {
	d.DropTables = map[string]DropTable{}
	recs, err := ReadRec(fsys, "drops.rec")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, r := range recs {
		var name string
		var t DropTable
		for _, f := range r {
			switch f.Name {
			case "internal_name":
				name = f.Value
			case "count":
				t.Count = util.NewIntervalFromString(f.Value)
			case "drop":
				w := strings.Fields(f.Value)
				p, err := 0, errors.New("malformed")
				if len(w) == 2 || len(w) == 4 && w[2] == "levels" {
					p, err = strconv.Atoi(strings.TrimSuffix(w[1], "%"))
				}
				if err != nil {
					return fmt.Errorf("drops.rec %s: drop %q: want <item> <chance>%% [levels <min>-<max>]", name, f.Value)
				}
				item := w[0]
				e := Drop{Chance: p, Nothing: item == "nothing"}
				if len(w) == 4 {
					e.Levels = util.NewIntervalFromString(w[3])
				}
				if item != "random" && item != "nothing" {
					if e.Item = d.ItemsByName[item]; e.Item == nil {
						return fmt.Errorf("drops.rec %s: items.rec has no item %q", name, item)
					}
				}
				t.Drops = append(t.Drops, e)
			}
		}
		d.DropTables[name] = t
	}
	return nil
}

// RollDrop rolls the named table on dungeon level dlvl: its item, or nil when nothing drops. random picks for a
// "random" drop. A line for other levels is skipped without a roll.
func (g *Generator) RollDrop(table string, dlvl int, random func() *ItemData) *ItemData {
	for _, e := range g.D.DropTables[table].Drops {
		if e.Levels.NotZero() && (dlvl < e.Levels.Min() || dlvl > e.Levels.Max()) {
			continue
		}
		if g.random(100) < e.Chance {
			if e.Nothing {
				return nil
			}
			if e.Item == nil {
				return random()
			}
			return e.Item
		}
	}
	return nil
}
