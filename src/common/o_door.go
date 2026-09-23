package common

import (
	"rpg/src/geometry"
	"rpg/src/rules"
)

type Door struct {
	position geometry.Point

	closedIcon int32
	openIcon   int32

	isOpen   bool
	onOpen   func()        // D1: the wall traps aimed at this door fire (Obj_Trap)
	Prop     *rules.Object // the D1 door object, nil outside D1 levels
	rogue    func() bool
	Disarmed bool // the rogue disarmed its trap (TryDisarm), kept in saves
}

func (d *Door) Turn() {

}

// ID is "door" or, open, "door_open".
func (d *Door) ID() string {
	if d.IsClosed() {
		return "door"
	}
	return "door_open"
}

func (d *Door) Name() string {
	if d.Prop != nil && d.Prop.TrapFlag && d.rogue != nil && d.rogue() {
		return "a trapped door" // GetObjectStr: only a rogue sees the trap
	}
	return "a door"
}
func (d *Door) ActorWalkedOn(actor *Actor) {
	d.Open()
}
func (d *Door) ActorBumpedInto(actor *Actor) {

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
