package gui

import (
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"
	"rpg/renderer"
)

type Targeter struct {
	rules             common.TargetRules
	hitEffect         *common.HitEffect
	currentTarget     geometry.Point
	currentHighlights []common.TargetHighlight
	isDone            bool
	uiController      *UIController
}

func (t *Targeter) IsDone() bool {
	return t.isDone
}

func (t *Targeter) OnCommand(command CommandType) bool {
	switch command {
	case PlayerCommandCancel:
		t.isDone = true
		return true
	case PlayerCommandConfirm:
		if t.rules.IsValidTarget(t.currentTarget) {
			t.hitEffect.Activate(t.rules.GetAttacker(), t.currentTarget, t.rules.GetHitpositions(t.currentTarget))
			t.isDone = true
			return true
		} else {
			t.isDone = true
			return true
		}
	case PlayerCommandUp:
		nextTargetPos := t.currentTarget.Add(geometry.Point{Y: -1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case PlayerCommandDown:
		nextTargetPos := t.currentTarget.Add(geometry.Point{Y: 1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case PlayerCommandLeft:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: -1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case PlayerCommandRight:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: 1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case PlayerCommandUpRight:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: 1, Y: -1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case PlayerCommandUpLeft:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: -1, Y: -1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case PlayerCommandDownRight:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: 1, Y: 1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case PlayerCommandDownLeft:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: -1, Y: 1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}

	}
	return false
}

func (t *Targeter) OnMouseClicked(mapPos geometry.Point) bool {
	if t.IsDone() {
		return false
	}
	t.updateTarget(mapPos)
	if t.rules.IsValidTarget(mapPos) {
		t.hitEffect.Activate(t.rules.GetAttacker(), mapPos, t.rules.GetHitpositions(mapPos))
		t.isDone = true
		return true
	}
	return false
}

func (t *Targeter) OnMouseMoved(mapPos geometry.Point) (bool, Tooltip) { // THIS IS THE PROBLEM
	if mapPos == t.currentTarget || !t.rules.IsVisiblePositionOnMap(mapPos) {
		return false, NoTooltip{}
	}
	t.updateTarget(mapPos)
	return true, NoTooltip{}
}

func (t *Targeter) Draw(renderer *renderer.MapRenderer) {
	if t.IsDone() {
		return
	}
	for _, highlight := range t.currentHighlights {
		renderer.DrawOnMap(highlight.Pos, defs.IconTargeting, highlight.Color)
	}
}

func (t *Targeter) updateTarget(pos geometry.Point) {
	t.currentTarget = pos
	t.currentHighlights = t.rules.GetHighlightsForTarget(pos)
}

func (t *Targeter) HasValidTarget() bool {
	return t.rules.IsValidTarget(t.currentTarget)
}

func NewTargeter(uiController *UIController, rules common.TargetRules, onTargetSelected *common.HitEffect) *Targeter {
	return &Targeter{
		rules:             rules,
		uiController:      uiController,
		hitEffect:         onTargetSelected,
		currentTarget:     rules.GetPreSelectedTarget(),
		currentHighlights: rules.GetHighlightsForTarget(rules.GetPreSelectedTarget()),
	}
}
