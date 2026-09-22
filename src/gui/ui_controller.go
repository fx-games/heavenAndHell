package gui

import (
	"fmt"
	"image/color"
	"io"
	"math"
	"path"
	"rpg/files"
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/renderer"
	"rpg/src/stats"
	"rpg/src/util"
	"slices"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type TilemapAnimInfo struct {
	FrameCount     int
	DelayInSeconds float64
}

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
	DrawTTFOnScreen(screenX float64, screenY float64, line string, color color.Color)
	MeasureString(line string) (float64, float64)
	DrawDefaultBorder(topLeftScreen geometry.Point, size geometry.Point)
	DrawColoredBorder(topLeftScreen geometry.Point, size geometry.Point, fillColor, borderColor color.Color)
	DrawColoredRect(topLeftScreen geometry.Point, size geometry.Point, fillColor color.Color)
	DrawTTFOnScreenWithColorCodes(screenX float64, screenY float64, textWithColorCodes []util.ColoredTextPart)
	DrawImageOnScreen(screenX int, screenY int, size geometry.Point, image *ebiten.Image)

	ParseColorCodedText(string) []util.ColoredTextPart
}

func NewUIController(tileRenderer *renderer.TileRenderer, context common.UIContext, config *common.Configuration) *UIController {
	u := &UIController{
		tileRenderer:   tileRenderer,
		screenSize:     config.WindowSize,
		configuration:  config,
		context:        context,
		worldTexture:   context.GameState().GetWorldTexture(),
		deviceDPIScale: ebiten.DeviceScaleFactor(),
	}
	u.setTileScale(4)
	return u
}

type UIConfig struct {
	DrawTilesBehindActors  bool
	DrawTilesBehindItems   bool
	DrawTilesBehindObjects bool
	ActorBobbingEnabled    bool
	KeepPlayerCentered     bool
}

type UIController struct {
	modals       []UIWidget // the top one gets the input; closing it goes back to the one below
	overlays     []UIWidget
	tileRenderer *renderer.TileRenderer
	mapRenderer  *renderer.MapRenderer
	worldTexture renderer.TextureAtlas
	asciiTexture renderer.TextureAtlas // the current level's glyphs atlas
	glyphs       string                // its file (levels.rec's glyphs); empty on a tiled level
	glyphAtlases map[string]renderer.TextureAtlas

	context                     common.UIContext
	isMouseCapturedByUI         bool
	shiftIsBeingPressedThisTick bool

	currentTooltip           Tooltip
	ticksUntilTooltipAppears uint64
	screenSize               geometry.Point
	uiBarWidth               int
	uiBar                    *ColoredRect
	stashOpen                bool // shows the sidebar even when the option hides it
	elevatedOverlay          UIWidget

	statsLabel *BasicLabel
	inventory  *IconOverlay
	skills     *IconOverlay
	belt       *IconOverlay // the quick slots, on keys 1 to common.BeltSize
	timer      *IconOverlay

	dragFrom  *IconOverlay // the overlay an icon is being dragged from while the button is held; nil otherwise
	dragIndex int
	dragged   UIElement
	miniMap   *MinimapOverlay
	message   PrintMessage

	configuration *common.Configuration

	setToExactFit    bool
	manualScroll     bool // arrow keys panned the map; suspends auto-centering until recentered
	isFullScreen     bool
	worldTicks       uint64
	thumbnail        *ebiten.Image // the last unobstructed frame, scaled down, for a save slot's screenshot
	shouldQuitGame   bool
	OnQuit           func() // runs inside the loop before it ends; GPU reads (the save's thumbnail) fail after RunGame returns
	mousePosInPixels geometry.Point

	targeter                   *Targeter
	tilemapAnimations          map[int32]TilemapAnimInfo
	deviceDPIScale             float64
	tileScale                  float64
	globalFlashColorTicksLeft  int
	globalFlashColorTicksStart int
	globalScaleColor           color.Color
	disableFogOfWar            bool
	mousePosOnMap              geometry.Point
	showMapInfo                bool
	msg                        func(message string) // this might be a smell already? Should the UI really generate messages to the player?
	renderingMode              RenderingMode
	globalFadeColorTicksLeft   int
	globalFadeColorTicksStart  int
	fadeFromBlack              bool
	keepBlack                  bool
}

func (u *UIController) FadeToBlack() {
	u.globalFadeColorTicksLeft = int(util.SecondsToTicks(2))
	u.globalFadeColorTicksStart = u.globalFadeColorTicksLeft
	u.fadeFromBlack = false
}

func (u *UIController) FadeFromBlack() {
	u.globalFadeColorTicksLeft = int(util.SecondsToTicks(2))
	u.globalFadeColorTicksStart = u.globalFadeColorTicksLeft
	u.fadeFromBlack = true
}

func (u *UIController) SetDebugActions(actions func() common.DebugActions) {
	u.context.DebugActions = actions
}

func (u *UIController) IsPlayerForcingAggression() bool {
	return u.shiftIsBeingPressedThisTick
}

// ToggleFogOfWar shows the whole map, monsters included.
func (u *UIController) ToggleFogOfWar() {
	u.disableFogOfWar = !u.disableFogOfWar
}

// CycleRenderingMode steps through the tile/graphic renderings of the world. The text renderings need a glyphs
// atlas, so they are only there on a level that names one.
func (u *UIController) CycleRenderingMode() {
	if u.glyphs == "" {
		return
	}
	u.renderingMode = (u.renderingMode + 1) % 3
	u.configuration.FlipActorsHorizontally = u.renderingMode == GraphicRendering
}

func (u *UIController) ToggleMapInfo() {
	u.showMapInfo = !u.showMapInfo
	u.updateMapTileHoverInfo(u.mousePosOnMap)
}

func (u *UIController) GetVisibleMap() geometry.Rect {
	return u.mapRenderer.GetVisibleMap()
}

func (u *UIController) ShowLogMessage(message string) {
	if message == "" {
		return
	}
	rawText := common.RemoveColorCodes(message)
	u.message.rawText = rawText
	u.message.textForRenderer = u.tileRenderer.ParseColorCodedText(message)
	ticks := util.SecondsToTicks(6)
	u.message.ticksLeft = ticks
	u.message.ticksStart = ticks
}

func GetAutoFitRectWithExtraSpace[T fmt.Stringer](text []T, screenSize geometry.Point, lineDist int, padding geometry.Point, measureString func(line string) (float64, float64)) (geometry.Point, PositionInfo) {
	posInfo := SizeNeededForText(measureString, text, lineDist)
	posInfo.SizeInPixels.X += padding.X
	posInfo.SizeInPixels.Y += padding.Y

	startX := (screenSize.X - posInfo.SizeInPixels.X) / 2

	startY := (screenSize.Y - posInfo.SizeInPixels.Y) / 2

	topLeft := geometry.Point{X: startX, Y: startY}
	return topLeft, posInfo
}

func (u *UIController) GetXPosAndHeightForIconText(text []string) (int, int, int) {
	//TODO
	return 0, 0, 0
}

func (u *UIController) ChangeMapZoom(sign int) {

	change := 0.25 * float64(sign)
	minScale := 0.25
	maxScale := 16.0

	currentScale := u.tileScale
	if sign > 0 && u.setToExactFit {
		// round to next 0.25
		currentScale = math.Floor(currentScale*4) / 4
		u.setToExactFit = false
	}
	targetScale := min(max(currentScale+change, minScale), maxScale)

	mapSize := u.context.GameState().GetMap().MapSize()
	tileSize := u.worldTexture.GetTileSize()

	targetMapSize := geometry.Point{
		X: int(float64(mapSize.X) * float64(tileSize.X) * targetScale),
		Y: int(float64(mapSize.Y) * float64(tileSize.Y) * targetScale),
	}

	windowSize := u.GetDeviceIndependentScreenSize()
	if u.HasOverlays() {
		windowSize = windowSize.Sub(geometry.Point{X: u.GetUIBarWidth(), Y: 0})
	}

	centerMapPos := u.mapRenderer.GetVisibleMap().Center()

	tooSmall := targetMapSize.X < windowSize.X && targetMapSize.Y < windowSize.Y
	if tooSmall {
		u.disableScrolling()
		return
	}

	u.setTileScale(targetScale)
	u.mapRenderer.CenterOn(centerMapPos)
}

func (u *UIController) disableScrolling() {
	u.setToExactFit = true
	state := u.context.GameState()
	mapSize := state.GetMap().MapSize()
	tileSize := u.worldTexture.GetTileSize()

	windowSize := u.GetDeviceIndependentScreenSize()
	if u.HasOverlays() {
		windowSize = windowSize.Sub(geometry.Point{X: u.GetUIBarWidth(), Y: 0})
	}
	// calculate exact scale to fit
	mapSizeW := float64(mapSize.X) * float64(tileSize.X)
	mapSizeH := float64(mapSize.Y) * float64(tileSize.Y)
	targetScaleH := float64(windowSize.X) / mapSizeW
	targetScaleV := float64(windowSize.Y) / mapSizeH
	targetScale := min(targetScaleH, targetScaleV)
	u.mapRenderer.ResetScrolling()

	u.setTileScale(targetScale)
}

