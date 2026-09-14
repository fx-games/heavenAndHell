package common

import (
	"fmt"
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
	infoForTarget       func(geometry.Point) string
}

func (r RangedTargetingRules) OnTargetChanged(targetPos geometry.Point) TargetInfo {
	var infoText string
	if r.infoForTarget != nil {
		infoText = r.infoForTarget(targetPos)
	}

	isValid := r.GetOrigin() != targetPos && r.IsVisiblePositionOnMap(targetPos)
	highlights := make([]TargetHighlight, 0)
	origin := r.GetOrigin()

	hitPositions := r.aoe.GetHitPositions(origin, targetPos)
	for _, highPos := range hitPositions {
		highlights = append(highlights, TargetHighlight{
			Pos:   highPos,
			Color: color.RGBA{R: 255, G: 80, B: 80, A: 255},
		})
	}
	return TargetInfo{
		AdjustedTarget: targetPos,
		Highlights:     highlights,
		Hitpositions:   hitPositions,
		IsValid:        isValid,
		InfoText:       infoText,
	}
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

func (r RangedTargetingRules) GetPreSelectedTarget() geometry.Point {
	return r.nearestHostileActor()
}


func NewRangedAttackTargetingRules(args util.Arguments) *TargetRulesProvider {
	hitChanceFunc := CalculateRangedWeaponHitChance
	if args.Get(0) == "throw" {
		hitChanceFunc = CalculateThrowHitChance
	}
	return &TargetRulesProvider{
		GetTargetRules: func(attacker *Actor, hitPositions *AoEResolver, currentMap *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
			return &RangedTargetingRules{
				attacker: attacker,
				isOnMap: func(target geometry.Point) bool {
					return currentMap.Contains(target)
				},
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
				infoForTarget: func(point geometry.Point) string {
					if !currentMap.IsActorAt(point) {
						return ""
					}
					defender := currentMap.GetActor(point)
					hitChance := hitChanceFunc(attacker, defender)
					return fmt.Sprintf("%.0f%%", hitChance*100)
				},
			}
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Ranged attack"),
		},
	}
}

type DirectTargetingRules struct {
	isOnMap       func(dest geometry.Point) bool
	attacker      *Actor
	aoe           *AoEResolver
	isValid       func(target geometry.Point) bool
	moveDist      func(origin, target geometry.Point) int
	maxRange      int
	infoForTarget func(geometry.Point) string
}












type CustomTargetingRules struct {
	attacker            *Actor
	isOnMap             func(target geometry.Point) bool
	nearestHostileActor func() geometry.Point
	aoe                 *AoEResolver
	isValidTarget       func(target geometry.Point) bool
	highlights          func(origin, target geometry.Point, hitPositions []geometry.Point) []TargetHighlight
}















