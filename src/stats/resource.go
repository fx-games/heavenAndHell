package stats

import (
	"fmt"
	"image/color"
	"math"
)

type Resource struct {
	min    int
	getMax func() int
	value  float64
}

func NewResource(min int, getMax func() int, value int) *Resource {
	return &Resource{
		min:    min,
		getMax: getMax,
		value:  float64(value),
	}
}

func (r *Resource) IsAtMin() bool {
	return math.Ceil(r.value) <= float64(r.min)
}

func (r *Resource) IsAtMax() bool {
	return math.Floor(r.value) >= float64(r.getMax())
}

func (r *Resource) Value() int {
	return int(r.value)
}

func (r *Resource) Increase(amount float64) {
	r.value += amount
	r.ensureRange()
}

func (r *Resource) Decrease(amount int) {
	r.value -= float64(amount)
	r.ensureRange()
}

func (r *Resource) ensureRange() {
	if r.value > float64(r.getMax()) {
		r.value = float64(r.getMax())
	} else if r.value < float64(r.min) {
		r.value = float64(r.min)
	}
}

func (r *Resource) IsPositive() bool {
	return r.value > 0
}

func (r *Resource) ToString() string {
	return fmt.Sprintf("%0.f/%d", math.Ceil(r.value), r.getMax())
}

func (r *Resource) GetMax() int {
	return r.getMax()
}

func (r *Resource) SetToMax() {
	r.value = float64(r.getMax())
}

func (r *Resource) SetToMin() {
	r.value = float64(r.min)
}

func (r *Resource) IsLessThanPercent(percentage float64) bool {
	percentageFilled := r.value / float64(r.getMax())
	return percentageFilled < percentage
}

func (r *Resource) IncreaseByPercentOfMax(factor float64) {
	increase := float64(r.getMax()) * factor
	r.value += increase
	r.ensureRange()
}

func (r *Resource) IncreaseByFloat(amount float64) {
	r.value += amount
	r.ensureRange()
}

func (r *Resource) Clone() *Resource {
	return NewResource(r.min, r.getMax, r.Value())
}

func (r *Resource) Min() int {
	return r.min
}

func (r *Resource) AsPercentage() float64 {
	return r.value / float64(r.getMax())
}

func (r *Resource) AsColor() color.RGBA {
	var resColor color.RGBA
	resColor = color.RGBA{R: 250, G: 250, B: 250, A: 255}

	if r.IsLessThanPercent(0.25) {
		resColor = color.RGBA{R: 255, G: 30, B: 30, A: 255}
	} else if r.IsLessThanPercent(0.5) {
		resColor = color.RGBA{R: 255, G: 150, B: 30, A: 255}
	} else if r.IsLessThanPercent(0.75) {
		resColor = color.RGBA{R: 255, G: 255, B: 30, A: 255}
	}
	return resColor
}