func (u *UIController) GetTopModal() UIWidget {
	if len(u.modals) == 0 {
		return nil
	}
	return u.modals[len(u.modals)-1]
}

func (u *UIController) IsUIModalVisible() bool {
	return len(u.modals) > 0
}
func (u *UIController) PushWidget(widget UIWidget) {
	u.modals = append(u.modals, widget)
	u.HideTooltip() // a tooltip belongs to the widget it was hovered in
}

// PopWidget closes the top window, going back to the one below it.
func (u *UIController) PopWidget() {
	if len(u.modals) > 0 {
		u.modals = u.modals[:len(u.modals)-1]
	}
	u.HideTooltip()
	u.UpdateStatsLabel() // restore the compact stat bar (e.g. the unspent-points reminder) over whatever a menu showed
}

func (u *UIController) OnCommand(command CommandType) bool {
	if u.IsUIModalVisible() && u.GetTopModal().OnCommand(command) {
		return true
	}
	for _, overlay := range u.overlays {
		if handled := overlay.OnCommand(command); handled {
			return true
		}
	}
	return false
}

func (u *UIController) OnMouseClicked(button ebiten.MouseButton, screenX int, screenY int) bool {
	u.isMouseCapturedByUI = false

	if u.IsUIModalVisible() && u.GetTopModal().OnMouseClicked(button, screenX, screenY) {
		return true
	}

	for _, overlay := range u.visibleOverlays() {
		if handled := overlay.OnMouseClicked(button, screenX, screenY); handled {
			u.isMouseCapturedByUI = true
			return true
		}
	}

	return false
}

func (u *UIController) OnMouseMoved(screenX int, screenY int) (bool, Tooltip) {
	u.isMouseCapturedByUI = false

	for _, overlay := range u.visibleOverlays() {
		if handled, toolTip := overlay.OnMouseMoved(screenX, screenY); handled {
			u.isMouseCapturedByUI = true
			return handled, toolTip
		}
	}

	if !u.IsUIModalVisible() {
		return false, NoTooltip{}
	}

	u.isMouseCapturedByUI = true

	return u.GetTopModal().OnMouseMoved(screenX, screenY)
}

func (u *UIController) IsMouseCapturedByUI() bool {
	return u.isMouseCapturedByUI
}

func (u *UIController) OnMouseWheel(x int, y int, dy float64) bool {
	if !u.IsUIModalVisible() {
		return false
	}
	return u.GetTopModal().OnMouseWheel(x, y, dy)
}

func (u *UIController) drawUI() {

	defer u.drawTooltip()
	defer u.drawDraggedIcon()
	for _, overlay := range u.visibleOverlays() {
		if overlay == u.elevatedOverlay {
			continue
		}
		overlay.Draw()
	}
	if !u.IsUIModalVisible() && u.elevatedOverlay == nil {
		return
	}
	renderer := u.GetRenderer()
	dimmed := u.screenSize
	if _, sheet := u.GetTopModal().(*CharacterSheet); sheet {
		dimmed.X -= u.GetUIBarWidth() // the sidebar stays live under the character sheet
	}
	renderer.DrawColoredRect(geometry.Point{X: 0, Y: 0}, dimmed, color.RGBA{A: 140})

	if u.elevatedOverlay != nil {
		u.elevatedOverlay.Draw()
	}

	if !u.IsUIModalVisible() {
		return
	}

	if top := u.dropClosedWidgets(); top != nil {
		top.Draw()
	}
}

// dropClosedWidgets removes every widget that asked to close, not just the top one: a widget can close while under
// another (a save slot screen under the confirmation it opened). It returns the new top, if any.
func (u *UIController) dropClosedWidgets() UIWidget {
	u.modals = slices.DeleteFunc(u.modals, func(w UIWidget) bool { return w.ShouldClose() })
	if len(u.modals) == 0 {
		u.UpdateStatsLabel()
	}
	return u.GetTopModal()
}

func (u *UIController) drawTooltip() {
	if u.currentTooltip != nil && u.ticksUntilTooltipAppears == 0 {
		u.currentTooltip.Draw()
	}
}

// Confirm opens a Yes/No menu asking message, one line per "\n"; onConfirm runs only if the player picks Yes.
func (u *UIController) Confirm(message string, onConfirm func()) {
	var items []util.MenuItem
	for _, line := range strings.Split(message, "\n") {
		items = append(items, util.MenuItem{MainText: line})
	}
	u.OpenMenu(append(items, util.MenuItem{MainText: "Yes", Action: onConfirm}, util.MenuItem{MainText: "No", Action: u.PopWidget})) // "No" goes back to what asked
}

func (u *UIController) OpenMenu(menuItems []util.MenuItem) {
	menu := NewBasicMenu(u)
	menu.SetItems(menuItems)
	menu.OnMouseMoved(
		u.mousePosInPixels.X,
		u.mousePosInPixels.Y,
	)
	u.PushWidget(menu)
}

// OpenUncancelableMenu is OpenMenu without an ESC/cancel escape hatch, for menus where dismissing without a
// choice would leave the game in a broken state (e.g. the start menu, whose default underlying game shouldn't
// become playable just by pressing ESC).
func (u *UIController) OpenUncancelableMenu(menuItems []util.MenuItem) {
	menu := NewBasicMenu(u)
	menu.cancelable = false
	menu.SetItems(menuItems)
	menu.OnMouseMoved(
		u.mousePosInPixels.X,
		u.mousePosInPixels.Y,
	)
	u.PopAll() // nothing to go back to from the start or death menu
	u.PushWidget(menu)
}
func (u *UIController) GetRenderer() *renderer.TileRenderer {
	return u.tileRenderer
}

func (u *UIController) OpenTextWindow(buffer []util.LabelText) {
	if len(buffer) == 0 {
		return
	}
	textWindow := NewCenteredTextLabel(u)
	textWindow.SetCanBeClosed(true)
	textWindow.SetBorderPadding(geometry.Point{X: 15, Y: 15}) // before SetText: the layout fits the window to the screen with it
	textWindow.SetText(util.WrapLabelText(buffer, u.GetScreenSize().X*2/3, u.tileRenderer.MeasureString))
	u.PushWidget(textWindow)
}

