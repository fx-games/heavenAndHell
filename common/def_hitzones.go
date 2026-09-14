package common

import (
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/util"
)

func OneTargetTile(gridMap func() *gridmap.GridMap[*Actor, *Item, Object], args util.Arguments) *AoEResolver {
	return &AoEResolver{
		GetHitPositions: func(origin, target geometry.Point) []geometry.Point {
			return []geometry.Point{target}
		},
		Description: []util.LabelText{util.LabelTextFromString("The tile you target")},
	}
}
