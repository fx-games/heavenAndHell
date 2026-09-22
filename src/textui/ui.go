// Package textui is the terminal UI framework taken from contractor's ui_console: panels and focus handling,
// menus, dialogs, the text inventory, key maps, map drawing and targeting. It knows nothing about this game or
// its graphics; package console connects it to the game.
package textui

import (
	"fmt"
	"io"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/geometry"
	"github.com/memmaker/go/textiles"
)

// MapSource is what the UI needs to draw the map.
type MapSource interface {
	IsPlayerAndMapInitialized() bool
	GetMapSize() geometry.Point
	GetPlayerPosition() geometry.Point
	IsVisibleToPlayer(loc geometry.Point) bool
	IsExplored(loc geometry.Point) bool
	// VisibleIconAt is what the player sees at loc, lighting applied.
	VisibleIconAt(loc geometry.Point) textiles.TextIcon
	// ExploredIconAt is what the player remembers at loc; the UI darkens it.
	ExploredIconAt(loc geometry.Point) textiles.TextIcon
}

// Settings configure a UI. Theme, Palette and KeyMap are the ui.rec, palette.rec and key map files.
type Settings struct {
	Theme, Palette, KeyMap io.Reader
	RightPanelWidth        int
	MinTerminalWidth       int
	MinTerminalHeight      int
	AnimationDelay         time.Duration
}

type UIState int

func (s UIState) IsTargeting() bool {
	return s == StateTargeting
}

const (
	StateNormal UIState = iota
	StateTargeting
)

type FocusInfo struct {
	Primitive   cview.Primitive
	BeforeFocus func(cview.Primitive) bool
}

type InputPrimitive interface {
	cview.Primitive
	SetInputCapture(f func(event *tcell.EventKey) *tcell.EventKey)
	GetInputCapture() func(event *tcell.EventKey) *tcell.EventKey
}

type UI struct {
	settings Settings
	game     MapSource

	uiTheme Theme

	mapOverlay  *Overlay
	mapScroll   geometry.Point
	mapRect     geometry.Rect // the map window on screen at the last draw
	recenterMap bool

	mainGrid        *cview.Grid
	lowerRightPanel *cview.TextView
	messageLabel    *cview.TextView
	statusBar       *cview.TextView
	rightPanel      *cview.TextView
	pages           *cview.Panels
	application     *cview.Application
	mapWindow       *cview.Box

	currentMouseX int
	currentMouseY int
	state         UIState

	targetingTiles  map[geometry.Point]bool
	targetPos       geometry.Point
	onTargetUpdated func(targetPos geometry.Point) (adjusted geometry.Point, highlights []geometry.Point, info string)

	animationIcons map[geometry.Point]textiles.TextIcon

	tooSmall     bool
	gamma        float64
	commandTable map[string]func()
	keyTable     map[KeyLayer]map[UIKey]string
	focusStack   []FocusInfo
	modals       map[string]bool

	// OnMapHovered returns the text to show for the map position under the mouse.
	OnMapHovered func(mapPos geometry.Point) string
	// OnMapClicked handles a click on the map outside of targeting.
	OnMapClicked func(mapPos geometry.Point, isRightClick bool)
	// OnRightPanelHovered returns the text to show for a line of the right panel under the mouse.
	OnRightPanelHovered func(line int) string
	// OnRightPanelClicked handles a click on a line of the right panel (the first line is 0).
	OnRightPanelClicked func(line int, isRightClick bool, modified bool)
	// OnQuit runs before the application stops.
	OnQuit func()
}

func NewTextUI(game MapSource, settings Settings) *UI {
	u := &UI{
		targetingTiles: make(map[geometry.Point]bool),
		gamma:          1.0,
		game:           game,
		settings:       settings,
		keyTable:       make(map[KeyLayer]map[UIKey]string),
		commandTable:   make(map[string]func()),
		modals:         make(map[string]bool),
	}

	cview.TrueColorTags = true

	u.application = cview.NewApplication()
	u.application.SetAfterResizeFunc(u.onTerminalResized)
	u.application.SetInputCapture(u.appInputCapture)

	u.pages = cview.NewPanels()
	u.application.SetRoot(u.pages, true)

	u.loadKeyMap(settings.KeyMap)
	u.initUI(textiles.ReadPaletteFileOrDefault(settings.Palette))
	return u
}

func (u *UI) initUI(palette textiles.ColorPalette) {
	u.application.EnableMouse(true)

	u.application.SetMouseCapture(u.handleMainMouse)
	u.application.SetBeforeFocusFunc(u.defaultFocusHandler)

	u.mapWindow = cview.NewBox()
	u.mapWindow.SetDrawFunc(u.drawMap)
	u.mapWindow.SetInputCapture(u.handleMainInput)

	u.messageLabel = cview.NewTextView()

	u.statusBar = cview.NewTextView()
	u.statusBar.SetDynamicColors(true)
	u.statusBar.SetScrollable(false)
	u.statusBar.SetScrollBarVisibility(cview.ScrollBarNever)

	u.rightPanel = cview.NewTextView()
	u.rightPanel.SetScrollable(false)
	u.rightPanel.SetScrollBarVisibility(cview.ScrollBarNever)
	u.rightPanel.SetDynamicColors(true)
	u.rightPanel.SetWrap(false)
	u.rightPanel.SetWordWrap(false)

	u.lowerRightPanel = cview.NewTextView()
	u.lowerRightPanel.SetScrollable(true)
	u.lowerRightPanel.SetScrollBarVisibility(cview.ScrollBarNever)
	u.lowerRightPanel.SetDynamicColors(true)
	u.lowerRightPanel.SetWrap(false)
	u.lowerRightPanel.SetWordWrap(false)

	grid := cview.NewGrid()
	grid.SetRows(1, 0, 0, 1)
	grid.SetColumns(0, u.settings.RightPanelWidth)
	grid.AddItem(u.messageLabel, 0, 0, 1, 2, 0, 0, false)
	grid.AddItem(u.mapWindow, 1, 0, 2, 1, 0, 0, true)
	grid.AddItem(u.rightPanel, 1, 1, 1, 1, 0, 0, false)
	grid.AddItem(u.lowerRightPanel, 2, 1, 1, 1, 0, 0, false)
	grid.AddItem(u.statusBar, 3, 0, 1, 2, 0, 0, false)

	u.mainGrid = grid

	u.pages.AddPanel("main", grid, true, true)

	u.application.SetFocus(grid)

	u.mapOverlay = NewOverlay()

	u.uiTheme = NewUIThemeFromRecords(u.settings.Theme, palette)
	u.applyStylingToUI()
}

