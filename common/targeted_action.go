package common

import (
	"image/color"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/util"
)

type AoEResolver func(origin, target geometry.Point) []geometry.Point
type HitEffect func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point)
type TargetRulesProvider func(attacker *Actor, hitPositions AoEResolver, currentMap *gridmap.GridMap[*Actor, Item, Object]) TargetRules

type TargetRules interface {
	IsVisiblePositionOnMap(target geometry.Point) bool
	IsValidTarget(target geometry.Point) bool
	GetPreSelectedTarget() geometry.Point
	GetHighlightsForTarget(target geometry.Point) []TargetHighlight
	GetHitpositions(target geometry.Point) []geometry.Point
	GetAttacker() *Actor
}
type UntargetedActionName string
type UntargetedAction struct {
	*BaseTimedBeing
	name                   string
	activationDelayInTurns int
	effect                 func(user *Actor)
	user                   *Actor
	turnsUntilRemoval      int
	icon                   int32
	description            string
	timeNeededInAut        int
	monetaryValue          int
	condition              func(user *Actor) bool
}

func NewUntargetedAction(name string, icon int32) *UntargetedAction {
	return &UntargetedAction{
		name: name,
		icon: icon,
		timeNeededInAut: 10,
	}
}

func (a *UntargetedAction) SetMonetaryValue(value int) {
	a.monetaryValue = value
}

func (a *UntargetedAction) GetTooltipLines() []util.LabelText {
	return []util.LabelText{
		{Text: a.description, TextColor: DarkTextColor},
	}
}

func (a *UntargetedAction) InventoryIcon() int32 {
	return a.icon
}

func (a *UntargetedAction) TintColor() color.Color {
	return color.White
}

func (a *UntargetedAction) IsAlive() bool {
	return a.activationDelayInTurns > 0
}

func (a *UntargetedAction) Turn() {
	a.activationDelayInTurns--
	if a.activationDelayInTurns == 0 {
		a.Invoke(a.user)
	}
}

func (a *UntargetedAction) IsDelayed() bool {
	return a.activationDelayInTurns > 0
}

func (a *UntargetedAction) Name() string {
	return string(a.name)
}
func (a *UntargetedAction) String() string {
	return string(a.name)
}

func (a *UntargetedAction) SetCurrentUser(user *Actor) {
	a.user = user
}

func (a *UntargetedAction) Invoke(actor *Actor) {
	a.effect(actor)
}

func (a *UntargetedAction) GetDelayInTurns() int {
	return a.activationDelayInTurns
}

func (a *UntargetedAction) Reset() {
	a.activationDelayInTurns = a.turnsUntilRemoval
}
func (a *UntargetedAction) Counter() int {
	if a.IsDelayed() {
		return a.GetDelayInTurns()
	}
	return -1
}

func (a *UntargetedAction) SetEffect(f func(user *Actor)) {
	a.effect = f
}

func (a *UntargetedAction) SetDelay(delayToActivationInTurns int) {
	a.BaseTimedBeing = NewBaseTimedBeing()
	a.activationDelayInTurns = delayToActivationInTurns
	a.turnsUntilRemoval = delayToActivationInTurns
}

func (a *UntargetedAction) SetDescription(description string) {
	a.description = description
}

func (a *UntargetedAction) GetDescription() string {
	return a.description
}

func (a *UntargetedAction) GetTimeNeededInAut() int {
	return a.timeNeededInAut
}

func (a *UntargetedAction) SetNoTimeNeeded() {
	a.timeNeededInAut = 0
}

func (a *UntargetedAction) GetMonetaryValue() int {
	return a.monetaryValue
}

func (a *UntargetedAction) Clone() *UntargetedAction {
	return &UntargetedAction{
		name:                   a.name,
		activationDelayInTurns: a.activationDelayInTurns,
		effect:                 a.effect,
		user:                   a.user,
		turnsUntilRemoval:      a.turnsUntilRemoval,
		icon:                   a.icon,
		description:            a.description,
		timeNeededInAut:        a.timeNeededInAut,
		monetaryValue:          a.monetaryValue,
	}
}

func (a *UntargetedAction) CanActivate(user *Actor) bool {
	if a.condition == nil {
		return true
	}
	return a.condition(user)
}

func (a *UntargetedAction) SetCondition(condition func(user *Actor) bool) {
	a.condition = condition
}

type TargetHighlight struct {
	Pos   geometry.Point
	Color color.Color
}
type TargetedActionName string
type TargetedAction struct {
	getTargetRules  TargetRulesProvider
	hitPattern      AoEResolver
	onSelected      HitEffect
	displayName     string
	icon            int32
	description     string
	timeNeededInAut int
	dungeonLevel    int
	internalName    string
}

func NewTargetedAction(name string, hitPattern AoEResolver, onSelected HitEffect, getTargetRules TargetRulesProvider) *TargetedAction {
	return &TargetedAction{
		hitPattern:      hitPattern,
		onSelected:      onSelected,
		displayName:     name,
		getTargetRules:  getTargetRules,
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

func (t *TargetedAction) GetTooltipLines() []util.LabelText {
	return []util.LabelText{
		{Text: string(t.displayName), TextColor: color.White},
	}
}

func (t *TargetedAction) InventoryIcon() int32 {
	return t.icon
}

func (t *TargetedAction) TintColor() color.Color {
	return color.White
}

func (t *TargetedAction) GetHitPositions(origin, target geometry.Point) []geometry.Point {
	return t.hitPattern(origin, target)
}
func (t *TargetedAction) OnSelected(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
	t.proc(attacker, target, hitPositions)
}
func (t *TargetedAction) Execute(attacker *Actor, origin, forTarget geometry.Point) {
	t.proc(attacker, forTarget, t.hitPattern(origin, forTarget))
}

func (t *TargetedAction) proc(user *Actor, target geometry.Point, hitPositions []geometry.Point) {
	t.onSelected(user, target, hitPositions)
}

func (t *TargetedAction) SetIcon(icon int32) {
	t.icon = icon
}

func (t *TargetedAction) GetTargetingRules(actor *Actor, currentMap *gridmap.GridMap[*Actor, Item, Object]) TargetRules {
	return t.getTargetRules(actor, t.hitPattern, currentMap)
}

func (t *TargetedAction) GetTimeNeededInAut() int {
	return t.timeNeededInAut
}

func (t *TargetedAction) GetDescription() string {
	return t.description
}

func (t *TargetedAction) GetMonetaryValue() int {
	return t.dungeonLevel
}

func (t *TargetedAction) SetInternalName(name string) {
	t.internalName = name
}
