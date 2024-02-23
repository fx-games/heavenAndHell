package common

import (
	"math"
	"math/rand"
	"rpg/stats"
	"rpg/util"
)

type MeleeAttackAction struct {
	BaseAction
	damageFactor     float64
	attacker         *Actor
	totalDamageDealt int
	defender         *Actor
	disableAnimation bool
}

func NewMeleeAttackAction(source Cause, attacker *Actor, defender *Actor) *MeleeAttackAction {
	return &MeleeAttackAction{
		BaseAction:   BaseAction{cause: source, name: ActionNameMeleeAttack},
		attacker:     attacker,
		defender:    defender,
		damageFactor: 1.0,
	}
}

func (d *MeleeAttackAction) Execute(context ActionContext) []Action {
	var actions []Action

	evasionChance := 0.0
	if d.defender.HasStatusEffect(StatusFlying) {
		evasionChance = 0.25
	}

	if rand.Float64() < evasionChance {
		// dodged the attack
		logDodged(context, d.attacker, d.defender)
		// play miss audio cue?
		return actions
	}

	damage := RollMeleeDamage(d.attacker, d.defender, d.damageFactor)

	logAttack(context, d.attacker, d.defender, damage)

	d.totalDamageDealt = damage

	actions = append(actions, NewChangeResourceAction(NewActorCause("Melee attack", d.attacker), d.defender, stats.Health, float64(-damage)))

	d.defender.SetLookDirFromPoI(d.attacker.MapPosition())
	d.attacker.SetLookDirFromPoI(d.defender.MapPosition())

	if d.attacker == context.GameState.GetPlayer() {
		context.AudioPlayer.PlayCue("melee_attack")
	}

	return actions
}

func RollMeleeDamage(attacker, defender *Actor, damageFactor float64) int {
	damageRoll := GetMeleeDamageRoll(attacker)
	return CalculateMeleeDamage(damageRoll, damageFactor, attacker, defender)
}

func CalculateMeleeDamage(damageRoll int, damageFactor float64, attacker, defender *Actor) int {
	attackerStr := attacker.GetStats().GetAttribute(stats.Strength)
	dndStyleModifier := defaultModifierPattern(attackerStr)
	rawDamage := max(1, damageRoll + dndStyleModifier)

	damageFactor *= GetWeaponQualityFactor(attacker)
	damage := int(float64(rawDamage) * damageFactor)

	armorPenetration := GetArmorPenetrationOfEquippedMeleeWeapon(attacker)
	defenderAC := max(0, defender.GetArmorClass() - armorPenetration)
	defenderADR := ADRFromAC(defenderAC)
	blockedDamage := int(math.Ceil(float64(damage) * defenderADR))

	// 50% chance to use the shield, if it's more effective
	if defender.GetEquipment().HasShieldEquipped() {
		defenderDex := defender.GetStats().GetAttribute(stats.Dexterity)
		dndStyleModifierDefender := defaultModifierPattern(defenderDex)
		shield := defender.GetEquipment().GetShield()
		shieldAC := max(1, shield.GetArmorStats().GetTotalArmorClass() + dndStyleModifierDefender)
		if shieldAC > blockedDamage && rand.Float64() > 0.5 {
			blockedDamage = shieldAC
		}
	}

	damage = max(0, damage-blockedDamage)
	return damage
}

func GetArmorPenetrationOfEquippedMeleeWeapon(attacker *Actor) int {
	if !attacker.GetEquipment().HasMeleeWeaponEquipped() {
		return 0
	}
	weaponUsed := attacker.GetEquipment().GetWeapon(MeleeAttack).GetWeaponStats()
	return weaponUsed.ArmorPenetration
}

func GetMeleeDamageRoll(attacker *Actor) int {
	damageDice := attacker.GetDamageDice(MeleeAttack)
	damageRoll := damageDice.Roll()
	return damageRoll
}

func GetWeaponQualityFactor(attacker *Actor) float64 {
	if !attacker.GetEquipment().HasMeleeWeaponEquipped() {
		return 1.0
	}
	weaponUsed := attacker.GetEquipment().GetWeapon(MeleeAttack).GetWeaponStats()
	return weaponUsed.Durability.AsPercentage()
}

func (d *MeleeAttackAction) GetAnimations() []Animation {
	if d.disableAnimation {
		return nil
	}
	ticksNeeded := util.SecondsToTicks(0.2)
	animation := &MeleeAttackAnimation{
		actor:       d.attacker,
		targetPos:   d.defender.MapPosition(),
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

func (d *MeleeAttackAction) GetDefender() *Actor {
	return d.defender
}

func (d *MeleeAttackAction) GetTotalDamageDealt() int {
	return d.totalDamageDealt
}

func (d *MeleeAttackAction) DisableAnimation() {
	d.disableAnimation = true
}
