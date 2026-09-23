package common

import (
	"rpg/src/geometry"
)

type Object interface {
	MapPosition() geometry.Point
	Icon(tick uint64) int32
	SetMapPosition(geometry.Point)
	IsWalkable(person *Actor) bool
	IsTransparent() bool
	IsPassableForProjectile() bool
	Turn()
	ActorWalkedOn(actor *Actor)
	ActorBumpedInto(actor *Actor)
	Name() string
	ID() string // what data files call this kind of object (a prop's OBJ_ name); "" for none
}
