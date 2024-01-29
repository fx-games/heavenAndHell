package game

import (
	"image/color"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/ui"
)


type Targeter struct {
	rules             common.TargetRules
	onTargetSelected  common.SelectionHandler
	currentTarget     geometry.Point
	currentHighlights []common.TargetHighlight
	renderer          *HybridRenderer
	isDone            bool
}

func (t *Targeter) IsDone() bool {
	return t.isDone
}

func (t *Targeter) OnCommand(command ui.CommandType) bool {
	switch command {
	case ui.PlayerCommandCancel:
		t.isDone = true
		return true
	case ui.PlayerCommandConfirm:
		if t.rules.IsValidTarget(t.currentTarget) {
			t.onTargetSelected(t.rules.GetAttacker(), t.currentTarget, t.rules.GetHitpositions(t.currentTarget))
			t.isDone = true
			return true
		} else {
			t.isDone = true
			return true
		}
	case ui.PlayerCommandUp:
		nextTargetPos := t.currentTarget.Add(geometry.Point{Y: -1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case ui.PlayerCommandDown:
		nextTargetPos := t.currentTarget.Add(geometry.Point{Y: 1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case ui.PlayerCommandLeft:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: -1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case ui.PlayerCommandRight:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: 1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case ui.PlayerCommandUpRight:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: 1, Y: -1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case ui.PlayerCommandUpLeft:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: -1, Y: -1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case ui.PlayerCommandDownRight:
		nextTargetPos := t.currentTarget.Add(geometry.Point{X: 1, Y: 1})
		if t.rules.IsValidTarget(nextTargetPos) {
			t.updateTarget(nextTargetPos)
			return true
		}
	case ui.PlayerCommandDownLeft:
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
		t.onTargetSelected(t.rules.GetAttacker(), mapPos, t.rules.GetHitpositions(mapPos))
		t.isDone = true
		return true
	}
	return false
}

func (t *Targeter) OnMouseMoved(mapPos geometry.Point) (bool, ui.Tooltip) { // THIS IS THE PROBLEM
	if mapPos == t.currentTarget || !t.rules.IsVisiblePositionOnMap(mapPos) {
		return false, ui.NoTooltip{}
	}
	t.updateTarget(mapPos)
	return true, ui.NoTooltip{}
}

func (t *Targeter) Draw() {
	if t.IsDone() {
		return
	}
	for _, highlight := range t.currentHighlights {
		t.renderer.DrawOnMap(highlight.Pos, common.IconTargeting, highlight.Color)
	}
}

func (t *Targeter) updateTarget(pos geometry.Point) {
	t.currentTarget = pos
	t.currentHighlights = t.rules.GetHighlightsForTarget(pos)
}

func (t *Targeter) HasValidTarget() bool {
	return t.rules.IsValidTarget(t.currentTarget)
}

func NewTargeter(renderer *HybridRenderer, rules common.TargetRules, onTargetSelected common.SelectionHandler) *Targeter {
	return &Targeter{
		rules:             rules,
		renderer:          renderer,
		onTargetSelected:  onTargetSelected,
		currentTarget:     rules.GetPreSelectedTarget(),
		currentHighlights: rules.GetHighlightsForTarget(rules.GetPreSelectedTarget()),
	}
}

type RangedTargetingRules struct {
	isOnMap             func(geometry.Point) bool
	getActorAt          func(geometry.Point) *common.Actor
	nearestHostileActor func() geometry.Point
	attacker            *common.Actor
	hitPositions        func(origin, target geometry.Point) []geometry.Point
}

func (r RangedTargetingRules) GetAttacker() *common.Actor {
	return r.attacker
}

func (r RangedTargetingRules) GetOrigin() geometry.Point {
	return r.attacker.MapPosition()
}
func (r RangedTargetingRules) GetHitpositions(targetPos geometry.Point) []geometry.Point {
	if !r.attacker.CanSee(targetPos) {
		return []geometry.Point{}
	}
	return r.hitPositions(r.GetOrigin(), targetPos)
}
func (r RangedTargetingRules) IsVisiblePositionOnMap(target geometry.Point) bool {
	return r.isOnMap(target) && r.attacker.CanSee(target)
}

func (r RangedTargetingRules) IsValidTarget(target geometry.Point) bool {
	return r.GetOrigin() != target && r.IsVisiblePositionOnMap(target)
}

func (r RangedTargetingRules) GetPreSelectedTarget() geometry.Point {
	return r.nearestHostileActor()
}

func (r RangedTargetingRules) GetHighlightsForTarget(target geometry.Point) []common.TargetHighlight {
	highlights := make([]common.TargetHighlight, 0)
	origin := r.GetOrigin()

	for _, pos := range r.hitPositions(origin, target) {
		highlights = append(highlights, common.TargetHighlight{
			Pos:   pos,
			Color: color.RGBA{R: 255, G: 80, B: 80, A: 255},
		})
	}
	return highlights
}

func NewRangedAttackTargetingRules(attacker *common.Actor, hitPositions common.AoEResolver, currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object]) common.TargetRules {
	return &RangedTargetingRules{
		attacker: attacker,
		isOnMap: func(target geometry.Point) bool {
			return currentMap.Contains(target)
		},
		/*
		getActorAt: func(target geometry.Point) *common.Actor {
			if !currentMap.Contains(target) || !currentMap.IsActorAt(target) {
				return nil
			}
			return currentMap.GetActor(target)
		},

		 */
		nearestHostileActor: func() geometry.Point {
			nearestActor := currentMap.GetNearestActor(attacker.MapPosition(), func(actor *common.Actor) bool {
				if !actor.IsHostileTo(attacker) || !attacker.CanSee(actor.MapPosition()) || actor.IsVendor() {
					return false
				}
				return true
			})
			if nearestActor == nil {
				return attacker.MapPosition()
			}
			return nearestActor.MapPosition()
		},
		hitPositions: hitPositions,
	}
}


type DirectTargetingRules struct {
	isOnMap      func(dest geometry.Point) bool
	attacker     *common.Actor
	hitPositions func(origin geometry.Point, target geometry.Point) []geometry.Point
	isValid      func(target geometry.Point) bool
	moveDist     func(origin, target geometry.Point) int
	maxRange     int
}

func NewDirectTargetingRules(attacker *common.Actor, hitPositions common.AoEResolver, currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object]) common.TargetRules {
	return &DirectTargetingRules{
		isOnMap:      currentMap.Contains,
		attacker:     attacker,
		hitPositions: hitPositions,
		moveDist: currentMap.MoveDistance,
		maxRange: 0,
		isValid: func(target geometry.Point) bool {
			return currentMap.IsTileWalkable(target)
		},
	}
}

func NewMeleeTargetingRules(attacker *common.Actor, hitPositions common.AoEResolver, currentMap *gridmap.GridMap[*common.Actor, common.Item, common.Object]) common.TargetRules {
	return &DirectTargetingRules{
		isOnMap:      currentMap.Contains,
		attacker:     attacker,
		hitPositions: hitPositions,
		moveDist: currentMap.MoveDistance,
		maxRange: 1,
		isValid: func(target geometry.Point) bool {
			return currentMap.IsTileWalkable(target)
		},
	}
}

func (d DirectTargetingRules) GetAttacker() *common.Actor {
	return d.attacker
}

func (d DirectTargetingRules) GetOrigin() geometry.Point {
	return d.attacker.MapPosition()
}

func (d DirectTargetingRules) IsVisiblePositionOnMap(target geometry.Point) bool {
	return d.isOnMap(target) && d.attacker.CanSee(target)
}

func (d DirectTargetingRules) IsValidTarget(target geometry.Point) bool {
	if !(d.IsVisiblePositionOnMap(target) && d.isValid(target)) {
		return false
	}
	if d.maxRange > 0 {
		if d.moveDist(d.GetOrigin(), target) > d.maxRange {
			return false
		}
	}
	return true
}

func (d DirectTargetingRules) GetPreSelectedTarget() geometry.Point {
	return d.GetOrigin().Add(geometry.Point{Y: 5})
}

func (d DirectTargetingRules) GetHighlightsForTarget(target geometry.Point) []common.TargetHighlight {
	highlights := make([]common.TargetHighlight, 0)
	origin := d.GetOrigin()

	for _, pos := range d.hitPositions(origin, target) {
		highlights = append(highlights, common.TargetHighlight{
			Pos:   pos,
			Color: color.RGBA{R: 255, G: 80, B: 80, A: 255},
		})
	}
	return highlights
}

func (d DirectTargetingRules) GetHitpositions(target geometry.Point) []geometry.Point {
	return d.hitPositions(d.GetOrigin(), target)
}
