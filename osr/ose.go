package osr

import (
	"fmt"
	"rpg/util"
)

type MonsterSpecs struct {
	AC         int
	HitDice    util.Dice
	AttackDice util.Dice
	Name       string
}
type ClassSpecs struct {
	HitDie        util.Dice
}

func GetFighterSpecs() ClassSpecs {
	return ClassSpecs{
		HitDie: util.ParseDice("1d8"),
	}
}

type Character struct {
	name         string

	strength     int
	intelligence int
	dexterity    int

	maxHitPoints     int
	currentHitPoints int

	level int
	xp    int

	armorClass          int

	gold       int
	attackDice util.Dice
}

func (c *Character) LevelUp(classSpecs ClassSpecs) {
	if c.level == 10 {
		println("Max level reached")
		return
	}
	c.level++
	/* OSE
	c.maxHitPoints += max(1, classSpecs.HitDie.Roll() + HitPointBonus(c.constitution))
	c.thac0 = classSpecs.THAC0PerLevel[c.level]

	*/

	c.maxHitPoints += classSpecs.HitDie.Avg()

	d20 := util.ParseDice("1d20")
	lowestValue := c.strength
	lowesStat := &c.strength
	noChange := true

	if d20.Roll() > c.strength {
		c.strength++
		noChange = false
	}
	if d20.Roll() > c.intelligence {
		c.intelligence++
		noChange = false
	}
	if c.intelligence < lowestValue {
		lowestValue = c.intelligence
		lowesStat = &c.intelligence
	}
	if d20.Roll() > c.dexterity {
		c.dexterity++
		noChange = false
	}
	if c.dexterity < lowestValue {
		lowestValue = c.dexterity
		lowesStat = &c.dexterity
	}

	if noChange {
		*lowesStat++
	}
}

func (c *Character) LevelTo(classSpecs ClassSpecs, targetLevel int) {
	if targetLevel > 10 {
		println("Max level is 10")
		return
	}
	for c.level < targetLevel {
		c.LevelUp(classSpecs)
	}
}

func (c *Character) IsAlive() bool {
	return c.currentHitPoints > 0
}

func (c *Character) CombatInfo() string {
	return fmt.Sprintf("%s - Lvl %d: %d HP, AC: %d, Att.: %s", c.name, c.level, c.maxHitPoints, c.armorClass, c.attackDice.String())
}

func (c *Character) MeleeDamageWithBonus(damageDice util.Dice) int {
	damage := damageDice.RollWithBonus(DamageBonus(c.strength))
	return max(1, damage)
}

func (c *Character) SetName(name string) {
	c.name = name
}

func (c *Character) Heal() {
	c.currentHitPoints = c.maxHitPoints
}

func (c *Character) GetArmorClass() int {
	return c.armorClass - ACBonus(c.dexterity)
}

func (c *Character) SetArmorClass(ac int) {
	c.armorClass = ac
}

func (c *Character) GetName() string {
	return c.name
}

func (c *Character) SetDamageDice(dice util.Dice) {
	c.attackDice = dice
}

func ToHitBonus(strength int) int {
	return defaultModifierPattern(strength)
}

func DamageBonus(strength int) int {
	switch {
	case strength <= 5:
		return -1
	case strength <= 15:
		return 0
	case strength <= 17:
		return 1
	case strength <= 18:
		return 2
	}
	return 0
}

func MagicSaveBonus(wisdom int) int {
	return defaultModifierPattern(wisdom)
}

func HitPointBonus(constitution int) int {
	return defaultModifierPattern(constitution)
}

func InitiativeBonus(dexterity int) int {
	switch {
	case dexterity <= 3:
		return -2
	case dexterity <= 5:
		return -1
	case dexterity <= 8:
		return -1
	case dexterity <= 12:
		return 0
	case dexterity <= 15:
		return 1
	case dexterity <= 17:
		return 1
	case dexterity <= 18:
		return 2
	}
	return 0
}
func RangedBonus(dexterity int) int {
	return defaultModifierPattern(dexterity)
}
func ACBonus(dexterity int) int {
	return defaultModifierPattern(dexterity)
}

func RollBashDoor(strength int) bool {
	return util.InSix(BashDoorChanceInSix(strength))
}

