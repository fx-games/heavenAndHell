package common

import "rpg/src/util"

type UntargetedEffect struct {
	Effect      func(user *Actor, source EffectSource, forced bool) // forced: the player asked to hit non-hostiles too
	Description []util.LabelText
}

func (u UntargetedEffect) Invoke(user *Actor, source EffectSource, forced bool) {
	u.Effect(user, source, forced)
}

func (u UntargetedEffect) GetDescription() []util.LabelText {
	return u.Description
}

type EffectSource string
