package gui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"

	"github.com/hajimehoshi/ebiten/v2"
)

// SaveSlotScreen is the load/save screen: one row per slot with the screenshot the game was saved at and who
// was playing it. Picking a row hands the slot back to the caller, which is what decides whether that means
// loading, saving or replacing.
type SaveSlotScreen struct {
	u           *UIController
	title       string
	slots       []common.SaveSlot
	thumbs      []*ebiten.Image // one per slot, nil where the save has no screenshot
	pick        func(slot int)
	back        func()
	focus       int
	shouldClose bool
	topLeft     geometry.Point
	size        geometry.Point
	lineH       int
	labelW      int
	rowH        int
}

const (
	slotThumbW  = 176
	slotThumbH  = 110
	slotPad     = 16 // inside the border, around the rows
	slotGap     = 8  // between rows
	slotTextGap = 16 // between a row's screenshot and its text
)

func (u *UIController) OpenSaveSlots(title string, slots []common.SaveSlot, pick func(slot int), back func()) {
	s := &SaveSlotScreen{u: u, title: title, slots: slots, pick: pick, back: back}
	for _, slot := range slots {
		s.thumbs = append(s.thumbs, loadThumbnail(slot.ThumbPath))
	}
	s.Layout()
	u.PushWidget(s)
}

// loadThumbnail reads a saved screenshot; a missing or broken file simply leaves the row without one.
func loadThumbnail(path string) *ebiten.Image {
	if path == "" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return nil
	}
	return ebiten.NewImageFromImage(img)
}

func (s *SaveSlotScreen) Layout() {
	r := s.u.GetRenderer()
	_, h := r.MeasureString("M")
	s.lineH = int(h) + 3
	s.rowH = max(slotThumbH, 5*s.lineH)
	s.labelW = 0
	textW := 0
	for _, slot := range s.slots {
		for _, row := range slot.Rows() {
			if w, _ := r.MeasureString(row[0]); int(w)+12 > s.labelW {
				s.labelW = int(w) + 12
			}
			if w, _ := r.MeasureString(row[1]); int(w) > textW {
				textW = int(w)
			}
		}
	}
	s.size = geometry.Point{
		X: slotThumbW + slotTextGap + s.labelW + textW + 20,
		Y: 2*s.lineH + len(s.slots)*(s.rowH+slotGap),
	}
	screen := s.u.GetScreenSize()
	s.topLeft = screen.Sub(s.size).Div(2)
}

// rowRect is a slot row's clickable area.
func (s *SaveSlotScreen) rowRect(i int) geometry.Rect {
	y := s.topLeft.Y + 2*s.lineH + i*(s.rowH+slotGap)
	return geometry.NewRect(s.topLeft.X, y, s.topLeft.X+s.size.X, y+s.rowH)
}

func (s *SaveSlotScreen) Draw() {
	r := s.u.GetRenderer()
	pad := geometry.Point{X: 2 * slotPad, Y: 2 * slotPad}
	r.DrawColoredRect(s.topLeft.Sub(pad.Div(2)), s.size.Add(pad), color.RGBA{R: 12, G: 9, B: 7, A: 240})
	r.DrawDefaultBorder(s.topLeft.Sub(pad.Div(2)), s.size.Add(pad))
	r.DrawTTFOnScreen(float64(s.topLeft.X), float64(s.topLeft.Y+s.lineH), s.title, defs.HeaderTextColor)

	for i, slot := range s.slots {
		rect := s.rowRect(i)
		if i == s.focus {
			r.DrawColoredRect(rect.Min, rect.Size(), color.RGBA{R: 50, G: 38, B: 14, A: 255})
		}
		// the frame goes down first: DrawDefaultBorder fills its inside with the default fill colour, which
		// would paint straight over a screenshot drawn before it
		thumbBox := geometry.Point{X: slotThumbW, Y: slotThumbH}
		r.DrawDefaultBorder(rect.Min, thumbBox)
		if thumb := s.thumbs[i]; thumb != nil {
			r.DrawImageOnScreen(rect.Min.X, rect.Min.Y, thumbBox, thumb)
		}

		x, y := rect.Min.X+slotThumbW+slotTextGap, rect.Min.Y
		rows := slot.Rows()
		if len(rows) == 0 {
			r.DrawTTFOnScreen(float64(x), float64(y+s.lineH), slot.Summary(), defs.DarkTextColor)
			continue
		}
		for _, row := range rows {
			y += s.lineH
			r.DrawTTFOnScreen(float64(x), float64(y), row[0], defs.DarkTextColor)
			r.DrawTTFOnScreen(float64(x+s.labelW), float64(y), row[1], sheetColor(sheetValueColors, row[0], defs.LightTextColor))
		}
	}
}

