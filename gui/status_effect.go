package gui

import (
	"fmt"
	"image/color"
	"rpg/common"
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

// statusEffectInfo is the display name and the one-line explanation shown when hovering a status icon.
var statusEffectInfo = map[common.StatusEffect][2]string{
	common.StatusInvisible:      {"Invisible", "Monsters cannot see you."},
	common.StatusStunned:        {"Stunned", "You cannot act."},
	common.StatusScared:         {"Scared", "You flee from what you see."},
	common.StatusSilenced:       {"Silenced", "You cannot cast spells."},
	common.StatusPacified:       {"Pacified", "You cannot attack."},
	common.StatusImmobile:       {"Immobile", "You cannot move."},
	common.StatusConfused:       {"Confused", "Your actions go astray."},
	common.StatusPoisoned:       {"Poisoned", "You lose health each turn."},
	common.StatusCharmed:        {"Charmed", "You fight for the other side."},
	common.StatusBerserk:        {"Berserk", "You attack whatever is nearest."},
	common.StatusHaywire:        {"Haywire", "Your magic goes off at random."},
	common.StatusFlying:         {"Flying", "You cross obstacles and dodge blows."},
	common.StatusVision:         {"Vision", "You see the whole level and everything alive on it."},
	common.StatusBlinded:        {"Blinded", "You cannot see and rarely hit."},
	common.StatusSlow:           {"Slowed", "You act less often."},
	common.StatusHasted:         {"Hasted", "You act more often."},
	common.StatusDamageOverTime: {"Bleeding", "You lose health each turn."},
	common.StatusRage:           {"Rage", "Your blows land harder."},
	common.StatusStone:          {"Petrified", "You are stone: unable to act, unable to be hurt."},
	common.StatusManaShield:     {"Mana Shield", "Mana pays for the damage you take, until it runs out."},
	common.StatusInfravision:    {"Infravision", "You see living things through walls."},
}

func (s *StatusEffectWidget) Name() string {
	if info, ok := statusEffectInfo[common.StatusEffect(s.statusEffect)]; ok {
		return info[0]
	}
	return s.statusEffect
}

func (s *StatusEffectWidget) GetTooltipLines(func(string) []util.ColoredTextPart) []util.LabelText {
	lines := []util.LabelText{{Text: s.Name(), TextColor: defs.White}}
	if info, ok := statusEffectInfo[common.StatusEffect(s.statusEffect)]; ok {
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
