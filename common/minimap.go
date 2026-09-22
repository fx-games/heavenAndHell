package common

import (
	"rpg/geometry"
	"rpg/gridmap"
)

// defaultMiniMapColors are the built-in minimap colors; config.rec's
// "MiniMapColor: <name>=rrggbb" lines override them by name.
var defaultMiniMapColors = map[string][4]byte{
	"player":        {100, 255, 100, 255},
	"unexplored":    {10, 10, 10, 255},
	"stairsUp":      {255, 204, 0, 255},
	"stairsDown":    {255, 255, 255, 255},
	"waterUnlit":    {43, 56, 97, 255},
	"floorUnlit":    {35, 30, 51, 255},
	"wallUnlit":     {85, 95, 112, 255},
	"hostileActor":  {255, 0, 0, 255},
	"peacefulActor": {220, 120, 75, 255},
	"water":         {86, 112, 194, 255},
	"floor":         {77, 60, 102, 255},
	"wall":          {170, 191, 224, 255},
}

// miniMapColor resolves a minimap color name against config.rec, falling back to the built-in.
func miniMapColor(cfg *Configuration, name string) [4]byte {
	if fromConfig, ok := cfg.MiniMapColors[name]; ok {
		return fromConfig
	}
	return defaultMiniMapColors[name]
}

// MiniMapColor is the minimap's color for a cell, shared by the tile UI's minimap and the minimap feed.
func MiniMapColor(state *State, cfg *Configuration, mapPos geometry.Point) [4]byte {
	currentMap := state.GetMap()
	if state.GetPlayer().MapPosition() == mapPos {
		return miniMapColor(cfg, "player")
	}
	if !currentMap.IsExplored(mapPos) {
		return miniMapColor(cfg, "unexplored")
	}
	if currentMap.IsSpecialAt(mapPos, gridmap.SpecialTileStairsUp) {
		return miniMapColor(cfg, "stairsUp")
	}
	if currentMap.IsSpecialAt(mapPos, gridmap.SpecialTileStairsDown) {
		return miniMapColor(cfg, "stairsDown")
	}
	if !state.GetPlayer().CanSee(mapPos) {
		if currentMap.IsSpecialAt(mapPos, gridmap.SpecialTileWater) {
			return miniMapColor(cfg, "waterUnlit")
		}
		if currentMap.IsWalkable(mapPos) {
			return miniMapColor(cfg, "floorUnlit")
		}
		return miniMapColor(cfg, "wallUnlit")
	}
	if currentMap.IsActorAt(mapPos) && currentMap.ActorAt(mapPos).IsVisible() {
		actorAt := currentMap.ActorAt(mapPos)
		if actorAt.IsHostileTo(state.GetPlayer()) {
			return miniMapColor(cfg, "hostileActor")
		}
		return miniMapColor(cfg, "peacefulActor")
	}

	if currentMap.IsSpecialAt(mapPos, gridmap.SpecialTileWater) {
		return miniMapColor(cfg, "water")
	}

	if currentMap.IsWalkable(mapPos) {
		return miniMapColor(cfg, "floor")
	}
	return miniMapColor(cfg, "wall")
}
