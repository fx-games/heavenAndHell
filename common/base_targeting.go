package common

import (
	"image/color"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/util"
)

type RangedTargetingRules struct {
	isOnMap             func(geometry.Point) bool
	getActorAt          func(geometry.Point) *Actor
	nearestHostileActor func() geometry.Point
	attacker            *Actor
	aoe                 *AoEResolver
}

func (r RangedTargetingRules) GetAttacker() *Actor {
	return r.attacker
}

func (r RangedTargetingRules) GetOrigin() geometry.Point {
	return r.attacker.MapPosition()
}
func (r RangedTargetingRules) GetHitpositions(targetPos geometry.Point) []geometry.Point {
	if !r.attacker.CanSee(targetPos) {
		return []geometry.Point{}
	}
	return r.aoe.GetHitPositions(r.GetOrigin(), targetPos)
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

func (r RangedTargetingRules) GetHighlightsForTarget(target geometry.Point) []TargetHighlight {
	highlights := make([]TargetHighlight, 0)
	origin := r.GetOrigin()

	for _, pos := range r.aoe.GetHitPositions(origin, target) {
		highlights = append(highlights, TargetHighlight{
			Pos:   pos,
			Color: color.RGBA{R: 255, G: 80, B: 80, A: 255},
		})
	}
	return highlights
}

func NewRangedAttackTargetingRules(args util.Arguments) *TargetRulesProvider {
	return &TargetRulesProvider{
		GetTargetRules: func(attacker *Actor, hitPositions *AoEResolver, currentMap *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
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
					nearestActor := currentMap.GetNearestActor(attacker.MapPosition(), func(actor *Actor) bool {
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
				aoe: hitPositions,
			}
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Ranged attack"),
		},
	}
}

type DirectTargetingRules struct {
	isOnMap  func(dest geometry.Point) bool
	attacker *Actor
	aoe      *AoEResolver
	isValid  func(target geometry.Point) bool
	moveDist func(origin, target geometry.Point) int
	maxRange int
}

func NewDirectTargetingRules(args util.Arguments) *TargetRulesProvider {
	return &TargetRulesProvider{
		GetTargetRules: func(attacker *Actor, hitPositions *AoEResolver, currentMap *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
			return &DirectTargetingRules{
				isOnMap:  currentMap.Contains,
				attacker: attacker,
				aoe:      hitPositions,
				moveDist: currentMap.MoveDistance,
				maxRange: 0,
				isValid: func(target geometry.Point) bool {
					return currentMap.IsTileWalkable(target)
				},
			}
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Line of sight"),
		},
	}
}

func NewMeleeTargetingRules(args util.Arguments) *TargetRulesProvider {
	return &TargetRulesProvider{
		GetTargetRules: func(attacker *Actor, hitPositions *AoEResolver, currentMap *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
			return &DirectTargetingRules{
				isOnMap:  currentMap.Contains,
				attacker: attacker,
				aoe:      hitPositions,
				moveDist: currentMap.MoveDistance,
				maxRange: 1,
				isValid: func(target geometry.Point) bool {
					return currentMap.IsTileWalkable(target)
				},
			}
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Melee range"),
		},
	}
}

func (d DirectTargetingRules) GetAttacker() *Actor {
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

func (d DirectTargetingRules) GetHighlightsForTarget(target geometry.Point) []TargetHighlight {
	highlights := make([]TargetHighlight, 0)
	origin := d.GetOrigin()

	for _, pos := range d.aoe.GetHitPositions(origin, target) {
		highlights = append(highlights, TargetHighlight{
			Pos:   pos,
			Color: color.RGBA{R: 255, G: 80, B: 80, A: 255},
		})
	}
	return highlights
}

func (d DirectTargetingRules) GetHitpositions(target geometry.Point) []geometry.Point {
	return d.aoe.GetHitPositions(d.GetOrigin(), target)
}
