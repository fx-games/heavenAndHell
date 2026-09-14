package defs

import (
	"rpg/util"
)

/* We got

AddToBaseHealth,
AddToBaseMana,
AddToBaseStrength,
AddToBaseDexterity,
AddToBaseIntelligence,
AddToDamage,
StealHealth,
StealMana,
AddDamage,
AddArmor,

*/

/*
	We need

+ Strength (-10 to +30) - DONE
+ Dexterity (-10 to +30) - DONE
+ Intelligence (-10 to +30) - DONE
+ All Attributes - WON'T DO
+ HP (-25 to +100) - DONE
+ Mana  (-25 to +100) - DONE
% Steal Life (3% to 5%) not cumulative - DONE
% Steal Mana (3% to 5%) not cumulative - DONE
+% Armor Class (-100% to +200%) - DONE
+% Damage Done (-100% to +280%) - DONE
+ Damage Done (1 to 20) - DONE
- Damage Taken (1 to 6)
+ Fire Damage (1 to 20)
+ Lightning Damage (1 to 20)
+% Vision Radius (-40 to +40) - DONE
+% Resist Magic
+% Resist Fire
+% Resist Lightning
+% Resist All
Fragility (Durability = 1)
+% Durability (-75% to +200%)
Indestructible
Ethereal (More power but cannot be repaired)
+% Attack Speed (-50% to +100%)
+ Spell Level (1 to 2)
x Charges (2 to 3)
Armor Penetration (2 to 24) - DONE
Knockback
Fast Block
Thorns (damage to attacker) - DONE
5% chance of doing 3x damage - DONE
Each attack does x0-6 damage. Usually 2x. (Not against unique monsters)
2x damage to monsters, 1x damage to self
*/

type ModRow[T1 any, T2 any] struct {
	QualityLevel int
	MinMax       util.Tuple[T1, T2]
}
