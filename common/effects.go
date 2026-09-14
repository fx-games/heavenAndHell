package common

import "rpg/util"

type UntargetedEffect struct {
	Effect      func(user *Actor, source EffectSource)
	Description []util.LabelText
}

func (u UntargetedEffect) Invoke(user *Actor, source EffectSource) {
	u.Effect(user, source)
}

func (u UntargetedEffect) GetDescription() []util.LabelText {
	return u.Description
}

type EffectSource string
