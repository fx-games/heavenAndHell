package common

import (
	"rpg/src/geometry"
	"rpg/src/stats"
)

type ThrowAction struct {
	BaseAction
	damageFactor float64
	attacker     *Actor
	damageAmount int
	defender     *Actor
	targetPos    geometry.Point
	thrownItem   *Item
}

func (d *ThrowAction) GetTotalDamageDealt() int {
	return d.damageAmount
}

func NewThrowAction(source Cause, attacker *Actor, thrownItem *Item, targetPos geometry.Point) *ThrowAction {
	return &ThrowAction{
		BaseAction:   BaseAction{cause: source, name: ActionNameThrow},
		attacker:     attacker,
		targetPos:    targetPos,
		thrownItem:   thrownItem,
		damageFactor: 1.0,
	}
}

func (d *ThrowAction) Execute(context ActionContext) []Action {
	context.ActionBroker.Emit(Thrown{Thrower: d.attacker, Item: d.thrownItem, From: d.attacker.MapPosition(), To: d.targetPos})

	throwInfo := d.thrownItem.GetThrowStats()

	var actions []Action

	// unequip
	if d.attacker.GetEquipment().IsEquipped(d.thrownItem) {
		unequipAction := NewUnequipItemAction(d.Cause(), d.attacker, d.thrownItem)
		actions = append(actions, unequipAction)
	}
	// remove from inventory & place on map
	moveItem := NewMoveItemAction(d.Cause(), d.attacker, d.targetPos, d.thrownItem)
	moveItem.SetChanceToBreak(throwInfo.ChanceToBreak)
	actions = append(actions, moveItem)

	currentMap := context.GameState.GetMap()
	if defender, ok := currentMap.TryGetActorAt(d.targetPos); !ok {
		context.GameState.factory.hitObject(currentMap, d.targetPos, d.attacker) // a thrown item that hits a barrel breaks it
	} else {
		d.defender = defender

		hitChance := CalculateThrowHitChance(d.attacker, d.defender)

		doesHit := hitChance > 0 && hitChance >= rng.Float64()
		if doesHit && throwInfo.Damage.NotZero() { // deal direct throw damage
			damage := throwInfo.Damage.Roll(rng)
			logAttack(context, d.attacker, d.defender, damage)
			actions = append(actions, NewChangeResourceAction(NewActorCause(ActionNameThrow, d.attacker), d.defender, stats.Health, float64(-damage)))
			d.damageAmount += damage
		}
	}
	if throwInfo.OnHitEffect != nil { // apply generic effect at target location
		hitLocations := throwInfo.HitZone.GetHitPositions(d.attacker.MapPosition(), d.targetPos)
		throwInfo.OnHitEffect.Activate(d.attacker, d.targetPos, hitLocations)
	}

	return actions
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

func (d *ThrowAction) GetAttacker() *Actor {
	return d.attacker
}

func (d *ThrowAction) GetDelayInAut() int {
	return d.attacker.AttackCost(RangedAttack)
}

func (d *ThrowAction) GetDefender() interface{} {
	return d.defender
}

func (d *ThrowAction) GetTargetPos() geometry.Point {
	return d.targetPos
}

func CalculateThrowHitChance(attacker, defender *Actor) float64 {
	distance := geometry.Distance(defender.MapPosition(), attacker.MapPosition())

	dexModifier := float64(defaultModifierPattern(attacker.GetStats().GetAttribute(stats.Dexterity))) * 0.2
	strModifier := float64(defaultModifierPattern(attacker.GetStats().GetAttribute(stats.Strength)))
	// dex modifier only applies in full to skill based ranged weapons
	// => in particular, crossbows don't get this penalty

	maxRange := 10 + (2 * strModifier)
	accuracyAtMaxRange := 0.5
	accuracyAtMinRange := 0.9

	if distance > maxRange {
		dropOffPerTile := 0.3
		delta := distance - maxRange
		return min(max(accuracyAtMaxRange-(delta*dropOffPerTile)+dexModifier, 0.0), 1.0)
	}

	if distance <= 1 {
		return min(max(accuracyAtMinRange+dexModifier, 0.0), 1.0)
	}

	// interpolate

	// min range == 2
	normalizedMaxRange := maxRange - 1
	percent := (distance - 1) / normalizedMaxRange
	accuracy := min(max(accuracyAtMinRange+(percent*(accuracyAtMaxRange-accuracyAtMinRange))+dexModifier, 0.0), 1.0)

	return accuracy
}
