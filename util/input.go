package util

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func IsBeingPressedRepeatedly(key ebiten.Key) bool {
	pressedForTicks := inpututil.KeyPressDuration(key)
	ticksBetweenMovement := 20
	if pressedForTicks > 30 {
		ticksBetweenMovement = 8
	}
	isRepetitionTick := pressedForTicks > 0 && pressedForTicks%ticksBetweenMovement == 0
	return inpututil.IsKeyJustPressed(key) || (ebiten.IsKeyPressed(key) && isRepetitionTick)
}
