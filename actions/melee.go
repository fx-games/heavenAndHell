package actions

import (
	"rpg/common"
	"rpg/geometry"
	"rpg/stats"
	"rpg/util"
)

type MeleeAttackAction struct {
	BaseAction
	damageFactor     float64
	attacker         *common.Actor
	targetPos        geometry.Point
	defenders        []*common.Actor
	damageLocations  []common.LocationDamagePrediction
	targetedDefender *common.Actor
	totalDamageDealt int
}

func NewMeleeAttackAction(source Cause, attacker *common.Actor, targetPos geometry.Point) *MeleeAttackAction {
	return &MeleeAttackAction{
		BaseAction:   BaseAction{cause: source, name: ActionNameMeleeAttack},
		attacker:     attacker,
		targetPos:    targetPos,
		damageFactor: 1.0,
	}
}
func (d *MeleeAttackAction) SetTargetPos(pos geometry.Point) {
	d.targetPos = pos
}
func (d *MeleeAttackAction) SimulateAttack(engine Engine) map[*common.Actor]stats.Interval {
	attacker := d.attacker
	d.damageLocations = attacker.GetDamageLocationPredictions(common.MeleeAttack, d.targetPos)
	// the weapons should now decide which locations are attacked for what damage
	currentMap := engine.GetWorldState().GetMap()
	armorPenetration := attacker.GetStats().GetAttribute(stats.ArmorPenetration)
	damageDone := make(map[*common.Actor]stats.Interval)
	for _, damageLocation := range d.damageLocations {
		if currentMap.IsActorAt(damageLocation.Location) {
			defender := currentMap.ActorAt(damageLocation.Location)
			d.defenders = append(d.defenders, defender)
			if defender.MapPosition() == d.targetPos {
				d.targetedDefender = defender
			}
			damageToDeal := damageLocation.Damage.Mul(d.damageFactor)

			armorDamageReduction := defender.GetArmorDamageReduction(armorPenetration)

			absorbedDamage := damageToDeal.MulCeil(armorDamageReduction)

			finalDamage := damageToDeal.Sub(absorbedDamage)

			damageDone[defender] = damageDone[defender].Add(finalDamage)
		}
	}
	return damageDone

}
func (d *MeleeAttackAction) Execute(engine Engine) []Action {
	var actions []Action
	attackResults := d.SimulateAttack(engine)

	for defender, damageInterval := range attackResults {
		damage := damageInterval.PyramidRandom()
		logAttack(engine, d.attacker, defender, damage)
		d.totalDamageDealt += damage
		actions = append(actions, NewChangeRessourceAction(NewActorCause("Melee attack", d.attacker), defender, stats.Health, -damage))
		defender.SetLookDirFromPoI(d.attacker.MapPosition())
	}
	d.attacker.SetLookDirFromPoI(d.targetPos)

	if d.attacker == engine.GetWorldState().GetPlayer() {
		engine.GetAudio().PlayCue("melee_attack")
	}

	return actions
}

func (d *MeleeAttackAction) GetAnimations() []common.Animation {
	ticksNeeded := util.SecondsToTicks(0.2)
	animation := &MeleeAttackAnimation{
		actor:       d.attacker,
		targetPos:   d.targetPos,
		origin:      d.attacker.MapPosition(),
		ticksNeeded: ticksNeeded,
		ticksLeft:   ticksNeeded,
	}
	return OneAnimation(animation)
}
func (d *MeleeAttackAction) GetDamageFactor() float64 {
	return d.damageFactor
}

func (d *MeleeAttackAction) SetDamageFactor(factor float64) {
	d.damageFactor = factor
}
func (d *MeleeAttackAction) GetAttacker() *common.Actor {
	return d.attacker
}

func (d *MeleeAttackAction) GetDelayInAut() int {
	return 100 / d.attacker.GetAttackSpeed(common.MeleeAttack)
}

func (d *MeleeAttackAction) GetDefenders() []*common.Actor {
	return d.defenders
}

func (d *MeleeAttackAction) GetTargetedDefender() *common.Actor {
	return d.targetedDefender
}

func (d *MeleeAttackAction) GetTotalDamageDealt() int {
	return d.totalDamageDealt
}
