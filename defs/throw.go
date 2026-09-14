package defs

import "rpg/stats"

type ThrowDefinition struct {
	Damage        stats.Interval
	ChanceToBreak float64
	HitZone       string
	OnHitEffect   string
}
