package common

import (
	"hash/fnv"
	"log/slog"
	"math/rand"
	"slices"

	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/util"
)

// Populating a dungen level: scatter the mod's own monsters and its own floor loot over the walkable cells,
// keeping both clear of the stairs so arriving is not an ambush.
//
// D1 does not populate this way and must not: its generators place their monsters and items themselves, at
// coordinates its own RNG chose, and copying those rules here would put D1's spawn tables in front of another
// game's records. What IS shared is everything below the choice: the actor and item factories, which already
// read whichever mod's tables are loaded.

const (
	dungenCellsPerMonster = 30  // D1's InitMonsters: one monster per 30 floor cells
	dungenMaxMonsters     = 190 // and never more than MAXMONSTERS-10
	dungenItemPercent     = 1   // chance in 100 that a floor cell starts with an item
	dungenStairsClear     = 7   // how far from any stairs both have to stay
)

// populateDungenLevel places D1's monster count (InitMonsters: floor cells / 30, in groups of one type: always
// one on level 1, else half the time 2-3 on level 2 and 3-5 deeper) and rolls an item on each floor cell. The
// walk is a permutation of a sorted cell list rather than a map iteration, because iterating a Go map is how
// reproducibility died here twice before.
func (g *MapGenerator) populateDungenLevel(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap, lvl dungeon.Level, stairs []geometry.Point) {
	dlvl := lvl.Depth
	var cells []geometry.Point
	width, height := layout.GetSize()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if p := (geometry.Point{X: x, Y: y}); layout.IsEmptySpace(p) {
				cells = append(cells, p)
			}
		}
	}
	types := g.spawnTypes(lvl)
	g.placeWanderers(m, lvl, cells, stairs)
	total, placed := min(len(cells)/dungenCellsPerMonster, dungenMaxMonsters), 0
	for _, i := range g.randomSource.Perm(len(cells)) {
		p := cells[i]
		if minDist(stairs, p) <= dungenStairsClear {
			continue
		}
		if placed < total && m.CanPlaceActorHere(p) {
			enemy, companions := g.factory.rollEnemy(lvl, types, false, nil)
			if enemy == nil {
				placed = total
				continue
			}
			if n := g.groupSize(dlvl); n > 1 {
				own, _ := g.factory.monsterByID(enemy.GetInternalName())
				companions = append(slices.Repeat([]defs.Monster{own}, n-1), companions...)
			}
			companions = companions[:min(len(companions), total-placed-1)]
			enemy.SetDrawPosition(p.ToPointF())
			enemy.monster.SpawnIndex = -1 // generated with a random source a load can't replay: saved whole
			m.AddActor(enemy, p)
			g.addSpawn(m, enemy, nil, nil)
			placed++
			for _, c := range g.spawnDungenCompanions(m, p, companions) {
				c.monster.SpawnIndex = -1
				g.addSpawn(m, c, nil, nil)
				placed++
			}
			continue
		}
		if g.randomSource.Intn(100) < dungenItemPercent && m.CanPlaceItemHere(p) {
			if it := g.factory.Gen().FloorItem(dlvl, false, false); it != nil {
				m.AddItem(g.factory.NewRolledItem(it), p)
			}
		}
	}
}

// placeWanderers gives a regular level at most one of each wander_chance monster its depth allows, at its odds,
// on a free cell away from the stairs.
func (g *MapGenerator) placeWanderers(m *gridmap.GridMap[*Actor, *Item, Object], lvl dungeon.Level, cells, stairs []geometry.Point) {
	for _, mon := range g.factory.Generator.D.Monsters {
		if len(lvl.Monsters) > 0 || mon.WanderChance == 0 || !mon.SpawnsAt(lvl.Depth) || g.randomSource.Float64() >= mon.WanderChance {
			continue
		}
		for _, i := range g.randomSource.Perm(len(cells)) {
			if p := cells[i]; minDist(stairs, p) > dungenStairsClear && m.CanPlaceActorHere(p) {
				a := g.factory.NewMonsterByID(mon.ID)
				a.SetDrawPosition(p.ToPointF())
				a.monster.SpawnIndex = -1
				m.AddActor(a, p)
				g.addSpawn(m, a, nil, nil)
				break
			}
		}
	}
}

