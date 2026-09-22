package common

import (
	"fmt"
	"slices"

	"rpg/defs"
	"rpg/dungen"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
)

// Populating a dungen level: scatter the mod's own monsters and its own floor loot over the walkable cells,
// keeping both clear of the stairs so arriving is not an ambush.
//
// D1 does not populate this way and must not: its generators place their monsters and items themselves, at
// coordinates its own RNG chose, and copying those rules here would put D1's spawn tables in front of another
// game's records. What IS shared is everything below the choice: the actor and item factories, which already
// read whichever mod's tables are loaded.

const (
	dungenEnemyPercent = 5 // chance in 100 that a walkable cell starts with a monster on it
	dungenItemPercent  = 1 // and, failing that, with an item
	dungenStairsClear  = 7 // how far from any stairs both have to stay
)

// populateDungenLevel walks the level's walkable cells in a seeded random order and rolls for a monster or an
// item on each. The walk is a permutation of a sorted cell list rather than a map iteration, because iterating
// a Go map is how reproducibility died here twice before.
func (g *MapGenerator) populateDungenLevel(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap, dlvl int, stairs []geometry.Point) {
	var cells []geometry.Point
	width, height := layout.GetSize()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if p := (geometry.Point{X: x, Y: y}); layout.IsEmptySpace(p) {
				cells = append(cells, p)
			}
		}
	}
	for _, i := range g.randomSource.Perm(len(cells)) {
		p := cells[i]
		if minDist(stairs, p) <= dungenStairsClear {
			continue
		}
		switch {
		case g.randomSource.Intn(100) < dungenEnemyPercent && m.CanPlaceActorHere(p):
			enemy, companions := g.factory.NewRandomEnemy(dlvl)
			if enemy == nil {
				continue
			}
			enemy.SetDrawPosition(p.ToPointF())
			enemy.d1Monster.SpawnIndex = -1 // generated with a random source a load can't replay: saved whole
			m.AddActor(enemy, p)
			g.addSpawn(m, enemy, nil, nil)
			for _, c := range g.spawnDungenCompanions(m, p, companions) {
				c.d1Monster.SpawnIndex = -1
				g.addSpawn(m, c, nil, nil)
			}
		case g.randomSource.Intn(100) < dungenItemPercent && m.CanPlaceItemHere(p):
			if it := g.factory.D1().FloorItem(dlvl, false, false); it != nil {
				m.AddItem(g.factory.NewD1Item(it), p)
			}
		}
	}
}

// spawnDungenCompanions puts a monster's group_spawn around it, as close as there is room for.
// Returns the ones it placed.
func (g *MapGenerator) spawnDungenCompanions(m *gridmap.GridMap[*Actor, *Item, Object], leader geometry.Point, companions []defs.Monster) (placed []*Actor) {
	if len(companions) == 0 {
		return
	}
	spots := freeCellsNear(m, leader, len(companions), m.CanPlaceActorHere)
	for i, mon := range companions {
		if i >= len(spots) {
			return
		}
		a := g.factory.NewMonsterByID(mon.ID)
		if a == nil {
			continue
		}
		a.SetDrawPosition(spots[i].ToPointF())
		m.AddActor(a, spots[i])
		placed = append(placed, a)
	}
	return
}

// NewRandomEnemy rolls one monster for dungeon level dlvl out of the active mod's own table, plus whatever its
// record says spawns alongside it. Nil when the mod's table has nothing that can appear this deep.
func (g *Factory) NewRandomEnemy(dlvl int) (*Actor, []defs.Monster) {
	return g.rollEnemy(dlvl, false, nil)
}

// rollEnemy is NewRandomEnemy that may also roll a unique, except one taken says is already about.
func (g *Factory) rollEnemy(dlvl int, uniques bool, taken func(id string) bool) (*Actor, []defs.Monster) {
	all := g.Generator.D.Monsters
	var eligible []int
	for i, mon := range all {
		unique := slices.Contains(mon.Tags, "unique")
		if unique && (!uniques || taken(mon.ID)) {
			continue
		}
		if mon.Available() && mon.CanAppearAt(g.RandomSource, dlvl) { // a unique lives in its room
			eligible = append(eligible, i)
		}
	}
	if len(eligible) == 0 {
		return nil, nil
	}
	pick := eligible[g.RandomSource.Intn(len(eligible))]
	return g.NewD1Monster(pick), g.groupSpawnOf(all[pick])
}

// groupSpawnOf reads a record's group_spawn entries: a monster's internal name, optionally with a count or a
// range in brackets ("rat(0-1)", "skeleton(2)").
func (g *Factory) groupSpawnOf(mon defs.Monster) []defs.Monster {
	var out []defs.Monster
	for _, entry := range mon.GroupSpawn {
		name, count := entry, 1
		if util.LooksLikeAFunction(entry) {
			n, args := util.GetNameAndArgs(entry)
			name, count = n, stats.NewIntervalFromString(args.Get(0)).LinearRandom()
		}
		companion, ok := g.monsterByID(name)
		if !ok {
			println(fmt.Sprintf("%s spawns with %q, which this mod's enemies.rec does not have", mon.ID, name))
			continue
		}
		for i := 0; i < count; i++ {
			out = append(out, companion)
		}
	}
	return out
}

func (g *Factory) monsterByID(id string) (defs.Monster, bool) {
	for _, mon := range g.Generator.D.Monsters {
		if mon.ID == id {
			return mon, true
		}
	}
	return defs.Monster{}, false
}

// NewMonsterByID builds one monster by the name its record carries, for a caller that knows which one it wants
// (a group spawn, a quest room's guardian) rather than rolling for depth.
func (g *Factory) NewMonsterByID(id string) *Actor {
	for i, mon := range g.Generator.D.Monsters {
		if mon.ID == id {
			return g.NewD1Monster(i)
		}
	}
	return nil
}
