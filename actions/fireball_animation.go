package actions

import (
	"rpg/common"
	"rpg/geometry"
)

type FireballAnimation struct {
	projectileAnimation *ProjectileDirectionAnimation
	hitAnimations       []common.Animation
	caster              *common.Actor
}

func (m *FireballAnimation) GetActor() *common.Actor {
	return m.caster
}
func (m *FireballAnimation) Cancel() {

}

func NewFireballAnimation(caster *common.Actor, destination geometry.Point, hitPositions []geometry.Point) *FireballAnimation {
	flightIcon := int32(common.IconEffectFireSmall)
	hitIcon := int32(common.IconEffectFireBig)
	durationOfHit := 0.4
	secondsPerTile := 0.02
	targetPos := destination

	projectileAnimation := NewProjectileDirectionAnimation(caster, caster.MapPosition(), targetPos, secondsPerTile, flightIcon, 2)

	var hitAnimations []common.Animation
	for _, pos := range hitPositions {
		hitAnimations = append(hitAnimations, NewHitAnimation(pos, durationOfHit, hitIcon, 2))
	}

	return &FireballAnimation{
		projectileAnimation: projectileAnimation,
		hitAnimations:       hitAnimations,
		caster:              caster,
	}
}

func (m *FireballAnimation) IsDone() bool {
	for _, hitAnim := range m.hitAnimations {
		if !hitAnim.IsDone() {
			return false
		}
	}
	if m.projectileAnimation != nil {
		return m.projectileAnimation.IsDone()
	}
	return true
}
func (m *FireballAnimation) Tick() {
	if m.IsDone() {
		return
	}
	if m.projectileAnimation != nil {
		m.projectileAnimation.Tick()
	}
	for _, hitAnim := range m.hitAnimations {
		hitAnim.Tick()
	}
}
func (m *FireballAnimation) Draw(renderer common.TileRenderer) {
	if m.IsDone() {
		return
	}
	if m.projectileAnimation != nil {
		m.projectileAnimation.Draw(renderer)
	}

	for _, hitAnim := range m.hitAnimations {
		hitAnim.Draw(renderer)
	}
}
