package common

import (
	"image/color"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/util"
)

type AoEResolver struct {
	GetHitPositions func(origin, target geometry.Point) []geometry.Point
	Description     []util.LabelText
}
type HitEffect struct {
	Activate    func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point)
	Description []util.LabelText
}
type TargetRulesProvider struct {
	GetTargetRules func(attacker *Actor, hitPositions *AoEResolver, currentMap *gridmap.GridMap[*Actor, *Item, Object]) TargetRules
	Description    []util.LabelText
}

type TargetRules interface {
	OnTargetChanged(target geometry.Point) TargetInfo
	GetPreSelectedTarget() geometry.Point
	GetAttacker() *Actor
}
type TargetInfo struct {
	AdjustedTarget geometry.Point
	Highlights     []TargetHighlight
	Hitpositions   []geometry.Point
	IsValid        bool
	InfoText       string
}
type TargetHighlight struct {
	Pos   geometry.Point
	Color color.Color
}
type TargetedAction struct {
	rulesProvider   *TargetRulesProvider
	aoeResolver     *AoEResolver
	onHitEffect     *HitEffect
	displayName     string
	icon            int32
	timeNeededInAut int
	dungeonLevel    int
	internalName    string
}

func NewTargetedAction(name string, hitPattern *AoEResolver, onSelected *HitEffect, getTargetRules *TargetRulesProvider) *TargetedAction {
	return &TargetedAction{
		aoeResolver:     hitPattern,
		onHitEffect:     onSelected,
		displayName:     name,
		rulesProvider:   getTargetRules,
		timeNeededInAut: 10,
	}
}

func (t *TargetedAction) SetDungeonLevel(value int) {
	t.dungeonLevel = value
}
func (t *TargetedAction) Counter() int {
	return 0
}
func (t *TargetedAction) Name() string {
	return string(t.displayName)
}
func (t *TargetedAction) String() string {
	return string(t.displayName)
}

func (t *TargetedAction) GetTooltipLines(func(string) []util.ColoredTextPart) []util.LabelText {
	name := []util.LabelText{
		{Text: string(t.displayName), TextColor: color.White},
	}

	return append(name, t.GetDescription()...)
}

func (t *TargetedAction) InventoryIcon() int32 {
	return t.icon
}

func (t *TargetedAction) TintColor() color.RGBA {
	return defs.White
}

func (t *TargetedAction) GetHitPositions(origin, target geometry.Point) []geometry.Point {
	return t.aoeResolver.GetHitPositions(origin, target)
}
func (t *TargetedAction) OnSelected(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
	t.proc(attacker, target, hitPositions)
}
func (t *TargetedAction) Execute(attacker *Actor, origin, forTarget geometry.Point) {
	t.proc(attacker, forTarget, t.aoeResolver.GetHitPositions(origin, forTarget))
}

func (t *TargetedAction) proc(user *Actor, target geometry.Point, hitPositions []geometry.Point) {
	t.onHitEffect.Activate(user, target, hitPositions)
}

func (t *TargetedAction) SetIcon(icon int32) {
	t.icon = icon
}

func (t *TargetedAction) GetTargetingRules(actor *Actor, currentMap *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
	return t.rulesProvider.GetTargetRules(actor, t.aoeResolver, currentMap)
}

func (t *TargetedAction) GetTimeNeededInAut() int {
	return t.timeNeededInAut
}

func (t *TargetedAction) GetDescription() []util.LabelText {
	var desc []util.LabelText
	desc = append(desc, util.LabelTextFromString(""))
	desc = append(desc, util.LabelTextFromString("Targeting"))
	desc = append(desc, colorize(t.rulesProvider.Description, defs.OffWhite)...)
	desc = append(desc, util.LabelTextFromString(""))
	desc = append(desc, util.LabelTextFromString("Area of Effect"))
	desc = append(desc, colorize(t.aoeResolver.Description, defs.OffWhite)...)
	desc = append(desc, util.LabelTextFromString(""))
	desc = append(desc, util.LabelTextFromString("Effect"))
	desc = append(desc, colorize(t.onHitEffect.Description, defs.OffWhite)...)
	return desc
}

func colorize(description []util.LabelText, white color.RGBA) []util.LabelText {
	for i := range description {
		description[i].TextColor = white
	}
	return description
}

func (t *TargetedAction) GetMonetaryValue() int {
	return t.dungeonLevel
}

func (t *TargetedAction) SetInternalName(name string) {
	t.internalName = name
}

func (t *TargetedAction) GetInternalName() string {
	return t.internalName
}