// ShowItemInfo shows the item's hover tooltip as a text window, for when the player needs to read it, not glance at it.
func (u *UIController) ShowItemInfo(item *common.Item) {
	u.OpenTextWindow(item.GetTooltipLines(u.ParseColorCodedText))
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

func (u *UIController) OpenInventoryListModal(tileSize geometry.Point, dataSource ListDataSource) *IconList {
	inventory := NewIconList(u, tileSize, dataSource)
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

func (u *UIController) SetUIBarWidth(uiBarWidth int) *ColoredRect {
	u.uiBarWidth = uiBarWidth
	background := NewColoredRect(u, uiBarWidth)
	background.SetFillColor(color.Black)
	u.PrependOverlay(background)
	return background
}

func (u *UIController) AddMinimap(offsetFromTopRightCorner geometry.Point, maxHeight, availableWidth int, miniMapImage *ebiten.Image) *MinimapOverlay {
	minimap := NewMiniMap(u, offsetFromTopRightCorner, maxHeight, availableWidth, miniMapImage)
	u.InsertOverlayAtIndex(1, minimap)
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

func (u *UIController) InsertOverlayAtIndex(index int, widget UIWidget) {
	u.overlays = append(u.overlays[:index], append([]UIWidget{widget}, u.overlays[index:]...)...)
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
	u.modals = nil
	u.HideTooltip()
	u.UpdateStatsLabel() // a menu choice (Pepin's heal) may have changed what the stat bar shows
}

func (u *UIController) SetTileMapAnimations(mapping map[int32]TilemapAnimInfo) {
	u.tilemapAnimations = mapping
}

func (u *UIController) SetAndInitTileRenderer(tileRenderer *renderer.TileRenderer) {
	u.tileRenderer = tileRenderer
	tileRenderer.SetColorFromName(defs.ColorFromName)
	tileRenderer.SetWhiteTile(defs.IconWhite)
	tileRenderer.SetDefaultColors(defs.DarkYellow, color.Black)
}

func (u *UIController) GetScreenSize() geometry.Point {
	return u.screenSize
}

func (u *UIController) OnScreenSizeChanged() {
	u.deviceDPIScale = ebiten.DeviceScaleFactor()
	newScreenSize := u.GetDeviceIndependentScreenSize()

	for _, overlay := range u.overlays {
		if overlay != nil {
			overlay.Layout()
		}
	}
	screenSizeForMap := newScreenSize
	if u.HasOverlays() {
		screenSizeForMap = newScreenSize.Sub(geometry.Point{X: u.GetUIBarWidth(), Y: 0})
	}
	u.tileRenderer.SetDeviceScale(u.deviceDPIScale)
	u.mapRenderer.OnScreenSizeChanged(screenSizeForMap)
	u.miniMap.Layout()
	if u.setToExactFit {
		u.disableScrolling()
	}
}

func (u *UIController) ClearAllOverlays() {
	u.overlays = []UIWidget{}
}

// visibleOverlays are the overlays drawn and under the mouse. A hidden sidebar leaves only the status effects,
// which sit on the map; hidden widgets still take keyboard commands.
func (u *UIController) visibleOverlays() []UIWidget {
	if u.sidebarHidden() {
		return []UIWidget{u.timer}
	}
	return u.overlays
}

func (u *UIController) HasOverlays() bool {
	return len(u.overlays) > 0
}

// sidebarHidden: the player's option, except that the stash needs the pack beside it.
func (u *UIController) sidebarHidden() bool {
	return u.configuration.HideSidebar && !u.stashOpen
}

// GetUIBarWidth is the room the sidebar takes from the map: none while it is hidden.
func (u *UIController) GetUIBarWidth() int {
	if u.sidebarHidden() {
		return 0
	}
	return u.uiBarWidth
}

func (u *UIController) OnKeyPressed(key ebiten.Key, shiftPressed bool) {
	if !u.IsUIModalVisible() {
		return
	}
	if keyListener, isKeylistener := u.GetTopModal().(Keylistener); isKeylistener {
		keyListener.OnKeyPressed(key, shiftPressed)
	}
}

func (u *UIController) ElevateOverlay(overlay UIWidget) {
	u.elevatedOverlay = overlay
}

func (u *UIController) StopElevation() {
	u.elevatedOverlay = nil
}

func (u *UIController) ParseColorCodedText(colorCodedText string) []util.ColoredTextPart {
	return u.tileRenderer.ParseColorCodedText(colorCodedText)
}

func (u *UIController) OpenStash(playerInv *common.Inventory, stashInTown *common.Inventory, drop func(item *common.Item)) {
	player := u.context.GameState().GetPlayer()
	// shift moves the whole stack, a plain click one item at a time
	moved := func(stack common.InventoryStack) []*common.Item {
		if u.shiftIsBeingPressedThisTick {
			return stack
		}
		return stack[:1]
	}
	toStashHandler := func(uiItem UIElement) {
		playerEquip := player.GetEquipment()
		for _, item := range moved(uiItem.(common.InventoryStack)) {
			if playerEquip.IsEquipped(item) { // unequip first..
				u.context.PlayerAction().UnequipItem(item)
			}
			playerInv.Remove(item)
			stashInTown.Add(item)
		}
	}

	dataSource := IconDataSource{
		IsHighlighted: func(element UIElement) bool {
			return false
		},
		GetElements: func() []UIElement {
			return sortStacksForUI(stashInTown.StackedItems())
		},
		OnSelection: func(uiItem UIElement) {
			for _, item := range moved(uiItem.(common.InventoryStack)) {
				stashInTown.Remove(item)
				playerInv.Add(item)
			}
		},
	}

	tileSize := u.worldTexture.GetTileSize()

	screenSize := u.GetDeviceIndependentScreenSize()
	invBounds := u.inventory.GetBounds()
	paddingX := 20
	offsetFromBottomRightCorner := geometry.Point{X: screenSize.X - invBounds.Max.X + invBounds.Size().X + paddingX, Y: screenSize.Y - invBounds.Max.Y} //- invBounds.Size().X - paddingX, Y: invBounds.Min.Y}

	u.stashOpen = true
	u.OnScreenSizeChanged()
	stashInv := u.OpenInventoryModal(offsetFromBottomRightCorner, tileSize, dataSource)
	u.ElevateOverlay(u.inventory)

	oldHandler := u.inventory.GetOnSelectedHandler()
	u.inventory.SetOnSelectedHandler(toStashHandler)

	onClose := func() { // revert inventory..
		u.stashOpen = false
		u.OnScreenSizeChanged()
		u.StopElevation()
		u.inventory.SetOnSelectedHandler(oldHandler)
	}

	stashInv.SetOnCloseHandler(onClose)
}

func (u *UIController) setupUI() {
	u.overlays = []UIWidget{}
	u.modals = nil

	paddingLeftAndRight := 6
	u.inventory = u.addInventoryOverlay(geometry.Point{X: paddingLeftAndRight, Y: 10})
	inventoryBoxSize := u.inventory.GetBounds().Size()

	u.uiBarWidth = inventoryBoxSize.X + (2 * paddingLeftAndRight)
	u.uiBar = u.SetUIBarWidth(u.uiBarWidth)
	u.statsLabel = NewCustomLabel(u, func(u *UIController, l *BasicLabel) {
		l.posInfo = SizeNeededForText(u.GetRenderer().MeasureString, l.textLines, 10)
		l.layoutFromTopLeftPos(geometry.Point{X: u.uiBarWidth - paddingLeftAndRight, Y: paddingLeftAndRight})
	})
	u.PushOverlay(u.statsLabel)
	u.statsLabel.SetText(util.ToLabelText([]string{ // hacky way to measure the right bounding box..
		"Level 1",
		"MoreInfo",
		"Health: 999",
		"Mana: 999",
		"Gold: 999",
		"Stat points: 9 (u)", // reserve the level-up row so the minimap never overlaps it
	}))
	statsBounds := u.statsLabel.GetBounds()

	// the skills' hover label sits above the row; leave it room below the belt
	labelSize := SizeNeededForText(u.GetRenderer().MeasureString, util.ToLabelText([]string{"Ag"}), 0)
	labelHeight := labelSize.YOffsetForFirstLine + labelSize.SizeInPixels.Y
	u.skills = u.addSkillsOverlay(geometry.Point{X: paddingLeftAndRight, Y: 180 - labelHeight})
	skillBounds := u.skills.GetBounds()
	u.belt = u.addBeltOverlay(geometry.Point{X: paddingLeftAndRight, Y: u.GetScreenSize().Y - skillBounds.Min.Y + labelHeight + paddingLeftAndRight})
	beltBounds := u.belt.GetBounds()

	topYForMap := statsBounds.Max.Y + paddingLeftAndRight*2
	bottomYForMap := beltBounds.Min.Y - paddingLeftAndRight
	maxHeightForMap := bottomYForMap - topYForMap

	u.miniMap = u.addMiniMap(geometry.Point{X: u.uiBarWidth, Y: topYForMap}, maxHeightForMap, paddingLeftAndRight)

	// the status effects sit in one row in the bottom-left corner, clear of the minimap on the right
	u.timer = u.addTimerOverlay(geometry.Point{X: paddingLeftAndRight, Y: paddingLeftAndRight})
	u.timer.SetRows(1)
	u.timer.SetBottomLeft(true)

	u.OnScreenSizeChanged()
}

func (u *UIController) addInventoryOverlay(pixelEdgeFromBottomRight geometry.Point) *IconOverlay {
	state := u.context.GameState()
	isEquipped := func(uiItem UIElement) bool {
		player := state.GetPlayer()
		itemStack := uiItem.(common.InventoryStack)
		firstItem := itemStack[0]
		return player.GetEquipment().IsEquipped(firstItem)
	}

	dataSource := IconDataSource{
		IsHighlighted: isEquipped,
		GetElements: func() []UIElement {
			return sortStacksForUI(state.GetPlayer().GetInventory().StackedItems())
		},
		OnSelection: u.onItemInInventoryClicked,
		Draggable:   true, // onto the belt
		OnRightClick: func(uiItem UIElement) {
			if !state.GetPlayer().IsAlive() {
				return
			}
			item := firstItemFromUIStack(uiItem)
			wasEquipped := state.GetPlayer().GetEquipment().IsEquipped(item)
			u.context.PlayerAction().DropItem(item)
			if wasEquipped {
				u.ShowFullStats()
			}
		},
	}
	tileSize := u.worldTexture.GetTileSize()
	return u.OpenInventoryOverlay(pixelEdgeFromBottomRight, tileSize, dataSource)
}

func firstItemFromUIStack(uiItem any) *common.Item {
	var firstItem *common.Item

	if item, isItem := uiItem.(*common.Item); isItem {
		firstItem = item
	} else if textStack, isTextualStack := uiItem.(*common.TextualStack); isTextualStack {
		firstItem = textStack.InventoryStack[0]
	} else if itemStack, isItemStack := uiItem.(common.InventoryStack); isItemStack {
		firstItem = itemStack[0]
	} else if liveStack, isLiveStack := uiItem.(*common.InventoryStack); isLiveStack && len(*liveStack) > 0 {
		firstItem = (*liveStack)[0]
	}

	return firstItem
}

func (u *UIController) onItemInInventoryClicked(uiItem UIElement) {
	state := u.context.GameState()
	if !state.GetPlayer().IsAlive() {
		return
	}
	act := u.context.PlayerAction()
	// shift state
	u.shiftIsBeingPressedThisTick = ebiten.IsKeyPressed(ebiten.KeyShift)
	typedItem := firstItemFromUIStack(uiItem)

	player := state.GetPlayer()

	wasEquipped := player.GetEquipment().IsEquipped(typedItem)
	if u.shiftIsBeingPressedThisTick { // drop
		act.DropItem(typedItem)
		if wasEquipped {
			u.ShowFullStats()
		}
		return
	}

	actions := common.ItemActions(act, player, typedItem, u.ShowFullStats)
	if len(actions) == 1 {
		actions[0].Action()
	} else if len(actions) > 1 {
		u.OpenMenu(actions)
	}
}

// ToggleFullStats opens the character sheet window, or closes it again ('c').
func (u *UIController) ToggleFullStats() { u.OpenCharacterSheet() }

// ShowFullStats relays out the character sheet if it is open. Equipping and dropping gear call it because the
// numbers changed; the window itself is opened with 'c' or by the level-up prompt.
func (u *UIController) ShowFullStats() {
	if sheet, open := u.GetTopModal().(*CharacterSheet); open {
		sheet.Layout()
	}
}

func (u *UIController) BeginPlayerSellingTo(vendor *common.Actor) {
	player := u.context.GameState().GetPlayer()
	playerInventory := player.GetInventory()
	msg := u.msg
	if playerInventory.IsEmpty() {
		msg("You have nothing to sell")
		return
	}
	var sellItems []ShopMenuItem
	textParser := u.tileRenderer.ParseColorCodedText

	for _, itemStack := range common.SellableStacks(vendor, player) {
		typedItem := itemStack[0]
		sellPrice := common.D1SellPrice(typedItem)
		itemName := itemStack.ShopName()
		if !typedItem.MeetsRequirements() {
			itemName = fmt.Sprintf("[:red]%s[:white]", itemName)
		}
		sellItems = append(sellItems, ShopMenuItem{
			ShortCut: playerInventory.GetLetterForItemStack(itemStack),
			Icon:     typedItem.InventoryIcon(),
			Tooltip:  typedItem.GetTooltipLines(textParser),
			// selling is a decision about gear too: show how it stacks up against what is worn
			MakeTooltip: func(mousePos geometry.Point) Tooltip { return NewItemTooltip(u, typedItem, mousePos) },
			TextColumns: []util.LabelText{
				{
					Text:               itemName,
					TextWithColorCodes: textParser(itemName),
					UseColorCodes:      true,
				},
				{
					Text:               fmt.Sprintf("%d", sellPrice),
					TextWithColorCodes: textParser(fmt.Sprintf("%d", sellPrice)),
					UseColorCodes:      true,
				},
			},
			Action: func() {
				msg(common.SellItem(player, typedItem))
				u.PopWidget()
				u.BeginPlayerSellingTo(vendor)
			},
		})
	}
	if len(sellItems) == 0 {
		msg("This vendor has no interest in anything you carry")
		return
	}
	sort.Slice(sellItems, func(a, b int) bool { return sellItems[a].ShortCut < sellItems[b].ShortCut })
	goldAmount := player.GetStats().GetAttribute(stats.Gold)
	menu := NewShopMenu(u, u.worldTexture.GetTileSize())
	u.PushWidget(menu)
	menu.SetHeader("You have:", fmt.Sprintf("%d gold", goldAmount))
	menu.SetItems(sellItems, []Alignment{AlignmentLeft, AlignmentRight})
}

// openLevelUpMenu is the same window as 'c': it has a "+" next to every attribute a point can still go into.
func (u *UIController) openLevelUpMenu() {
	if !u.context.GameState().GetPlayer().CanLevelUp() {
		return
	}
	if _, open := u.GetTopModal().(*CharacterSheet); !open {
		u.OpenCharacterSheet()
	}
}
func (u *UIController) addMiniMap(offset geometry.Point, maxHeight int, horizontalPadding int) *MinimapOverlay {
	miniMap := u.AddMinimap(offset.Sub(geometry.Point{X: horizontalPadding}), maxHeight, offset.X-(horizontalPadding*2), u.mapRenderer.GetMiniMap())
	miniMap.SetOnMapClickedHandler(u.onMapClicked)
	return miniMap
}

func (u *UIController) addSkillsOverlay(pixelEdgeFromBottomRight geometry.Point) *IconOverlay {
	tileSize := u.worldTexture.GetTileSize()
	state := u.context.GameState()
	dataSource := IconDataSource{
		IsHighlighted: func(UIElement) bool { return false },
		GetElements: func() []UIElement {
			var items []UIElement
			for _, entry := range common.SkillBar(state.GetPlayer()) {
				items = append(items, entry.(UIElement))
			}
			return items
		},
		OnSelection: func(element UIElement) { element.(common.QuickSlotEntry).Use(u.context.PlayerAction()) },
		Draggable:   true,
		OnDrop: func(_ UIElement, from *IconOverlay, fromIndex, to int) {
			if from == u.skills {
				common.MoveSkill(state.GetPlayer(), fromIndex, to)
			}
		},
	}

	return u.OpenSkillsOverlay(pixelEdgeFromBottomRight, tileSize, dataSource)
}

// addBeltOverlay is D1's belt: eight quick slots, filled by dragging skills and inventory stacks onto them.
func (u *UIController) addBeltOverlay(pixelEdgeFromBottomRight geometry.Point) *IconOverlay {
	state := u.context.GameState()
	dataSource := IconDataSource{
		IsHighlighted: func(UIElement) bool { return false },
		GetElements: func() []UIElement {
			items := make([]UIElement, common.BeltSize)
			for i := range items {
				if e := common.BeltEntry(state.GetPlayer(), i); e != nil {
					items[i] = e.(UIElement)
				}
			}
			return items
		},
		OnSelection: func(element UIElement) { element.(common.QuickSlotEntry).Use(u.context.PlayerAction()) },
		HotkeyHint: func(i int) rune {
			if i >= common.BeltSize {
				return 0
			}
			return rune('1' + i)
		},
		Draggable: true,
		OnDrop: func(dragged UIElement, from *IconOverlay, fromIndex, to int) {
			player := state.GetPlayer()
			if to < 0 || to >= common.BeltSize {
				return
			}
			if from == u.belt {
				common.SwapBeltEntries(player, fromIndex, to)
			} else if entry := u.quickSlotEntryFor(dragged); entry != nil {
				common.SetBeltEntry(player, to, entry)
			}
		},
		OnDropOutside: func(fromIndex int) { common.SetBeltEntry(state.GetPlayer(), fromIndex, nil) },
	}
	belt := NewIconOverlay(u, pixelEdgeFromBottomRight, u.worldTexture.GetTileSize(), dataSource)
	belt.SetRows(1)
	belt.SetShowEmptySlots(true)
	belt.SetShowCounter(true)
	u.PushOverlay(belt)
	return belt
}

// quickSlotEntryFor is what the belt holds for an icon dragged onto it: an inventory icon (a stack snapshot) or a
// staff on the skill bar becomes the pack's live stack, a skill stays itself.
func (u *UIController) quickSlotEntryFor(dragged UIElement) common.QuickSlotEntry {
	if item := firstItemFromUIStack(dragged); item != nil {
		if stack := u.context.GameState().GetPlayer().GetInventory().StackOf(item); stack != nil {
			return stack
		}
		return nil
	}
	entry, _ := dragged.(common.QuickSlotEntry)
	return entry
}

// UseBeltSlot fires belt slot i (0-based), as its number key does; an empty slot does nothing.
func (u *UIController) UseBeltSlot(i int) {
	if e := common.BeltEntry(u.context.GameState().GetPlayer(), i); e != nil {
		u.CancelTargeting() // a new slot replaces the spell being aimed
		e.Use(u.context.PlayerAction())
	}
	u.RefreshIconBars()
}

// RefreshIconBars rebuilds the sidebar's icon overlays, and any icon window open over them (the stash).
// ponytail: refreshed on turns, pack and gear changes, clicks and drags; anything else (debug menu) shows a turn late.
func (u *UIController) RefreshIconBars() {
	for _, o := range []*IconOverlay{u.inventory, u.skills, u.belt, u.timer} {
		if o != nil {
			o.Refresh()
		}
	}
	for _, m := range u.modals {
		if o, isIcons := m.(*IconOverlay); isIcons {
			o.Refresh()
		}
	}
}

func (u *UIController) beginIconDrag(from *IconOverlay, index int, dragged UIElement) {
	u.dragFrom, u.dragIndex, u.dragged = from, index, dragged
}

// endIconDrag drops the dragged icon on the icon overlay under the mouse, if any.
func (u *UIController) endIconDrag(x, y int) {
	from := u.dragFrom
	if from == nil {
		return
	}
	dragged := u.dragged
	u.dragFrom, u.dragged = nil, nil
	point := geometry.Point{X: x, Y: y}
	from.onIconReleased(u.dragIndex, dragged, u.iconOverlayAt(point), point)
	u.RefreshIconBars()
}

func (u *UIController) iconOverlayAt(point geometry.Point) *IconOverlay {
	for _, w := range append(slices.Clone(u.modals), u.overlays...) {
		if o, isIcons := w.(*IconOverlay); isIcons && o.bounds.Contains(point) {
			return o
		}
	}
	return nil
}

// drawDraggedIcon draws the icon being dragged under the mouse, once it has left its own slot.
func (u *UIController) drawDraggedIcon() {
	from := u.dragFrom
	if from == nil || (from.bounds.Contains(u.mousePosInPixels) && from.getItemIndexFromScreenPosition(u.mousePosInPixels) == u.dragIndex) {
		return
	}
	mouse := u.mousePosInPixels.Sub(from.tileSizeInPixels.Div(2))
	u.GetRenderer().DrawOnScreenWithScale(mouse.X, mouse.Y, u.dragged.InventoryIcon(), from.scale)
}

// openSpellMenu lists the memorized D1 spells; picking one casts it (targeted spells start targeting first).
func (u *UIController) openSpellMenu() {
	items := common.SpellMenuItems(u.context.GameState(), u.context.PlayerAction())
	if len(items) == 0 {
		u.ShowLogMessage("You don't know any spells and carry no scrolls or staves.")
		return
	}
	u.OpenInventoryListModal(u.worldTexture.GetTileSize(), ListDataSource{GetElements: func() []IconListElement {
		elements := make([]IconListElement, len(items))
		for i, item := range items {
			elements[i] = menuListElement{item, u}
		}
		return elements
	}})
}

func (u *UIController) addTimerOverlay(pixelEdgeFromBottomRight geometry.Point) *IconOverlay {
	tileSize := u.worldTexture.GetTileSize()

	isEquipped := func(item UIElement) bool {
		return false // TODO:
	}

	dataSource := IconDataSource{
		IsHighlighted: isEquipped,
		GetElements:   u.getCurrentTimedEffects,
		OnSelection: func(element UIElement) {
		},
	}

	return u.OpenTimerOverlay(pixelEdgeFromBottomRight, tileSize, dataSource)
}
func (u *UIController) getCurrentTimedEffects() []UIElement {
	var result []UIElement
	for _, timedBeing := range u.context.GameState().GetTimeKeeper().GetBeingsByFilter(func(being common.TimedBeing) bool {
		switch being.(type) {
		case *common.Actor:
			return false
		case *common.UntargetedAction: // this is used to delay activation of spells
			return true
		default:
			return false
		}
	}) {
		result = append(result, timedBeing.(UIElement))
	}

	for statusName, turnsLeft := range u.context.GameState().GetPlayer().GetStatusEffects().AllEntries() {
		result = append(result, NewStatusEffectWidget(statusName, turnsLeft))
	}
	for _, statusEffect := range u.context.GameState().GetPlayer().GetParametrizedStatusEffects() {
		result = append(result, NewStatusEffectWidget(statusEffect.GetName(), statusEffect.TurnsLeft()))
	}
	return result
}
func (u *UIController) SetTTFFont(file io.ReaderAt, size float64) {
	tt, err := opentype.ParseReaderAt(file)
	if err != nil {
		println(err.Error())
		return
	}
	dpi := 72 * u.deviceDPIScale
	fontFace, faceErr := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    size,
		DPI:     dpi,
		Hinting: font.HintingVertical,
	})
	if faceErr != nil {
		println(faceErr.Error())
	}
	u.tileRenderer.SetTTF(fontFace)
}

// SetFont switches the UI font while the game runs; open overlays lay themselves out again for its size.
func (u *UIController) SetFont(file string) {
	u.SetTTFFont(files.MustOpen(file), u.configuration.DefaultFontSize)
	for _, overlay := range u.overlays {
		if overlay != nil {
			overlay.Layout()
		}
	}
}

func (u *UIController) OnMapChanged() {
	loadedMap := u.context.GameState().GetMap()
	u.useLevelGlyphs(u.context.GameState().CurrentLevel().Glyphs)

	gridSize := u.worldTexture.GetTileSize()
	mapWindow := renderer.NewMapWindow(
		u.GetDeviceIndependentScreenSize().Sub(geometry.Point{X: u.GetUIBarWidth(), Y: 0}),
		geometry.Point{X: loadedMap.GetWidth(), Y: loadedMap.GetHeight()},
		gridSize,
		u.GetTileScaleFactor,
		u.StaticMapLookup)

	u.mapRenderer = renderer.NewMapRenderer(u.tileRenderer, mapWindow)
	u.mapRenderer.SetMiniMapColorHandler(u.getMiniMapColor)

	u.setupUI()

	u.OnPlayerTurnTaken()
}

func (u *UIController) getMiniMapColor(mapPos geometry.Point, tileColor *[4]byte) {
	*tileColor = common.MiniMapColor(u.context.GameState(), u.configuration, mapPos)
}

func prepend[T any](slice []T, s T) []T {
	return append([]T{s}, slice...)
}
func (u *UIController) StaticMapLookup(x int, y int, tick uint64) []renderer.CellDrawInfo {
	mapPos := geometry.Point{X: x, Y: y}
	state := u.context.GameState()
	currentMap := state.GetMap()
	if currentMap == nil || !currentMap.Contains(mapPos) {
		return []renderer.CellDrawInfo{}
	}

	if !currentMap.IsExplored(mapPos) && !u.disableFogOfWar {
		return []renderer.CellDrawInfo{}
	}

	var drawColor color.RGBA = color.RGBA{255, 255, 255, 255}

	tileAt := currentMap.GetCell(mapPos).TileType
	worldIcon, worldTileColor := u.getTileIcon(tileAt)
	if tileAt.Special == gridmap.SpecialTileWater {
		worldTileColor = state.GetWaterColor()
	}

	// animate the worldIcon
	if u.renderingMode == GraphicRendering && (state.GetPlayer().CanSee(mapPos) || u.disableFogOfWar) {
		if !u.configuration.DrawTilesBehindActors && currentMap.IsActorAt(mapPos) && currentMap.ActorAt(mapPos).IsVisible() {
			return []renderer.CellDrawInfo{}
		}
		if info, ok := u.tilemapAnimations[worldIcon]; ok {
			// if it is, then calculate the current frame based on the tick
			frameOffset := util.GetLoopingFrameFromTick(tick+1000-uint64(x*17)-uint64(y*19), info.DelayInSeconds, info.FrameCount)
			worldIcon += frameOffset
		}
	}
	// fake lighting
	if !u.disableFogOfWar {
		drawColor = state.ApplyLighting(mapPos, drawColor, tick)
		worldTileColor = state.ApplyLighting(mapPos, worldTileColor, tick)
	}

	var drawInfo []renderer.CellDrawInfo

	// corpses lie under objects and items (loot dropped on a corpse stays visible)
	var corpse *renderer.CellDrawInfo
	if currentMap.IsDownedActorAt(mapPos) && state.GetPlayer().GetFOV().Visible(mapPos) && !currentMap.DownedActorAt(mapPos).VisualsPending() {
		corpse = &renderer.CellDrawInfo{
			Atlas: u.GetCurrentAtlas(),
			Icon:  currentMap.DownedActorAt(mapPos).Icon(u.worldTicks),
			Color: drawColor,
		}
	}

	// 1. items
	if itemAt, isItemAt := currentMap.TryGetItemAt(mapPos); isItemAt && itemAt.IsDrawn() && (!itemAt.IsHidden() || u.context.GameState().GetFlags().IsSet(string(common.FlagHasTrueSight))) {
		itemIcon := itemAt.Icon(u.worldTicks)
		drawItem := renderer.CellDrawInfo{
			Atlas: u.worldTexture,
			Icon:  itemIcon,
			Color: drawColor,
		}
		drawInfo = prepend(drawInfo, drawItem)
		if !u.configuration.DrawTilesBehindItems {
			if corpse != nil {
				drawInfo = prepend(drawInfo, *corpse)
			}
			return drawInfo
		}
	}

	// 2. objects
	if currentMap.IsObjectAt(mapPos) {
		objectAt := currentMap.ObjectAt(mapPos)
		objIcon := objectAt.Icon(u.worldTicks)
		drawObject := renderer.CellDrawInfo{
			Atlas: u.worldTexture,
			Icon:  objIcon,
			Color: drawColor,
		}
		if d1Obj, ok := objectAt.(*common.D1Object); ok && d1Obj.TintColor() != defs.White {
			tint := d1Obj.TintColor()
			if !u.disableFogOfWar {
				tint = state.ApplyLighting(mapPos, tint, tick)
			}
			drawObject.Color = tint
		}
		drawInfo = prepend(drawInfo, drawObject)
		if !u.configuration.DrawTilesBehindObjects {
			return drawInfo
		}
	}

	if corpse != nil {
		drawInfo = prepend(drawInfo, *corpse)
	}

	// 2. decals
	if decalIcon, exists := currentMap.GetDecal(mapPos); exists {
		drawInfo = prepend(drawInfo, renderer.CellDrawInfo{
			Atlas: u.worldTexture,
			Icon:  decalIcon,
			Color: color.White,
		})
	}

	// 1. world
	drawInfo = prepend(drawInfo, renderer.CellDrawInfo{
		Atlas: u.GetCurrentAtlas(),
		Icon:  worldIcon,
		Color: worldTileColor,
	})

	return drawInfo
}

func (u *UIController) GetDeviceIndependentScreenSize() geometry.Point {
	if u.isFullScreen {
		w, h := ebiten.ScreenSizeInFullscreen()
		return geometry.Point{X: w, Y: h}
	}
	return u.screenSize
}

func (u *UIController) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	panic("should use layoutf")
}

