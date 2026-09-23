package common

import (
	"fmt"
	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/dungeon"
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
		if _, ok := l.Config.(dungen.LevelParams); ok {
			return true
		}
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

// d1DoorObjectKinds is repeated from map_gen.go's doorKinds so this file doesn't need to import it twice; see
// there for why doors are terrain + an H&H Door object, not a Prop.

// d1Strategy is the LevelStrategy of D1's four level builders (dungen.D1), d1SetStrategy of its quest set levels.
type d1Strategy struct{ dungeon.Parser }

func (d1Strategy) Generate(g *MapGenerator, lvl dungeon.Level) *gridmap.GridMap[*Actor, *Item, Object] {
	return g.generateMap(lvl, nil)
}

type d1SetStrategy struct{ dungeon.Parser }

func (d1SetStrategy) Generate(g *MapGenerator, lvl dungeon.Level) *gridmap.GridMap[*Actor, *Item, Object] {
	set := lvl.Config.(dungen.SetLevelParams)
	return g.generateMap(lvl, &set)
}

// generateMap builds a D1 dungeon level, or with set a quest set level (LoadSetMap), with D1's own generators.
// It places the named locations D1's triggers mark; levels.rec's exits say where they lead.
func (g *MapGenerator) generateMap(level dungeon.Level, set *dungen.SetLevelParams) *gridmap.GridMap[*Actor, *Item, Object] {
	targetMap, dlvl := level.Map, level.Depth
	sl := rules.SLNone
	if set != nil {
		if q := g.factory.State.QuestByKey(set.Quest); q >= 0 {
			sl = g.questTable()[q].SetLevel
		}
		if sl == rules.SLNone {
			panic(fmt.Sprintf("levels.rec: %s: quest %q has no D1 set level", targetMap, set.Quest))
		}
	}
	lvl, err := g.factory.Levels.CreateLevel(g.factory.Gen(), g.questTable(), g.levelSeeds(targetMap, dlvl), dlvl, sl, dungen.EntryMain)
	if err != nil {
		panic(err)
	}
	if g.generated == nil {
		g.generated = map[string]*dungen.Level{}
	}
	g.generated[targetMap] = lvl

	m := gridmap.NewEmptyMap[*Actor, *Item, Object](dungen.LevelSize, dungen.LevelSize)
	m.SetName(targetMap)
	g.addBasicTiles(m, lvl)

	if set == nil {
		g.placeTriggers(m, lvl, dlvl)
	} else {
		g.placeSetLevelReturn(m, lvl, sl)
	}

	g.placeMonsters(m, lvl)
	g.placeObjects(m, lvl)
	g.placeItems(m, lvl)

	if snap := g.snapshots[targetMap]; snap != nil {
		snap.apply(g.factory, m)
	}
	for _, a := range append(append([]*Actor(nil), m.Actors()...), g.walledIn[targetMap]...) {
		if a.monster != nil {
			g.factory.initTalk(a)
		}
	}
	PlacePortal(g.factory.State, m)
	return m
}

// setPiece is the level's quest set piece in megatiles: x, y, w, h (D1's setpc_*).
func (g *MapGenerator) setPiece(m *gridmap.GridMap[*Actor, *Item, Object]) [4]int {
	if lvl := g.generated[m.GetName()]; lvl != nil {
		return lvl.SetPiece
	}
	return [4]int{}
}

func stairsUp() gridmap.Tile {
	return gridmap.Tile{DefinedIcon: defs.IconStairsUp, DefinedDescription: "stairs leading up", IsWalkable: true, IsTransparent: true, IsMissilePassable: true, Special: gridmap.SpecialTileStairsUp}
}

// placeTriggers draws the stairs and warp D1's own generator positioned (Level.Triggers) and names them
// ladder_up, ladder_down and warp, plus each available quest's entrance on its level (CheckQuests), named by the
// quest's key. Lazarus' spot is only named: its red portal opens when levels.rec's exit condition holds.
func (g *MapGenerator) placeTriggers(m *gridmap.GridMap[*Actor, *Item, Object], lvl *dungen.Level, dlvl int) {
	for _, t := range lvl.Triggers {
		p := toCell(t.X, t.Y)
		switch t.Msg {
		case dungen.TrigPrevLevel:
			m.AddNamedLocation("ladder_up", p)
			m.SetTile(p, stairsUp())
		case dungen.TrigNextLevel:
			m.AddNamedLocation("ladder_down", p)
			m.SetTile(p, stairsDown())
		case dungen.TrigTownWarpUp:
			m.AddNamedLocation("warp", p)
			warpTile := stairsDown()
			warpTile.DefinedDescription = "a way back to town"
			m.SetTile(p, warpTile)
		}
	}
	for q, def := range g.factory.State.QuestDefs() {
		if q >= len(g.questTable()) {
			break
		}
		qu := g.questTable()[q]
		if qu.SetLevel == rules.SLNone || qu.Level != dlvl || qu.Active == rules.QuestNotAvail {
			continue
		}
		p := toCell(qu.X, qu.Y)
		m.AddNamedLocation(def.Key, p)
		if qu.SetLevel == rules.SLVileBetrayer {
			m.AddNamedLocation("ladder_down", toCell(qu.X+3, qu.Y+3)) // L4PENTA2's stairs piece (370), InitL4Triggers
			continue
		}
		tile := stairsDown()
		tile.DefinedDescription = "a way further down"
		m.SetTile(p, tile)
	}
}

// placeSetLevelReturn names where the player arrives in a quest set level, its way back out (SetReturnLvlPos),
// and for Lazarus' Lair the portal spot (exit) whose way home opens once he is dead.
func (g *MapGenerator) placeSetLevelReturn(m *gridmap.GridMap[*Actor, *Item, Object], lvl *dungen.Level, sl int) {
	for _, t := range lvl.Triggers {
		if t.Msg != dungen.TrigReturnLevel {
			continue
		}
		p := toCell(t.X, t.Y)
		m.AddNamedLocation("ladder_up", p)
		m.SetTile(p, stairsUp())
	}
	if sl == rules.SLVileBetrayer {
		m.AddNamedLocation("ladder_up", toCell(lvl.View[0], lvl.View[1]))
	}
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
