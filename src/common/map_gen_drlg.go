package common

import (
	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
)

// SetSeed sets the game seed levels are generated from (decision 11: levels rebuild from their seed). D1Seeds and
// Quests are fully deterministic from it, so they need no separate save-file entry: a fresh game and a just-loaded
// one both derive the same table and the same quest availability by calling this once at startup.
func (g *MapGenerator) SetSeed(seed int64) {
	g.seed = seed
	g.seeds = rules.LevelSeeds(int32(seed))
	g.quests = nil // rolled from the seed on first use (questTable): the mod's quest table comes with the factory
}

// questTable is D1's quest table for this game's seed, rolled once.
func (g *MapGenerator) questTable() *rules.Quests {
	if g.quests == nil && g.factory != nil {
		q := g.factory.Gen().D.InitQuests(g.seeds)
		g.quests = &q
	}
	return g.quests
}

// quest is D1's own table row for quest q: its availability roll, its level and its starting vars. The table
// belongs to D1's level builders, so a mod that builds none of its levels with them gets a plain quest that has
// not started - not whatever D1 rolled for the quest that happens to share its index.
func (g *MapGenerator) quest(q int) rules.Quest {
	if qs := g.questTable(); qs != nil && q >= 0 && q < len(qs) && g.buildsLevels() {
		return qs[q]
	}
	return rules.Quest{Active: rules.QuestInit}
}

// buildsLevels is whether any dungeon level comes from D1's generators. No level list at all counts, as it
// does in GenerateMap, where a level without an entry is D1's.
func (g *MapGenerator) buildsLevels() bool {
	for _, l := range g.levels {
		if _, ok := dungen.PopulationOf(l.Config); ok {
			return true
		}
	}
	return false
}

// SetSnapshots sets saved level state applied after a level is generated.
func (g *MapGenerator) SetSnapshots(snapshots map[string]*LevelSnapshot) { g.snapshots = snapshots }

// toCell converts D1 piece coordinates (16..95) to our 0..79 cell grid.
func toCell(px, py int) geometry.Point {
	return geometry.Point{X: px - dungen.CellOrigin, Y: py - dungen.CellOrigin}
}

// setPiece is the level's quest set piece in megatiles: x, y, w, h (D1's setpc_*).
func (g *MapGenerator) setPiece(m *gridmap.GridMap[*Actor, *Item, Object]) [4]int {
	if lvl := g.generated[m.GetName()]; lvl != nil {
		return lvl.SetPiece
	}
	return [4]int{}
}

// placeMonsters ports the actor side of InitMonsters: everything D1's generator placed, except the four reserved
// golem slots and skeletons still hidden inside a sarcophagus or barrel (those rise when the container is opened).
func (g *MapGenerator) placeMonsters(m *gridmap.GridMap[*Actor, *Item, Object], lvl *dungen.Level) {
	if g.walledIn == nil {
		g.walledIn = map[string][]*Actor{}
	}
	g.walledIn[m.GetName()] = nil
	delete(g.rosters, m.GetName())
	for i, mon := range lvl.Monsters {
		if mon == nil || mon.Reserved || mon.Hidden {
			continue
		}
		a := g.factory.NewMonsterFromLevel(mon)
		a.monster.SpawnIndex = i
		p := toCell(mon.X, mon.Y)
		a.SetDrawPosition(p.ToPointF())
		if !canPlaceMonster(m, p) {
			// walled in, like Lazarus and his helpers until their rooms open (changeMap places them then), or
			// inside what our simplified terrain treats as solid (see isLavaMega etc.)
			a.SetMapPosition(p)
			g.walledIn[m.GetName()] = append(g.walledIn[m.GetName()], a)
			continue
		}
		m.AddActor(a, p)
		g.addSpawn(m, a, func() *Actor {
			b := g.factory.NewMonsterFromLevel(mon)
			b.monster.SpawnIndex = i
			return b
		}, func(b *Actor) bool { return b.monster != nil && b.monster.SpawnIndex == i })
	}
}

