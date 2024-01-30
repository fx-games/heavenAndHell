package game

import (
    "fmt"
)

func (g *GameInstance) NewRandomLightSource(dungeonLevel int) *GameItem {
    // get defs
    itemDefinitions := g.GetItemDefinitions()

    // filter for dungeon level
    var filtered []ItemDefinition
    for _, itemDef := range itemDefinitions {

        if !itemDef.IsLightSource() || !itemDef.CanAppearAt(g.randomSource, dungeonLevel) {
            continue
        }

        filtered = append(filtered, itemDef)
    }

    if len(filtered) == 0 {
        g.ErrLog(fmt.Sprintf("no light source found for dungeon level %d", dungeonLevel))
        return nil
    }
    // pick random type
    accessory := filtered[g.randomSource.Intn(len(filtered))]
    return g.NewItemFromDefinition(accessory, dungeonLevel)
}

func getFireFlickerPattern() []float64 {
    return []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 0.85, 0.92, 0.9, 0.90, 0.92, 0.96, 0.97, 0.99, 1.0, 1.0, 1.0, 0.9, 1.0, 1.0, 1.0, 1.0, 0.9, 0.93, 0.98}
}

func getSmoothFlickerPattern() []float64 {
    var smoothPattern []float64
    low := 0.7
    high := 1.0
    current := high
    stepCount := 40
    // we want to go from high -> low -> high
    stepSize := ((high - low) * 2) / float64(stepCount)
    reversedDirection := false
    for i := 0; i < stepCount; i++ {
        smoothPattern = append(smoothPattern, current)
        if !reversedDirection {
            current -= stepSize
            if current < low {
                reversedDirection = true
                current = low
            }
        } else {
            current += stepSize
        }
    }
    return smoothPattern
}
