package common

import (
	"fmt"

	"rpg/src/defs"
	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/gridmap"
)

// Level is the mod's record for level id (docs/LEVELS.md), the one way to ask about a level; the zero Level when it
// declares none.
func (s *State) Level(id string) dungeon.Level { return s.mapGen.levels[id] }

// CurrentLevel is the record of the level the player is on.
func (s *State) CurrentLevel() dungeon.Level {
	if s.GetMap() == nil {
		return dungeon.Level{}
	}
	return s.Level(s.GetMap().GetName())
}

// StartLevel is the level a new game begins on.
func (s *State) StartLevel() dungeon.Level {
	for _, l := range s.mapGen.Levels() {
		if l.Start {
			return l
		}
	}
	return dungeon.Level{}
}

// Levels is every level the mod declares, in levels.rec's order.
func (s *State) Levels() []dungeon.Level { return s.mapGen.Levels() }

// Map, SetMap and HasMap are the one store of built maps, by level id.
func (s *State) Map(id string) *gridmap.GridMap[*Actor, *Item, Object] { return s.maps[id] }
func (s *State) SetMap(id string, m *gridmap.GridMap[*Actor, *Item, Object]) {
	s.maps[id] = m
}
func (s *State) HasMap(id string) bool { _, ok := s.maps[id]; return ok }

// Maps is every level built this game, by id.
func (s *State) Maps() map[string]*gridmap.GridMap[*Actor, *Item, Object] { return s.maps }

// Visit records that the hero reached level id: the deepest level and the deepest town are what "deepest"
// conditions, the recall spell and the town portal go by (the portal: the last visited of equally deep towns).
func (s *State) Visit(id string) {
	l := s.Level(id)
	if l.Map == "" {
		return
	}
	if l.Town {
		if s.deepestTown == "" || l.Depth >= s.Level(s.deepestTown).Depth {
			s.deepestTown = id
		}
		return
	}
	if s.deepestLevel == "" || l.Depth > s.Level(s.deepestLevel).Depth {
		s.deepestLevel = id
	}
}

// Deepest is the greatest depth the hero reached below a town (0 before the first step down).
func (s *State) Deepest() int { return s.Level(s.deepestLevel).Depth }

// DeepestLevel and DeepestTown are the ids Visit keeps; SetVisited restores them from a save.
func (s *State) DeepestLevel() string { return s.deepestLevel }
func (s *State) DeepestTown() string {
	if s.deepestTown == "" {
		return s.StartLevel().Map
	}
	return s.deepestTown
}
func (s *State) SetVisited(level, town string) { s.deepestLevel, s.deepestTown = level, town }

// InTown: the player stands on a level its record calls a town.
func (s *State) IsPlayerInTown() bool { return s.CurrentLevel().Town }

// exitTiles are what an exit's spot turns into when it opens (exit_tile).
func exitTiles() map[string]gridmap.Tile {
	return map[string]gridmap.Tile{
		"stairs_down": stairsDown(),
		"stairs_up":   {DefinedIcon: defs.IconStairsUp, DefinedDescription: "stairs leading up", IsWalkable: true, IsTransparent: true, IsMissilePassable: true, Special: gridmap.SpecialTileStairsUp},
		"red_portal":  redPortal,
	}
}

// wireExits lays the level's declared exits on m: each open one becomes a transition on its location. It runs when
// a level is built and whenever the quests change, and is idempotent. An exit whose location the level lacks
// (a quest entrance on a game that rolled the quest away) is skipped.
func (s *State) wireExits(m *gridmap.GridMap[*Actor, *Item, Object]) {
	for name, p := range s.Level(m.GetName()).Locations {
		m.AddNamedLocation(name, geometry.Point{X: p[0], Y: p[1]})
	}
	for _, e := range s.Level(m.GetName()).Exits {
		p := m.GetNamedLocation(e.Location)
		if m.GetNamedLocationByPos(p) != e.Location {
			continue
		}
		if _, done := m.GetTransitionAt(p); done {
			continue
		}
		if e.OpenWhen != "" && !s.factory.questCondition(e.OpenWhen) {
			continue
		}
		if t, ok := exitTiles()[e.Tile]; ok {
			m.SetTile(p, t)
		} else if e.Tile != "" {
			panic(fmt.Sprintf("levels.rec: %s: unknown exit_tile %q", m.GetName(), e.Tile))
		} else if m.GetCell(p).TileType.Special == gridmap.SpecialTileNone {
			m.SetTile(p, m.GetCell(p).TileType.WithSpecial(gridmap.SpecialTileStairsDown))
		}
		m.AddTransitionAt(p, gridmap.Transition{TargetMap: e.Target, TargetLocation: e.Arrival})
	}
}
