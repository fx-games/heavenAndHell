package common

import (
	"rpg/d1"
	"rpg/defs"
	"rpg/dungen"
	"rpg/geometry"
	"rpg/gridmap"
	"slices"
)

// generateD1Map builds a dungeon level from D1's generator (Cathedral for now).
func (g *MapGenerator) generateD1Map(targetMap string, dlvl int) *gridmap.GridMap[*Actor, *Item, Object] {
	lvl := d1.GenerateL1(g.randomSource, dlvl)

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

	g.spawnD1Monsters(m, dlvl, []geometry.Point{up, dn})
	// AddInitItems: 3-5 healing or mana potions lie around.
	for n := g.randomSource.Intn(3) + 3; n > 0; n-- {
		idx := 25 // IDI_MANA
		if g.randomSource.Intn(2) != 0 {
			idx = 24 // IDI_HEAL
		}
		for try := 0; try < 1000; try++ {
			if p := (geometry.Point{X: g.randomSource.Intn(d1.LevelSize), Y: g.randomSource.Intn(d1.LevelSize)}); m.CanPlaceItemHere(p) && !m.IsActorAt(p) {
				m.AddItem(g.factory.NewD1Item(g.factory.D1().PlainItem(idx)), p)
				break
			}
		}
	}
	m.SetDisplayName(levelDef.LevelName)
	return m
}

// Uniques tied to quests that aren't active (Gharbad, Zhar, Snotspill, Lachdanan, Warlord) stay out, like D1's PlaceUniques.
var d1QuestUniques = map[int]bool{0: true, 2: true, 3: true, 7: true, 8: true}

// spawnD1Monsters ports InitMonsters/PlaceUniques/PlaceGroup.
func (g *MapGenerator) spawnD1Monsters(m *gridmap.GridMap[*Actor, *Item, Object], dlvl int, avoid []geometry.Point) {
	gen := g.factory.D1()
	types := gen.LevelMonsterTypes(dlvl)
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
	free := func(p geometry.Point) bool { return m.CanPlaceActorHere(p) && minDist(avoid, p) > 10 }
	randomFree := func() (geometry.Point, bool) {
		for try := 0; try < 1000; try++ {
			p := geometry.Point{X: g.randomSource.Intn(d1.LevelSize), Y: g.randomSource.Intn(d1.LevelSize)}
			if free(p) {
				return p, true
			}
		}
		return geometry.Point{}, false
	}
	place := func(a *Actor, p geometry.Point) {
		a.SetDrawPosition(p.ToPointF())
		m.AddActor(a, p)
	}
	group := func(mt, num int, leader *Actor, leaderf int) int {
		var cells []geometry.Point
		if leader != nil {
			cells = m.GetFreeCellsForDistribution(leader.MapPosition(), num, free)
		} else if origin, ok := randomFree(); ok {
			cells = append([]geometry.Point{origin}, m.GetFreeCellsForDistribution(origin, num-1, free)...)
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
		placed += group(types[g.randomSource.Intn(len(types))], gen.GroupSize(dlvl), nil, 0)
	}
}
