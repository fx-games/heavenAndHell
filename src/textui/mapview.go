package textui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/geometry"
	"github.com/memmaker/go/textiles"
)

// Map drawing

// UpdateMapScrollPosition scrolls the map so the player stays away from the window's borders.
func (u *UI) UpdateMapScrollPosition() {
	u.scrollToKeepVisible(u.game.GetPlayerPosition())
}

// ScrollMapBy scrolls the map view, until the player moves close to a border again.
func (u *UI) ScrollMapBy(delta geometry.Point) {
	mapSize, window := u.game.GetMapSize(), u.GetMapWindowGridSize()
	u.mapScroll = u.mapScroll.Add(delta)
	u.mapScroll.X = max(0, min(u.mapScroll.X, mapSize.X-window.X))
	u.mapScroll.Y = max(0, min(u.mapScroll.Y, mapSize.Y-window.Y))
}

// CenterMapOnPlayer puts the player in the middle of the map window at the next draw, e.g. after the map changed.
func (u *UI) CenterMapOnPlayer() {
	u.recenterMap = true
}

func (u *UI) scrollToKeepVisible(pos geometry.Point) {
	mapSize := u.game.GetMapSize()
	window := u.GetMapWindowGridSize()
	windowWidth, windowHeight := window.X, window.Y
	if windowWidth <= 0 || windowHeight <= 0 {
		return
	}
	minDistToBorders := min(5, windowWidth/4, windowHeight/4)

	if pos.X-u.mapScroll.X < minDistToBorders {
		u.mapScroll.X = pos.X - minDistToBorders
	} else if pos.X-u.mapScroll.X > windowWidth-minDistToBorders {
		u.mapScroll.X = pos.X + minDistToBorders - windowWidth
	}
	if pos.Y-u.mapScroll.Y < minDistToBorders {
		u.mapScroll.Y = pos.Y - minDistToBorders
	} else if pos.Y-u.mapScroll.Y > windowHeight-minDistToBorders {
		u.mapScroll.Y = pos.Y + minDistToBorders - windowHeight
	}
	u.mapScroll.X = max(0, min(u.mapScroll.X, mapSize.X-windowWidth))
	u.mapScroll.Y = max(0, min(u.mapScroll.Y, mapSize.Y-windowHeight))
}

func (u *UI) drawMap(screen tcell.Screen, x int, y int, width int, height int) (int, int, int, int) {
	if !u.game.IsPlayerAndMapInitialized() {
		return x, y, width, height
	}
	windowChanged := u.mapRect.Size() != geometry.Point{X: width, Y: height}
	u.mapRect = geometry.NewRect(x, y, x+width, y+height)
	if u.recenterMap {
		u.recenterMap = false
		u.mapScroll = u.game.GetPlayerPosition().Sub(u.GetMapWindowGridSize().Div(2))
		u.UpdateMapScrollPosition()
	} else if windowChanged { // hand scrolling stands until the player nears a border again (UpdateMapScrollPosition)
		u.UpdateMapScrollPosition()
	}
	if u.state.IsTargeting() {
		u.scrollToKeepVisible(u.targetPos)
	}

	defaultMapStyle := u.uiTheme.GetMapDefaultStyle()

	for row := y; row < y+height; row++ {
		for col := x; col < x+width; col++ {
			mapPos := geometry.Point{X: col - x + u.mapScroll.X, Y: row - y + u.mapScroll.Y}
			ch, style := u.renderMapPosition(mapPos, defaultMapStyle)
			screen.SetContent(col, row, ch, nil, style)
		}
	}
	return x, y, width, height
}

func (u *UI) renderMapPosition(mapPos geometry.Point, style tcell.Style) (rune, tcell.Style) {
	var textIcon textiles.TextIcon
	foundIcon := false

	if animIcon, exists := u.animationIcons[mapPos]; exists {
		foundIcon = true
		textIcon = animIcon
		if !animIcon.HasBackground() {
			mapIcon, _ := u.mapLookup(mapPos)
			textIcon = animIcon.WithBg(mapIcon.Bg)
		}
	} else if u.mapOverlay.IsSet(mapPos.X, mapPos.Y) {
		textIcon = u.mapOverlay.Get(mapPos.X, mapPos.Y)
		foundIcon = true
	} else {
		textIcon, foundIcon = u.mapLookup(mapPos)
	}

	ch := ' '
	fg := u.uiTheme.GetUIColor(UIColorUIForeground)
	bg := u.uiTheme.GetUIColor(UIColorUIBackground)
	if foundIcon {
		ch, fg, bg = textIcon.Char, textIcon.Fg, textIcon.Bg
		style = style.Attributes(textIcon.Attributes)
	}

	style = style.Foreground(tcell.NewRGBColor(int32(applyGamma(fg.R, u.gamma)), int32(applyGamma(fg.G, u.gamma)), int32(applyGamma(fg.B, u.gamma))))
	style = style.Background(tcell.NewRGBColor(int32(applyGamma(bg.R, u.gamma)), int32(applyGamma(bg.G, u.gamma)), int32(applyGamma(bg.B, u.gamma))))

	if u.state.IsTargeting() && u.targetingTiles[mapPos] {
		if mapPos == u.targetPos {
			style = style.Background(tcell.ColorGreen)
		} else {
			style = style.Background(tcell.ColorDarkGreen)
		}
	}
	return ch, style
}