func (s *SaveSlotScreen) choose(i int) {
	if i < 0 || i >= len(s.slots) {
		return
	}
	s.shouldClose = true // the pick may open a confirmation on top, so this screen gets out of the way first
	s.pick(s.slots[i].Slot)
}

func (s *SaveSlotScreen) OnCommand(command CommandType) bool {
	switch command {
	case PlayerCommandUp:
		s.focus = max(s.focus-1, 0)
	case PlayerCommandDown:
		s.focus = min(s.focus+1, len(s.slots)-1)
	case PlayerCommandConfirm:
		s.choose(s.focus)
	case PlayerCommandCancel:
		s.shouldClose = true
		s.back()
	}
	return true
}

func (s *SaveSlotScreen) OnKeyPressed(key ebiten.Key, shiftPressed bool) {
	switch key {
	case ebiten.KeyW:
		s.OnCommand(PlayerCommandUp)
	case ebiten.KeyS:
		s.OnCommand(PlayerCommandDown)
	}
}

func (s *SaveSlotScreen) OnMouseClicked(button ebiten.MouseButton, x int, y int) bool {
	p := geometry.Point{X: x, Y: y}
	for i := range s.slots {
		if s.rowRect(i).Contains(p) {
			s.focus = i
			s.choose(i)
			return true
		}
	}
	if button == ebiten.MouseButtonRight {
		s.shouldClose = true
		s.back()
	}
	return true
}

func (s *SaveSlotScreen) OnMouseMoved(x int, y int) (bool, Tooltip) {
	p := geometry.Point{X: x, Y: y}
	for i := range s.slots {
		if s.rowRect(i).Contains(p) {
			s.focus = i
		}
	}
	return true, NoTooltip{}
}

func (s *SaveSlotScreen) OnMouseWheel(x int, y int, dy float64) bool { return true }
func (s *SaveSlotScreen) ShouldClose() bool                          { return s.shouldClose }
func (s *SaveSlotScreen) IsNull() bool                               { return false }

// thumbEveryTicks is how often the screenshot that goes with a save is refreshed. One scaled draw is cheap but
// not free, so it happens a few times a second rather than every frame.
const thumbEveryTicks = 20

// captureThumbnail keeps a small copy of the last frame of the world, which SaveGame stores next to the save.
// It is called before the UI is drawn, so no menu or panel ends up in the picture, and the first frame is taken
// right away: a game saved seconds after it started still has a screenshot.
func (u *UIController) captureThumbnail(screen *ebiten.Image) {
	if u.thumbnail != nil && u.worldTicks%thumbEveryTicks != 0 {
		return
	}
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	if w == 0 || h == 0 {
		return
	}
	height := max(slotThumbW*h/w, 1)
	if u.thumbnail == nil || u.thumbnail.Bounds().Dy() != height {
		u.thumbnail = ebiten.NewImage(slotThumbW, height)
	}
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(float64(slotThumbW)/float64(w), float64(height)/float64(h))
	u.thumbnail.DrawImage(screen, op)
}

// CaptureThumbnail hands the last captured frame over as a plain image, for the save slot's screenshot.
func (u *UIController) CaptureThumbnail() image.Image {
	if u.thumbnail == nil {
		return nil
	}
	bounds := u.thumbnail.Bounds()
	shot := image.NewRGBA(bounds)
	u.thumbnail.ReadPixels(shot.Pix)
	return shot
}
