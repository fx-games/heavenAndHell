package common

import (
	"rpg/d1"
	"rpg/geometry"
	"rpg/gridmap"
	"testing"
)

// Auto-aim skips a monster behind a bookcase: the bolt would stop on the bookcase.
func TestD1BoltReaches(t *testing.T) {
	m := gridmap.NewEmptyMap[*Actor, *Item, Object](20, 20)
	from, to := geometry.Point{X: 2, Y: 5}, geometry.Point{X: 8, Y: 5}
	if !d1BoltReaches(m, from, to) {
		t.Fatal("open floor blocked")
	}
	m.AddObject(&D1Object{O: &d1.Object{Kind: d1.ObjBOOKCASEL}}, geometry.Point{X: 5, Y: 5})
	if d1BoltReaches(m, from, to) {
		t.Fatal("bolt flies through a bookcase")
	}
}
