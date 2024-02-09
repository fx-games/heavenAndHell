package common

import (
	"rpg/geometry"
	"rpg/stats"
)

type RangedAttackAction struct {
	BaseAction
	damageFactor    float64
	attacker        *Actor
	damageAmount    int
	defender        *Actor
	targetPos       geometry.Point
	damageLocations []LocationDamagePrediction
	defenders       []*Actor
}

func (d *RangedAttackAction) GetTotalDamageDealt() int {
	return d.damageAmount
}

func NewRangedAttackAction(source Cause, attacker *Actor, targetPos geometry.Point) *RangedAttackAction {
	return &RangedAttackAction{
		BaseAction:   BaseAction{cause: source, name: ActionNameRangedAttack},
		attacker:     attacker,
		targetPos:    targetPos,
		damageFactor: 1.0,
	}
}
func (d *RangedAttackAction) SimulateAttack(context ActionContext) map[*Actor]stats.Interval {
	attacker := d.attacker
	d.damageLocations = attacker.GetDamageLocationPredictions(RangedAttack, d.targetPos)
	// the weapons should now decide which locations are attacked for what damage
	currentMap := context.GameState.GetMap()
	armorPenetration := attacker.GetStats().GetAttribute(stats.ArmorPenetration)
	damageDone := make(map[*Actor]stats.Interval)
	for _, damageLocation := range d.damageLocations {
		if currentMap.IsActorAt(damageLocation.Location) {
			defender := currentMap.ActorAt(damageLocation.Location)
			d.defenders = append(d.defenders, defender)
			damageToDeal := damageLocation.Damage.Mul(d.damageFactor)

			armorDamageReduction := defender.GetArmorDamageReduction(armorPenetration)

			absorbedDamage := damageToDeal.MulCeil(armorDamageReduction)

			finalDamage := damageToDeal.Sub(absorbedDamage)

			damageDone[defender] = damageDone[defender].Add(finalDamage)
		}
	}
	return damageDone
}

func (d *RangedAttackAction) Execute(context ActionContext) []Action {
	audio := context.AudioPlayer
	audio.PlayCue("bow_fire")

	var actions []Action

	attackResults := d.SimulateAttack(context)
	d.damageAmount = 0
	for defender, damageInterval := range attackResults {
		damage := damageInterval.PyramidRandom()
		logAttack(context, d.attacker, defender, damage)
		actions = append(actions, NewChangeResourceAction(NewActorCause(ActionNameRangedAttack, d.attacker), defender, stats.Health, -damage))
		d.damageAmount += damage
	}

	return actions
}

func (d *RangedAttackAction) GetAnimations() []Animation {
	return OneAnimation(NewProjectileDirectionAnimation(d.attacker, d.attacker.MapPosition(), d.targetPos, 0.02, 41, 1))
	//return OneAnimation(NewProjectileAnimation(d.attacker, d.flightPath, 0.02, 41, 1))
	//return &DummyAnimation{time: 5, cancelRunningAnimations: true}
}
func (d *RangedAttackAction) GetDamageFactor() float64 {
	return d.damageFactor
}

func (d *RangedAttackAction) SetDamageFactor(factor float64) {
	d.damageFactor = factor
}

func (d *RangedAttackAction) GetDamageDealt() int {
	return d.damageAmount
}

func (d *RangedAttackAction) GetAttacker() *Actor {
	return d.attacker
}

func (d *RangedAttackAction) GetDelayInAut() int {
	return 100 / d.attacker.GetAttackSpeed(RangedAttack)
}

func (d *RangedAttackAction) GetDefender() interface{} {
	return d.defender
}

func (d *RangedAttackAction) GetTargetPos() geometry.Point {
	return d.targetPos
}