func (u *UIController) LayoutF(outsideWidth, outsideHeight float64) (screenWidth, screenHeight float64) {
	intWidth := int(outsideWidth)
	intHeight := int(outsideHeight)
	if u.screenSize.X != intWidth || u.screenSize.Y != intHeight {
		u.screenSize = geometry.Point{X: intWidth, Y: intHeight}
		u.OnScreenSizeChanged()
	}
	return outsideWidth * u.deviceDPIScale, outsideHeight * u.deviceDPIScale
}

func (u *UIController) GetTileScaleFactor() float64 {
	return u.tileScale
}

// canCancel reports whether Cancel has something to back out of, so Esc knows
// not to fall through to the pause menu. Keep in step with Cancel below.
func (u *UIController) canCancel() bool {
	return u.IsUIModalVisible() || u.targeter != nil || u.inventory.IsActive()
}

func (u *UIController) Cancel() {
	if u.IsUIModalVisible() {
		u.OnCommand(PlayerCommandCancel)
		return
	}
	if u.targeter != nil {
		u.CancelTargeting()
		return
	}
	if u.inventory.IsActive() {
		u.inventory.Deactivate()
		return
	}
}

func (u *UIController) CancelTargeting() {
	u.targeter = nil
}

func (u *UIController) BeginThrowTargeting(targetRules common.TargetRules, onSelected func(attacker *common.Actor, target geometry.Point)) {
	state := u.context.GameState()
	if !state.GetPlayer().IsAlive() {
		return
	}
	if u.nothingToTarget(targetRules) {
		return
	}
	act := u.context.PlayerAction()
	act.CancelAutoMove()
	msg := u.msg
	u.targeter = NewTargeter(u, targetRules, &common.HitEffect{
		Activate: func(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
			u.CancelTargeting()

			origin := state.GetPlayer().MapPosition()
			if origin == target {
				msg("You can't attack yourself like that")
				return
			}
			u.context.Animator().CancelAllAnimations()
			act.CancelAutoMove()
			onSelected(attacker, target)
		},
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Throw")),
		},
	})
}
func (u *UIController) BeginTargeting(targetRules common.TargetRules, onHit *common.HitEffect) {
	if u.nothingToTarget(targetRules) {
		return
	}
	u.targeter = NewTargeter(u, targetRules, onHit)
}

