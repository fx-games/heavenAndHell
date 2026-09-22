package defs

import "rpg/src/util"

type ThrowDefinition struct {
	Damage        util.Interval
	ChanceToBreak float64
	HitZone       string
	OnHitEffect   string
}

func (d ThrowDefinition) IsValid() bool {
	return d.OnHitEffect != "" || d.Damage.NotZero()
}
