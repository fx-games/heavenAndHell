package gui

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"image/color"
	"io"
	"math"
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/renderer"
	"rpg/stats"
	"rpg/util"
	"sort"
	"strings"
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
	ItemNamesInShop        common.ItemNamingScheme
	KeepPlayerCentered     bool
}

type UIController struct {
	modal        UIWidget
	overlays     []UIWidget
	tileRenderer *renderer.TileRenderer
	mapRenderer  *renderer.MapRenderer
	worldTexture renderer.TextureAtlas
	asciiTexture renderer.TextureAtlas

	context                     common.UIContext
	isMouseCapturedByUI         bool
	shiftIsBeingPressedThisTick bool

	currentTooltip           Tooltip
	ticksUntilTooltipAppears uint64
	screenSize               geometry.Point
	uiBarWidth               int
	baseUIBarWidth           int // uiBarWidth while the character sheet is closed
	uiBar                    *ColoredRect
	elevatedOverlay          UIWidget

	statsLabel *BasicLabel
	inventory  *IconOverlay
	skills     *IconOverlay
	timer      *IconOverlay
	miniMap    *MinimapOverlay
	message    PrintMessage

	configuration *common.Configuration

	setToExactFit    bool
	manualScroll     bool // arrow keys panned the map; suspends auto-centering until recentered
	isFullScreen     bool
	logHook          func(string)
	worldTicks       uint64
	shouldQuitGame   bool
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
	//endX := startX + width

	startY := (screenSize.Y - posInfo.SizeInPixels.Y) / 2
	//endY := startY + height

	topLeft := geometry.Point{X: startX, Y: startY}
	//bottomRight := geometry.Point{X: endX, Y: endY}
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
		windowSize = windowSize.Sub(geometry.Point{X: u.uiBarWidth, Y: 0})
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
		windowSize = windowSize.Sub(geometry.Point{X: u.uiBarWidth, Y: 0})
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
	return u.modal
}

func (u *UIController) IsUIModalVisible() bool {
	return u.modal != nil
}
func (u *UIController) PushWidget(widget UIWidget) {
	u.modal = widget
	u.HideTooltip() // a tooltip belongs to the widget it was hovered in
}

