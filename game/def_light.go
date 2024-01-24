package game

import (
    "math/rand"
    "rpg/actions"
    "rpg/common"
    "rpg/geometry"
    "rpg/gridmap"
)

func (g *GameInstance) addRandomLightSource(source *rand.Rand, emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) {
    createLightSourceFuncs := []func(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem{
        NewTorch,
        NewLantern,
        NewStarGlass,
    }

    creationFunction := createLightSourceFuncs[source.Intn(len(createLightSourceFuncs))]
    item := creationFunction(g.GetWorldState(), source, dungeonLevel)
    emptyMap.AddItem(item, pos)
}
