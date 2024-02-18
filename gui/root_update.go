package gui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"rpg/common"
)

func (u *UIController) Update() error {

	if u.shouldQuitGame {
		return ebiten.Termination
	}

	u.worldTicks++
	if u.ticksUntilTooltipAppears > 0 {
		u.ticksUntilTooltipAppears--
		//println("decrementing tooltip ticks", u.ticksUntilTooltipAppears)
	}
	state := u.context.GameState()
	player := state.GetPlayer()

	u.context.AudioPlayer.Update()

	if u.globalFlashColorTicksLeft > 0 {
		//percentage := float64(u.globalFlashColorTicksLeft) / float64(u.globalFlashColorTicksStart)
		//lerpedColor := util.LerpColor(color.White, u.globalFlashColor, percentage)
		u.tileRenderer.SetGlobalScaleColor(colorAsScaledFloat(u.globalFlashColor, 10.0))
		u.globalFlashColorTicksLeft--
	} else if player.HasStatusEffect(common.StatusBerserk) {
		u.tileRenderer.SetGlobalScaleColor([4]float32{1.0, 0.5, 0.5, 1})
	} else {
		u.tileRenderer.SetGlobalScaleColor([4]float32{1, 1, 1, 1})
	}

	u.handleMouseInput()

	u.handleKeyboardInput()

	u.context.Animator().Tick() // this is animations

	if !player.IsAlive() {
		return nil
	}
	if !u.setToExactFit && u.configuration.KeepPlayerCentered {
		u.mapRenderer.CenterOnFloat(player.DrawPosition())
	}

	for _, actor := range state.GetMap().Actors() {
		actor.Tick()
	}

	return nil
}

func colorAsScaledFloat(lerpedColor color.Color, scale float32) [4]float32 {
	r, g, b, a := lerpedColor.RGBA()
	return [4]float32{float32(r) * scale / 65535.0, float32(g) * scale / 65535.0, float32(b) * scale / 65535.0, float32(a) * scale / 65535.0}
}