func NewRandomFighter(name string) *Character {
	character := NewRandomCharacter(GetFighterSpecs())
	character.SetName(name)
	return character
}
func NewMinFighter(name string) *Character {
	character := NewMinCharacter(GetFighterSpecs())
	character.SetName(name)
	return character
}
func NewMaxFighter(name string) *Character {
	character := NewMaxCharacter(GetFighterSpecs())
	character.SetName(name)
	return character
}

func NewAverageFighter(name string) *Character {
	character := NewAverageCharacter(GetFighterSpecs())
	character.SetName(name)
	return character
}
func NewMonster(specs MonsterSpecs) *Character {
	level := specs.HitDice.DiceCount()
	hitPoints := max(1, specs.HitDice.Avg()) // OSE
	//hitPoints := specs.HitDice.DiceCount() + 1 // ITDR
	return &Character{
		name:             specs.Name,
		strength:         level,
		intelligence:     level,
		dexterity:        level,
		maxHitPoints:     hitPoints,
		currentHitPoints: hitPoints,
		armorClass:       specs.AC,
		attackDice:       specs.AttackDice,
		level:            level,
	}
}
func NewRandomCharacter(classSpecs ClassSpecs) *Character {
	attrDice := util.ParseDice("3d6")
	strength := attrDice.Roll()
	intelligence := attrDice.Roll()
	dexterity := attrDice.Roll()
	constitution := attrDice.Roll()
	maxHitPoints := max(1, classSpecs.HitDie.Roll()+HitPointBonus(constitution))

	return &Character{
		level:            1,
		strength:         strength,
		intelligence:     intelligence,
		dexterity:        dexterity,
		maxHitPoints:     maxHitPoints,
		currentHitPoints: maxHitPoints,
		gold:             attrDice.Roll() * 10,
		armorClass:       0,
		attackDice:       util.ParseDice("1d6"),
	}
}

func NewAverageCharacter(classSpecs ClassSpecs) *Character {
	attrDice := util.ParseDice("3d6")
	strength := attrDice.Avg()
	intelligence := attrDice.Avg()
	dexterity := attrDice.Avg()
	constitution := attrDice.Avg()

	maxHitPoints := max(1, classSpecs.HitDie.Avg()+HitPointBonus(constitution))

	return &Character{
		level:            1,
		strength:         strength,
		intelligence:     intelligence,
		dexterity:        dexterity,
		maxHitPoints:     maxHitPoints,
		currentHitPoints: maxHitPoints,
		gold:             attrDice.Avg() * 10,
		armorClass:       0,
		attackDice:       util.ParseDice("1d6"),
	}
}

func NewMinCharacter(classSpecs ClassSpecs) *Character {
	attrDice := util.ParseDice("3d6")
	constitution := attrDice.Min()
	maxHitPoints := max(1, classSpecs.HitDie.Min()+HitPointBonus(constitution))
	return &Character{
		level:            1,
		strength:         attrDice.Min(),
		intelligence:     attrDice.Min(),
		dexterity:        attrDice.Min(),
		maxHitPoints:     maxHitPoints,
		currentHitPoints: maxHitPoints,
		gold:             attrDice.Min() * 10,
		armorClass:       0,
		attackDice:       util.ParseDice("1d6"),
	}
}

func NewMaxCharacter(classSpecs ClassSpecs) *Character {
	attrDice := util.ParseDice("3d6")
	constitution := attrDice.Max()
	maxHitPoints := max(1, classSpecs.HitDie.Max()+HitPointBonus(constitution))
	return &Character{
		level:            1,
		strength:         attrDice.Max(),
		intelligence:     attrDice.Max(),
		dexterity:        attrDice.Max(),
		maxHitPoints:     maxHitPoints,
		currentHitPoints: maxHitPoints,
		gold:             attrDice.Max() * 10,
		armorClass:       0,
		attackDice:       util.ParseDice("1d6"),
	}
}

func defaultModifierPattern(attribute int) int {
	switch {
	case attribute <= 3:
		return -3
	case attribute <= 5:
		return -2
	case attribute <= 8:
		return -1
	case attribute <= 12:
		return 0
	case attribute <= 15:
		return 1
	case attribute <= 17:
		return 2
	case attribute <= 18:
		return 3
	}
	return 0
}

