package gui

import (
	"fmt"
	"image/color"
	"rpg/defs"
	"rpg/util"
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
	return s.statusEffect
}

func (s *StatusEffectWidget) GetTooltipLines(func(string) []util.ColoredTextPart) []util.LabelText {
	return []util.LabelText{
		{
			Text:      fmt.Sprintf("%s for %d turns", s.statusEffect, s.turnsLeft),
			TextColor: defs.White,
		},
	}
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