// nothingToTarget is true when neither the preselected cell nor any cyclable target would be a valid choice:
// entering targeting mode then only costs the player a cancel.
func (u *UIController) nothingToTarget(rules common.TargetRules) bool {
	if len(rules.Targets()) > 0 || rules.OnTargetChanged(rules.GetPreSelectedTarget()).IsValid {
		return false
	}
	u.msg("There is nothing to target.")
	return true
}

func (u *UIController) ShowGameOverMenu() {
	u.OpenUncancelableMenu(u.context.StartMenuItems(true))
}

func (u *UIController) QuitGame() {
	u.shouldQuitGame = true
}

func (u *UIController) OpenPlayerStashInTown() {
	state := u.context.GameState()
	gui := u
	stashInTown := state.GetPlayerStashInTown()
	playerInv := state.GetPlayer().GetInventory()

	// NEW
	onDrop := func(item *common.Item) {
		item.DroppedByPlayer()
		state.AddItemToMapWithDistribution(item, state.GetPlayer().MapPosition())
	}
	gui.OpenStash(playerInv, stashInTown, onDrop)
	// on close of the stash modal, restore the onclick handler of the inventory..
}

func (u *UIController) OnPlayerTurnTaken() {
	u.RefreshIconBars()
	u.UpdateMiniMap()
	u.UpdateStatsLabel()
	player := u.context.GameState().GetPlayer()

	// Panning with the arrows is a look-around, not a camera mode: taking a turn hands the view back.
	// (The border check below can't do that job - on a map only a little wider than the window, like
	// the town, scrolling is clamped before the player ever leaves the visible rect.)
	u.manualScroll = false

	if (!u.setToExactFit && !u.configuration.KeepPlayerCentered) || u.configuration.KeepPlayerCentered {
		distFromBorder := 4
		playerPos := player.MapPosition()
		visibleMap := u.mapRenderer.GetVisibleMap()
		visibleMap = visibleMap.Shift(distFromBorder, distFromBorder, -distFromBorder, -distFromBorder)
		if !visibleMap.Contains(playerPos) {
			u.mapRenderer.CenterOnFloat(player.DrawPosition())
		}
	}
}

