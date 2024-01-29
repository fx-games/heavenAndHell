package game

import (
    "fmt"
    "image/color"
    "rpg/common"
    "rpg/util"
)

type StatusEffectWidget struct {
    statusEffect string
    turnsLeft int
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

func (s *StatusEffectWidget) GetTooltipLines() []util.LabelText {
    return []util.LabelText{
        {
            Text:      fmt.Sprintf("%s for %d turns", s.statusEffect, s.turnsLeft),
            TextColor: color.White,
        },
    }
}

func (s *StatusEffectWidget) InventoryIcon() int32 {
    return common.IconEffectLightning
}

func (s *StatusEffectWidget) TintColor() color.Color {
    return color.White
}

func (s *StatusEffectWidget) Counter() int {
    return s.turnsLeft
}