// screenSize is the terminal's size, or the minimum size while there is no screen yet.
func (u *UI) screenSize() (int, int) {
	if screen := u.application.GetScreen(); screen != nil {
		return screen.Size()
	}
	return u.settings.MinTerminalWidth, u.settings.MinTerminalHeight
}

// Run starts the terminal event loop and calls tick fps times per second on it until the UI quits.
// onScreenReady runs once the screen exists, which is when the first widgets can be sized.
func (u *UI) Run(fps int, tick func(), onScreenReady func()) error {
	u.application.QueueUpdate(func() {
		screen := u.application.GetScreen()
		tty, isTerm := screen.Tty()
		if isTerm && tty != nil {
			tty.Write([]byte{0x1B, 0x3E})
		}
		w, h := screen.Size()
		u.application.QueueEvent(tcell.NewEventResize(w, h))
		screen.HideCursor()
		if onScreenReady != nil {
			onScreenReady()
		}
	})
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(fps))
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				u.application.QueueUpdateDraw(tick)
			}
		}
	}()
	defer close(stop)
	return u.application.Run()
}

func (u *UI) QuitGame() {
	if u.OnQuit != nil {
		u.OnQuit()
	}
	u.application.Stop()
}

// GetMapWindowGridSize is the map window's size at the last draw (cview locks the box while it draws).
func (u *UI) GetMapWindowGridSize() geometry.Point {
	return u.mapRect.Size()
}

// VisibleMap is the map area on screen.
func (u *UI) VisibleMap() geometry.Rect {
	size := u.GetMapWindowGridSize()
	return geometry.NewRect(u.mapScroll.X, u.mapScroll.Y, u.mapScroll.X+size.X, u.mapScroll.Y+size.Y)
}

func (u *UI) setColoredText(view *cview.TextView, text string) {
	view.SetDynamicColors(true)
	view.SetText(text)
}

// Print shows a message in the message line.
func (u *UI) Print(message string) {
	if message == "" {
		return
	}
	u.setColoredText(u.messageLabel, message)
}

// UpdateLogWindow shows the last log lines in the message area, older ones fading out.
func (u *UI) UpdateLogWindow(logMessages []string) {
	if len(logMessages) > 100 {
		logMessages = logMessages[len(logMessages)-100:]
	}
	u.setColoredText(u.messageLabel, joinLines(logMessages))
	u.messageLabel.ScrollToEnd()
}

func (u *UI) SetRightPanel(text string) { u.rightPanel.SetText(text) }

func (u *UI) SetLowerRightPanel(text string) {
	u.lowerRightPanel.SetText(text)
	u.lowerRightPanel.ScrollToBeginning()
}

func (u *UI) SetStatusBar(text string) {
	width, _ := u.application.GetScreenSize()
	u.statusBar.SetText(expandToWidth(text, width))
}

// SetAnimationIcons sets what is drawn over the map until the next call.
func (u *UI) SetAnimationIcons(icons map[geometry.Point]textiles.TextIcon) {
	u.animationIcons = icons
}

func (u *UI) Theme() Theme { return u.uiTheme }

func (u *UI) ScreenToMap(point geometry.Point) geometry.Point {
	return point.Sub(u.mapRect.Min).Add(u.mapScroll)
}

func (u *UI) onTerminalResized(width int, height int) {
	if u.mainGrid == nil {
		return
	}

	tSizeX, tSizeY := u.settings.MinTerminalWidth, u.settings.MinTerminalHeight

	if height <= tSizeY {
		u.mainGrid.SetRows(1, 0, 0, 1)
		u.messageLabel.SetScrollable(false)
		u.messageLabel.SetScrollBarVisibility(cview.ScrollBarNever)
	} else {
		additionalHeight := min(height-tSizeY, 6)
		u.mainGrid.SetRows(1+additionalHeight, 0, 0, 1)
		u.messageLabel.SetScrollable(true)
		u.messageLabel.SetScrollBarVisibility(cview.ScrollBarNever)
	}
	u.pages.SetRect(0, 0, width, height)
	if width < tSizeX || height < tSizeY {
		u.tooSmall = true
		view := cview.NewTextView()
		view.SetText(fmt.Sprintf("Min. terminal size is %dx%d", tSizeX, tSizeY))
		u.pages.AddPanel("tooSmall", view, true, true)
	} else if u.tooSmall {
		u.pages.RemovePanel("tooSmall")
		u.tooSmall = false
	}
}

func (u *UI) isModalOpen() bool {
	_, front := u.pages.GetFrontPanel()
	return front != u.mainGrid
}

func (u *UI) FadeToBlack() {
	cview.FadeToBlack(u.application, u.settings.AnimationDelay/2, 10, false)
}

func (u *UI) FadeFromBlack() {
	cview.FadeFromBlack(u.application, u.settings.AnimationDelay/2, 10, false)
}
