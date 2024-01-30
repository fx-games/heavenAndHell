package game

import (
    "rpg/common"
    "rpg/util"
)


func (g *GameInstance) GetAllTargetingRules() map[string]func(args util.Arguments)common.TargetRulesProvider {
    return map[string]func(args util.Arguments)common.TargetRulesProvider{
        "direct": func(args util.Arguments) common.TargetRulesProvider {
            return NewDirectTargetingRules
        },
        "melee": func(args util.Arguments) common.TargetRulesProvider {
            return NewMeleeTargetingRules
        },
        "ranged": func(args util.Arguments) common.TargetRulesProvider {
            return NewRangedAttackTargetingRules
        },
    }
}

func (g *GameInstance) NewTargetingRulesFromString(targetingRulesName string) common.TargetRulesProvider {
    allRules := g.GetAllTargetingRules()
    if util.LooksLikeAFunction(targetingRulesName) {
        parametrizedRules, args := util.GetNameAndArgs(targetingRulesName)
        if resolver, ok := allRules[parametrizedRules]; ok {
            return resolver(args)
        }
    }
    if resolver, ok := allRules[targetingRulesName]; ok {
        return resolver(util.Arguments{})
    }
    return NewDirectTargetingRules
}
