package defs

import "rpg/stats"

type ThrowDefinition struct {
	Damage        stats.Interval
	ChanceToBreak float64
	HitZone       string
	OnHitEffect   string
}

func (d ThrowDefinition) IsValid() bool {
	return d.OnHitEffect != "" || d.Damage.NotZero()
}
