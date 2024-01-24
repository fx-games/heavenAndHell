package stats

import (
	"fmt"
	"math"
	"math/rand"
)

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

func (i Interval) Min() int {
	return i.min
}

func (i Interval) Max() int {
	return i.max
}

func (i Interval) LinearRandom() int {
	if i.min == i.max {
		return i.min
	}
	// 2..6 (2,3,4,5,6) -> 6-2 = 4 -> Intn(4+1) -> 0..4 -> +2 -> 2..6
	return rand.Intn(i.max-i.min + 1) + i.min
}

func (i Interval) PyramidRandom() int {
	return (i.LinearRandom() + i.LinearRandom()) / 2
}

func (i Interval) ToString() string {
	if i.min == i.max {
		return fmt.Sprintf("%d", i.min)
	}
	return fmt.Sprintf("%d-%d", i.min, i.max)
}

func (i Interval) Div(count int) Interval {
	return Interval{
		min: i.min / count,
		max: i.max / count,
	}
}

func (i Interval) Mul(factor float64) Interval {
	return Interval{
		min: int(float64(i.min) * factor),
		max: int(float64(i.max) * factor),
	}
}

func (i Interval) MulCeil(factor float64) Interval {
	return Interval{
		min: int(math.Ceil(float64(i.min) * factor)),
		max: int(math.Ceil(float64(i.max) * factor)),
	}
}

func (i Interval) Sub(other Interval) Interval {
	return Interval{
		min: i.min - other.min,
		max: i.max - other.max,
	}
}

func (i Interval) Add(other Interval) Interval {
	return Interval{
		min: i.min + other.min,
		max: i.max + other.max,
	}
}
