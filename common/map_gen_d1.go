package common

import (
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
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

// SetSnapshots sets saved level state applied after a level is generated.
func (g *MapGenerator) SetSnapshots(snapshots map[string]*LevelSnapshot) { g.snapshots = snapshots }

// toCell converts D1 piece coordinates (16..95) to our 0..79 cell grid.
func toCell(px, py int) geometry.Point {
	return geometry.Point{X: px - d1.CellOrigin, Y: py - d1.CellOrigin}
}

// d1DoorObjectKinds is repeated from map_gen.go's d1DoorKinds so this file doesn't need to import it twice; see
// there for why doors are terrain + an H&H Door object, not a D1Object.

// generateD1Map builds dungeon level dlvl (1-16), or a quest set level entered from it, with D1's own generators.
func (g *MapGenerator) generateD1Map(targetMap string, dlvl int) *gridmap.GridMap[*Actor, *Item, Object] {
	setLevel := d1SetLevelOf(targetMap)
	sl := d1.SLNone
	if setLevel != nil {
		sl = setLevel.SL
	}
	lvl, err := g.factory.D1().CreateLevel(g.d1Quests, g.d1Seeds, dlvl, sl, d1.EntryMain)
	if err != nil {
		panic(err)
	}
	if g.levels == nil {
		g.levels = map[string]*d1.Level{}
	}
	g.levels[targetMap] = lvl

	m := gridmap.NewEmptyMap[*Actor, *Item, Object](d1.LevelSize, d1.LevelSize)
	m.SetName(targetMap)
	g.addBasicTiles(m, lvl)

	if setLevel == nil {
		g.wireD1Triggers(m, lvl, dlvl)
		m.SetDisplayName(defs.D1LevelName(dlvl))
	} else {
		g.wireD1SetLevelReturn(m, lvl, setLevel, dlvl)
		m.SetDisplayName(setLevel.Name)
	}

	g.placeD1Monsters(m, lvl)
	g.placeD1Objects(m, lvl)
	g.placeD1Items(m, lvl)

	if snap := g.snapshots[targetMap]; snap != nil {
		snap.apply(g.factory, m)
	}
	for _, a := range m.Actors() {
		if a.d1Monster != nil {
			g.factory.initD1Talk(a)
		}
	}
	PlaceD1Portal(g.factory.State, m)
	return m
}

// setPiece is the level's quest set piece in megatiles: x, y, w, h (D1's setpc_*).
func (g *MapGenerator) setPiece(m *gridmap.GridMap[*Actor, *Item, Object]) [4]int {
	if lvl := g.levels[m.GetName()]; lvl != nil {
		return lvl.SetPiece
	}
	return [4]int{}
}

// D1TownGates are the town locations of the entrances to levels 1, 5, 9 and 13.
var D1TownGates = map[int]string{1: "to_cathedral", 5: "to_catacombs", 9: "to_caves", 13: "to_hell"}

// wireD1Triggers places the stairs/warp tiles D1's own generator positioned (Level.Triggers), plus the quest
// entrances on ordinary dungeon levels (CheckQuests: King Leoric's Tomb, the Chamber of Bone, the Poisoned Water
// Supply) and level 15's Lazarus portal spot (the portal itself opens later, from UpdateD1Quests).
func (g *MapGenerator) wireD1Triggers(m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level, dlvl int) {
	for _, t := range lvl.Triggers {
		p := toCell(t.X, t.Y)
		switch t.Msg {
		case d1.TrigPrevLevel:
			m.AddNamedLocation("ladder_up", p)
			up := gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl - 1), TargetLocation: "ladder_down"}
			if dlvl == 1 {
				up = gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: D1TownGates[1]}
			}
			m.SetTile(p, gridmap.Tile{DefinedIcon: defs.IconStairsUp, DefinedDescription: "stairs leading up", IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsUp})
			m.AddTransitionAt(p, up)
		case d1.TrigNextLevel:
			m.AddNamedLocation("ladder_down", p)
			m.SetTile(p, d1StairsDown)
			m.AddTransitionAt(p, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl + 1), TargetLocation: "ladder_up"})
		case d1.TrigTownWarpUp:
			m.AddNamedLocation("warp", p)
			warpTile := d1StairsDown
			warpTile.DefinedDescription = "a way back to town"
			m.SetTile(p, warpTile)
			m.AddTransitionAt(p, gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: D1TownGates[dlvl]})
		}
	}

	for _, sl := range D1SetLevels {
		q := g.d1Quests[sl.Quest]
		if q.Level != dlvl || q.Active == d1.QuestNotAvail {
			continue
		}
		p := toCell(q.X, q.Y)
		m.AddNamedLocation(sl.Entrance, p)
		if sl.SL == d1.SLVileBetrayer {
			m.AddNamedLocation("ladder_down", toCell(q.X+3, q.Y+3)) // L4PENTA2's stairs piece (370), InitL4Triggers
			continue
		}
		tile := d1StairsDown
		tile.DefinedDescription = "the way to " + sl.Name
		m.SetTile(p, tile)
		m.AddTransitionAt(p, gridmap.Transition{TargetMap: sl.Map, TargetLocation: "ladder_up"})
	}
}

