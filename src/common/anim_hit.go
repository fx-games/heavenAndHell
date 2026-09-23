package common

import (
	"image/color"

	"rpg/src/geometry"
	"rpg/src/util"
)

type HitAnimation struct {
	BaseAnimation
	targetPos geometry.Point

	firstTile  int32
	frameCount int

	runTimeInSeconds float64
	ticksAlive       uint64
	ticksNeeded      uint64
	onFinished       func()
	fade             bool
}

func NewHitAnimation(targetPos geometry.Point, runtimeInSeconds float64, startFrame int32, frameCount int) *HitAnimation {
	ticksNeeded := util.SecondsToTicks(runtimeInSeconds)
	return &HitAnimation{
		targetPos:        targetPos,
		firstTile:        startFrame,
		frameCount:       frameCount,
		runTimeInSeconds: runtimeInSeconds,
		ticksNeeded:      ticksNeeded,
	}
}
func (m *HitAnimation) IsDone() bool {
	return m.ticksAlive >= m.ticksNeeded
}

func (m *HitAnimation) Draw(renderer TileRenderer) {
	if !m.shouldRunNow() || m.IsDone() {
		return
	}
	icon := m.firstTile + util.GetLoopingFrameFromTick(m.ticksAlive, m.runTimeInSeconds/float64(m.frameCount), m.frameCount)
	tint := color.Color(color.White)
	if m.fade {
		tint = color.NRGBA{R: 255, G: 255, B: 255, A: uint8(255 * (m.ticksNeeded - m.ticksAlive) / m.ticksNeeded)}
	}
	renderer.DrawOnMap(m.targetPos, icon, tint)
}
func (m *HitAnimation) Tick() {
	if !m.shouldRunNow() || m.IsDone() {
		return
	}
	m.ticksAlive++
	if m.IsDone() && m.onFinished != nil {
		m.onFinished()
	}
}

func (m *HitAnimation) GetActor() *Actor {
	return nil
}

func (m *HitAnimation) Cancel() {
	m.ticksAlive = m.ticksNeeded
}

func (m *HitAnimation) SetOnFinished(onFinished func()) {
	m.onFinished = onFinished
}