func (u *UI) mapLookup(loc geometry.Point) (textiles.TextIcon, bool) {
	if u.game.IsVisibleToPlayer(loc) {
		return u.game.VisibleIconAt(loc), true
	} else if u.game.IsExplored(loc) {
		mapIcon := u.game.ExploredIconAt(loc)
		mapIcon.Fg = darken(desaturate(mapIcon.Fg))
		mapIcon.Bg = darken(desaturate(mapIcon.Bg))
		return mapIcon, true
	}
	return textiles.TextIcon{}, false
}

// Mouse

func (u *UI) handleMainMouse(event *tcell.EventMouse, action cview.MouseAction) (*tcell.EventMouse, cview.MouseAction) {
	if u.isModalOpen() || event == nil {
		return event, action
	}

	newX, newY := event.Position()
	mousePos := geometry.Point{X: newX, Y: newY}
	overMap := u.mapRect.Contains(mousePos)
	rightX, rightY, _, rightH := u.rightPanel.GetInnerRect()
	overRightPanel := newX >= rightX && newY >= rightY && newY < rightY+rightH

	if newX != u.currentMouseX || newY != u.currentMouseY {
		u.currentMouseX = newX
		u.currentMouseY = newY
		if action == cview.MouseMove && overMap && u.OnMapHovered != nil {
			u.Print(u.OnMapHovered(u.ScreenToMap(mousePos)))
			return nil, -1
		}
		if action == cview.MouseMove && overRightPanel && u.OnRightPanelHovered != nil {
			u.Print(u.OnRightPanelHovered(newY - rightY))
			return nil, -1
		}
	}

	isRightClick := action == cview.MouseRightClick
	if action != cview.MouseLeftClick && !isRightClick {
		return event, action
	}
	if overRightPanel && u.OnRightPanelClicked != nil {
		u.OnRightPanelClicked(newY-rightY, isRightClick, event.Modifiers() != 0)
		return nil, -1
	}
	if overMap && u.OnMapClicked != nil {
		u.OnMapClicked(u.ScreenToMap(mousePos), isRightClick)
		return nil, -1
	}
	return event, action
}

// Targeting

// SelectTarget lets the player pick a map position with the targeting key layer or the mouse. Next/previous cycle
// through candidates. onTargetUpdated adjusts a position and returns the positions to highlight and a label.
func (u *UI) SelectTarget(candidates []geometry.Point, preselected geometry.Point, onTargetUpdated func(targetPos geometry.Point) (geometry.Point, []geometry.Point, string), onSelected func(targetPos geometry.Point), onCancel func()) {
	u.state = StateTargeting
	u.onTargetUpdated = onTargetUpdated
	u.updateTarget(preselected)
	cancel := func() {
		u.CancelTargeting()
		if onCancel != nil {
			onCancel()
		}
	}
	u.mapWindow.SetInputCapture(u.handleTargetingInput(candidates, onSelected, cancel))
	u.application.SetMouseCapture(func(event *tcell.EventMouse, action cview.MouseAction) (*tcell.EventMouse, cview.MouseAction) {
		if event == nil {
			return event, action
		}
		newX, newY := event.Position()
		mapPos := u.ScreenToMap(geometry.Point{X: newX, Y: newY})
		switch action {
		case cview.MouseMove:
			if newX != u.currentMouseX || newY != u.currentMouseY {
				u.currentMouseX = newX
				u.currentMouseY = newY
				u.updateTarget(mapPos)
				return nil, action
			}
		case cview.MouseLeftClick, cview.MouseRightClick:
			u.updateTarget(mapPos)
			target := u.targetPos
			u.CancelTargeting()
			onSelected(target)
			return nil, action
		}
		return event, action
	})
}

func (u *UI) handleTargetingInput(candidates []geometry.Point, onSelected func(targetPos geometry.Point), cancel func()) func(ev *tcell.EventKey) *tcell.EventKey {
	candidateIndex := 0
	cycle := func(step int) {
		if len(candidates) == 0 {
			return
		}
		candidateIndex = (candidateIndex + step + len(candidates)) % len(candidates)
		u.updateTarget(candidates[candidateIndex])
	}
	return func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyCtrlC {
			return ev
		}
		targetingCommand := u.getTargetingCommandForKey(toUIKey(ev))
		if targetingCommand == "target_cancel" {
			cancel()
		} else if targetingCommand == "target_next" {
			cycle(1)
		} else if targetingCommand == "target_previous" {
			cycle(-1)
		} else if direction, isDirectionalCommand := directionFromCommand(targetingCommand); isDirectionalCommand {
			u.updateTarget(u.targetPos.Add(direction.ToPoint()))
		} else if targetingCommand == "target_confirm" {
			target := u.targetPos
			u.CancelTargeting()
			onSelected(target)
		}
		return nil
	}
}

func (u *UI) IsTargeting() bool { return u.state.IsTargeting() }

func (u *UI) CancelTargeting() {
	if !u.state.IsTargeting() {
		return
	}
	u.mapWindow.SetInputCapture(u.handleMainInput)
	u.application.SetMouseCapture(u.handleMainMouse)
	u.state = StateNormal
	u.mapOverlay.ClearAll()
	clear(u.targetingTiles)
}

func (u *UI) updateTarget(targetPos geometry.Point) {
	clear(u.targetingTiles)
	u.mapOverlay.ClearAll()
	adjusted, highlights, info := u.onTargetUpdated(targetPos)
	u.targetPos = adjusted
	u.targetingTiles[adjusted] = true
	for _, p := range highlights {
		u.targetingTiles[p] = true
	}
	if info != "" {
		if u.game.GetPlayerPosition().Y < adjusted.Y {
			u.mapOverlay.AddBelow(adjusted, info)
		} else {
			u.mapOverlay.AddAbove(adjusted, info)
		}
	}
}
