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

type StatusEffect string

const (
    // binary
    StatusInvisible StatusEffect = "invisible" // cannot be seen
    StatusStunned   StatusEffect = "stunned" // cannot act
    StatusScared    StatusEffect = "scared" // runs away
    StatusSilenced  StatusEffect = "silenced" // cannot cast spells
    StatusPacified  StatusEffect = "pacified" // cannot attack
    StatusImmobile  StatusEffect = "immobile" // cannot move
    StatusConfused  StatusEffect = "confused" // random actions
    StatusCharmed   StatusEffect = "charmed" // change faction
    StatusBerserk   StatusEffect = "berserk" // random attacks
    StatusHaywire   StatusEffect = "haywire" // random magic
    StatusFlying    StatusEffect = "flying" // can move over some obstacles, chance to dodge melee attacks
    // parametrized
    StatusBlinded   StatusEffect = "blinded" // are really timed modifiers
    StatusSlow      StatusEffect = "slow" // are really timed modifiers
    StatusHasted    StatusEffect = "hasted" // are really timed modifiers
    StatusDamageOverTime  StatusEffect = "damage over time" // timed ressource changes
)

type ParametrizedStatusEffect struct {
    name StatusEffect
}