// groupSize ports InitMonsters' pack size: one on level 1, else half the time 2-3 on level 2 and 3-5 deeper.
func (g *MapGenerator) groupSize(dlvl int) int {
	switch {
	case dlvl <= 1 || g.randomSource.Intn(2) == 0:
		return 1
	case dlvl == 2:
		return g.randomSource.Intn(2) + 2
	default:
		return g.randomSource.Intn(3) + 3
	}
}

// spawnTypes draws the level's monster types that have a spawn_cost, until defs.SpawnBudget is spent (D1's
// GetLevelMTypes); free ones are always in. Seeded by the game and the level, so a respawn or a reload draws the
// same types without saving them.
func (g *MapGenerator) spawnTypes(lvl dungeon.Level) map[int]bool {
	all := g.factory.Generator.D.Monsters
	var candidates []int
	for i, mon := range all {
		if rolls(lvl, mon) && mon.SpawnCost > 0 && !slices.Contains(mon.Tags, "unique") {
			candidates = append(candidates, i)
		}
	}
	h := fnv.New64a()
	h.Write([]byte(lvl.Map))
	rnd := rand.New(rand.NewSource(g.seed ^ int64(h.Sum64())))
	types := map[int]bool{}
	for _, i := range defs.PickSpawnTypes(all, candidates, rnd.Intn) {
		types[i] = true
	}
	return types
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

// rollEnemy rolls one monster for the level out of the active mod's own table, plus whatever its record says
// spawns alongside it; it may also roll a unique, except one taken says is already about. Nil when nothing can
// appear.
func (g *Factory) rollEnemy(lvl dungeon.Level, types map[int]bool, uniques bool, taken func(id string) bool) (*Actor, []defs.Monster) {
	all := g.Generator.D.Monsters
	var eligible []int
	for i, mon := range all {
		unique := slices.Contains(mon.Tags, "unique")
		if unique && (!uniques || taken(mon.ID)) {
			continue
		}
		if !rolls(lvl, mon) || mon.SpawnCost > 0 && !types[i] {
			continue
		}
		eligible = append(eligible, i)
	}
	if len(eligible) == 0 {
		return nil, nil
	}
	pick := eligible[g.RandomSource.Intn(len(eligible))]
	return g.NewMonster(pick), g.groupSpawnOf(all[pick])
}

// rolls reports whether lvl may roll mon: a side level its monster lines, any other level every random_spawn
// record whose dlvl..max_dlvl takes in its depth.
func rolls(lvl dungeon.Level, mon defs.Monster) bool {
	if len(lvl.Monsters) > 0 {
		return slices.Contains(lvl.Monsters, mon.ID)
	}
	return mon.RandomSpawn && mon.WanderChance == 0 && mon.SpawnsAt(lvl.Depth)
}

// groupSpawnOf reads a record's group_spawn entries: a monster's internal name, optionally with a count or a
// range in brackets ("rat(0-1)", "skeleton(2)").
func (g *Factory) groupSpawnOf(mon defs.Monster) []defs.Monster {
	var out []defs.Monster
	for _, entry := range mon.GroupSpawn {
		name, count := entry, 1
		if util.LooksLikeAFunction(entry) {
			n, args := util.GetNameAndArgs(entry)
			name, count = n, util.NewIntervalFromString(args.Get(0)).Roll(rng)
		}
		companion, ok := g.monsterByID(name)
		if !ok {
			slog.Warn("monster spawns with an enemy the mod's enemies.rec does not have", "monster", mon.ID, "enemy", name)
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
			return g.NewMonster(i)
		}
	}
	return nil
}
