package common

import (
	"math/rand"
	"rpg/d1"
	"rpg/defs"
	"rpg/dungen"
	"rpg/geometry"
	"rpg/gridmap"
	"slices"
)

// SetSeed sets the game seed levels are generated from (decision 11: levels rebuild from their seed).
func (g *MapGenerator) SetSeed(seed int64) { g.seed = seed }

// SetSnapshots sets saved level state applied after a level is generated.
func (g *MapGenerator) SetSnapshots(snapshots map[string]*LevelSnapshot) { g.snapshots = snapshots }

// generateD1Map builds a dungeon level from D1's generator (Cathedral for now).
func (g *MapGenerator) generateD1Map(targetMap string, dlvl int) *gridmap.GridMap[*Actor, *Item, Object] {
	rng := rand.New(rand.NewSource(g.seed*1000003 + int64(dlvl)))
	gen := g.factory.D1()
	gameRng := gen.Rng
	gen.Rng = rng // monster HP rolls must come from the level seed too
	defer func() { gen.Rng = gameRng }()

	lvl := d1.GenerateL1(rng, dlvl)

	layout := dungen.NewDungeonMap(d1.LevelSize, d1.LevelSize)
	for x := 0; x < d1.LevelSize; x++ {
		for y := 0; y < d1.LevelSize; y++ {
			switch lvl.Cells[x][y] {
			case d1.CellFloor:
				layout.SetRoom(x, y)
			case d1.CellDoor:
				layout.SetDoor(x, y)
			}
		}
	}

	m := gridmap.NewEmptyMap[*Actor, *Item, Object](d1.LevelSize, d1.LevelSize)
	m.SetName(targetMap)
	levelDef := defs.GetLevelDefinition(g.randomSource, dlvl)
	g.addBasicTiles(m, layout, levelDef.GetTilemapForBasicTiles(layout, dlvl))

	up := geometry.Point{X: lvl.StairsUp[0], Y: lvl.StairsUp[1]}
	m.SetTile(up, gridmap.Tile{DefinedIcon: defs.IconStairsUp, DefinedDescription: "stairs leading up", IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsUp})
	m.AddNamedLocation("ladder_up", up)
	upTarget := gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl - 1), TargetLocation: "ladder_down"}
	if dlvl == 1 {
		upTarget = gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "to_cathedral"}
	}
	m.AddTransitionAt(up, upTarget)

	dn := geometry.Point{X: lvl.StairsDn[0], Y: lvl.StairsDn[1]}
	m.SetTile(dn, gridmap.Tile{DefinedIcon: defs.IconStairsDown, DefinedDescription: "stairs leading down", IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsDown})
	m.AddNamedLocation("ladder_down", dn)
	m.AddTransitionAt(dn, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl + 1), TargetLocation: "ladder_up"})

	// CreateLevel's order: themes, monsters, objects, items, then the theme rooms' contents.
	types := gen.LevelMonsterTypes(dlvl)
	placer := gen.NewPlacer(&d1World{f: g.factory, m: m}, lvl, dlvl, types)
	placer.InitThemes()
	placer.HoldThemeRooms()
	g.spawnD1Monsters(rng, m, lvl, dlvl, types, []geometry.Point{up, dn})
	placer.InitObjects()
	// AddInitItems: 3-5 healing or mana potions lie around.
	for n := rng.Intn(3) + 3; n > 0; n-- {
		idx := d1.IdxMana
		if rng.Intn(2) != 0 {
			idx = d1.IdxHeal
		}
		for try := 0; try < 1000; try++ {
			if p := (geometry.Point{X: rng.Intn(d1.LevelSize), Y: rng.Intn(d1.LevelSize)}); m.CanPlaceItemHere(p) && !m.IsActorAt(p) && !lvl.Held[p.X][p.Y] {
				m.AddItem(g.factory.NewD1Item(gen.PlainItem(idx)), p)
				break
			}
		}
	}
	placer.CreateThemeRooms()
	if snap := g.snapshots[targetMap]; snap != nil {
		snap.apply(g.factory, m)
	}
	PlaceD1Portal(g.factory.state, m)

	m.SetDisplayName(levelDef.LevelName)
	return m
}

// Uniques tied to quests that aren't active (Gharbad, Zhar, Snotspill, Lachdanan, Warlord) stay out, like D1's PlaceUniques.
var d1QuestUniques = map[int]bool{0: true, 2: true, 3: true, 7: true, 8: true}

// spawnD1Monsters ports InitMonsters/PlaceUniques/PlaceGroup.
func (g *MapGenerator) spawnD1Monsters(rng *rand.Rand, m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level, dlvl int, types []int, avoid []geometry.Point) {
	gen := g.factory.D1()
	if len(types) == 0 {
		return
	}
	walkable := 0
	for x := 0; x < d1.LevelSize; x++ {
		for y := 0; y < d1.LevelSize; y++ {
			if m.IsWalkable(geometry.Point{X: x, Y: y}) {
				walkable++
			}
		}
	}
	// ponytail: D1 keeps spawns out of the stairs' line of sight (radius 15); we use a plain radius.
	free := func(p geometry.Point) bool {
		return m.CanPlaceActorHere(p) && minDist(avoid, p) > 10 && !lvl.Held[p.X][p.Y]
	}
	randomFree := func() (geometry.Point, bool) {
		for try := 0; try < 1000; try++ {
			p := geometry.Point{X: rng.Intn(d1.LevelSize), Y: rng.Intn(d1.LevelSize)}
			if free(p) {
				return p, true
			}
		}
		return geometry.Point{}, false
	}
	spawned := 0
	place := func(a *Actor, p geometry.Point) {
		a.d1Monster.SpawnIndex = spawned
		spawned++
		a.SetDrawPosition(p.ToPointF())
		m.AddActor(a, p)
	}
	group := func(mt, num int, leader *Actor, leaderf int) int {
		var cells []geometry.Point
		if leader != nil {
			cells = freeCellsNear(m, leader.MapPosition(), num, free)
		} else if origin, ok := randomFree(); ok {
			cells = append([]geometry.Point{origin}, freeCellsNear(m, origin, num-1, free)...)
		}
		n := min(num, len(cells))
		for i := 0; i < n; i++ {
			a := g.factory.NewD1Monster(mt)
			if leader != nil {
				MakeD1Minion(a, leader, leaderf)
			}
			place(a, cells[i])
		}
		return n
	}

	for u, um := range gen.D.UniqueMonsters {
		mt := gen.D.MonsterByID(um.Type)
		if um.Level != dlvl || d1QuestUniques[u] || !slices.Contains(types, mt) {
			continue
		}
		if p, ok := randomFree(); ok {
			leader := g.factory.NewD1UniqueMonster(u)
			place(leader, p)
			if um.UnqAttr&1 != 0 {
				group(mt, 8, leader, um.UnqAttr)
			}
		}
	}
	na := d1.MonsterCount(walkable)
	for placed, tries := 0, 0; placed < na && tries < 1000; tries++ {
		placed += group(types[rng.Intn(len(types))], gen.GroupSize(dlvl), nil, 0)
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
