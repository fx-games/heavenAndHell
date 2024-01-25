package ui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"rpg/geometry"
	"rpg/util"
)

type UIWidget interface {
	InputReceiver
	Draw()
	ShouldClose() bool
	Layout()
}

type UIRenderer interface {
	DrawStringOnGrid(gridX int, gridY int, text string, color color.Color)
	DrawCharOnGrid(gridX int, gridY int, char rune, textColor color.Color)

	DrawOnScreen(screenX int, screenY int, icon int32)
	DrawCharOnScreen(screenX int, screenY int, char rune, scale float64, textColor color.Color)
	DrawOnScreenWithScale(screenX int, screenY int, icon int32, scale float64)
	DrawOnMap(highlight geometry.Point, icon int32, color color.Color)
	DrawOnGrid(gridX int, gridY int, icon int32)
	DrawOnFontGrid(gridX int, gridY int, icon int32)
	GetFontScale() float64
	GetFontGridSize() geometry.Point
	DrawTTFOnScreen(screenX int, screenY int, line string, color color.Color)
	MeasureString(line string) (int, int)
	DrawDefaultBorder(topLeftScreen geometry.Point, size geometry.Point)
	DrawColoredBorder(topLeftScreen geometry.Point, size geometry.Point, fillColor, borderColor color.Color)
	DrawColoredRect(topLeftScreen geometry.Point, size geometry.Point, fillColor color.Color)
	DrawTTFOnScreenWithColorCodes(screenX int, screenY int, textWithColorCodes string)
	DrawImageOnScreen(screenX int, screenY int, size geometry.Point, image *ebiten.Image)
}

func NewUIController(tileRenderer UIRenderer, realScreenSize geometry.Point) *UIController {
	return &UIController{
		tileRenderer: tileRenderer,
		screenSize:   realScreenSize,
	}
}

type UIController struct {
	modal               UIWidget
	overlays            []UIWidget
	tileRenderer        UIRenderer
	isMouseCapturedByUI bool

	currentTooltip           Tooltip
	ticksUntilTooltipAppears uint64
	screenSize               geometry.Point
}

func (u *UIController) GetAutoFitRectWithExtraSpace(text []util.LabelText, padding geometry.Point) (geometry.Point, PositionInfo) {
	screenSize := u.GetScreenSize()

	posInfo := SizeNeededForText(u.tileRenderer, text, 5)
	width := posInfo.SizeInPixels.X + padding.X
	height := posInfo.SizeInPixels.Y + padding.Y

	startX := (screenSize.X - width) / 2
	//endX := startX + width

	startY := (screenSize.Y - height) / 2
	//endY := startY + height

	topLeft := geometry.Point{X: startX, Y: startY}
	//bottomRight := geometry.Point{X: endX, Y: endY}
	return topLeft, posInfo
}

func (u *UIController) GetXPosAndHeightForIconText(text []string) (int, int, int) {
	//TODO
	return 0, 0, 0
}

func (u *UIController) Tick() {
	if u.ticksUntilTooltipAppears > 0 {
		u.ticksUntilTooltipAppears--
		//println("decrementing tooltip ticks", u.ticksUntilTooltipAppears)
	}
}
func (u *UIController) GetTopModal() UIWidget {
	return u.modal
}

func (u *UIController) IsUIVisible() bool {
	return u.modal != nil
}
func (u *UIController) PushWidget(widget UIWidget) {
	u.modal = widget
}

func (u *UIController) PopWidget() {
	u.modal = nil
}

func (u *UIController) OnCommand(command CommandType) bool {
	if !u.IsUIVisible() {
		return false
	}
	return u.GetTopModal().OnCommand(command)
}

func (u *UIController) OnMouseClicked(button ebiten.MouseButton, screenX int, screenY int) bool {
	u.isMouseCapturedByUI = false

	if u.IsUIVisible() && u.GetTopModal().OnMouseClicked(button, screenX, screenY) {
		return true
	}

	for _, overlay := range u.overlays {
		if handled := overlay.OnMouseClicked(button, screenX, screenY); handled {
			u.isMouseCapturedByUI = true
			return true
		}
	}

	return false
}

func (u *UIController) OnMouseMoved(screenX int, screenY int) (bool, Tooltip) {
	u.isMouseCapturedByUI = false

	for _, overlay := range u.overlays {
		if handled, toolTip := overlay.OnMouseMoved(screenX, screenY); handled {
			u.isMouseCapturedByUI = true
			return handled, toolTip
		}
	}

	if !u.IsUIVisible() {
		return false, NoTooltip{}
	}

	u.isMouseCapturedByUI = true

	return u.GetTopModal().OnMouseMoved(screenX, screenY)
}

func (u *UIController) IsMouseCapturedByUI() bool {
	return u.isMouseCapturedByUI
}

func (u *UIController) OnMouseWheel(x int, y int, dy float64) bool {
	if !u.IsUIVisible() {
		return false
	}
	return u.GetTopModal().OnMouseWheel(x, y, dy)
}

