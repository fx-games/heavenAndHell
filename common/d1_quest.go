package common

import (
	"fmt"
	"io"
	"math/rand"
	"path"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/util"
	"strings"
)

// Quest flags (saved with the other state flags) and the set levels (decisions 2 and 15).
const (
	QuestButcher = "quest:butcher"
	QuestLeoric  = "quest:leoric"
	QuestZhar    = "quest:zhar"
	QuestWarlord = "quest:warlord"
	QuestStaff   = "quest:staff_to_cain"
	QuestLazarus = "quest:lazarus"
	QuestDiablo  = "quest:diablo"

	D1LeoricLevel  = "!gen_leoric_3"   // King Leoric's Tomb, entered from level 3
	D1LazarusLevel = "!gen_lazarus_15" // Lazarus' lair, entered through the red portal on level 15

	d1StaffName        = "Staff of Lazarus"
	d1SealDescription  = "a sealed wall"
	d1QuestObjectIndex = 1000 // quest objects' save indices, clear of the placer's
)

// D1Quests is the quest log (D1's questlog): name, the flag set once done, one line of description.
var D1Quests = []struct{ Name, Flag, Text string }{
	{"The Butcher", QuestButcher, "Slay the Butcher on level 2."},
	{"The Curse of King Leoric", QuestLeoric, "Find King Leoric's Tomb on level 3 and slay the Skeleton King."},
	{"Zhar the Mad", QuestZhar, "Slay Zhar the Mad on level 8."},
	{"Warlord of Blood", QuestWarlord, "Slay the Warlord of Blood on level 13."},
	{"Archbishop Lazarus", QuestLazarus, "Bring the Staff of Lazarus from level 15 to Cain, then slay Lazarus beyond the red portal."},
	{"Diablo", QuestDiablo, "Pull the four levers on level 16 and slay Diablo."},
}

// d1QuestKills maps UniqMonst indices to the quest their death completes (CheckQuestKill).
var d1QuestKills = map[int]string{9: QuestButcher, 1: QuestLeoric, 2: QuestZhar, 8: QuestWarlord, 4: QuestLazarus}

var (
	d1StairsDown = gridmap.Tile{DefinedIcon: defs.IconStairsDown, DefinedDescription: "stairs leading down", IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsDown}
	d1RedPortal  = gridmap.Tile{DefinedIcon: 72, DefinedDescription: "a red portal", IsWalkable: true, IsTransparent: true}
)

func (s *State) D1QuestLog() []string {
	lines := []string{"Quests", ""}
	for _, q := range D1Quests {
		status := "active"
		if s.GetFlags().IsSet(q.Flag) {
			status = "done"
		}
		lines = append(lines, fmt.Sprintf("%s - %s", q.Name, status), "  "+q.Text)
	}
	return lines
}

// d1QuestBosses are the uniques PlaceQuestMonsters puts on a level.
func d1QuestBosses(mapName string) []int {
	switch mapName {
	case defs.LevelNameFromDepth(2):
		return []int{9}
	case D1LeoricLevel:
		return []int{1}
	case defs.LevelNameFromDepth(8):
		return []int{2}
	case defs.LevelNameFromDepth(13):
		return []int{8}
	case D1LazarusLevel:
		return []int{4, 5, 6}
	}
	return nil
}

// randomQuestSpot picks a random open cell (all 8 neighbours walkable, so an object there cuts nothing off) that is
// reachable from the stairs up and away from them.
func randomQuestSpot(rng *rand.Rand, m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level, keep func(geometry.Point) bool) (geometry.Point, bool) {
	up := m.GetNamedLocation("ladder_up")
	cells := freeCellsNear(m, up, d1.LevelSize*d1.LevelSize, func(p geometry.Point) bool {
		_, isTransition := m.GetTransitionAt(p)
		return m.CanPlaceActorHere(p) && !m.IsObjectAt(p) && !isTransition && !lvl.Held[p.X][p.Y] &&
			geometry.DistanceChebyshev(p, up) > 10 && len(m.GetFilteredNeighbors(p, m.IsTileWalkable)) == 8 && (keep == nil || keep(p))
	})
	if len(cells) == 0 {
		return geometry.Point{}, false
	}
	return cells[rng.Intn(len(cells))], true
}

// placeD1QuestExits runs before monsters are placed: the stairs to Leoric's tomb on level 3, the red portal's spot
// on level 15 (DrawSkelKing, the Vile1 set piece). It returns the cells to keep monsters away from.
func placeD1QuestExits(rng *rand.Rand, m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level) []geometry.Point {
	loc := map[string]string{defs.LevelNameFromDepth(3): "leoric", defs.LevelNameFromDepth(15): "red_portal"}[m.GetName()]
	if loc == "" {
		return nil
	}
	p, ok := randomQuestSpot(rng, m, lvl, nil)
	if !ok {
		return nil
	}
	m.AddNamedLocation(loc, p)
	if loc == "leoric" {
		tile := d1StairsDown
		tile.DefinedDescription = "the way to King Leoric's Tomb"
		m.SetTile(p, tile)
		m.AddTransitionAt(p, gridmap.Transition{TargetMap: D1LeoricLevel, TargetLocation: "ladder_up"})
	}
	return []geometry.Point{p}
}