func (u *UIController) UpdateStatsLabel() {
	state := u.context.GameState()
	player := state.GetPlayer()
	if player == nil {
		return
	}
	playerStats := player.GetStats()
	var hpColor color.Color
	var manaColor color.Color
	hpColor = defs.DarkTextColor
	manaColor = defs.DarkTextColor

	if playerStats.GetResource(stats.Health).IsLessThanPercent(0.25) {
		hpColor = color.RGBA{R: 255, G: 30, B: 30, A: 255}
	} else if playerStats.GetResource(stats.Health).IsLessThanPercent(0.5) {
		hpColor = color.RGBA{R: 255, G: 150, B: 30, A: 255}
	} else if playerStats.GetResource(stats.Health).IsLessThanPercent(0.75) {
		hpColor = color.RGBA{R: 255, G: 255, B: 30, A: 255}
	}

	if playerStats.GetResource(stats.Mana).IsLessThanPercent(0.25) {
		manaColor = color.RGBA{R: 255, G: 30, B: 30, A: 255}
	} else if playerStats.GetResource(stats.Mana).IsLessThanPercent(0.5) {
		manaColor = color.RGBA{R: 255, G: 150, B: 30, A: 255}
	} else if playerStats.GetResource(stats.Mana).IsLessThanPercent(0.75) {
		manaColor = color.RGBA{R: 255, G: 255, B: 30, A: 255}
	}
	label := u.statsLabel
	mapNameParts := util.ToLabelTextWithColor(strings.Split(state.GetMap().GetDisplayName(), "\n"), defs.DarkTextColor)
	statusParts := []util.LabelText{
		{
			Text:      fmt.Sprintf("HP: %s", playerStats.GetResource(stats.Health).ToString()),
			TextColor: hpColor,
		},
		{
			Text:      fmt.Sprintf("MP: %s", playerStats.GetResource(stats.Mana).ToString()),
			TextColor: manaColor,
		},
		{
			Text:      fmt.Sprintf("Gold: %d", playerStats.GetAttribute(stats.Gold)),
			TextColor: defs.DarkTextColor,
		},
	}
	if h := player.Hero(); h != nil && h.StatPts > 0 {
		statusParts = append(statusParts, util.LabelText{Text: fmt.Sprintf("Stat points: %d (u)", h.StatPts), TextColor: defs.PositiveHighlightColor})
	}
	statLines := append(mapNameParts, statusParts...)

	label.SetText(statLines)
}

// helpSection is one titled block of "key -> what it does" lines in the F1 help.
type helpSection struct {
	title   string
	entries [][2]string
}

