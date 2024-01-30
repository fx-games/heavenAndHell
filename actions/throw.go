package actions

import (
	"rpg/common"
	"rpg/geometry"
	"rpg/stats"
)

type ThrowAction struct {
	BaseAction
	damageFactor float64
	attacker     *common.Actor
	damageAmount int
	defender     *common.Actor
	targetPos    geometry.Point
	thrownItem   common.Item
}

func (d *ThrowAction) GetTotalDamageDealt() int {
	return d.damageAmount
}

func NewThrowAction(source Cause, attacker *common.Actor, thrownItem common.Item, targetPos geometry.Point) *ThrowAction {
	return &ThrowAction{
		BaseAction:   BaseAction{cause: source, name: ActionNameThrow},
		attacker:     attacker,
		targetPos:    targetPos,
		thrownItem:   thrownItem,
		damageFactor: 1.0,
	}
}

func (d *ThrowAction) Execute(engine Engine) []Action {
	//audio := engine.GetAudio()
	//audio.PlayCue("bow_fire")

	d.thrownItem.SetDrawnToScreen(false) // hide for animation

	throwInfo := d.thrownItem.GetThrowStats()

	var actions []Action
	gameItem := d.thrownItem.(GameItem)

	// unequip
	if d.attacker.GetEquipment().IsEquipped(d.thrownItem) {
		unequipAction := NewUnequipItemAction(d.Cause(), d.attacker, gameItem)
		actions = append(actions, unequipAction)
	}
	// remove from inventory & place on map
	moveItem := NewMoveItemAction(d.Cause(), d.attacker, d.targetPos, gameItem)
	moveItem.SetChanceToBreak(throwInfo.ChanceToBreak)
	actions = append(actions, moveItem)


	currentMap := engine.GetWorldState().GetMap()
	if currentMap.IsActorAt(d.targetPos) {
		d.defender = currentMap.ActorAt(d.targetPos)
		if throwInfo.Damage.NotZero() { // deal direct throw damage
			damage := throwInfo.Damage.PyramidRandom()
			logAttack(engine, d.attacker, d.defender, damage)
			actions = append(actions, NewChangeRessourceAction(NewActorCause(ActionNameThrow, d.attacker), d.defender, stats.Health, -damage))
			d.damageAmount += damage
		}
	}
	if throwInfo.OnHitEffect != nil { // apply generic effect at target location
		hitLocations := throwInfo.HitZone(d.attacker.MapPosition(), d.targetPos)
		throwInfo.OnHitEffect(d.attacker, d.targetPos, hitLocations)
	}

	return actions
}

func (d *ThrowAction) GetAnimations() []common.Animation {
	animation := NewProjectileDirectionAnimation(d.attacker, d.attacker.MapPosition(), d.targetPos, 0.04, 41, 1)
	animation.SetIcon(d.thrownItem.Icon(0))
	animation.SetOnFinish(func() {
		d.thrownItem.SetDrawnToScreen(true)
	})
	return OneAnimation(animation)
	//return OneAnimation(NewProjectileAnimation(d.attacker, d.flightPath, 0.02, 41, 1))
	//return &DummyAnimation{time: 5, cancelRunningAnimations: true}
}
func (d *ThrowAction) GetDamageFactor() float64 {
	return d.damageFactor
}

func (d *ThrowAction) SetDamageFactor(factor float64) {
	d.damageFactor = factor
}

func (d *ThrowAction) GetDamageDealt() int {
	return d.damageAmount
}

func (d *ThrowAction) GetAttacker() *common.Actor {
	return d.attacker
}

func (d *ThrowAction) GetDelayInAut() int {
	return 100 / d.attacker.GetAttackSpeed(common.RangedAttack)
}

func (d *ThrowAction) GetDefender() interface{} {
	return d.defender
}

func (d *ThrowAction) GetTargetPos() geometry.Point {
	return d.targetPos
}