// wireD1SetLevelReturn places where the player arrives in a quest set level, and its way back out (SetReturnLvlPos).
// Lazarus' Lair has no return trigger in D1 either - the way home only opens once Lazarus is dead (UpdateD1Quests),
// so only the named location is set there.
func (g *MapGenerator) wireD1SetLevelReturn(m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level, sl *D1SetLevel, dlvl int) {
	for _, t := range lvl.Triggers {
		if t.Msg != d1.TrigReturnLevel {
			continue
		}
		p := toCell(t.X, t.Y)
		m.AddNamedLocation("ladder_up", p)
		m.SetTile(p, gridmap.Tile{DefinedIcon: defs.IconStairsUp, DefinedDescription: "stairs leading up", IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsUp})
		m.AddTransitionAt(p, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl), TargetLocation: sl.Entrance})
	}
	if sl.SL == d1.SLVileBetrayer {
		m.AddNamedLocation("ladder_up", toCell(lvl.View[0], lvl.View[1]))
	}
}

// placeD1Monsters ports the actor side of InitMonsters: everything D1's generator placed, except the four reserved
// golem slots and skeletons still hidden inside a sarcophagus or barrel (those rise when the container is opened).
func (g *MapGenerator) placeD1Monsters(m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level) {
	for i, mon := range lvl.Monsters {
		if mon == nil || mon.Reserved || mon.Hidden {
			continue
		}
		a := g.factory.NewD1MonsterFromLevel(mon)
		a.d1Monster.SpawnIndex = i
		p := toCell(mon.X, mon.Y)
		if !m.CanPlaceActorHere(p) {
			continue // a monster D1 placed inside what our simplified terrain treats as solid (see d1IsLavaMega etc.)
		}
		a.SetDrawPosition(p.ToPointF())
		m.AddActor(a, p)
	}
}

// d1RenderedObjects are the D1Object kinds decision 12 keeps as interactive containers/fixtures (everything else -
// candles, banners, crosses, torture bodies, empty stands, traps - is decoration or deferred and is
// skipped). Door kinds are excluded: those become terrain + an H&H Door object in addBasicTiles.
var d1RenderedObjects = map[int]bool{
	d1.ObjL1LIGHT: true, d1.ObjLEVER: true, d1.ObjCHEST1: true, d1.ObjCHEST2: true, d1.ObjCHEST3: true,
	d1.ObjTCHEST1: true, d1.ObjTCHEST2: true, d1.ObjTCHEST3: true, d1.ObjSARC: true, d1.ObjBARREL: true,
	d1.ObjBARRELEX: true, d1.ObjSHRINEL: true, d1.ObjSHRINER: true, d1.ObjSKELBOOK: true, d1.ObjBOOKCASEL: true,
	d1.ObjBOOKCASER: true, d1.ObjBOOKSTAND: true, d1.ObjBLOODFTN: true, d1.ObjDECAP: true, d1.ObjPURIFYINGFTN: true,
	d1.ObjARMORSTAND: true, d1.ObjGOATSHRINE: true, d1.ObjCAULDRON: true, d1.ObjMURKYFTN: true, d1.ObjTEARFTN: true,
	d1.ObjWEAPONRACK: true, d1.ObjLAZSTAND: true, d1.ObjTORCHL: true, d1.ObjTORCHR: true, d1.ObjTORCHL2: true,
	d1.ObjTORCHR2: true, d1.ObjSWITCHSKL: true, d1.ObjMUSHPATCH: true, d1.ObjSIGNCHEST: true, d1.ObjBLINDBOOK: true,
	d1.ObjBLOODBOOK: true, d1.ObjSTEELTOME: true, d1.ObjPEDISTAL: true, d1.ObjBOOK2L: true, d1.ObjBOOK2R: true,
	d1.ObjSTORYBOOK: true, d1.ObjSLAINHERO: true,
}

// placeD1Objects ports the object side of InitObjects/CreateThemeRooms: everything D1's generator placed, at the
// exact kinds decision 12 keeps.
func (g *MapGenerator) placeD1Objects(m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level) {
	for _, o := range lvl.Objects {
		if !d1RenderedObjects[o.Kind] {
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
