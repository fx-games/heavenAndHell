package common

import (
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/util"
	"slices"
	"testing"
)

// Bone Spirit reaching an empty target turns to the closest actor; with none it flies straight on.
func TestHomingBoltAoE(t *testing.T) {
	m := gridmap.NewEmptyMap[*Actor, *Item, Object](30, 30)
	zone := HomingBoltAoE(func() *gridmap.GridMap[*Actor, *Item, Object] { return m }, util.Arguments{"112"})
	from, target := geometry.Point{X: 2, Y: 10}, geometry.Point{X: 8, Y: 10}
	straight := zone.GetHitPositions(from, target)
	if !slices.Contains(straight, geometry.Point{X: 12, Y: 10}) {
		t.Fatalf("with no one about it should fly on past the target: %v", straight)
	}
	victim := geometry.Point{X: 8, Y: 13}
	m.AddActor(&Actor{}, victim)
	cells := zone.GetHitPositions(from, target)
	if cells[len(cells)-1] != victim {
		t.Fatalf("it should end on the closest actor %v: %v", victim, cells)
	}
}
