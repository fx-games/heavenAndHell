package common

import (
	"math/rand"
	"rpg/geometry"
	"rpg/stats"
	"rpg/util"
)

type MeleeAttackAction struct {
	BaseAction
	damageFactor     float64
	attacker         *Actor
	targetPos        geometry.Point
	defenders        []*Actor
	damageLocations  []LocationDamagePrediction
	targetedDefender *Actor
	totalDamageDealt int
}

func NewMeleeAttackAction(source Cause, attacker *Actor, targetPos geometry.Point) *MeleeAttackAction {
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
func (d *MeleeAttackAction) SimulateAttack(context ActionContext) map[*Actor]stats.Interval {
	attacker := d.attacker
	d.damageLocations = attacker.GetDamageLocationPredictions(MeleeAttack, d.targetPos)
	// the weapons should now decide which locations are attacked for what damage
	currentMap := context.GameState.GetMap()
	armorPenetration := attacker.GetStats().GetAttribute(stats.ArmorPenetration)
	damageDone := make(map[*Actor]stats.Interval)
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
func (d *MeleeAttackAction) Execute(context ActionContext) []Action {
	var actions []Action
	attackResults := d.SimulateAttack(context)

	for defender, damageInterval := range attackResults {

		evasionChance := 0.0
		if defender.HasStatusEffect(StatusFlying) {
			evasionChance = 0.25
		}

		if rand.Float64() < evasionChance {
			// dodged the attack
			logDodged(context, d.attacker, defender)
			// play miss audio cue?
			continue
		}

		damage := damageInterval.PyramidRandom()
		logAttack(context, d.attacker, defender, damage)
		d.totalDamageDealt += damage
		actions = append(actions, NewChangeResourceAction(NewActorCause("Melee attack", d.attacker), defender, stats.Health, -damage))
		defender.SetLookDirFromPoI(d.attacker.MapPosition())
	}
	d.attacker.SetLookDirFromPoI(d.targetPos)

	if d.attacker == context.GameState.GetPlayer() {
		context.AudioPlayer.PlayCue("melee_attack")
	}

	return actions
}

func (d *MeleeAttackAction) GetAnimations() []Animation {
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
func (d *MeleeAttackAction) GetAttacker() *Actor {
	return d.attacker
}

func (d *MeleeAttackAction) GetDelayInAut() int {
	return 100 / d.attacker.GetAttackSpeed(MeleeAttack)
}

func (d *MeleeAttackAction) GetDefenders() []*Actor {
	return d.defenders
}

func (d *MeleeAttackAction) GetTargetedDefender() *Actor {
	return d.targetedDefender
}

func (d *MeleeAttackAction) GetTotalDamageDealt() int {
	return d.totalDamageDealt
}
