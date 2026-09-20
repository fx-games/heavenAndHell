package common

import (
	"image/color"
	"math"

	"rpg/geometry"
	"rpg/util"
)

type ProjectileDirectionAnimation struct {
	BaseAnimation
	firstTile  int32
	frameCount int

	ticksAlive  uint64
	ticksNeeded uint64

	path           []geometry.Point // flown cell to cell at secondsPerTile
	secondsPerTile float64
	attacker       *Actor
	onCancel       func()
	onFinish       func()
}

func (m *ProjectileDirectionAnimation) Cancel() {
	if m.IsDone() {
		return
	}
	m.ticksAlive = m.ticksNeeded
	if m.onCancel != nil {
		m.onCancel()
	}
	if m.onFinish != nil {
		m.onFinish()
	}
}

func NewProjectileDirectionAnimation(attacker *Actor, start, end geometry.Point, secondsPerTile float64, startFrame int32, frameCount int) *ProjectileDirectionAnimation {
	return NewProjectilePathAnimation(attacker, []geometry.Point{start, end}, secondsPerTile, startFrame, frameCount)
}

// NewProjectilePathAnimation flies through every point of path in turn, e.g. a charged bolt's zig-zag.
func NewProjectilePathAnimation(attacker *Actor, path []geometry.Point, secondsPerTile float64, startFrame int32, frameCount int) *ProjectileDirectionAnimation {
	m := &ProjectileDirectionAnimation{firstTile: startFrame, frameCount: frameCount, secondsPerTile: secondsPerTile, attacker: attacker}
	m.path = path
	m.updateTicksNeeded()
	return m
}

// Extend continues the flight on to p.
func (m *ProjectileDirectionAnimation) Extend(p geometry.Point) {
	m.path = append(m.path, p)
	m.updateTicksNeeded()
}

func (m *ProjectileDirectionAnimation) End() geometry.Point { return m.path[len(m.path)-1] }

func (m *ProjectileDirectionAnimation) length() float64 {
	l := 0.0
	for i := 1; i < len(m.path); i++ {
		l += geometry.Distance(m.path[i-1], m.path[i])
	}
	return l
}

func (m *ProjectileDirectionAnimation) updateTicksNeeded() {
	m.ticksNeeded = max(1, util.SecondsToTicks(m.secondsPerTile*m.length())) // per tile would round 0.03 s down to 1 tick
}

// position is where the projectile is after flying fraction t of the path.
func (m *ProjectileDirectionAnimation) position(t float64) geometry.PointF {
	left := t * m.length()
	for i := 1; i < len(m.path); i++ {
		d := geometry.Distance(m.path[i-1], m.path[i])
		if left <= d && d > 0 {
			return util.LerpPointF(m.path[i-1].ToPointF(), m.path[i].ToPointF(), left/d)
		}
		left -= d
	}
	return m.End().ToPointF()
}

// arrival is how far along the path the projectile passes closest to p, and how close it gets.
func (m *ProjectileDirectionAnimation) arrival(p geometry.Point) (along, dist float64) {
	dist, flown := math.Inf(1), 0.0
	for i := 1; i < len(m.path); i++ {
		a, b := m.path[i-1].ToPointF(), m.path[i].ToPointF()
		dx, dy := b.X-a.X, b.Y-a.Y
		l := math.Hypot(dx, dy)
		t := 0.0
		if l > 0 {
			t = max(0, min(1, ((float64(p.X)-a.X)*dx+(float64(p.Y)-a.Y)*dy)/(l*l)))
		}
		if d := math.Hypot(a.X+t*dx-float64(p.X), a.Y+t*dy-float64(p.Y)); d < dist-1e-9 {
			along, dist = flown+t*l, d
		}
		flown += l
	}
	return along, dist
}

// flown is how far along its path the projectile is.
func (m *ProjectileDirectionAnimation) flown() float64 {
	return float64(m.ticksAlive) / float64(m.ticksNeeded) * m.length()
}

func (m *ProjectileDirectionAnimation) GetActor() *Actor {
	return m.attacker
}
func (m *ProjectileDirectionAnimation) IsDone() bool {
	return m.ticksAlive >= m.ticksNeeded
}
func (m *ProjectileDirectionAnimation) Tick() {
	if !m.shouldRunNow() || m.IsDone() {
		return
	}
	m.ticksAlive++
	if m.IsDone() {
		if m.onFinish != nil {
			m.onFinish()
		}
	}
}
func (m *ProjectileDirectionAnimation) Draw(renderer TileRenderer) {
	if !m.shouldRunNow() || m.IsDone() {
		return
	}
	icon := m.firstTile
	if m.frameCount > 1 {
		icon += util.GetLoopingFrameFromTick(m.ticksAlive, 0.1, m.frameCount)
	}
	renderer.DrawOnMapF(m.position(float64(m.ticksAlive)/float64(m.ticksNeeded)), icon, color.White)
}

func (m *ProjectileDirectionAnimation) SetIcon(icon int32) {
	m.firstTile = icon
}

func (m *ProjectileDirectionAnimation) SetOnCancel(onCancel func()) {
	m.onCancel = onCancel
}

func (m *ProjectileDirectionAnimation) SetOnFinish(onFinish func()) {
	m.onFinish = onFinish
}
