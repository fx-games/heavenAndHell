package common

import (
	"image/color"

	"rpg/geometry"
	"rpg/util"
)

type ProjectileTileAnimation struct {
	firstTile  int32
	frameCount int

	ticksAlive  uint64
	ticksNeeded uint64

	currentWaypoint  int
	path             []geometry.Point
	ticksPerWaypoint int
	attacker         *Actor
}

func (m *ProjectileTileAnimation) Cancel() {

}

func NewProjectileAnimation(attacker *Actor, path []geometry.Point, secondsPerTile float64, startFrame int32, frameCount int) *ProjectileTileAnimation {
	ticksPerTile := util.SecondsToTicks(secondsPerTile)
	return &ProjectileTileAnimation{
		path:             path,
		firstTile:        startFrame,
		frameCount:       frameCount,
		ticksNeeded:      ticksPerTile * uint64(len(path)),
		ticksPerWaypoint: int(ticksPerTile),
		attacker:         attacker,
	}
}

func (m *ProjectileTileAnimation) GetActor() *Actor {
	return m.attacker
}
func (m *ProjectileTileAnimation) IsDone() bool {
	return m.ticksAlive >= m.ticksNeeded
}
func (m *ProjectileTileAnimation) Tick() {
	if m.IsDone() {
		return
	}
	m.ticksAlive++
	m.currentWaypoint = int(float64(m.ticksAlive) / float64(m.ticksPerWaypoint))
}
func (m *ProjectileTileAnimation) Draw(renderer TileRenderer) {
	if m.IsDone() {
		return
	}
	icon := m.firstTile + util.GetLoopingFrameFromTick(m.ticksAlive, 0.1, m.frameCount)
	for i := m.currentWaypoint; i < len(m.path)-1; i++ {
		renderer.DrawOnMap(m.path[i], icon, color.White)
	}
}
