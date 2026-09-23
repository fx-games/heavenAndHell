package gui

import (
	"image"
	"rpg/files"
	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/util"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// titleScreen is a mod's optional splash image (mod.rec's TitleImage), shown once over whatever is already on
// screen on startup - the start menu, itself already open over the loaded save - until the player acts or
// titleScreenTimeout passes, then it fades out fast to reveal what was underneath it the whole time.
type titleScreen struct {
	image          *ebiten.Image
	ticksLeft      int // until the fade starts on its own; a button press starts it early
	fading         bool
	fadeTicksLeft  int
	fadeTicksStart int
}

const titleScreenTimeout = 8   // seconds shown before it fades on its own
const titleScreenFadeOut = 0.4 // seconds the fade itself takes

// LoadTitleImage is the active mod's title screen image (mod.rec's TitleImage), if it has one.
func LoadTitleImage() (*ebiten.Image, bool) {
	assetPath, ok := common.TitleImagePath()
	if !ok {
		return nil, false
	}
	f, err := files.Open(assetPath)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, false
	}
	return ebiten.NewImageFromImage(img), true
}

// ShowTitleScreen puts splashImage up over whatever is already drawn, see titleScreen.
func (u *UIController) ShowTitleScreen(splashImage *ebiten.Image) {
	u.title = &titleScreen{
		image:          splashImage,
		ticksLeft:      int(util.SecondsToTicks(titleScreenTimeout)),
		fadeTicksStart: int(util.SecondsToTicks(titleScreenFadeOut)),
	}
}

func (u *UIController) updateTitleScreen() {
	t := u.title
	if !t.fading {
		t.ticksLeft--
		if t.ticksLeft <= 0 || anyButtonJustPressed() {
			t.fading = true
			t.fadeTicksLeft = t.fadeTicksStart
		}
		return
	}
	t.fadeTicksLeft--
	if t.fadeTicksLeft <= 0 {
		u.title = nil
	}
}

func anyButtonJustPressed() bool {
	if len(inpututil.AppendJustPressedKeys(nil)) > 0 {
		return true
	}
	return inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle)
}

// uiFadeAlpha hides the menu under the title screen completely, then fades it in exactly as the title fades
// out, so the menu arrives with the artwork's exit instead of sitting behind it from the first frame.
func (u *UIController) uiFadeAlpha() float32 {
	if u.title == nil {
		return 1
	}
	if !u.title.fading {
		return 0
	}
	return 1 - float32(u.title.fadeTicksLeft)/float32(u.title.fadeTicksStart)
}

// drawUIWithAlpha draws the UI off-screen and composites that layer at alpha, so border, text and the dimming
// behind them fade in as one image - drawing them piece by piece would mean an alpha on every UI primitive.
func (u *UIController) drawUIWithAlpha(screen *ebiten.Image, alpha float32) {
	if alpha <= 0 {
		return
	}
	size := screen.Bounds().Size()
	if u.uiLayer == nil || u.uiLayer.Bounds().Size() != size {
		u.uiLayer = ebiten.NewImage(size.X, size.Y)
	} else {
		u.uiLayer.Clear()
	}
	u.tileRenderer.SetRenderTarget(u.uiLayer)
	u.drawUI()
	u.tileRenderer.SetRenderTarget(screen)

	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(alpha)
	screen.DrawImage(u.uiLayer, op)
}

// drawTitleScreen fits the image within most of the screen, centered, preserving its aspect ratio - so it
// reads well whatever size the mod's artwork was made at or however the window is sized.
func (u *UIController) drawTitleScreen() {
	t := u.title
	screen := u.GetScreenSize()
	b := t.image.Bounds()
	const fitFraction = 0.6 // leaves a margin around the image instead of crowding the screen edges
	scale := min(float64(screen.X)*fitFraction/float64(b.Dx()), float64(screen.Y)*fitFraction/float64(b.Dy()))
	size := geometry.Point{X: int(float64(b.Dx()) * scale), Y: int(float64(b.Dy()) * scale)}
	pos := geometry.Point{X: (screen.X - size.X) / 2, Y: (screen.Y - size.Y) / 2}

	alpha := float32(1)
	if t.fading {
		alpha = float32(t.fadeTicksLeft) / float32(t.fadeTicksStart)
	}
	u.tileRenderer.DrawImageOnScreenWithAlpha(pos.X, pos.Y, size, t.image, alpha)
}
