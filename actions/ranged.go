package actions

import (
	"rpg/common"
	"rpg/geometry"
	"rpg/stats"
)

type RangedAttackAction struct {
	BaseAction
	damageFactor    float64
	attacker        *common.Actor
	damageAmount    int
	defender        *common.Actor
	targetPos       geometry.Point
	damageLocations []common.LocationDamagePrediction
	defenders       []*common.Actor
}

func (d *RangedAttackAction) GetTotalDamageDealt() int {
	return d.damageAmount
}

func NewRangedAttackAction(source Cause, attacker *common.Actor, targetPos geometry.Point) *RangedAttackAction {
	return &RangedAttackAction{
		BaseAction:   BaseAction{cause: source, name: ActionNameRangedAttack},
		attacker:     attacker,
		targetPos:    targetPos,
		damageFactor: 1.0,
	}
}
func (d *RangedAttackAction) SimulateAttack(engine Engine) map[*common.Actor]stats.Interval {
	attacker := d.attacker
	d.damageLocations = attacker.GetDamageLocationPredictions(common.RangedAttack, d.targetPos)
	// the weapons should now decide which locations are attacked for what damage
	currentMap := engine.GetWorldState().GetMap()
	armorPenetration := attacker.GetStats().GetAttribute(stats.ArmorPenetration)
	damageDone := make(map[*common.Actor]stats.Interval)
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

func (d *RangedAttackAction) Execute(engine Engine) []Action {
	audio := engine.GetAudio()
	audio.PlayCue("bow_fire")

	var actions []Action

	attackResults := d.SimulateAttack(engine)
	d.damageAmount = 0
	for defender, damageInterval := range attackResults {
		damage := damageInterval.PyramidRandom()
		logAttack(engine, d.attacker, defender, damage)
		actions = append(actions, NewChangeRessourceAction(NewActorCause(ActionNameRangedAttack, d.attacker), defender, stats.Health, -damage))
		d.damageAmount += damage
	}

	return actions
}

func (d *RangedAttackAction) GetAnimations() []common.Animation {
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

func (d *RangedAttackAction) GetAttacker() *common.Actor {
	return d.attacker
}

func (d *RangedAttackAction) GetDelayInAut() int {
	return 100 / d.attacker.GetAttackSpeed(common.RangedAttack)
}

func (d *RangedAttackAction) GetDefender() interface{} {
	return d.defender
}

func (d *RangedAttackAction) GetTargetPos() geometry.Point {
	return d.targetPos
}
