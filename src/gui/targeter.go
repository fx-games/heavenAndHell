package gui

import (
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/renderer"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Targeter struct {
	rules        common.TargetRules
	hitEffect    *common.HitEffect
	isDone       bool
	uiController *UIController
	targetInfo   common.TargetInfo
	confirmKey   ebiten.Key // the key that started targeting (e.g. a spell menu hotkey) confirms it too
	hasConfirm   bool
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
		if t.HasValidTarget() {
			t.Activate()
		} else {
			t.isDone = true
			return true
		}
	case PlayerCommandUp:
		nextTargetPos := t.targetInfo.AdjustedTarget.Add(geometry.Point{Y: -1})
		t.updateTarget(nextTargetPos)
		return true
	case PlayerCommandDown:
		nextTargetPos := t.targetInfo.AdjustedTarget.Add(geometry.Point{Y: 1})
		t.updateTarget(nextTargetPos)
		return true
	case PlayerCommandLeft:
		nextTargetPos := t.targetInfo.AdjustedTarget.Add(geometry.Point{X: -1})
		t.updateTarget(nextTargetPos)
		return true
	case PlayerCommandRight:
		nextTargetPos := t.targetInfo.AdjustedTarget.Add(geometry.Point{X: 1})
		t.updateTarget(nextTargetPos)
		return true
	case PlayerCommandUpRight:
		nextTargetPos := t.targetInfo.AdjustedTarget.Add(geometry.Point{X: 1, Y: -1})
		t.updateTarget(nextTargetPos)
		return true
	case PlayerCommandUpLeft:
		nextTargetPos := t.targetInfo.AdjustedTarget.Add(geometry.Point{X: -1, Y: -1})
		t.updateTarget(nextTargetPos)
		return true
	case PlayerCommandDownRight:
		nextTargetPos := t.targetInfo.AdjustedTarget.Add(geometry.Point{X: 1, Y: 1})
		t.updateTarget(nextTargetPos)
		return true
	case PlayerCommandDownLeft:
		nextTargetPos := t.targetInfo.AdjustedTarget.Add(geometry.Point{X: -1, Y: 1})
		t.updateTarget(nextTargetPos)
		return true

	}
	return false
}

func (t *Targeter) OnMouseClicked(mapPos geometry.Point) bool {
	if t.IsDone() {
		return false
	}
	t.updateTarget(mapPos)
	return t.Activate()
}

func (t *Targeter) OnMouseMoved(mapPos geometry.Point) (bool, Tooltip) { // THIS IS THE PROBLEM
	if mapPos == t.targetInfo.AdjustedTarget {
		return false, NoTooltip{}
	}
	t.updateTarget(mapPos)
	return true, NoTooltip{}
}

func (t *Targeter) Draw(renderer *renderer.MapRenderer) {
	if t.IsDone() {
		return
	}
	for _, highlight := range t.targetInfo.Highlights {
		if highlight.Pos == t.targetInfo.AdjustedTarget && t.targetInfo.InfoText != "" {
			screenOffset := geometry.Point{X: 0, Y: 0}
			renderer.DrawStringOnMapWithOffset(t.targetInfo.AdjustedTarget, screenOffset, t.targetInfo.InfoText, defs.OffWhite)
		}
		renderer.DrawOnMap(highlight.Pos, defs.IconTargeting, highlight.Color)
	}
}

// Cycle moves the cursor to the next (step 1) or previous (step -1) valid target, wrapping around.
func (t *Targeter) Cycle(step int) {
	ts := t.rules.Targets()
	if len(ts) == 0 {
		return
	}
	i := slices.Index(ts, t.targetInfo.AdjustedTarget)
	if i < 0 && step < 0 {
		i = 0 // off every target: Tab lands on the nearest, Shift+Tab on the farthest
	}
	t.updateTarget(ts[((i+step)%len(ts)+len(ts))%len(ts)])
}

func (t *Targeter) updateTarget(pos geometry.Point) {
	t.targetInfo = t.rules.OnTargetChanged(pos)
}

func (t *Targeter) HasValidTarget() bool {
	return t.targetInfo.IsValid
}

func (t *Targeter) Activate() bool {
	if t.HasValidTarget() {
		t.hitEffect.Activate(t.rules.GetAttacker(), t.targetInfo.AdjustedTarget, t.targetInfo.Hitpositions)
		t.isDone = true
		return true
	}
	return false
}

func NewTargeter(uiController *UIController, rules common.TargetRules, onTargetSelected *common.HitEffect) *Targeter {
	t := &Targeter{
		rules:        rules,
		uiController: uiController,
		hitEffect:    onTargetSelected,
		targetInfo:   rules.OnTargetChanged(rules.GetPreSelectedTarget()),
	}
	t.updateTarget(rules.GetPreSelectedTarget())
	for _, key := range inpututil.AppendJustPressedKeys(nil) {
		if key != ebiten.KeyShift && key != ebiten.KeyControl && key != ebiten.KeyAlt {
			t.confirmKey, t.hasConfirm = key, true
		}
	}
	return t
}
