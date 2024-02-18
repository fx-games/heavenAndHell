package common

import (
	"image/color"
	"math"
	"rpg/defs"
	"rpg/geometry"
	"rpg/util"
	"strconv"
)

type DamageNumberAnimation struct {
	BaseAnimation
	targetPos        geometry.Point
	damageAmount     int
	damageColor      color.Color
	runTimeInSeconds float64
	ticksAlive       uint64
	ticksNeeded      uint64
}

func (m *DamageNumberAnimation) GetActor() *Actor {
	return nil
}

func (m *DamageNumberAnimation) Cancel() {

}
func NewDamageNumberAnimation(mapPosition geometry.Point, amount int) *DamageNumberAnimation {
	timeInSeconds := 1.0
	ticksNeeded := util.SecondsToTicks(timeInSeconds)
	var damageColor color.Color
	if amount > 0 {
		damageColor = defs.PositiveHighlightColor
	} else {
		damageColor = defs.PureRed
	}

	return &DamageNumberAnimation{
		targetPos:        mapPosition,
		damageAmount:     int(math.Abs(float64(amount))),
		runTimeInSeconds: timeInSeconds,
		ticksNeeded:      ticksNeeded,
		damageColor:      damageColor,
	}
}
func (m *DamageNumberAnimation) IsDone() bool {
	return m.ticksAlive >= m.ticksNeeded
}

func (m *DamageNumberAnimation) Tick() {
	if !m.shouldRunNow() || m.IsDone() {
		return
	}
	m.ticksAlive++
}
func (m *DamageNumberAnimation) Draw(renderer TileRenderer) {
	if !m.shouldRunNow() || m.IsDone() {
		return
	}
	damageString := strconv.Itoa(m.damageAmount)
	originPos := m.targetPos

	percentDone := float64(m.ticksAlive) / float64(m.ticksNeeded)
	percentDone = util.EaseInExpo(percentDone)

	tWidth, _ := renderer.MeasureString(damageString)
	// we want to center that string on a 16 pixel wide tile
	xOffset := float64(renderer.GetScaledTileSize().X-float64(tWidth)) / 2.0
	screenOffset := geometry.Point{X: int(xOffset), Y: -(4 + int(float64(20)*percentDone))}

	renderer.DrawStringOnMapWithOffset(originPos, screenOffset.Add(geometry.Point{Y: 2}), damageString, color.Black)
	renderer.DrawStringOnMapWithOffset(originPos, screenOffset, damageString, m.damageColor)
}