// placeD1QuestObjects adds the Staff of Lazarus' stand on level 15 (AddLazStand) and Diablo's seal with one lever
// per quarter of level 16.
func (g *MapGenerator) placeD1QuestObjects(rng *rand.Rand, m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level) {
	w := &d1World{f: g.factory, m: m}
	add := func(kind, index int, keep func(geometry.Point) bool) {
		p, ok := randomQuestSpot(rng, m, lvl, keep)
		if !ok {
			p, ok = randomQuestSpot(rng, m, lvl, nil)
		}
		if ok {
			w.AddObject(&d1.Object{Kind: kind, X: p.X, Y: p.Y, Skeleton: -1, Active: true, Index: index})
		}
	}
	switch m.GetName() {
	case defs.LevelNameFromDepth(15):
		add(d1.ObjLazStand, d1QuestObjectIndex, nil)
	case defs.LevelNameFromDepth(16):
		c := geometry.Point{X: lvl.Chamber[0], Y: lvl.Chamber[1]}
		for dx := -2; dx <= 2; dx++ {
			for dy := -2; dy <= 2; dy++ {
				if p := c.Add(geometry.Point{X: dx, Y: dy}); geometry.DistanceChebyshev(p, c) == 2 {
					tile := m.GetCell(p).TileType
					tile.DefinedDescription = d1SealDescription
					m.SetTile(p, tile)
				}
			}
		}
		for q := 0; q < 4; q++ {
			add(d1.ObjLever, d1QuestObjectIndex+q, func(p geometry.Point) bool {
				return (p.X < d1.LevelSize/2) == (q&1 == 0) && (p.Y < d1.LevelSize/2) == (q&2 == 0)
			})
		}
	}
}

// UpdateD1Quests opens what the quest flags unlock on a level. It is idempotent and runs on arrival and whenever a
// flag changes: the red portal on 15 once Cain has the staff, the portal back from Lazarus' lair and the stairs down
// on 15 once Lazarus is dead (CheckQuestKill), Diablo's chamber once every lever on 16 is pulled (OperateLever).
func UpdateD1Quests(s *State, m *gridmap.GridMap[*Actor, *Item, Object]) {
	flags := s.GetFlags()
	open := func(loc string, tile gridmap.Tile, to gridmap.Transition) {
		p := m.GetNamedLocation(loc)
		if _, done := m.GetTransitionAt(p); done || m.GetNamedLocationByPos(p) != loc {
			return
		}
		m.SetTile(p, tile)
		m.AddTransitionAt(p, to)
	}
	switch m.GetName() {
	case defs.LevelNameFromDepth(15):
		if flags.IsSet(QuestStaff) && !flags.IsSet(QuestLazarus) {
			open("red_portal", d1RedPortal, gridmap.Transition{TargetMap: D1LazarusLevel, TargetLocation: "ladder_up"})
		}
		if flags.IsSet(QuestLazarus) {
			open("ladder_down", d1StairsDown, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(16), TargetLocation: "ladder_up"})
		}
	case D1LazarusLevel:
		if flags.IsSet(QuestLazarus) {
			open("ladder_up", d1RedPortal, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(15), TargetLocation: "red_portal"})
		}
	case defs.LevelNameFromDepth(16):
		levers, pulled := 0, 0
		for _, obj := range m.Objects() {
			if o, ok := obj.(*D1Object); ok && o.O.Kind == d1.ObjLever {
				levers++
				if !o.O.Active {
					pulled++
				}
			}
		}
		if levers == 0 || pulled < levers {
			return
		}
		var seal []geometry.Point
		m.IterAll(func(p geometry.Point, c gridmap.MapCell[*Actor, *Item, Object]) {
			if c.TileType.DefinedDescription == d1SealDescription {
				seal = append(seal, p)
			}
		})
		for _, p := range seal {
			m.SetTile(p, gridmap.Tile{DefinedIcon: defs.IconDungeonRedEarthFloor, DefinedDescription: "a floor", IsWalkable: true, IsTransparent: true})
		}
	}
}

// d1QuestKill ports CheckQuestKill: a quest boss' death completes its quest, Diablo's ends the game.
func d1QuestKill(ctx ActionContext, mon *D1Monster) {
	state := ctx.GameState
	flag := ""
	for u, f := range d1QuestKills {
		if mon.Unique == &state.factory.d1.D.UniqueMonsters[u] {
			flag = f
		}
	}
	if mon.Data.ID == "MT_DIABLO" {
		flag = QuestDiablo
	}
	if flag == "" || state.GetFlags().IsSet(flag) {
		return
	}
	state.GetFlags().SetTrue(flag)
	UpdateD1Quests(state, state.GetMap())
	for _, q := range D1Quests {
		if q.Flag == flag {
			ctx.Msg(fmt.Sprintf("Quest completed: %s", q.Name))
		}
	}
	if flag == QuestDiablo {
		if ui := ctx.UserInterface(); ui != nil {
			ui.OpenTextWindow(util.ToLabelText(d1EndText()))
		}
	}
}

func d1EndText() []string {
	r := util.MustOpenReader(path.Join("assets", "endMessage.txt"))
	defer r.Close()
	data, _ := io.ReadAll(r)
	return strings.Split(string(data), "\n")
}

// D1GiveStaffToCain: bumping Cain while carrying the Staff of Lazarus hands it over (decision 15: one fixed line).
func D1GiveStaffToCain(s *State, msg func(string)) bool {
	inv := s.GetPlayer().GetInventory()
	for _, it := range inv.Items() {
		if it.d1 != nil && it.d1.Name == d1StaffName {
			inv.Remove(it)
			s.GetFlags().SetTrue(QuestStaff)
			msg("Cain: \"The staff of Archbishop Lazarus! He waits beyond a red portal on level 15. Stop him.\"")
			return true
		}
	}
	return false
}
