package util

import (
	"fmt"
	"log/slog"
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

// DiceNotation is the mod's choice (mod.rec DamageNotation: dice) to show damage as NdS+B; otherwise every Dice
// shows as its min-max interval, like an Interval.
var DiceNotation bool

func (r Dice) String() string {
	if !DiceNotation {
		return NewInterval(r.Min(), r.Max()).ToString()
	}
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
	if !DiceNotation {
		return NewInterval(r.Min(), r.Max()).ToString()
	}
	if r.isFixed() {
		flatValue := r.Min()
		return fmt.Sprintf("%d", flatValue)
	}
	if r.bonus == 0 {
		return fmt.Sprintf("%dd%d", r.dice, r.sides)
	}
	return fmt.Sprintf("%dd%d%+d", r.dice, r.sides, r.bonus)
}
func (r Dice) Roll(rng *rand.Rand) int {
	if r.isFixed() {
		flatValue := r.Min()
		return flatValue
	}
	result := r.bonus
	for i := 0; i < r.dice; i++ {
		result += 1 + rng.Intn(r.sides)
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

// ParseDice reads "N", "min-max" or "NdS+B". Anything else is 0, with a warning: a typo in mod data should not
// pass as a harmless zero.
func ParseDice(dice string) Dice {
	d, ok := parseDice(dice)
	if !ok && strings.TrimSpace(dice) != "" {
		slog.Warn("mod data: not a dice expression, using 0", "value", dice)
	}
	return d
}

func parseDice(dice string) (Dice, bool) {
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
			return Dice{}, false
		}
		numVal, _ := strconv.Atoi(match[1])
		return NewDice(0, 0, numVal), true
	}

	if !hasDiceNotation && !isFlatNumber && isInterval {
		match := intervalPattern.FindStringSubmatch(dice)
		if match == nil {
			return Dice{}, false
		}
		minVal, _ := strconv.Atoi(match[1])
		maxVal, _ := strconv.Atoi(match[2])
		dieRange := (maxVal - minVal) + 1
		dieBonus := minVal - 1
		return NewDice(1, dieRange, dieBonus), true
	}

	if !hasDiceNotation {
		return Dice{}, false
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
		return Dice{}, false
	}
	numDice, _ := strconv.Atoi(match[1])
	numSides, _ := strconv.Atoi(match[2])
	bonus, _ := strconv.Atoi(match[3])
	return NewDice(numDice, numSides, bonus), true
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
	if !strings.Contains(strings.TrimPrefix(str, "-"), "-") { // a single number, maybe negative
		minDamg, err := strconv.Atoi(str)
		if err != nil && str != "" {
			slog.Warn("mod data: not a number or range, using 0", "value", str)
		}
		return Interval{
			min: minDamg,
			max: minDamg,
		}
	}
	var minDamg, maxDamg int
	if n, _ := fmt.Sscanf(str, "%d-%d", &minDamg, &maxDamg); n != 2 {
		slog.Warn("mod data: not a range", "value", str)
	}
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

func (i Interval) Roll(rng *rand.Rand) int {
	if i.min == i.max {
		return i.min
	}
	// 2..6 (2,3,4,5,6) -> 6-2 = 4 -> Intn(4+1) -> 0..4 -> +2 -> 2..6
	return rng.Intn(i.max-i.min+1) + i.min
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