func (u *UIController) Draw() {
	defer u.drawTooltip()
	for _, overlay := range u.overlays {
		overlay.Draw()
	}
	if !u.IsUIVisible() {
		return
	}
	if u.GetTopModal().ShouldClose() {
		u.PopWidget()
	} else {
		u.GetTopModal().Draw()
	}
}

func (u *UIController) drawTooltip() {
	if u.currentTooltip != nil && u.ticksUntilTooltipAppears == 0 {
		u.currentTooltip.Draw()
	}
}

func (u *UIController) OpenMenu(menuItems []util.MenuItem) *BasicMenu {
	menu := NewBasicMenu(u)
	menu.SetItems(menuItems)
	u.PushWidget(menu)
	return menu
}
func (u *UIController) GetRenderer() UIRenderer {
	return u.tileRenderer
}

func (u *UIController) OpenTextWindow(buffer []util.LabelText) *BasicLabel {
	if len(buffer) == 0 {
		return nil
	}
	textWindow := NewCenteredTextLabel(u)
	textWindow.SetCanBeClosed(true)
	textWindow.SetText(buffer)
	textWindow.SetBorderPadding(geometry.Point{X: 15, Y: 15})
	u.PushWidget(textWindow)
	return textWindow
}
func (u *UIController) OpenInventoryOverlay(pos, tileSize geometry.Point, dataSource IconDataSource) *IconOverlay {
	inventory := NewIconOverlay(u, pos, tileSize, dataSource)
	inventory.SetShowEmptySlots(true)
	inventory.SetShowCounter(true)
	u.PushOverlay(inventory)
	return inventory
}

func (u *UIController) OpenInventoryModal(pos, tileSize geometry.Point, dataSource IconDataSource) *IconOverlay {
	inventory := NewIconOverlay(u, pos, tileSize, dataSource)
	inventory.SetShowEmptySlots(true)
	inventory.SetShowCounter(true)
	inventory.SetCanBeClosed(true)
	u.PushWidget(inventory)
	return inventory
}

func (u *UIController) OpenSkillsOverlay(pos, tileSize geometry.Point, dataSource IconDataSource) *IconOverlay {
	skills := NewIconOverlay(u, pos, tileSize, dataSource)
	skills.SetShowEmptySlots(false)
	u.PushOverlay(skills)
	return skills
}
func (u *UIController) OpenTimerOverlay(pos, tileSize geometry.Point, dataSource IconDataSource) *IconOverlay {
	timer := NewIconOverlay(u, pos, tileSize, dataSource)
	timer.SetShowEmptySlots(false)
	timer.SetShowCounter(true)
	u.PushOverlay(timer)
	return timer
}

func (u *UIController) AddButton(offsetFromTopRightCorner, tileSize geometry.Point, icon int32, onClick func()) *ButtonOverlay {
	button := NewIconButton(u, offsetFromTopRightCorner, tileSize, icon)
	button.SetAction(onClick)
	u.PushOverlay(button)
	return button
}

func (u *UIController) AddBlackBackgroundBorder(distanceFromRightScreenBorder int) *ColoredRect {
	background := NewColoredRect(u, distanceFromRightScreenBorder)
	background.SetFillColor(color.Black)
	u.PrependOverlay(background)
	return background
}

func (u *UIController) AddMinimap(offsetFromTopRightCorner geometry.Point, maxHeight, availableWidth int, miniMapImage *ebiten.Image) *MinimapOverlay {
	minimap := NewMiniMap(u, offsetFromTopRightCorner, maxHeight, availableWidth, miniMapImage)
	u.PushOverlay(minimap)
	return minimap
}

func (u *UIController) AddLabel(pos geometry.Point) *BasicLabel {
	label := NewTextLabel(u, pos)
	u.PushOverlay(label)
	return label
}

func (u *UIController) PushOverlay(widget UIWidget) {
	u.overlays = append(u.overlays, widget)
}

func (u *UIController) PrependOverlay(widget UIWidget) {
	u.overlays = append([]UIWidget{widget}, u.overlays...)
}

func (u *UIController) ShowTooltipWithDelay(tip Tooltip) {
	u.handleTooltipWithDelay(tip, 0.5)
}

func (u *UIController) handleTooltipWithDelay(tooltip Tooltip, delay float64) {
	if tooltip.IsNull() {
		u.currentTooltip = nil
	} else {
		u.currentTooltip = tooltip
		u.ticksUntilTooltipAppears = util.SecondsToTicks(delay)
	}
}

func (u *UIController) HideTooltip() {
	u.currentTooltip = nil
}

func (u *UIController) PopAll() {
	u.modal = nil
}

func (u *UIController) GetScreenSize() geometry.Point {
	return u.screenSize
}

func (u *UIController) OnScreenSizeChanged(newScreenSize geometry.Point) {
	u.screenSize = newScreenSize
	for _, overlay := range u.overlays {
		if overlay != nil {
			overlay.Layout()
		}
	}
}

func (u *UIController) ClearAllOverlays() {
	u.overlays = []UIWidget{}
}