func (u *UIController) PopWidget() {
	u.modal = nil
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
	for _, overlay := range u.overlays {
		if overlay == u.elevatedOverlay {
			continue
		}
		overlay.Draw()
	}
	if !u.IsUIModalVisible() && u.elevatedOverlay == nil {
		return
	}
	renderer := u.GetRenderer()
	renderer.DrawColoredRect(geometry.Point{X: 0, Y: 0}, u.screenSize, color.RGBA{A: 140})

	if u.elevatedOverlay != nil {
		u.elevatedOverlay.Draw()
	}

	if !u.IsUIModalVisible() {
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

// Confirm opens a Yes/No menu asking message; onConfirm runs only if the player picks Yes.
func (u *UIController) Confirm(message string, onConfirm func()) {
	u.OpenMenu([]util.MenuItem{
		{MainText: message},
		{MainText: "Yes", Action: onConfirm},
		{MainText: "No"},
	})
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
	textWindow.SetText(util.WrapLabelText(buffer, u.GetScreenSize().X*2/3, u.tileRenderer.MeasureString))
	textWindow.SetBorderPadding(geometry.Point{X: 15, Y: 15})
	u.PushWidget(textWindow)
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
	u.modal = nil
	u.HideTooltip()
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
		screenSizeForMap = newScreenSize.Sub(geometry.Point{X: u.uiBarWidth, Y: 0})
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

func (u *UIController) HasOverlays() bool {
	return len(u.overlays) > 0
}

func (u *UIController) GetUIBarWidth() int {
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
	toStashHandler := func(uiItem UIElement) {
		playerEquip := player.GetEquipment()
		itemStack := uiItem.(common.InventoryStack)
		firstItem := itemStack[0]
		if playerEquip.IsEquipped(firstItem) { // unequip first..
			u.context.PlayerAction().UnequipItem(firstItem)
		}
		playerInv.Remove(firstItem)
		if u.shiftIsBeingPressedThisTick {
			drop(firstItem)
		} else {
			stashInTown.Add(firstItem)
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
			itemStack := uiItem.(common.InventoryStack)
			firstItem := itemStack[0]
			stashInTown.Remove(firstItem)

			if u.shiftIsBeingPressedThisTick { // drop
				firstItem.DroppedByPlayer()
				u.context.PlayerAction().DropItem(firstItem)
			} else { // move
				playerInv.Add(firstItem)
			}
		},
	}

	tileSize := u.worldTexture.GetTileSize()

	screenSize := u.GetDeviceIndependentScreenSize()
	invBounds := u.inventory.GetBounds()
	paddingX := 20
	offsetFromBottomRightCorner := geometry.Point{X: screenSize.X - invBounds.Max.X + invBounds.Size().X + paddingX, Y: screenSize.Y - invBounds.Max.Y} //- invBounds.Size().X - paddingX, Y: invBounds.Min.Y}

	stashInv := u.OpenInventoryModal(offsetFromBottomRightCorner, tileSize, dataSource)
	u.ElevateOverlay(u.inventory)

	oldHandler := u.inventory.GetOnSelectedHandler()
	u.inventory.SetOnSelectedHandler(toStashHandler)

	onClose := func() { // revert inventory..
		u.StopElevation()
		u.inventory.SetOnSelectedHandler(oldHandler)
	}

	stashInv.SetOnCloseHandler(onClose)
}

func (u *UIController) setupUI() {
	u.overlays = []UIWidget{}
	u.modal = nil

	paddingLeftAndRight := 6
	u.inventory = u.addInventoryOverlay(geometry.Point{X: paddingLeftAndRight, Y: 10})
	inventoryBoxSize := u.inventory.GetBounds().Size()

	u.uiBarWidth = inventoryBoxSize.X + (2 * paddingLeftAndRight)
	u.baseUIBarWidth = u.uiBarWidth
	u.uiBar = u.SetUIBarWidth(u.uiBarWidth)
	u.statsLabel = NewCustomLabel(u, func(u *UIController, l *BasicLabel) { // follows the bar as the character sheet resizes it
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

	u.skills = u.addSkillsOverlay(geometry.Point{X: paddingLeftAndRight, Y: 180})
	skillBounds := u.skills.GetBounds()

	topYForMap := statsBounds.Max.Y + paddingLeftAndRight*2
	bottomYForMap := skillBounds.Min.Y - paddingLeftAndRight
	maxHeightForMap := bottomYForMap - topYForMap

	u.miniMap = u.addMiniMap(geometry.Point{X: u.uiBarWidth, Y: topYForMap}, maxHeightForMap, paddingLeftAndRight)

	u.timer = u.addTimerOverlay(geometry.Point{X: paddingLeftAndRight, Y: 360})

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

// isCharacterSheetShowing reports whether the expanded sidebar ('c') is up: it replaces
// the minimap and skill icons with the full stat list.
func (u *UIController) isCharacterSheetShowing() bool {
	return u.miniMap.isHidden
}

// ToggleFullStats opens the character sheet, or returns to the normal collapsed sidebar.
func (u *UIController) ToggleFullStats() {
	if u.isCharacterSheetShowing() {
		u.UpdateStatsLabel() // collapses the bar and shows the minimap and skills again
		return
	}
	u.ShowFullStats()
}

func (u *UIController) ShowFullStats() {
	u.miniMap.Hide()
	label := u.statsLabel

	player := u.context.GameState().GetPlayer()
	label.SetText(player.GetFullStats())
	u.resizeUIBar(max(u.baseUIBarWidth, label.GetBounds().Size().X+12)) // 6px padding on both sides, as in setupUI
}

// resizeUIBar sets the right sidebar's width and shrinks or grows the map view to match.
func (u *UIController) resizeUIBar(width int) {
	if u.uiBarWidth == width {
		return
	}
	u.uiBarWidth = width
	u.uiBar.distanceToScreen = width
	u.OnScreenSizeChanged()
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
		itemName := common.ShopItemName(u.configuration.ItemNamesInShop, itemStack)
		if !typedItem.MeetsRequirements() {
			itemName = fmt.Sprintf("[:red]%s[:white]", itemName)
		}
		sellItems = append(sellItems, ShopMenuItem{
			ShortCut: playerInventory.GetLetterForItemStack(itemStack),
			Icon:     typedItem.InventoryIcon(),
			Tooltip:  typedItem.GetTooltipLines(textParser),
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
				u.BeginPlayerSellingTo(vendor)
			},
		})
	}
	sort.Slice(sellItems, func(a, b int) bool { return sellItems[a].ShortCut < sellItems[b].ShortCut })
	goldAmount := player.GetStats().GetAttribute(stats.Gold)
	menu := NewShopMenu(u, u.worldTexture.GetTileSize())
	u.PushWidget(menu)
	menu.SetHeader("You have:", fmt.Sprintf("%d gold", goldAmount))
	menu.SetItems(sellItems, []Alignment{AlignmentLeft, AlignmentRight})
}
func (u *UIController) openLevelUpMenu() {
	player := u.context.GameState().GetPlayer()
	if !player.CanLevelUp() {
		return
	}
	u.ShowFullStats()
	items := common.LevelUpMenuItems(player, u.openLevelUpMenu, u.ShowFullStats)
	u.OpenMenu(items)
}
func (u *UIController) addMiniMap(offset geometry.Point, maxHeight int, horizontalPadding int) *MinimapOverlay {
	miniMap := u.AddMinimap(offset.Sub(geometry.Point{X: horizontalPadding}), maxHeight, offset.X-(horizontalPadding*2), u.mapRenderer.GetMiniMap())
	miniMap.SetOnMapClickedHandler(u.onMapClicked)
	return miniMap
}

func (u *UIController) addSkillsOverlay(pixelEdgeFromBottomRight geometry.Point) *IconOverlay {
	tileSize := u.worldTexture.GetTileSize()
	state := u.context.GameState()
	getList := func() []UIElement {
		player := state.GetPlayer()
		items := make([]UIElement, 0)
		if u.miniMap.isHidden { // the character sheet ('c') is showing
			return items
		}
		for _, item := range u.GetActiveSkillsForUI(player) {
			items = append(items, item)
		}
		return items
	}
	isEquipped := func(item UIElement) bool {
		return false // TODO:
	}

	dataSource := IconDataSource{
		IsHighlighted: isEquipped,
		GetElements:   getList,
		OnSelection: func(element UIElement) {
			switch typedElement := element.(type) {
			case *common.UntargetedAction:
				u.context.PlayerAction().ActivateUntargetedAction(typedElement, common.EffectSource(typedElement.GetInternalName()))
			case *common.TargetedAction:
				u.context.PlayerAction().BeginActionTargeting(typedElement)
			}
		},
	}

	return u.OpenSkillsOverlay(pixelEdgeFromBottomRight, tileSize, dataSource)
}

// openSpellMenu lists the memorized D1 spells; picking one casts it (targeted spells start targeting first).
func (u *UIController) openSpellMenu() {
	items := common.SpellMenuItems(u.context.GameState(), u.context.PlayerAction())
	if len(items) == 0 {
		u.ShowLogMessage("You don't know any spells and carry no scrolls or staves.")
		return
	}
	u.OpenMenu(items)
}

func (u *UIController) GetActiveSkillsForUI(player *common.Actor) []UIElement {
	var activeSkills []UIElement
	for _, skill := range player.GetTargetedSkills() {
		activeSkills = append(activeSkills, skill)
	}
	for _, skill := range player.GetUntargetedSkills() {
		activeSkills = append(activeSkills, skill)
	}
	return activeSkills
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
	//mplusBigFont = text.FaceWithLineHeight(mplusBigFont, 54) // adjust line height
	u.tileRenderer.SetTTF(fontFace)
}

func (u *UIController) OnMapChanged() {
	loadedMap := u.context.GameState().GetMap()

	gridSize := u.worldTexture.GetTileSize()
	mapWindow := renderer.NewMapWindow(
		u.GetDeviceIndependentScreenSize().Sub(geometry.Point{X: u.uiBarWidth, Y: 0}),
		geometry.Point{X: loadedMap.GetWidth(), Y: loadedMap.GetHeight()},
		gridSize,
		u.GetTileScaleFactor,
		u.StaticMapLookup)

	u.mapRenderer = renderer.NewMapRenderer(u.tileRenderer, mapWindow)
	u.mapRenderer.SetMiniMapColorHandler(u.getMiniMapColor)

	u.setupUI()

	u.OnPlayerTurnTaken()
}

// defaultMiniMapColors are the built-in minimap colors; config.rec's
// "MiniMapColor: <name>=rrggbb" lines override them by name.
var defaultMiniMapColors = map[string][4]byte{
	"player":        {100, 255, 100, 255},
	"unexplored":    {10, 10, 10, 255},
	"stairsUp":      {255, 204, 0, 255},
	"stairsDown":    {255, 255, 255, 255},
	"waterUnlit":    {43, 56, 97, 255},
	"floorUnlit":    {35, 30, 51, 255},
	"wallUnlit":     {85, 95, 112, 255},
	"hostileActor":  {255, 0, 0, 255},
	"peacefulActor": {220, 120, 75, 255},
	"water":         {86, 112, 194, 255},
	"floor":         {77, 60, 102, 255},
	"wall":          {170, 191, 224, 255},
}

// miniMapColor resolves a minimap color name against config.rec, falling back to the built-in.
func (u *UIController) miniMapColor(name string) [4]byte {
	if fromConfig, ok := u.configuration.MiniMapColors[name]; ok {
		return fromConfig
	}
	return defaultMiniMapColors[name]
}

func (u *UIController) getMiniMapColor(mapPos geometry.Point, tileColor *[4]byte) {
	state := u.context.GameState()
	currentMap := state.GetMap()
	if state.GetPlayer().MapPosition() == mapPos {
		*tileColor = u.miniMapColor("player")
		return
	}
	if !currentMap.IsExplored(mapPos) {
		*tileColor = u.miniMapColor("unexplored")
		return
	}
	if currentMap.IsSpecialAt(mapPos, gridmap.SpecialTileStairsUp) {
		*tileColor = u.miniMapColor("stairsUp")
		return
	}
	if currentMap.IsSpecialAt(mapPos, gridmap.SpecialTileStairsDown) {
		*tileColor = u.miniMapColor("stairsDown")
		return
	}
	if !state.GetPlayer().CanSee(mapPos) {
		if currentMap.IsSpecialAt(mapPos, gridmap.SpecialTileWater) {
			*tileColor = u.miniMapColor("waterUnlit")
			return
		}
		if currentMap.IsWalkable(mapPos) {
			*tileColor = u.miniMapColor("floorUnlit")
		} else {
			*tileColor = u.miniMapColor("wallUnlit")
		}
		return
	}
	if currentMap.IsActorAt(mapPos) && currentMap.ActorAt(mapPos).IsVisible() {
		actorAt := currentMap.ActorAt(mapPos)
		if actorAt.IsHostileTo(state.GetPlayer()) {
			*tileColor = u.miniMapColor("hostileActor")
			return
		}
		*tileColor = u.miniMapColor("peacefulActor")
		return
	}

	if currentMap.IsSpecialAt(mapPos, gridmap.SpecialTileWater) {
		*tileColor = u.miniMapColor("water")
		return
	}

	if currentMap.IsWalkable(mapPos) {
		*tileColor = u.miniMapColor("floor")
	} else {
		*tileColor = u.miniMapColor("wall")
	}
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
	if currentMap.IsDownedActorAt(mapPos) && state.GetPlayer().GetFOV().Visible(mapPos) {
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
	//u.deviceDPIScale = ebiten.DeviceScaleFactor()
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
	return u.IsUIModalVisible() || u.targeter != nil || u.inventory.IsActive() || u.isCharacterSheetShowing()
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
	if u.isCharacterSheetShowing() {
		u.UpdateStatsLabel()
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
	u.targeter = NewTargeter(u, targetRules, onHit)
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
	//playerEquip := g.state.GetPlayer().GetEquipment()

	// NEW
	onDrop := func(item *common.Item) {
		item.DroppedByPlayer()
		state.AddItemToMapWithDistribution(item, state.GetPlayer().MapPosition())
	}
	gui.OpenStash(playerInv, stashInTown, onDrop)
	// on close of the stash modal, restore the onclick handler of the inventory..
}

func (u *UIController) OnPlayerTurnTaken() {
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

	u.resizeUIBar(u.baseUIBarWidth)
	label.SetText(statLines)
	u.miniMap.Show()
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
			}},
			{"DEBUG", [][2]string{
				{"F8", "Rendering mode"},
				{"F9", "Fog of war"},
				{"F10", "Debug menu"},
				{"F12", "Flash screen"},
				{"p", "Kill the level"},
			}},
		},
		{
			{"EXPLORE", [][2]string{
				{"g", pickup},
				{"o", "Auto-explore"},
				{"j / k", "Stairs up / down"},
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
	//logWindow.ScrollToBottom()
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
				{Text: item.BandStyleName(), TextColor: nameColor},
				{Text: fmt.Sprintf("%d", cost), TextWithColorCodes: u.ParseColorCodedText(fmt.Sprintf("[:%s]%d", priceColor, cost)), UseColorCodes: true},
			},
			Action: func() {
				u.PopAll()
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
		itemName := item.Name()
		if u.configuration.ItemNamesInShop == common.ItemNamingSchemeEnchantments {
			itemName = item.PlusName()
		} else if u.configuration.ItemNamesInShop == common.ItemNamingSchemeBandStyle {
			itemName = item.BandStyleName()
		}
		if len(i) > 1 {
			itemName = fmt.Sprintf("%s (x%d)", itemName, len(i))
		}
		nameString := fmt.Sprintf("%s", itemName)
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
	u.OpenShopMenuFiltered(seller, filter)
}

func (u *UIController) showFlags() {
	state := u.context.GameState()
	asStrings := state.GetFlags().AsStrings()
	u.OpenTextWindow(util.ToLabelText(asStrings))
	//logWindow.ScrollToBottom()
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
	health := actor.GetStats().GetResource(stats.Health).Value()
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

func (u *UIController) SetAsciiAtlas(atlas renderer.TextureAtlas) {
	u.asciiTexture = atlas
}

func redToGreen(percent float64) color.RGBA {
	// 0 -> red, 0.5 -> yellow, 1 -> green
	red := uint8(255 * (1 - percent))
	green := uint8(255 * percent)
	return color.RGBA{R: red, G: green, B: 0, A: 255}
}
