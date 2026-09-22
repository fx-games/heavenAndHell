package util

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
)

type Dice struct {
	dice  int
	sides int
	bonus int
}

func NewDice(dice, sides, bonus int) Dice {
	return Dice{dice, sides, bonus}
}

func (r Dice) Max() int {
	return (r.dice * r.sides) + r.bonus
}

func (r Dice) Min() int {
	return r.dice + r.bonus
}

func (r Dice) String() string {
	if r.isFixed() {
		flatValue := r.Min()
		return fmt.Sprintf("%d", flatValue)
	}
	if r.bonus == 0 {
		return fmt.Sprintf("%dd%d (%d-%d)", r.dice, r.sides, r.Min(), r.Max())
	}
	return fmt.Sprintf("%dd%d%+d (%d-%d)", r.dice, r.sides, r.bonus, r.Min(), r.Max())
}

func (r Dice) isFixed() bool {
	return r.Min() == r.Max()
}

func (r Dice) ShortString() string {
	if r.isFixed() {
		flatValue := r.Min()
		return fmt.Sprintf("%d", flatValue)
	}
	if r.bonus == 0 {
		return fmt.Sprintf("%dd%d", r.dice, r.sides)
	}
	return fmt.Sprintf("%dd%d%+d", r.dice, r.sides, r.bonus)
}
func (r Dice) Roll() int {
	if r.isFixed() {
		flatValue := r.Min()
		return flatValue
	}
	result := r.bonus
	for i := 0; i < r.dice; i++ {
		result += 1 + rand.Intn(r.sides)
	}
	return result
}

func (r Dice) DiceCount() int {
	return r.dice
}

func (r Dice) WithBonus(plus int) Dice {
	return NewDice(r.dice, r.sides, r.bonus+plus)
}

func (r Dice) NotZero() bool {
	return r.dice > 0 || r.sides > 0 || r.bonus > 0
}

func ParseDice(dice string) Dice {
	// check if it's an interval, eg. 12-24
	dice = strings.ToLower(strings.Replace(dice, " ", "", -1))

	intervalPattern := regexp.MustCompile(`^(\d+)-(\d+)$`)

	flatNumberPattern := regexp.MustCompile(`^(\d+)$`)

	hasDiceNotation := strings.ContainsAny(dice, "d")

	isInterval := intervalPattern.MatchString(dice)

	isFlatNumber := flatNumberPattern.MatchString(dice)

	if !hasDiceNotation && !isInterval && isFlatNumber {
		match := flatNumberPattern.FindStringSubmatch(dice)
		if match == nil {
			return Dice{0, 0, 0}
		}
		numVal, _ := strconv.Atoi(match[1])
		return NewDice(0, 0, numVal)
	}

	if !hasDiceNotation && !isFlatNumber && isInterval {
		match := intervalPattern.FindStringSubmatch(dice)
		if match == nil {
			return Dice{0, 0, 0}
		}
		minVal, _ := strconv.Atoi(match[1])
		maxVal, _ := strconv.Atoi(match[2])
		dieRange := (maxVal - minVal) + 1
		dieBonus := minVal - 1
		return NewDice(1, dieRange, dieBonus)
	}

	if !hasDiceNotation {
		return Dice{0, 0, 0}
	}

	// dice is a string in the format "NdM" where N is the number of dice to roll and M is the number of sides on each die
	if strings.HasPrefix(dice, "d") {
		dice = "1" + dice
	}
	if !strings.ContainsAny(dice, "-+") {
		dice = dice + "+0"
	}
	pattern := regexp.MustCompile(`(\d+)d(\d+)\s*[+\-]\s*(\d+)`)
	match := pattern.FindStringSubmatch(dice)
	if match == nil {
		return Dice{0, 0, 0}
	}
	numDice, _ := strconv.Atoi(match[1])
	numSides, _ := strconv.Atoi(match[2])
	bonus, _ := strconv.Atoi(match[3])
	return NewDice(numDice, numSides, bonus)
}

// Interval is a flat range, every value from min to max equally likely; Dice is a sum of rolls (bell-shaped).
// Both parse "min-max"; only Dice parses "NdM+B".
type Interval struct {
	min int
	max int
}

func NewInterval(min, max int) Interval {
	return Interval{
		min: min,
		max: max,
	}
}

func NewIntervalFromString(str string) Interval {
	if !strings.Contains(str, "-") {
		minDamg, _ := strconv.Atoi(str)
		return Interval{
			min: minDamg,
			max: minDamg,
		}
	}
	var minDamg, maxDamg int
	fmt.Sscanf(str, "%d-%d", &minDamg, &maxDamg)
	return Interval{
		min: minDamg,
		max: maxDamg,
	}
}

func (i Interval) Min() int {
	return i.min
}

func (i Interval) Max() int {
	return i.max
}

func (i Interval) Roll() int {
	if i.min == i.max {
		return i.min
	}
	// 2..6 (2,3,4,5,6) -> 6-2 = 4 -> Intn(4+1) -> 0..4 -> +2 -> 2..6
	return rand.Intn(i.max-i.min+1) + i.min
}

func (i Interval) ToString() string {
	if i.min == i.max {
		return fmt.Sprintf("%d", i.min)
	}
	return fmt.Sprintf("%d-%d", i.min, i.max)
}

func (i Interval) NotZero() bool {
	return i.min != 0 || i.max != 0
}

func (i Interval) AsTuple() Tuple[int, int] {
	return Tuple[int, int]{Item1: i.min, Item2: i.max}
}
