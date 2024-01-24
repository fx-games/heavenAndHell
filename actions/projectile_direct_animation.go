package actions

import (
	"image/color"
	"rpg/common"
	"rpg/geometry"
	"rpg/util"
)

type ProjectileDirectionAnimation struct {
	firstTile  int32
	frameCount int

	ticksAlive  uint64
	ticksNeeded uint64

	start    geometry.Point
	end      geometry.Point
	attacker *common.Actor
}

func (m *ProjectileDirectionAnimation) Cancel() {

}

func NewProjectileDirectionAnimation(attacker *common.Actor, start, end geometry.Point, secondsPerTile float64, startFrame int32, frameCount int) *ProjectileDirectionAnimation {
	ticksPerTile := util.SecondsToTicks(secondsPerTile)
	dist := geometry.Distance(start, end)
	return &ProjectileDirectionAnimation{
		start:       start,
		end:         end,
		firstTile:   startFrame,
		frameCount:  frameCount,
		ticksNeeded: uint64(float64(ticksPerTile) * dist),
		attacker:    attacker,
	}
}

func (m *ProjectileDirectionAnimation) GetActor() *common.Actor {
	return m.attacker
}
func (m *ProjectileDirectionAnimation) IsDone() bool {
	return m.ticksAlive >= m.ticksNeeded
}
func (m *ProjectileDirectionAnimation) Tick() {
	if m.IsDone() {
		return
	}
	m.ticksAlive++
}
func (m *ProjectileDirectionAnimation) Draw(renderer common.TileRenderer) {
	if m.IsDone() {
		return
	}
	icon := m.firstTile + util.GetLoopingFrameFromTick(m.ticksAlive, 0.1, m.frameCount)
	percentage := float64(m.ticksAlive) / float64(m.ticksNeeded)
	lerpedPos := util.LerpPointF(m.start.ToPointF(), m.end.ToPointF(), percentage)
	renderer.DrawOnMapF(lerpedPos, icon, color.White)
}
