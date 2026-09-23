package gui

import (
	"fmt"
	"image/color"
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/util"
)

type StatusEffectWidget struct {
	statusEffect string
	turnsLeft    int
}

func (s *StatusEffectWidget) String() string {
	return s.statusEffect
}

func NewStatusEffectWidget(statusName string, turnsLeft int) *StatusEffectWidget {
	return &StatusEffectWidget{
		statusEffect: statusName,
		turnsLeft:    turnsLeft,
	}
}

func (s *StatusEffectWidget) Name() string {
	if info, ok := common.StatusInfo[common.StatusEffect(s.statusEffect)]; ok {
		return info[0]
	}
	return s.statusEffect
}

func (s *StatusEffectWidget) GetTooltipLines(func(string) []util.ColoredTextPart) []util.LabelText {
	lines := []util.LabelText{{Text: s.Name(), TextColor: defs.White}}
	if info, ok := common.StatusInfo[common.StatusEffect(s.statusEffect)]; ok {
		lines = append(lines, util.LabelText{Text: info[1], TextColor: defs.LightGray})
	}
	if s.turnsLeft < 0 {
		lines = append(lines, util.LabelText{Text: "Lasts until dispelled.", TextColor: defs.DarkYellow})
	} else {
		lines = append(lines, util.LabelText{Text: fmt.Sprintf("%d turns left.", s.turnsLeft), TextColor: defs.DarkYellow})
	}
	return lines
}

func (s *StatusEffectWidget) InventoryIcon() int32 {
	return defs.IconEffectLightning
}

func (s *StatusEffectWidget) TintColor() color.RGBA {
	return defs.White
}

func (s *StatusEffectWidget) Counter() int {
	return s.turnsLeft
}
