package actions

import (
	"fmt"
	"rpg/common"
	"rpg/geometry"
	"rpg/stats"
)

type DamageSpellAction struct {
	BaseAction
	damageFactor         float64
	damageAmount         int
	hitPositions         []geometry.Point
	baseDamage           int
	caster               *common.Actor
	actionDescription    ActionName
	targetPos            geometry.Point
	flightIcon           int32
	flightIconFrameCount int
}

func NewDamageSpellAction(cause Cause, caster *common.Actor, damagePerTile int, targetPos geometry.Point, hitPositions []geometry.Point) *DamageSpellAction {
	return &DamageSpellAction{
		BaseAction:           BaseAction{cause: cause, name: ActionNameFireball},
		hitPositions:         hitPositions,
		targetPos:            targetPos,
		baseDamage:           damagePerTile,
		damageFactor:         1.0,
		caster:               caster,
		actionDescription:    "Fireball",
		flightIcon:           int32(common.IconEffectFireSmall),
		flightIconFrameCount: 2,
	}
}

func (d *DamageSpellAction) SetFlightIcon(icon int32, frameCount int) {
	d.flightIcon = icon
	d.flightIconFrameCount = frameCount
}
func (d *DamageSpellAction) Execute(engine Engine) []Action {
	d.damageAmount = int(float64(d.baseDamage) * d.damageFactor)
	state := engine.GetWorldState()
	currentMap := state.GetMap()

	var hitActions []Action
	for _, pos := range d.hitPositions {
		if !currentMap.IsActorAt(pos) {
			continue
		}
		hitActor := currentMap.ActorAt(pos)
		hitActions = append(hitActions, NewChangeRessourceAction(NewActorCause(d.actionDescription, d.caster), hitActor, stats.Health, -d.damageAmount))
	}
	casterPart := fmt.Sprintf("%s casts", d.caster.Name())
	if d.caster == state.GetPlayer() {
		casterPart = "You cast"
	}
	engine.Log(fmt.Sprintf("%s %s", casterPart, d.actionDescription))
	return hitActions
}

func (d *DamageSpellAction) GetAnimations() []common.Animation {
	animation := NewFireballAnimation(d.caster, d.targetPos, d.flightIcon, d.flightIconFrameCount, d.hitPositions)
	return OneAnimation(animation)
}

func (d *DamageSpellAction) GetDamageFactor() float64 {
	return d.damageFactor
}

func (d *DamageSpellAction) SetDamageFactor(factor float64) {
	d.damageFactor = factor
}

func (d *DamageSpellAction) GetDamageDealt() int {
	return d.damageAmount
}