func BashDoorChanceInSix(strength int) int {
	switch {
	case strength <= 8:
		return 1
	case strength <= 12:
		return 2
	case strength <= 15:
		return 3
	case strength <= 17:
		return 4
	case strength <= 18:
		return 5
	}
	return 0
}

type CombatAnalysis struct {
	Character  *Character
	Encounters []EncounterReport
	WinRate    float64

	MissRate float64
}

func (a CombatAnalysis) String() string {
	return fmt.Sprintf("Survival Rate: %.2f%%", a.WinRate*100)
}
func AnalyseReports(char *Character, encounters []EncounterReport) CombatAnalysis {
	wins := 0
	misses, attacks := 0, 0
	for _, encounter := range encounters {
		if encounter.Winner == char {
			wins++
		}
		m, a := encounter.MissesAndAttacks(char)
		misses += m
		attacks += a
	}
	winRate := float64(wins) / float64(len(encounters))
	missRate := float64(misses) / float64(attacks)
	return CombatAnalysis{
		Character:  char,
		Encounters: encounters,
		WinRate:    winRate,
		MissRate:   missRate,
	}
}

type EncounterReport struct {
	FirstStrike *Character
	Attacks     []AttackReport
	Winner      *Character
}

func (e EncounterReport) MissesAndAttacks(char *Character) (int, int) {
	misses := 0
	attackCount := 0
	for _, attack := range e.Attacks {
		if attack.Attacker == char && attack.Missed {
			misses++
		}
		attackCount++
	}
	return misses, attackCount
}
func (e EncounterReport) DamageAnalysis(char *Character) (minDmg, maxDmg int, avgDmg float64) {
	total := 0
	attackCount := 0
	minDmg = 1000
	maxDmg = 0
	for _, attack := range e.Attacks {
		if attack.Attacker == char {
			total += attack.DamageDone
			attackCount++
			if attack.DamageDone < minDmg {
				minDmg = attack.DamageDone
			}
			if attack.DamageDone > maxDmg {
				maxDmg = attack.DamageDone
			}
		}
	}
	return minDmg, maxDmg, float64(total) / float64(attackCount)
}

type AttackReport struct {
	Attacker   *Character
	Defender   *Character
	DamageDone int
	Missed     bool
}

func SimulateEncounter(player *Character, monster *Character, silent bool, isHit func(attacker, defender *Character) bool, meleeDamage func(attacker, defender *Character, damageDice util.Dice) int) EncounterReport {
	report := EncounterReport{}
	sixer := util.ParseDice("1d6")

	initiativePlayer := 0
	initiativeMonster := 0
	for initiativePlayer == initiativeMonster {
		initiativePlayer = sixer.Roll()
		initiativeMonster = sixer.Roll()
	}

	var firstStriker *Character
	var otherStriker *Character
	if initiativePlayer > initiativeMonster {
		firstStriker = player
		otherStriker = monster
	} else {
		firstStriker = monster
		otherStriker = player
	}
	report.FirstStrike = firstStriker

	printHealthReport := func() {
		if silent {
			return
		}
		println(fmt.Sprintf("\n%s HP: %d/%d", player.name, player.currentHitPoints, player.maxHitPoints))
		println(fmt.Sprintf("%s HP: %d/%d", monster.name, monster.currentHitPoints, monster.maxHitPoints))
	}
	if !silent {
		println(fmt.Sprintf("\n%s attacks first", firstStriker.name))
	}
	for firstStriker.IsAlive() && otherStriker.IsAlive() {
		printHealthReport()
		if !silent {
			println(fmt.Sprintf("%s attacks %s", firstStriker.name, otherStriker.name))
		}
		if isHit(firstStriker, otherStriker) {
			damage := meleeDamage(firstStriker, otherStriker, firstStriker.attackDice)
			otherStriker.currentHitPoints -= damage
			report.Attacks = append(report.Attacks, AttackReport{
				Attacker:   firstStriker,
				Defender:   otherStriker,
				DamageDone: damage,
				Missed:     false,
			})
			if !silent {
				println(fmt.Sprintf("%s hits for %d damage", firstStriker.name, damage))
			}
		} else {
			if !silent {
				println("the attack misses.")
			}
			report.Attacks = append(report.Attacks, AttackReport{
				Attacker:   firstStriker,
				Defender:   otherStriker,
				DamageDone: 0,
				Missed:     true,
			})
		}

		firstStriker, otherStriker = otherStriker, firstStriker
	}

	if player.IsAlive() {
		report.Winner = player
	} else {
		report.Winner = monster
	}

	if !silent {
		println("\n")
		if player.IsAlive() {
			println(fmt.Sprintf("%s WON", player.name))
		} else {
			println(fmt.Sprintf("%s WON", monster.name))
		}
	}

	return report
}
func SurvivalRate(player *Character, monster *Character, silent bool, iterations int) float64 {
	fxIsHit := func(attacker, defender *Character) bool {
		return true
	}
	fxMeleeDamage := func(attacker, defender *Character, damageDice util.Dice) int {
		protectionValue := defender.armorClass
		rawDamage := damageDice.Roll()
		/*
			if attacker == monster {
				monsterDamageDice := thac0ToDamageDice(attacker.thac0)
				rawDamage = monsterDamageDice.Roll()
			}

		*/

		flatReduced := max(1, rawDamage - protectionValue)
		//reducedDamage := math.Pow(1.1, float64(flatReduced))
		return max(1, int(flatReduced))
	}

	winsFxMechanics := 0
	for i := 0; i < iterations; i++ {
		player.Heal()
		monster.Heal()
		report := SimulateEncounter(player, monster, silent, fxIsHit, fxMeleeDamage)
		if report.Winner == player {
			winsFxMechanics++
		}
	}
	fxRate := float64(winsFxMechanics) / float64(iterations)

	return fxRate
}

