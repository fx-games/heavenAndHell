package common

import (
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"testing"
)

// Auto-aim skips a monster behind a bookcase: the bolt would stop on the bookcase.
func TestBoltReaches(t *testing.T) {
	m := gridmap.NewEmptyMap[*Actor, *Item, Object](20, 20)
	from, to := geometry.Point{X: 2, Y: 5}, geometry.Point{X: 8, Y: 5}
	if !boltReaches(m, from, to) {
		t.Fatal("open floor blocked")
	}
	m.AddObject(&Prop{O: &rules.Object{Kind: rules.ObjBOOKCASEL}}, geometry.Point{X: 5, Y: 5})
	if boltReaches(m, from, to) {
		t.Fatal("bolt flies through a bookcase")
	}
}