// helpColumns is every hotkey the game binds, split into the two columns of the help window.
func helpColumns(autoPickup bool) [2][]helpSection {
	pickup := "Pick up items"
	if autoPickup {
		pickup = "Pick up dropped"
	}
	return [2][]helpSection{
		{
			{"MOVE & LOOK", [][2]string{
				{"w a s d", "Move or attack"},
				{"Left click", "Walk there"},
				{"Right click", "Look at it"},
				{"Arrow keys", "Scroll the view"},
				{"x", "Center on you"},
				{"m / n", "Zoom in / out"},
			}},
			{"FIGHT", [][2]string{
				{"tab", "Attack nearest"},
				{"q", "Quick shot"},
				{"f", "Aim a shot"},
				{"t", "Throw an item"},
				{"e", "Cast a spell"},
				{"space", "Recast at nearest"},
				{"1 - 8", "Use a belt slot"},
			}},
			{"WHILE AIMING", [][2]string{
				{"tab / shift+tab", "Next / prev target"},
				{"w a s d", "Move the cursor"},
				{"Enter or f", "Fire"},
			}},
			{"DEBUG", [][2]string{
				{"F10", "Debug menu"},
			}},
		},
		{
			{"EXPLORE", [][2]string{
				{"g", pickup},
				{"o", "Auto-explore"},
				{"z", "Rest until healed"},
				{".", "Wait a turn"},
			}},
			{"CHARACTER", [][2]string{
				{"i", "Inventory"},
				{"c", "Character stats"},
				{"u", "Spend stat points"},
				{"l", "Message log"},
				{"r", "Quest log"},
				{"b", "Story flags"},
			}},
			{"SYSTEM", [][2]string{
				{"F1", "This help"},
				{"F5 / F9", "Quick save / load"},
				{"Esc", "Cancel or pause"},
				{"F11", "Fullscreen"},
			}},
			{"IN MENUS", [][2]string{
				{"Arrows/wasd", "Move selection"},
				{"e or Enter", "Confirm"},
				{"Letter", "Pick that entry"},
				{"Esc", "Close the menu"},
			}},
		},
	}
}

// helpLines lays the hotkeys out in two columns. Every part is placed at a measured x offset instead of
// being padded with spaces, so the columns line up in any font and the text window leaves them alone
// (it only word-wraps lines without color parts, which would collapse the padding).
func helpLines(measure func(string) (float64, float64), autoPickup bool) []util.LabelText {
	columns := helpColumns(autoPickup)
	spaceWidth, _ := measure(" ")
	widest := func(pick func(entry [2]string) string) float64 {
		widest := 0.0
		for _, sections := range columns {
			for _, section := range sections {
				for _, entry := range section.entries {
					if w, _ := measure(pick(entry)); w > widest {
						widest = w
					}
				}
			}
		}
		return widest
	}
	keyOffset := 2 * spaceWidth
	descriptionOffset := keyOffset + widest(func(e [2]string) string { return e[0] }) + 2*spaceWidth
	columnWidth := descriptionOffset + widest(func(e [2]string) string { return e[1] }) + 3*spaceWidth

	part := func(text string, xOffset float64, textColor color.RGBA) util.ColoredTextPart {
		_, height := measure(text)
		return util.ColoredTextPart{Text: text, Color: textColor, XOffset: xOffset, Height: height}
	}
	var rows [2][][]util.ColoredTextPart
	for column, sections := range columns {
		left := float64(column) * columnWidth
		for i, section := range sections {
			if i > 0 {
				rows[column] = append(rows[column], nil) // a blank row between sections
			}
			rows[column] = append(rows[column], []util.ColoredTextPart{part(section.title, left, defs.White)})
			for _, entry := range section.entries {
				rows[column] = append(rows[column], []util.ColoredTextPart{
					part(entry[0], left+keyOffset, defs.LightTextColor),
					part(entry[1], left+descriptionOffset, defs.DarkTextColor),
				})
			}
		}
	}

	const title = "HEAVEN AND HELL - CONTROLS"
	titleWidth, _ := measure(title)
	lines := []util.LabelText{helpLine([]util.ColoredTextPart{part(title, (2*columnWidth-titleWidth)/2, defs.LightTextColor)}), {}}
	for row := 0; row < max(len(rows[0]), len(rows[1])); row++ {
		lines = append(lines, helpLine(append(helpRow(rows[0], row), helpRow(rows[1], row)...)))
	}
	return lines
}

func helpRow(rows [][]util.ColoredTextPart, row int) []util.ColoredTextPart {
	if row >= len(rows) {
		return nil
	}
	return rows[row]
}

func helpLine(parts []util.ColoredTextPart) util.LabelText {
	if len(parts) == 0 {
		return util.LabelText{}
	}
	texts := make([]string, len(parts))
	for i, p := range parts {
		texts[i] = p.Text
	}
	// Text is what the label measures the line height with; the parts are what it draws
	return util.LabelText{Text: strings.Join(texts, " "), UseColorCodes: true, TextWithColorCodes: parts}
}

func (u *UIController) showHelpText() {
	u.OpenTextWindow(helpLines(u.GetRenderer().MeasureString, u.configuration.AutoPickup))
}
func (u *UIController) ShowLog() {
	buffer := u.context.GetLog()
	if len(buffer) > 20 {
		buffer = buffer[len(buffer)-20:]
	}
	u.OpenTextWindow(toGradientLabelText(u.tileRenderer.ParseColorCodedText, buffer))
}

func (u *UIController) OpenListInventory() {
	noFilter := func(element *common.Item) bool { return true }
	u.OpenListInventoryForSelection(noFilter, func(invItemKey rune) {
		inv := u.context.GameState().GetPlayer().GetInventory()
		u.onItemInInventoryClicked(inv.GetItemByLetter(invItemKey))
	})
}

func (u *UIController) FlashScreen(color color.Color, seconds float64) {
	u.globalScaleColor = color
	u.globalFlashColorTicksLeft = int(util.SecondsToTicks(seconds))
	u.globalFlashColorTicksStart = int(util.SecondsToTicks(seconds))
}

func (u *UIController) OpenItemList(items []*common.TextualStack) {
	u.OpenInventoryListModal(u.worldTexture.GetTileSize(), ListDataSource{GetElements: func() []IconListElement {
		result := make([]IconListElement, len(items))
		for i, item := range items {
			result[i] = item
		}
		return result
	}})
}

// closeIfFinal ends a chain of menus after a choice that opened nothing on top of the menu. Only a choice that
// opens another window keeps the menu, so closing that window goes back to it.
func (u *UIController) closeIfFinal(menu UIWidget) {
	if u.GetTopModal() == menu {
		u.PopAll()
	}
}

// OpenServiceMenu lists items for a paid service (repair, identify, recharge) in the buy/sell menus' layout: name
// left, price right-aligned and red when unaffordable.
func (u *UIController) OpenServiceMenu(items []*common.Item, price func(*common.Item) int, pick func(*common.Item)) {
	stacks := make([]common.InventoryStack, len(items))
	for i, it := range items {
		stacks[i] = common.InventoryStack{it}
	}
	common.SortInventory(stacks)
	gold := u.context.GameState().GetPlayer().GetStats().GetAttribute(stats.Gold)
	var menuItems []ShopMenuItem
	for idx, stack := range stacks {
		item, cost := stack[0], price(stack[0])
		priceColor := "darkText"
		if cost > gold {
			priceColor = "negative"
		}
		nameColor := item.TintColor()
		if !item.MeetsRequirements() {
			nameColor = defs.Red
		}
		menuItems = append(menuItems, ShopMenuItem{
			ShortCut: 'a' + rune(idx),
			Tooltip:  item.GetTooltipLines(u.ParseColorCodedText),
			Icon:     item.InventoryIcon(),
			TextColumns: []util.LabelText{
				{Text: item.ShopName(), TextColor: nameColor},
				{Text: fmt.Sprintf("%d", cost), TextWithColorCodes: u.ParseColorCodedText(fmt.Sprintf("[:%s]%d", priceColor, cost)), UseColorCodes: true},
			},
			Action: func() {
				u.PopWidget() // pick reopens the list if anything is left, else it's back to the menu below
				pick(item)
			},
		})
	}
	menu := NewShopMenu(u, u.worldTexture.GetTileSize())
	u.PushWidget(menu)
	menu.SetHeader("You have:", fmt.Sprintf("%d gold", gold))
	menu.SetItems(menuItems, []Alignment{AlignmentLeft, AlignmentRight})
}

func (u *UIController) OpenListInventoryForSelection(filter func(element *common.Item) bool, onSelected func(invItemKey rune)) {
	dataSource := ListDataSource{
		GetElements: func() []IconListElement {
			inventory := u.context.GameState().GetPlayer().GetInventory()
			typed := common.SortStacksForTextUITyped(inventory.StackedItemsWithFilter(filter), inventory.GetLetterForItemStack, onSelected)
			var result []IconListElement
			for _, item := range typed {
				result = append(result, item)
			}
			return result
		},
	}
	u.PopAll()
	u.OpenInventoryListModal(u.worldTexture.GetTileSize(), dataSource)
}

type ColorCodeParser func(draw string) []util.ColoredTextPart

func toGradientLabelText(textParser ColorCodeParser, buffer []string) []util.LabelText {
	// we want each item to be less white than the one before
	intensityFromIndex := func(index int) uint8 {
		maxIntensity := uint8(255)
		minIntensity := uint8(100)
		maxIndex := len(buffer) - 1
		percentage := 1.0 - (float64(index) / float64(maxIndex))
		return uint8(float64(maxIntensity) - (float64(maxIntensity)-float64(minIntensity))*percentage)
	}
	var result []util.LabelText
	for i, line := range buffer {
		fadedBaseColor := color.RGBA{R: intensityFromIndex(i), G: intensityFromIndex(i), B: intensityFromIndex(i), A: 255}
		baseColorCode := defs.RGBAToColorCode(fadedBaseColor)
		result = append(result, util.LabelText{
			UseColorCodes:      true,
			TextWithColorCodes: textParser(baseColorCode + line),
			Text:               common.RemoveColorCodes(line),
			TextColor:          fadedBaseColor,
		})
	}
	return result
}

