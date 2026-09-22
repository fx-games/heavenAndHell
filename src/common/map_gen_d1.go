package common

import (
	"fmt"
	"rpg/src/d1"
	"rpg/src/defs"
	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/gridmap"
)

// SetSeed sets the game seed levels are generated from (decision 11: levels rebuild from their seed). D1Seeds and
// D1Quests are fully deterministic from it, so they need no separate save-file entry: a fresh game and a just-loaded
// one both derive the same table and the same quest availability by calling this once at startup.
func (g *MapGenerator) SetSeed(seed int64) {
	g.seed = seed
	g.d1Seeds = d1.LevelSeeds(int32(seed))
	q := d1.InitQuests(g.d1Seeds)
	g.d1Quests = &q
}

// d1Quest is D1's own table row for quest q: its availability roll, its level and its starting vars. The table
// belongs to D1's level builders, so a mod that builds none of its levels with them gets a plain quest that has
// not started - not whatever D1 rolled for the quest that happens to share its index.
func (g *MapGenerator) d1Quest(q int) d1.Quest {
	if g.d1Quests != nil && q >= 0 && q < len(g.d1Quests) && g.buildsD1Levels() {
		return g.d1Quests[q]
	}
	return d1.Quest{Active: d1.QuestInit}
}

// buildsD1Levels is whether any dungeon level comes from D1's generators. No level list at all counts, as it
// does in GenerateMap, where a level without an entry is D1's.
func (g *MapGenerator) buildsD1Levels() bool {
	for _, l := range g.levels {
		if _, ok := l.Config.(d1.LevelParams); ok {
			return true
		}
	}
	return false
}

// SetSnapshots sets saved level state applied after a level is generated.
func (g *MapGenerator) SetSnapshots(snapshots map[string]*LevelSnapshot) { g.snapshots = snapshots }

// toCell converts D1 piece coordinates (16..95) to our 0..79 cell grid.
func toCell(px, py int) geometry.Point {
	return geometry.Point{X: px - d1.CellOrigin, Y: py - d1.CellOrigin}
}

// d1DoorObjectKinds is repeated from map_gen.go's d1DoorKinds so this file doesn't need to import it twice; see
// there for why doors are terrain + an H&H Door object, not a D1Object.

// generateD1Map builds a D1 dungeon level, or with set a quest set level (LoadSetMap), with D1's own generators.
// It places the named locations D1's triggers mark; levels.rec's exits say where they lead.
func (g *MapGenerator) generateD1Map(level dungeon.Level, set *d1.SetLevelParams) *gridmap.GridMap[*Actor, *Item, Object] {
	targetMap, dlvl := level.Map, level.Depth
	sl := d1.SLNone
	if set != nil {
		if q := g.factory.State.QuestByKey(set.Quest); q >= 0 {
			sl = g.d1Quests[q].SetLevel
		}
		if sl == d1.SLNone {
			panic(fmt.Sprintf("levels.rec: %s: quest %q has no D1 set level", targetMap, set.Quest))
		}
	}
	lvl, err := g.factory.D1().CreateLevel(g.d1Quests, g.d1LevelSeeds(targetMap, dlvl), dlvl, sl, d1.EntryMain)
	if err != nil {
		panic(err)
	}
	if g.d1Levels == nil {
		g.d1Levels = map[string]*d1.Level{}
	}
	g.d1Levels[targetMap] = lvl

	m := gridmap.NewEmptyMap[*Actor, *Item, Object](d1.LevelSize, d1.LevelSize)
	m.SetName(targetMap)
	g.addBasicTiles(m, lvl)

	if set == nil {
		g.placeD1Triggers(m, lvl, dlvl)
	} else {
		g.placeD1SetLevelReturn(m, lvl, sl)
	}

	g.placeD1Monsters(m, lvl)
	g.placeD1Objects(m, lvl)
	g.placeD1Items(m, lvl)

	if snap := g.snapshots[targetMap]; snap != nil {
		snap.apply(g.factory, m)
	}
	for _, a := range append(append([]*Actor(nil), m.Actors()...), g.walledIn[targetMap]...) {
		if a.d1Monster != nil {
			g.factory.initD1Talk(a)
		}
	}
	PlaceD1Portal(g.factory.State, m)
	return m
}