// canPlaceMonster is CanPlaceActorHere minus D1's objects. D1 runs InitMonsters before InitObjects, and set pieces
// like the Poisoned Water candles (addCandles) go down without checking dMonster, so a monster can own the cell of
// a solid object. Anything placing monsters after objects exist (respawn, walled-in release, save restore) must
// allow the same.
func canPlaceMonster(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point) bool {
	if m.CanPlaceActorHere(p) {
		return true
	}
	if !m.Contains(p) {
		return false
	}
	obj, _ := m.TryGetObjectAt(p)
	_, isD1 := obj.(*Prop)
	return isD1 && m.GetCell(p).TileType.IsWalkable && !m.IsActorAt(p) && !m.IsObviousHazardAt(p)
}

// placeObjects ports the object side of InitObjects/CreateThemeRooms: everything D1's generator placed. Doors
// become terrain + an H&H Door object in addBasicTiles; wall traps stay hidden in the walls (trap.go).
func (g *MapGenerator) placeObjects(m *gridmap.GridMap[*Actor, *Item, Object], lvl *dungen.Level) {
	for _, o := range lvl.Objects {
		if doorKinds[o.Kind] {
			if door, ok := doorAt(m, toCell(o.X, o.Y)); ok {
				x, y := o.X, o.Y
				door.onOpen = func() { g.factory.springWallTraps(m, x, y) }
				door.Prop = o
				door.rogue = func() bool {
					p := g.factory.State.GetPlayer()
					return p != nil && p.hero != nil && p.hero.Rules().HasClassFlag(p.hero.Class, defs.ClassTrapSense)
				}
			}
			continue
		}
		if o.Kind == rules.ObjTRAPL || o.Kind == rules.ObjTRAPR {
			continue
		}
		p := toCell(o.X, o.Y)
		if !m.Contains(p) || m.IsObjectAt(p) {
			continue
		}
		placeObject(g.factory, m, lvl, o, p)
	}
}

// placeItems ports the item side of InitItems/AddInitItems/CreateThemeRooms: the floor items D1's generator
// placed (potions lying around, theme-room treasure, quest items).
func (g *MapGenerator) placeItems(m *gridmap.GridMap[*Actor, *Item, Object], lvl *dungen.Level) {
	for _, it := range lvl.Items {
		p := toCell(it.X, it.Y)
		if !m.CanPlaceItemHere(p) {
			if cells := freeCellsNear(m, p, 1, m.CanPlaceItemHere); len(cells) > 0 {
				p = cells[0]
			} else {
				continue
			}
		}
		m.AddItem(g.factory.NewRolledItem(it.Item), p)
	}
}

// freeCellsNear finds up to n free cells around origin by breadth-first search. Unlike GetFreeCellsForDistribution
// it doesn't iterate a Go map, so spawns stay reproducible from the level seed.
func freeCellsNear(m *gridmap.GridMap[*Actor, *Item, Object], origin geometry.Point, n int, free func(geometry.Point) bool) []geometry.Point {
	seen := map[geometry.Point]bool{origin: true}
	queue := []geometry.Point{origin}
	var out []geometry.Point
	for len(queue) > 0 && len(out) < n {
		p := queue[0]
		queue = queue[1:]
		for _, d := range geometry.Dirs {
			q := p.Add(d)
			if seen[q] || !m.Contains(q) || !m.IsTileWalkable(q) {
				continue
			}
			seen[q] = true
			queue = append(queue, q)
			if free(q) && len(out) < n {
				out = append(out, q)
			}
		}
	}
	return out
}

// SameRoom ports the AIs' dTransVal comparison: both cells lie in the same transparency region (room) of the
// current D1 level. Anywhere without D1 level data counts as one room.
func SameRoom(s *State, a, b geometry.Point) bool {
	lvl := s.mapGen.generated[s.GetMap().GetName()]
	if lvl == nil {
		return true
	}
	at := func(p geometry.Point) int8 {
		x, y := p.X+dungen.CellOrigin, p.Y+dungen.CellOrigin
		if x < 0 || y < 0 || x >= len(lvl.Trans) || y >= len(lvl.Trans[0]) {
			return -1
		}
		return lvl.Trans[x][y]
	}
	return at(a) == at(b)
}