func sortStacksForUI(stacks []common.InventoryStack) []UIElement {
	items := make([]UIElement, 0)
	common.SortInventory(stacks)
	for _, item := range stacks {
		items = append(items, item)
	}
	return items
}

func (u *UIController) OpenShopMenu(vendor *common.Actor) {
	u.OpenShopMenuFiltered(vendor, func(*common.Item) bool { return true })
}

// OpenShopMenuFiltered is OpenShopMenu restricted to a subset of the vendor's stock, e.g. Griswold's basic vs.
// premium items (S_StartSBuy/S_ScrollSBuy split those into two separate menus in D1).
func (u *UIController) OpenShopMenuFiltered(vendor *common.Actor, filter func(*common.Item) bool) {
	u.openShopMenuWithHandler(vendor.GetInventory(), filter, func(item *common.Item) {
		u.tryToBuy(item, vendor, filter)
	})
}
func (u *UIController) openShopMenuWithHandler(inventory *common.Inventory, filter func(*common.Item) bool, handler func(item *common.Item)) {
	if inventory.IsEmpty() {
		return
	}
	player := u.context.GameState().GetPlayer()
	goldAmount := player.GetStats().GetAttribute(stats.Gold)

	var menuItems []ShopMenuItem
	inventoryStacks := inventory.StackedItemsWithFilter(filter)
	if len(inventoryStacks) == 0 {
		return
	}
	common.SortInventory(inventoryStacks)
	textParser := u.ParseColorCodedText

	for itemIndex, i := range inventoryStacks {

		item := i[0]
		cost := item.BuyPrice()
		cannotPayForItem := cost > goldAmount

		colorCodeForPrice := "darkText"
		if cannotPayForItem {
			colorCodeForPrice = "negative"
		}
		nameString := i.ShopName()
		nameColor := item.TintColor()
		if !item.MeetsRequirements() {
			nameColor = defs.Red
		}
		priceString := fmt.Sprintf("%d", cost)
		priceStringWithColor := fmt.Sprintf("[:%s]%d", colorCodeForPrice, cost)
		// shortcuts start with 0 => 'a'
		shortCut := rune('a') + rune(itemIndex)

		menuItems = append(menuItems, ShopMenuItem{
			ShortCut:    shortCut,
			MakeTooltip: func(mousePos geometry.Point) Tooltip { return NewItemCompareTooltip(u, item, mousePos) },
			Icon:        item.InventoryIcon(),
			TextColumns: []util.LabelText{
				{
					Text: nameString,
					//TextWithColorCodes: textParser(nameStringWithColor),
					UseColorCodes: false,
					TextColor:     nameColor,
				},
				{
					Text:               priceString,
					TextWithColorCodes: textParser(priceStringWithColor),
					UseColorCodes:      true,
				},
			},
			Action: func() {
				handler(item)
			},
		})
	}

	menu := NewShopMenu(u, u.worldTexture.GetTileSize())
	u.PushWidget(menu)
	menu.SetHeader("You have:", fmt.Sprintf("%d gold", goldAmount))
	menu.SetItems(menuItems, []Alignment{AlignmentLeft, AlignmentRight})
}

func (u *UIController) tryToBuy(item *common.Item, seller *common.Actor, filter func(*common.Item) bool) {
	u.msg(common.BuyItem(u.context.GameState().GetPlayer(), seller, item))
	u.PopWidget() // the list is rebuilt with the new gold and stock
	u.OpenShopMenuFiltered(seller, filter)
}

func (u *UIController) showFlags() {
	state := u.context.GameState()
	asStrings := state.GetFlags().AsStrings()
	u.OpenTextWindow(util.ToLabelText(asStrings))
}

func (u *UIController) setTileScale(scale float64) {
	u.tileScale = scale
	u.tileRenderer.SetTileScale(scale)
}

func (u *UIController) UpdateMiniMap() {
	u.mapRenderer.UpdateMiniMap()
}

func (u *UIController) SetPlayerMessage(message func(message string)) {
	u.msg = message
}

// updateMapTileHoverInfo shows what's on the map tile under the mouse (when it isn't over any UI element) in the
// inventory panel's info label, the same place an inventory item's name shows on hover.
func (u *UIController) updateMapTileHoverInfo(mapPos geometry.Point) {
	if u.IsUIModalVisible() {
		return
	}
	player := u.context.GameState().GetPlayer()
	m := u.context.GameState().GetMap()
	if u.showMapInfo { // debug readout: the cell under the cursor, perceivable or not
		u.inventory.infoLabel.SetText([]util.LabelText{{Text: fmt.Sprintf("(%d, %d)", mapPos.X, mapPos.Y), TextColor: color.White}})
		return
	}
	if player == nil || !m.Contains(mapPos) || !player.CanPerceive(mapPos) {
		u.inventory.infoLabel.ClearText()
		return
	}
	var name string
	switch { // infravision reveals the monster, not the floor it stands on
	case m.IsActorAt(mapPos) && m.ActorAt(mapPos).IsVisible():
		name = m.ActorAt(mapPos).Name()
	case !player.CanSee(mapPos):
	case m.IsItemAt(mapPos):
		name = m.ItemAt(mapPos).Name()
	case m.IsObjectAt(mapPos):
		name = m.ObjectAt(mapPos).Name()
	}
	if name == "" {
		u.inventory.infoLabel.ClearText()
		return
	}
	u.inventory.infoLabel.SetText([]util.LabelText{{Text: name, TextColor: color.White}})
}

func (u *UIController) drawHealthBar(actor *common.Actor) {
	health := actor.ShownHealth()
	maxHealth := actor.GetStats().GetAttribute(stats.MaxHealth)
	healthPercent := float64(health) / float64(maxHealth)

	gridSize := u.worldTexture.GetTileSize()
	tileScale := u.GetTileScaleFactor()
	pos := actor.DrawPosition()
	screenPos := u.mapRenderer.MapFloatToScreen(pos)
	healthBarHeight := 2
	healthBarWidth := float64(gridSize.X) * tileScale

	healthBarX := screenPos.X
	healthBarY := float64(screenPos.Y) + (float64(gridSize.Y) * tileScale) - float64(healthBarHeight)

	healthBarColor := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	backgroundColor := color.RGBA{R: 255, G: 0, B: 0, A: 255}

	if actor != u.context.GameState().GetPlayer() {
		backgroundColor = color.RGBA{R: 0, G: 0, B: 0, A: 255}
		healthBarColor = redToGreen(healthPercent)
	}

	topLeft := geometry.Point{X: int(healthBarX), Y: int(healthBarY)}
	size := geometry.Point{X: int(healthBarWidth), Y: healthBarHeight}
	barSize := geometry.Point{X: int(healthBarWidth * healthPercent), Y: healthBarHeight}
	u.tileRenderer.DrawColoredRect(topLeft, size, backgroundColor)
	u.tileRenderer.DrawColoredRect(topLeft, barSize, healthBarColor)
}

func (u *UIController) GetCurrentAtlas() renderer.TextureAtlas {
	switch u.renderingMode {
	case UnicodeRendering:
		fallthrough
	case CodePage437Rendering:
		fallthrough
	case AsciiRendering:
		return u.asciiTexture
	default:
		return u.worldTexture
	}
}

// useLevelGlyphs draws a level that names a glyphs atlas (levels.rec) as text with it, and any other level with
// its tiles.
func (u *UIController) useLevelGlyphs(glyphs string) {
	u.glyphs, u.renderingMode = glyphs, GraphicRendering
	if glyphs != "" {
		atlas, ok := u.glyphAtlases[glyphs]
		if !ok {
			if u.glyphAtlases == nil {
				u.glyphAtlases = map[string]renderer.TextureAtlas{}
			}
			atlas = renderer.NewTextureAtlas(path.Join("assets", glyphs), 8, 8) // the mod's own file
			u.glyphAtlases[glyphs] = atlas
		}
		u.asciiTexture, u.renderingMode = atlas, AsciiRendering
	}
	u.configuration.FlipActorsHorizontally = u.renderingMode == GraphicRendering
}

func redToGreen(percent float64) color.RGBA {
	// 0 -> red, 0.5 -> yellow, 1 -> green
	red := uint8(255 * (1 - percent))
	green := uint8(255 * percent)
	return color.RGBA{R: red, G: green, B: 0, A: 255}
}

// playUICue plays one of the engine's interface cues (menu_move, menu_select, ...) if the mod's sounds.rec has it.
func (u *UIController) playUICue(cue string) {
	if ap := u.context.AudioPlayer; ap != nil && ap.HasCue(cue) {
		ap.PlayCue(cue)
	}
}
