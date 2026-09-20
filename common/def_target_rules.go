package common

import (
	"rpg/util"
)

func GetAllTargetingRules() map[string]func(args util.Arguments) *TargetRulesProvider {
	return map[string]func(args util.Arguments) *TargetRulesProvider{
		"direct":      NewDirectTargetingRules,
		"melee":       NewMeleeTargetingRules,
		"ranged":      NewRangedAttackTargetingRules,
		"jumpstart":   NewJumpStartTargetingRules,
		"earthshaker": NewEarthShakerTargetingRules,
	}
}

func NewTargetingRulesFromString(targetingRulesName string) *TargetRulesProvider {
	allRules := GetAllTargetingRules()
	if util.LooksLikeAFunction(targetingRulesName) {
		parametrizedRules, args := util.GetNameAndArgs(targetingRulesName)
		if resolver, ok := allRules[parametrizedRules]; ok {
			return resolver(args)
		}
	}
	if resolver, ok := allRules[targetingRulesName]; ok {
		return resolver(util.Arguments{})
	}
	return NewDirectTargetingRules(util.Arguments{})
}
