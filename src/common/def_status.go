package common

/*
Status Effects
==============
  - Invisible (not visible)
  - Stunned (no actions)
  - Blinded (no vision, high chance to miss even when attacking the right direction)
  - Slow / Hasted (less / more actions)
  - Scared (run away)
  - Silenced (no magic) / Pacified (no attacks)
  - Poisoned (damage over time)
  - Confused (random actions)
  - Charmed (change faction)
  - Berserker Rage (random attacks) / Haywire (random magic)
  - Flying (can move over some obstacles, chance to dodge melee attacks)
  - Increased/reduced stats/resources temporarily
*/

func IsValidStatusEffect(name string) bool {
	switch StatusEffect(name) {
	case StatusInvisible, StatusStunned, StatusPoisoned, StatusImmobile, StatusBlinded, StatusSlow, StatusHasted, StatusScared, StatusSilenced, StatusPacified, StatusDamageOverTime, StatusConfused, StatusCharmed, StatusBerserk, StatusHaywire, StatusFlying, StatusWinCondition:
		return true
	case StatusStone, StatusStoneSkin, StatusBlessed, StatusManaShield, StatusInfravision, StatusVision:
		// declared next to the spells that apply them; effects reach them by name like any other status
		return true
	}
	return false
}

type StatusEffect string

const (
	// binary
	StatusInvisible StatusEffect = "invisible" // cannot be seen
	StatusStunned   StatusEffect = "stunned"   // cannot act
	StatusScared    StatusEffect = "scared"    // runs away
	StatusSilenced  StatusEffect = "silenced"  // cannot cast spells
	StatusPacified  StatusEffect = "pacified"  // cannot attack
	StatusImmobile  StatusEffect = "immobile"  // cannot move
	StatusConfused  StatusEffect = "confused"  // random actions
	StatusPoisoned  StatusEffect = "poisoned"  // random actions
	StatusCharmed   StatusEffect = "charmed"   // change faction
	StatusBerserk   StatusEffect = "berserk"   // random attacks
	StatusHaywire   StatusEffect = "haywire"   // random magic
	StatusFlying    StatusEffect = "flying"    // can move over some obstacles, chance to dodge melee attacks
	StatusVision    StatusEffect = "vision"    // all entities on the level are visible, ignoring fog of war and line of sight
	// parametrized
	StatusBlinded        StatusEffect = "blinded"          // are really timed modifiers
	StatusSlow           StatusEffect = "slow"             // are really timed modifiers
	StatusHasted         StatusEffect = "hasted"           // are really timed modifiers
	StatusDamageOverTime StatusEffect = "damage over time" // timed resource changes
	StatusWinCondition   StatusEffect = "win condition"    // killing this enemy is part of the win condition
	StatusRage           StatusEffect = "rage"             // the Barbarian's Rage, and the hangover after it
	StatusStoneSkin      StatusEffect = "stoneskin"        // HeroSheet and MoveCost read it
	StatusBlessed        StatusEffect = "blessed"          // HeroSheet reads it
	StatusSatiated       StatusEffect = "satiated"         // resting heals (RestUntilHealed)
	StatusSatiatedRests  StatusEffect = "satiated_rests"   // value -rests the last meal still heals
)

type ParametrizedStatusEffect struct {
	name      StatusEffect
	nextTurn  func(a *Actor)
	turnsLeft int
}

func NewParametrizedStatusEffect(name StatusEffect, nextTurn func(a *Actor), turnsLeft int) *ParametrizedStatusEffect {
	return &ParametrizedStatusEffect{name: name, nextTurn: nextTurn, turnsLeft: turnsLeft}
}
func (e *ParametrizedStatusEffect) NextTurn(a *Actor) {
	if e.turnsLeft <= 0 {
		return
	}
	e.nextTurn(a)
	e.turnsLeft--
}

func (e *ParametrizedStatusEffect) TurnsLeft() int {
	return e.turnsLeft
}

func (e *ParametrizedStatusEffect) GetName() string {
	return string(e.name)
}

// StatusInfo is the display name and the one-line explanation shown when hovering a status icon.
var StatusInfo = map[StatusEffect][2]string{
	StatusInvisible:      {"Invisible", "Monsters cannot see you."},
	StatusStunned:        {"Stunned", "You cannot act."},
	StatusScared:         {"Scared", "You flee from what you see."},
	StatusSilenced:       {"Silenced", "You cannot cast spells."},
	StatusPacified:       {"Pacified", "You cannot attack."},
	StatusImmobile:       {"Immobile", "You cannot move."},
	StatusConfused:       {"Confused", "Your actions go astray."},
	StatusPoisoned:       {"Poisoned", "You lose health each turn."},
	StatusCharmed:        {"Charmed", "You fight for the other side."},
	StatusBerserk:        {"Berserk", "You attack whatever is nearest."},
	StatusHaywire:        {"Haywire", "Your magic goes off at random."},
	StatusFlying:         {"Flying", "You cross obstacles and dodge blows."},
	StatusVision:         {"Vision", "You see the whole level and everything alive on it."},
	StatusBlinded:        {"Blinded", "You cannot see and rarely hit."},
	StatusSlow:           {"Slowed", "You act less often."},
	StatusHasted:         {"Hasted", "You act more often."},
	StatusDamageOverTime: {"Bleeding", "You lose health each turn."},
	StatusRage:           {"Rage", "Your blows land harder."},
	StatusSatiated:       {"Satiated", "Each turn you rest heals 10% of your health."},
	StatusBlessed:        {"Blessed", "Your armor class is raised: +5, or doubled from 5 up."},
	StatusStoneSkin:      {"Stoneskin", "Your armor is hard as stone, your steps slow."},
	StatusStone:          {"Petrified", "You are stone: unable to act, unable to be hurt."},
	StatusManaShield:     {"Mana Shield", "Mana pays for the damage you take, until it runs out."},
	StatusInfravision:    {"Infravision", "You see living things through walls."},
}

// StatusName is the status's display name, or its id when StatusInfo has none.
func StatusName(e StatusEffect) string {
	if info, ok := StatusInfo[e]; ok {
		return info[0]
	}
	return string(e)
}
