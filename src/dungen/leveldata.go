package dungen

import (
	"io/fs"

	"rpg/src/dungeon"
	"rpg/src/rules"
)

// Levels is what D1's population knows of a mod's levels: the level type each dungeon level is populated as.
type Levels struct{ types [rules.NumLevels]int }

// LoadLevels reads which D1 level type populates each depth from the mod's levels.rec (its population knobs).
func LoadLevels(fsys fs.FS) (*Levels, error) {
	l := &Levels{}
	recs, err := rules.ReadRec(fsys, "levels.rec")
	if err != nil {
		return l, err
	}
	for _, r := range recs {
		p, ok, err := ParsePopulation(r)
		if err != nil {
			return l, err
		}
		depth, err := dungeon.Int(r, "depth", 0)
		if err != nil {
			return l, err
		}
		if ok && p.SetLevel == "" && depth > 0 && depth < rules.NumLevels {
			l.types[depth] = p.Type
		}
	}
	return l, nil
}

// LevelType is the D1 level type dungeon level dlvl is populated as, town for none.
func (l *Levels) LevelType(dlvl int) int {
	if dlvl <= 0 || dlvl >= rules.NumLevels {
		return rules.DTypeTown
	}
	return l.types[dlvl]
}