func getProtectionFromAC(defenderAC int) int {
	armorProtectionTable := map[int]int{
		10: 0,
		9:  0,
		8:  1,
		7:  1,
		6:  1,
		5:  1,
		4:  2,
		3:  2,
		2:  2,
		1:  2,
		0:  3,
		-1: 3,
	}
	protectionValue := 0
	if defenderAC < -1 {
		protectionValue = 6
	} else if defenderAC > 10 {
		protectionValue = 0
	} else {
		protectionValue = armorProtectionTable[defenderAC]
	}
	return protectionValue
}

func thac0ToDamageDice(thac0 int) util.Dice {
	if thac0 > 19 {
		return util.ParseDice("d4-1")
	}
	switch thac0 {
	case 19:
		return util.ParseDice("d4-1")
	case 18:
		return util.ParseDice("d4")
	case 17:
		return util.ParseDice("d6")
	case 16:
		return util.ParseDice("d8")
	case 15:
		return util.ParseDice("d8+1")
	case 14:
		return util.ParseDice("2d6-1")
	case 13:
		return util.ParseDice("2d6")
	case 12:
		return util.ParseDice("2d6+1")
	case 11:
		return util.ParseDice("2d8")
	case 10:
		return util.ParseDice("3d6")
	case 9:
		return util.ParseDice("3d6+1")
	case 8:
		return util.ParseDice("3d8-1")
	case 7:
		return util.ParseDice("3d8")
	case 6:
		return util.ParseDice("4d6")
	default:
		return util.ParseDice("4d6+1")
	}
}
func IsMeleeHit(attackerStrength int, attackerThac0 int, defenderAC int) bool {
	d20 := util.ParseDice("1d20")
	roll := d20.Roll()
	if roll == 20 { // natural 20 is always a hit
		return true
	}
	if roll == 1 { // natural 1 is always a miss
		return false
	}

	modifier := ToHitBonus(attackerStrength)
	roll += modifier

	requiredRoll := attackerThac0 - defenderAC

	return roll >= requiredRoll
}

func MonsterLevelToTHAC0(level int) int {
	switch {
	case level <= 1:
		return 19
	case level <= 2:
		return 18
	case level <= 3:
		return 17
	case level <= 4:
		return 16
	case level <= 5:
		return 15
	case level <= 6:
		return 14
	case level <= 7:
		return 13
	case level <= 9:
		return 12
	case level <= 11:
		return 11
	case level <= 13:
		return 10
	case level <= 15:
		return 9
	case level <= 17:
		return 8
	case level <= 19:
		return 7
	case level <= 21:
		return 6
	default:
		return 5
	}
}
