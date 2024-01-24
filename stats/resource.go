package stats

import "fmt"

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
	return r.Value() == r.min
}

func (r *Resource) IsAtMax() bool {
	return r.Value() == r.getMax()
}

func (r *Resource) Value() int {
	return int(r.value)
}

func (r *Resource) Increase(amount int) {
	r.value += float64(amount)
	r.ensureRange()
}

func (r *Resource) Decrease(amount int) {
	r.value -= float64(amount)
	r.ensureRange()
}

func (r *Resource) ensureRange() {
	if r.Value() > r.getMax() {
		r.value = float64(r.getMax())
	} else if r.Value() < r.min {
		r.value = float64(r.min)
	}
}

func (r *Resource) IsPositive() bool {
	return r.Value() > 0
}

func (r *Resource) ToString() string {
	return fmt.Sprintf("%d/%d", r.Value(), r.getMax())
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
