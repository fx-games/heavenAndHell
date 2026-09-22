package common

import (
	"rpg/src/geometry"
)

type BumpObject struct {
	position   geometry.Point
	icon       int32
	activation func(actor *Actor)
	name       string
}

func (d *BumpObject) Name() string {
	return d.name
}

func (d *BumpObject) Turn() {

}

func (d *BumpObject) ActorWalkedOn(actor *Actor) {

}

func (d *BumpObject) ActorBumpedInto(actor *Actor) {
	d.activation(actor)
}

func (d *BumpObject) MapPosition() geometry.Point {
	return d.position
}

func (d *BumpObject) Icon(tick uint64) int32 {
	return d.icon
}

func (d *BumpObject) SetMapPosition(point geometry.Point) {
	d.position = point
}

func (d *BumpObject) IsWalkable(person *Actor) bool {
	return false
}

func (d *BumpObject) IsTransparent() bool {
	return true
}

func (d *BumpObject) IsPassableForProjectile() bool {
	return true
}

func NewBumpObject(name string, icon int32, onActivation func(actor *Actor)) *BumpObject {
	return &BumpObject{
		name:       name,
		icon:       icon,
		activation: onActivation,
	}
}
