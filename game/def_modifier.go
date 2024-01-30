package game

import (
	"fmt"
	"math/rand"
	"rpg/stats"
	"rpg/util"
)

/* We got

AddToBaseHealth,
AddToBaseMana,
AddToBaseStrength,
AddToBaseDexterity,
AddToBaseIntelligence,
AddToBaseDamage,
StealHealth,
StealMana,
MultiplyDamage,
MultiplyArmor,

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
func (g *GameInstance) GetAllModifiers() map[string]func(source *rand.Rand, level int) *stats.Modifier {
	modifierFunctions := map[string]func(source *rand.Rand, level int) *stats.Modifier{
		"health":            AddToBaseHealth,
		"mana":              AddToBaseMana,
		"strength":          AddToBaseStrength,
		"dexterity":         AddToBaseDexterity,
		"intelligence":      AddToBaseIntelligence,
		"vision_range":      AddToBaseVisionRange,
		"armor_penetration": AddToBaseArmorPenetration,
		"damage":            AddToBaseDamage,
		"armor_class":       AddToArmorClass,
		"steal_health":      StealHealth,
		"steal_mana":        StealMana,
		"damage_multiplier": MultiplyDamage,
		"armor_multiplier":  MultiplyArmor,
		"light_radius": 	 AddToBaseLightRadius,
		"health_regen": 	 AddToHealthRegen,
	}
	return modifierFunctions
}
func (g *GameInstance) NewModifiersForItem(source *rand.Rand, itemLevel int, count int) []*stats.Modifier {
	modifierFunctions := valuesOf(g.GetAllModifiers())
	if count > len(modifierFunctions) {
		count = len(modifierFunctions)
	}
	var modifiers []*stats.Modifier
	randomIndices := source.Perm(len(modifierFunctions))
	for i := 0; i < count; i++ {
		modifiers = append(modifiers, modifierFunctions[randomIndices[i]](source, itemLevel))
	}
	return modifiers
}

func (g *GameInstance) NewModifierFromNameWithRandomRange(modName string, dungeonLevel int) *stats.Modifier {
	source := g.randomSource
	modifierFunctions := g.GetAllModifiers()
	modifierFunction := modifierFunctions[modName]
	return modifierFunction(source, dungeonLevel)
}

func (g *GameInstance) NewModifierFromNameWithFixedBonus(modName string, value float64) *stats.Modifier {
	source := g.randomSource
	modifierFunctions := g.GetAllModifiers()
	modifierFunction := modifierFunctions[modName]
	modifier := modifierFunction(source, 1)
	modifier.Value = value
	return modifier
}


func StealHealth(source *rand.Rand, level int) *stats.Modifier {
	interval := util.NewTuple(0.03, 0.05)
	return StealBaseResource(source, stats.LifeStealFactor, interval)
}
func StealMana(source *rand.Rand, level int) *stats.Modifier {
	interval := util.NewTuple(0.03, 0.05)
	return StealBaseResource(source, stats.ManaStealFactor, interval)
}

func AddToArmorClass(source *rand.Rand, level int) *stats.Modifier {
	levelTable := []ModRow[int, int]{
		{1, util.NewTuple(2, 4)},
		{1, util.NewTuple(5, 8)},
	}
	interval := chooseIntervalFromTable(source, level, levelTable)
	acAdded := randIntervalValue(source, interval)
	mod := &stats.Modifier{
		Name:           "of protection",
		Attribute:      stats.BaseArmorClass,
		Value:          float64(acAdded),
		Mode:           stats.ModifyAdditive,
		IsPercentBased: false,
	}

	return mod
}
func AddToHealthRegen(source *rand.Rand, level int) *stats.Modifier {
	levelTable := []ModRow[int, int]{
		{1, util.NewTuple(-2, -1)},
		{1, util.NewTuple(1, 2)},
	}
	interval := chooseIntervalFromTable(source, level, levelTable)

	mod := &stats.Modifier{
		Name:           "of regeneration",
		Attribute:      stats.HealthRegeneration,
		Value:          randIntervalValue(source, interval),
		Mode:           stats.ModifyAdditive,
		IsPercentBased: false,
	}
	return mod
}
func AddToBaseHealth(source *rand.Rand, level int) *stats.Modifier {
	levelTable := []ModRow[int, int]{
		{4, util.NewTuple(-25, -11)},
		{1, util.NewTuple(-10, -1)},
		{1, util.NewTuple(10, 15)},
		{5, util.NewTuple(16, 20)},
		{9, util.NewTuple(21, 30)},
		{15, util.NewTuple(31, 40)},
		{21, util.NewTuple(41, 50)},
		{27, util.NewTuple(51, 60)},
		{35, util.NewTuple(61, 80)},
		{60, util.NewTuple(81, 100)},
	}
	interval := chooseIntervalFromTable(source, level, levelTable)
	return AddToBaseResource(source, stats.MaxHealth, interval)
}

func AddToBaseMana(source *rand.Rand, level int) *stats.Modifier {
	levelTable := []ModRow[int, int]{
		{4, util.NewTuple(-25, -11)},
		{1, util.NewTuple(-10, -1)},
		{1, util.NewTuple(10, 15)},
		{5, util.NewTuple(16, 20)},
		{9, util.NewTuple(21, 30)},
		{15, util.NewTuple(31, 40)},
		{21, util.NewTuple(41, 50)},
		{27, util.NewTuple(51, 60)},
		//        {35, util.NewTuple(61, 80)},
		//        {60, util.NewTuple(81, 100)},
	}
	interval := chooseIntervalFromTable(source, level, levelTable)
	return AddToBaseResource(source, stats.MaxMana, interval)
}
func AddToBaseDamage(source *rand.Rand, level int) *stats.Modifier {
	interval := util.NewTuple(1, 20)
	return AddToBaseAttribute(source, stats.BaseDamage, interval)
}
func AddToBaseStrength(source *rand.Rand, level int) *stats.Modifier {
	levelTable := []ModRow[int, int]{
		{3, util.NewTuple(-10, -6)},
		{1, util.NewTuple(-5, -1)},
		{1, util.NewTuple(1, 5)},
		{5, util.NewTuple(6, 10)},
		{11, util.NewTuple(11, 15)},
		{17, util.NewTuple(16, 20)},
		{23, util.NewTuple(21, 30)},
	}
	interval := chooseIntervalFromTable(source, level, levelTable)
	return AddToBaseAttribute(source, stats.Strength, interval)
}
func AddToBaseDexterity(source *rand.Rand, level int) *stats.Modifier {
	levelTable := []ModRow[int, int]{
		{3, util.NewTuple(-10, -6)},
		{1, util.NewTuple(-5, -1)},
		{1, util.NewTuple(1, 5)},
		{5, util.NewTuple(6, 10)},
		{11, util.NewTuple(11, 15)},
		{17, util.NewTuple(16, 20)},
		{23, util.NewTuple(21, 30)},
	}
	interval := chooseIntervalFromTable(source, level, levelTable)
	return AddToBaseAttribute(source, stats.Dexterity, interval)
}

func AddToBaseIntelligence(source *rand.Rand, level int) *stats.Modifier {
	levelTable := []ModRow[int, int]{
		{3, util.NewTuple(-10, -6)},
		{1, util.NewTuple(-5, -1)},
		{1, util.NewTuple(1, 5)},
		{5, util.NewTuple(6, 10)},
		{11, util.NewTuple(11, 15)},
		{17, util.NewTuple(16, 20)},
		{23, util.NewTuple(21, 30)},
	}
	interval := chooseIntervalFromTable(source, level, levelTable)
	return AddToBaseAttribute(source, stats.Intelligence, interval)
}

func AddToBaseArmorPenetration(source *rand.Rand, level int) *stats.Modifier {
	interval := util.NewTuple(2, 24)
	return AddToBaseAttribute(source, stats.ArmorPenetration, interval)
}
func AddToBaseVisionRange(source *rand.Rand, level int) *stats.Modifier {
	interval := util.NewTuple(-7, 16)
	return AddToBaseAttribute(source, stats.VisionRange, interval)
}

func AddToBaseLightRadius(source *rand.Rand, level int) *stats.Modifier {
	interval := util.NewTuple(-7, 16)
	return AddToBaseAttribute(source, stats.LightRadius, interval)
}
func MultiplyDamage(source *rand.Rand, level int) *stats.Modifier {
	interval := util.NewTuple(-1.0, 2.8)
	return MultiplyDamageOrArmor(source, stats.EnhancedDamageFactor, interval)
}
func MultiplyArmor(source *rand.Rand, level int) *stats.Modifier {
	table := []ModRow[float64, float64]{
		{3, util.NewTuple(-1.0, -0.51)},
		{1, util.NewTuple(-0.5, -0.25)},
		{1, util.NewTuple(0.2, 0.3)},
		{6, util.NewTuple(0.31, 0.4)},
		{10, util.NewTuple(0.41, 0.55)},
		{14, util.NewTuple(0.56, 0.70)},
		{19, util.NewTuple(0.71, 0.90)},
		{24, util.NewTuple(0.91, 1.1)},
		{28, util.NewTuple(1.11, 1.3)},
		{35, util.NewTuple(1.31, 1.7)},
		{60, util.NewTuple(1.71, 2.0)},
	}
	interval := chooseIntervalFromTable(source, level, table)
	return MultiplyDamageOrArmor(source, stats.EnhancedArmorClassFactor, interval)
}

func StealBaseResource(source *rand.Rand, randomAttribute stats.Name, minMax util.Tuple[float64, float64]) *stats.Modifier {
	intervalMin := minMax.Item1
	intervalMax := minMax.Item2
	var bonusValue float64
	if intervalMin == intervalMax {
		bonusValue = float64(intervalMin)
	} else {
		bonusValue = float64(source.Float64()*(intervalMax-intervalMin) + intervalMin)
	}
	modifier := &stats.Modifier{
		Attribute:      randomAttribute,
		Name:           fmt.Sprintf("of %s stealing", randomAttribute),
		Value:          bonusValue,
		Mode:           stats.ModifyAdditive,
		IsPercentBased: true,
	}

	return modifier
}

func MultiplyDamageOrArmor(source *rand.Rand, randomAttribute stats.Name, minMax util.Tuple[float64, float64]) *stats.Modifier {
	intervalMin := minMax.Item1
	intervalMax := minMax.Item2

	var bonusValue float64
	if intervalMin == intervalMax {
		bonusValue = float64(intervalMin)
	} else {
		bonusValue = float64(source.Float64()*(intervalMax-intervalMin) + intervalMin)
	}
	bonusValue += 1.0

	modifier := &stats.Modifier{
		Attribute:      randomAttribute,
		Name:           fmt.Sprintf("of %s", randomAttribute),
		Value:          bonusValue,
		Mode:           stats.ModifyMultiplicative,
		IsPercentBased: true,
	}

	return modifier
}

func AddToBaseResource(source *rand.Rand, randomAttribute stats.Name, minMax util.Tuple[int, int]) *stats.Modifier {
	intervalMin := minMax.Item1
	intervalMax := minMax.Item2
	var bonusValue float64
	if intervalMin == intervalMax {
		bonusValue = float64(intervalMin)
	} else {
		bonusValue = float64(source.Intn(intervalMax-intervalMin) + intervalMin)
	}
	modifier := &stats.Modifier{
		Attribute:      randomAttribute,
		Name:           fmt.Sprintf("of %s", randomAttribute),
		Value:          bonusValue,
		Mode:           stats.ModifyAdditive,
		IsPercentBased: false,
	}

	return modifier
}
func AddToBaseAttribute(source *rand.Rand, randomAttribute stats.Name, minMax util.Tuple[int, int]) *stats.Modifier {
	bonusValue := randIntervalValue(source, minMax)

	modifier := &stats.Modifier{
		Attribute:      randomAttribute,
		Name:           fmt.Sprintf("of %s", randomAttribute),
		Value:          bonusValue,
		Mode:           stats.ModifyAdditive,
		IsPercentBased: false,
	}

	return modifier
}

func randIntervalValue(source *rand.Rand, minMax util.Tuple[int, int]) float64 {
	intervalMin := minMax.Item1
	intervalMax := minMax.Item2

	var bonusValue float64
	if intervalMin == intervalMax {
		bonusValue = float64(intervalMin)
	} else {
		bonusValue = float64(source.Intn(intervalMax-intervalMin) + intervalMin)
	}
	return bonusValue
}

type ModRow[T1 any, T2 any] struct {
	QualityLevel int
	MinMax       util.Tuple[T1, T2]
}

func chooseIntervalFromTable[T1 any, T2 any](source *rand.Rand, level int, levelTable []ModRow[T1, T2]) util.Tuple[T1, T2] {
	minQLevel := level / 2
	maxQLevel := level

	allowedIntervals := make([]util.Tuple[T1, T2], 0)
	for _, row := range levelTable {
		if row.QualityLevel >= minQLevel && row.QualityLevel <= maxQLevel {
			allowedIntervals = append(allowedIntervals, row.MinMax)
		}
	}

	if len(allowedIntervals) == 0 {
		println(fmt.Sprintf("ERR?! No allowed value intervals for level %d! Choosing randomly.", level))
		return levelTable[source.Intn(len(levelTable))].MinMax
	}
	interval := allowedIntervals[source.Intn(len(allowedIntervals))]
	return interval
}
func valuesOf(modifiers map[string]func(source *rand.Rand, level int) *stats.Modifier) []func(source *rand.Rand, level int) *stats.Modifier {
	result := make([]func(source *rand.Rand, level int) *stats.Modifier, 0)
	for _, value := range modifiers {
		result = append(result, value)
	}
	return result
}
