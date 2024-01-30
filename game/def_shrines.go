package game

import (
	"fmt"
	"math/rand"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
)

func (g *GameInstance) addRandomShrine(source *rand.Rand, emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) {
	actionForShrine := g.GetRandomUntargetedAction(dungeonLevel)
	newShrine := common.NewBumpObject("a shrine", common.IconShrine, func(actor *common.Actor, startAnimating func() bool) {
		actionForShrine.SetCurrentUser(actor) // TODO: should this trigger an animation, then it should listen to startAnimating
		actionForShrine.Invoke(actor)
	})
	emptyMap.AddObject(newShrine, pos)
	shrineName := fmt.Sprintf("Shrine of %s", actionForShrine.Name())
	emptyMap.AddToMetaInfo(shrineName)
}

func (g *GameInstance) newSign(text []string) *common.BumpObject {
	return common.NewBumpObject("a sign", common.IconSign, func(actor *common.Actor, startAnimating func() bool) {
		label := g.uiController.OpenTextWindow(toLabelText(text))
		label.SetCentered()
	})
}