// setPiece is the level's quest set piece in megatiles: x, y, w, h (D1's setpc_*).
func (g *MapGenerator) setPiece(m *gridmap.GridMap[*Actor, *Item, Object]) [4]int {
	if lvl := g.d1Levels[m.GetName()]; lvl != nil {
		return lvl.SetPiece
	}
	return [4]int{}
}

func d1StairsUp() gridmap.Tile {
	return gridmap.Tile{DefinedIcon: defs.IconStairsUp, DefinedDescription: "stairs leading up", IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsUp}
}

// placeD1Triggers draws the stairs and warp D1's own generator positioned (Level.Triggers) and names them
// ladder_up, ladder_down and warp, plus each available quest's entrance on its level (CheckQuests), named by the
// quest's key. Lazarus' spot is only named: its red portal opens when levels.rec's exit condition holds.
func (g *MapGenerator) placeD1Triggers(m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level, dlvl int) {
	for _, t := range lvl.Triggers {
		p := toCell(t.X, t.Y)
		switch t.Msg {
		case d1.TrigPrevLevel:
			m.AddNamedLocation("ladder_up", p)
			m.SetTile(p, d1StairsUp())
		case d1.TrigNextLevel:
			m.AddNamedLocation("ladder_down", p)
			m.SetTile(p, d1StairsDown())
		case d1.TrigTownWarpUp:
			m.AddNamedLocation("warp", p)
			warpTile := d1StairsDown()
			warpTile.DefinedDescription = "a way back to town"
			m.SetTile(p, warpTile)
		}
	}
	for q, def := range g.factory.State.QuestDefs() {
		if q >= len(g.d1Quests) {
			break
		}
		qu := g.d1Quests[q]
		if qu.SetLevel == d1.SLNone || qu.Level != dlvl || qu.Active == d1.QuestNotAvail {
			continue
		}
		p := toCell(qu.X, qu.Y)
		m.AddNamedLocation(def.Key, p)
		if qu.SetLevel == d1.SLVileBetrayer {
			m.AddNamedLocation("ladder_down", toCell(qu.X+3, qu.Y+3)) // L4PENTA2's stairs piece (370), InitL4Triggers
			continue
		}
		tile := d1StairsDown()
		tile.DefinedDescription = "a way further down"
		m.SetTile(p, tile)
	}
}

// placeD1SetLevelReturn names where the player arrives in a quest set level, its way back out (SetReturnLvlPos),
// and for Lazarus' Lair the portal spot (exit) whose way home opens once he is dead.
func (g *MapGenerator) placeD1SetLevelReturn(m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level, sl int) {
	for _, t := range lvl.Triggers {
		if t.Msg != d1.TrigReturnLevel {
			continue
		}
		p := toCell(t.X, t.Y)
		m.AddNamedLocation("ladder_up", p)
		m.SetTile(p, d1StairsUp())
	}
	if sl == d1.SLVileBetrayer {
		m.AddNamedLocation("ladder_up", toCell(lvl.View[0], lvl.View[1]))
		m.AddNamedLocation("exit", pieceCell(D1VilePortal))
	}
}

