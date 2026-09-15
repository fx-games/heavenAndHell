package common

import (
	"rpg/d1"
	"rpg/geometry"
)

type Door struct {
	position geometry.Point

	closedIcon int32
	openIcon   int32

	isOpen   bool
	onOpen   func()     // D1: the wall traps aimed at this door fire (Obj_Trap)
	D1       *d1.Object // the D1 door object, nil outside D1 levels
	rogue    func() bool
	Disarmed bool // the rogue disarmed its trap (TryDisarm), kept in saves
}

func (d *Door) Turn() {

}
func (d *Door) Name() string {
	if d.D1 != nil && d.D1.TrapFlag && d.rogue != nil && d.rogue() {
		return "a trapped door" // GetObjectStr: only a rogue sees the trap
	}
	return "a door"
}
func (d *Door) ActorWalkedOn(actor *Actor, startAnimating func() bool) {
	d.Open()
}
func (d *Door) ActorBumpedInto(actor *Actor, startAnimating func() bool) {

}

func (d *Door) MapPosition() geometry.Point {
	return d.position
}

func (d *Door) Icon(tick uint64) int32 {
	if d.isOpen {
		return d.openIcon
	}
	return d.closedIcon
}

func (d *Door) SetMapPosition(point geometry.Point) {
	d.position = point
}

func (d *Door) IsWalkable(person *Actor) bool {
	return true
}

func (d *Door) IsTransparent() bool {
	return d.isOpen
}

func (d *Door) IsPassableForProjectile() bool {
	return d.isOpen
}

func (d *Door) IsClosed() bool {
	return !d.isOpen
}

func (d *Door) Open() {
	if !d.isOpen && d.onOpen != nil {
		defer d.onOpen()
	}
	d.isOpen = true
}
func NewDoor(open, closed int32) *Door {
	return &Door{
		openIcon:   open,
		closedIcon: closed,
	}
}
