package common

import (
	"fmt"
	"image/color"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/util"
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
	targets       func() []geometry.Point
}

func (d DirectTargetingRules) Targets() []geometry.Point {
	if d.targets == nil {
		return nil
	}
	return d.targets()
}

func (d DirectTargetingRules) OnTargetChanged(target geometry.Point) TargetInfo {
	isValid := d.IsValidTarget(target)
	highlights := d.GetHighlightsForTarget(target)
	return TargetInfo{
		AdjustedTarget: target,
		Highlights:     highlights,
		Hitpositions:   d.GetHitpositions(target),
		IsValid:        isValid,
		InfoText:       d.GetInfoTextForTarget(target),
	}
}

func (d DirectTargetingRules) GetInfoTextForTarget(pos geometry.Point) string {
	if d.infoForTarget != nil {
		return d.infoForTarget(pos)
	}
	return ""
}

func NewDirectTargetingRules(args util.Arguments) *TargetRulesProvider {
	return &TargetRulesProvider{
		GetTargetRules: func(attacker *Actor, hitPositions *AoEResolver, currentMap *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
			return &DirectTargetingRules{
				isOnMap:  currentMap.Contains,
				attacker: attacker,
				aoe:      hitPositions,
				targets:  hostileTargets(currentMap, attacker),
				moveDist: currentMap.MoveDistance,
				maxRange: 0,
				isValid: func(target geometry.Point) bool {
					return currentMap.IsTileWalkable(target)
				},
			}
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
				targets:  hostileTargets(currentMap, attacker),
				moveDist: currentMap.MoveDistance,
				maxRange: 1,
				isValid: func(target geometry.Point) bool {
					return currentMap.IsTileWalkable(target)
				},
			}
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

type CustomTargetingRules struct {
	attacker            *Actor
	isOnMap             func(target geometry.Point) bool
	nearestHostileActor func() geometry.Point
	aoe                 *AoEResolver
	isValidTarget       func(target geometry.Point) bool
	highlights          func(origin, target geometry.Point, hitPositions []geometry.Point) []TargetHighlight
	targets             func() []geometry.Point
}

func (c CustomTargetingRules) Targets() []geometry.Point {
	if c.targets == nil {
		return nil
	}
	return c.targets()
}

func (c CustomTargetingRules) OnTargetChanged(target geometry.Point) TargetInfo {
	isValid := c.IsValidTarget(target)
	highlights := c.GetHighlightsForTarget(target)
	return TargetInfo{
		AdjustedTarget: target,
		Highlights:     highlights,
		Hitpositions:   c.GetHitpositions(target),
		IsValid:        isValid,
		InfoText:       c.GetInfoTextForTarget(target),
	}
}

func (c CustomTargetingRules) IsVisiblePositionOnMap(target geometry.Point) bool {
	return c.isOnMap(target)
}

func (c CustomTargetingRules) IsValidTarget(target geometry.Point) bool {
	return c.isOnMap(target) && c.isValidTarget(target)
}

func (c CustomTargetingRules) GetPreSelectedTarget() geometry.Point {
	return c.nearestHostileActor()
}

func (c CustomTargetingRules) GetHighlightsForTarget(target geometry.Point) []TargetHighlight {
	origin := c.attacker.MapPosition()
	return c.highlights(origin, target, c.GetHitpositions(target))
}

func (c CustomTargetingRules) GetHitpositions(target geometry.Point) []geometry.Point {
	return c.aoe.GetHitPositions(c.attacker.MapPosition(), target)
}

func (c CustomTargetingRules) GetAttacker() *Actor {
	return c.attacker
}

func (c CustomTargetingRules) GetInfoTextForTarget(pos geometry.Point) string {
	return ""
}

func NewJumpStartTargetingRules(args util.Arguments) *TargetRulesProvider {
	maxSprint := args.GetIntOrDefault(0, 5)
	maxJump := args.GetIntOrDefault(1, 5)
	return &TargetRulesProvider{
		GetTargetRules: func(attacker *Actor, hitPositions *AoEResolver, currentMap *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
			return &CustomTargetingRules{
				attacker:            attacker,
				isOnMap:             currentMap.Contains,
				nearestHostileActor: GetPositionOfNearestEnemy(attacker, currentMap),
				targets:             hostileTargets(currentMap, attacker),
				aoe:                 hitPositions,
				isValidTarget:       jumpStartValidTarget(currentMap, attacker, maxSprint, maxJump),
				highlights:          jumpStartHighlights(currentMap, maxSprint, maxJump),
			}
		},
	}
}

func jumpStartHighlights(currentMap *gridmap.GridMap[*Actor, *Item, Object], maxSprint int, maxJump int) func(origin geometry.Point, target geometry.Point, positions []geometry.Point) []TargetHighlight {
	return func(origin geometry.Point, target geometry.Point, positions []geometry.Point) []TargetHighlight {
		jumpInfo := currentMap.JumpOverPositions(origin, target, maxSprint, maxJump)

		if !jumpInfo.ActorFound || len(jumpInfo.Jump) == 0 {
			return []TargetHighlight{}
		}

		highlights := make([]TargetHighlight, 0, len(jumpInfo.Sprint)+2)
		for _, pos := range jumpInfo.Sprint {
			highlights = append(highlights, TargetHighlight{
				Pos:   pos,
				Color: color.RGBA{R: 80, G: 80, B: 255, A: 255},
			})
		}
		if len(jumpInfo.Jump) > 0 {
			highlights = append(highlights, TargetHighlight{
				Pos:   jumpInfo.Jump[len(jumpInfo.Jump)-1],
				Color: color.RGBA{R: 80, G: 255, B: 80, A: 255},
			})
		}

		highlights = append(highlights, TargetHighlight{
			Pos:   jumpInfo.JumpedPos,
			Color: color.RGBA{R: 255, G: 80, B: 80, A: 255},
		})
		return highlights
	}
}

func jumpStartValidTarget(currentMap *gridmap.GridMap[*Actor, *Item, Object], attacker *Actor, maxSprint int, maxJump int) func(target geometry.Point) bool {
	return func(target geometry.Point) bool {
		jumpInfo := currentMap.JumpOverPositions(attacker.MapPosition(), target, maxSprint, maxJump)
		return jumpInfo.ActorFound && len(jumpInfo.Jump) > 0
	}
}

func NewEarthShakerTargetingRules(args util.Arguments) *TargetRulesProvider {
	maxSprint := args.GetIntOrDefault(0, 5)
	return &TargetRulesProvider{
		GetTargetRules: func(attacker *Actor, hitPositions *AoEResolver, currentMap *gridmap.GridMap[*Actor, *Item, Object]) TargetRules {
			return &CustomTargetingRules{
				attacker:            attacker,
				isOnMap:             currentMap.Contains,
				nearestHostileActor: GetPositionOfNearestEnemy(attacker, currentMap),
				targets:             hostileTargets(currentMap, attacker),
				aoe:                 hitPositions,
				isValidTarget:       sprintValidTarget(currentMap, attacker, maxSprint),
				highlights:          earthShakerHighlights(currentMap, maxSprint),
			}
		},
	}
}

func earthShakerHighlights(currentMap *gridmap.GridMap[*Actor, *Item, Object], maxSprint int) func(origin geometry.Point, target geometry.Point, positions []geometry.Point) []TargetHighlight {
	return func(origin geometry.Point, target geometry.Point, positions []geometry.Point) []TargetHighlight {
		sprintInfo := currentMap.SprintToPosition(origin, target, maxSprint)

		if !currentMap.IsValidSprintTarget(origin, target, maxSprint) {
			return []TargetHighlight{}
		}

		highlights := make([]TargetHighlight, 0, len(sprintInfo.Sprint))
		sprintEndPos := sprintInfo.SprintEndPos
		for _, pos := range sprintInfo.Sprint {
			highlights = append(highlights, TargetHighlight{
				Pos:   pos,
				Color: color.RGBA{R: 80, G: 80, B: 255, A: 255},
			})
		}
		highlights = append(highlights, TargetHighlight{
			Pos:   sprintEndPos,
			Color: color.RGBA{R: 255, G: 80, B: 80, A: 255},
		})

		neighbors := currentMap.NeighborsAll(target, func(p geometry.Point) bool {
			return currentMap.Contains(p) && currentMap.IsWalkable(p)
		})

		for _, pos := range neighbors {
			highlights = append(highlights, TargetHighlight{
				Pos:   pos,
				Color: color.RGBA{R: 50, G: 220, B: 0, A: 255},
			})
		}

		return highlights
	}
}

func sprintValidTarget(currentMap *gridmap.GridMap[*Actor, *Item, Object], attacker *Actor, maxSprint int) func(target geometry.Point) bool {
	return func(target geometry.Point) bool {
		return currentMap.IsValidSprintTarget(attacker.mapPosition, target, maxSprint)
	}
}

func GetPositionOfNearestEnemy(attacker *Actor, currentMap *gridmap.GridMap[*Actor, *Item, Object]) func() geometry.Point {
	return func() geometry.Point {
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
	}
}

func hostileTargets(m *gridmap.GridMap[*Actor, *Item, Object], attacker *Actor) func() []geometry.Point {
	return func() []geometry.Point {
		return targetsByDistance(m, attacker.MapPosition(), func(actor *Actor) bool {
			return actor.IsVisible() && actor.IsHostileTo(attacker) && attacker.CanSee(actor.MapPosition()) && !actor.IsVendor()
		})
	}
}