// placeD1Monsters ports the actor side of InitMonsters: everything D1's generator placed, except the four reserved
// golem slots and skeletons still hidden inside a sarcophagus or barrel (those rise when the container is opened).
func (g *MapGenerator) placeD1Monsters(m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level) {
	if g.walledIn == nil {
		g.walledIn = map[string][]*Actor{}
	}
	g.walledIn[m.GetName()] = nil
	delete(g.rosters, m.GetName())
	for i, mon := range lvl.Monsters {
		if mon == nil || mon.Reserved || mon.Hidden {
			continue
		}
		a := g.factory.NewD1MonsterFromLevel(mon)
		a.d1Monster.SpawnIndex = i
		p := toCell(mon.X, mon.Y)
		a.SetDrawPosition(p.ToPointF())
		if !canPlaceD1Monster(m, p) {
			// walled in, like Lazarus and his helpers until their rooms open (changeD1Map places them then), or
			// inside what our simplified terrain treats as solid (see d1IsLavaMega etc.)
			a.SetMapPosition(p)
			g.walledIn[m.GetName()] = append(g.walledIn[m.GetName()], a)
			continue
		}
		m.AddActor(a, p)
		g.addSpawn(m, a, func() *Actor {
			b := g.factory.NewD1MonsterFromLevel(mon)
			b.d1Monster.SpawnIndex = i
			return b
		}, func(b *Actor) bool { return b.d1Monster != nil && b.d1Monster.SpawnIndex == i })
	}
}

// canPlaceD1Monster is CanPlaceActorHere minus D1's objects. D1 runs InitMonsters before InitObjects, and set pieces
// like the Poisoned Water candles (addCandles) go down without checking dMonster, so a monster can own the cell of
// a solid object. Anything placing monsters after objects exist (respawn, walled-in release, save restore) must
// allow the same.
func canPlaceD1Monster(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point) bool {
	if m.CanPlaceActorHere(p) {
		return true
	}
	if !m.Contains(p) || !m.IsObjectAt(p) {
		return false
	}
	_, isD1 := m.ObjectAt(p).(*D1Object)
	return isD1 && m.GetCell(p).TileType.IsWalkable && !m.IsActorAt(p) && !m.IsObviousHazardAt(p)
}

// placeD1Objects ports the object side of InitObjects/CreateThemeRooms: everything D1's generator placed. Doors
// become terrain + an H&H Door object in addBasicTiles; wall traps stay hidden in the walls (d1_trap.go).
func (g *MapGenerator) placeD1Objects(m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level) {
	for _, o := range lvl.Objects {
		if d1DoorKinds[o.Kind] {
			if door, ok := d1DoorAt(m, toCell(o.X, o.Y)); ok {
				x, y := o.X, o.Y
				door.onOpen = func() { g.factory.springD1WallTraps(m, x, y) }
				door.D1 = o
				door.rogue = func() bool {
					p := g.factory.State.GetPlayer()
					return p != nil && p.hero != nil && d1.HasClassFlag(p.hero.Class, d1.ClassTrapSense)
				}
			}
			continue
		}
		if o.Kind == d1.ObjTRAPL || o.Kind == d1.ObjTRAPR {
			continue
		}
		p := toCell(o.X, o.Y)
		if !m.Contains(p) || m.IsObjectAt(p) {
			continue
		}
		d1PlaceObject(g.factory, m, lvl, o, p)
	}
}

// placeD1Items ports the item side of InitItems/AddInitItems/CreateThemeRooms: the floor items D1's generator
// placed (potions lying around, theme-room treasure, quest items).
func (g *MapGenerator) placeD1Items(m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level) {
	for _, it := range lvl.Items {
		p := toCell(it.X, it.Y)
		if !m.CanPlaceItemHere(p) {
			if cells := freeCellsNear(m, p, 1, m.CanPlaceItemHere); len(cells) > 0 {
				p = cells[0]
			} else {
				continue
			}
		}
		m.AddItem(g.factory.NewD1Item(it.Item), p)
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
		for _, d := range D1Dirs {
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

// D1SameRoom ports the AIs' dTransVal comparison: both cells lie in the same transparency region (room) of the
// current D1 level. Anywhere without D1 level data counts as one room.
func D1SameRoom(s *State, a, b geometry.Point) bool {
	lvl := s.mapGen.d1Levels[s.GetMap().GetName()]
	if lvl == nil {
		return true
	}
	at := func(p geometry.Point) int8 {
		x, y := p.X+d1.CellOrigin, p.Y+d1.CellOrigin
		if x < 0 || y < 0 || x >= len(lvl.Trans) || y >= len(lvl.Trans[0]) {
			return -1
		}
		return lvl.Trans[x][y]
	}
	return at(a) == at(b)
}
