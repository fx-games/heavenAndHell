package common

import (
	"rpg/geometry"
	"rpg/stats"
)

type LocationDamage struct {
	Location geometry.Point
	Damage   int
}

type LocationDamagePrediction struct {
	Location geometry.Point
	Damage   stats.Interval
}
