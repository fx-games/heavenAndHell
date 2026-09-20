package common

import (
	"fmt"
	"image/color"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/util"
	"slices"
)

type RangedTargetingRules struct {
	isOnMap       func(geometry.Point) bool
	getActorAt    func(geometry.Point) *Actor
	targets       func() []geometry.Point // valid targets, nearest first
	attacker      *Actor
	aoe           *AoEResolver
	infoForTarget func(geometry.Point) string
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
	if ts := r.targets(); len(ts) > 0 {
		return ts[0]
	}
	return r.GetOrigin()
}

func (r RangedTargetingRules) Targets() []geometry.Point { return r.targets() }

// targetsByDistance lists where m's actors that pass filter stand, nearest to from first (ties in map order).
func targetsByDistance(m *gridmap.GridMap[*Actor, *Item, Object], from geometry.Point, filter func(*Actor) bool) []geometry.Point {
	var ps []geometry.Point
	for _, a := range m.Actors() {
		if filter(a) {
			ps = append(ps, a.MapPosition())
		}
	}
	slices.SortStableFunc(ps, func(a, b geometry.Point) int {
		return geometry.DistanceSquared(from, a) - geometry.DistanceSquared(from, b)
	})
	return ps
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
				targets: func() []geometry.Point {
					return targetsByDistance(currentMap, attacker.MapPosition(), func(actor *Actor) bool {
						return actor.IsVisible() && actor.IsHostileTo(attacker) && attacker.CanSee(actor.MapPosition()) && !actor.IsVendor()
					})
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
