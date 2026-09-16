package textui

import (
	"fmt"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
)

// InventoryItem is one line of a TextInventory.
type InventoryItem interface {
	Shortcut() rune
	InventoryName() string // with color tags
	RightColumn() string   // right aligned, may be empty
	IsUsable() bool        // can be equipped or used
}

// InventoryLabel is an item's inventory line, "a - name" or "a + name" when equipped.
func InventoryLabel(item InventoryItem, isEquipped bool) string {
	sign := '-'
	if isEquipped {
		sign = '+'
	}
	return fmt.Sprintf("%c %c %s", item.Shortcut(), sign, item.InventoryName())
}

func (u *UI) openInventory(items []InventoryItem) *TextInventory {
	panelName := "inventory"
	screenWidth, screenHeight := u.screenSize()
	inventory := NewTextInventory(screenWidth, screenHeight)
	fg := u.uiTheme.GetUIColorForTcell(UIColorUIForeground)
	bg := u.uiTheme.GetUIColorForTcell(UIColorUIBackground)

	borderFgFocus := u.uiTheme.GetUIColorForTcell(UIColorBorderForegroundFocus)

	inventory.SetBorder(false)
	inventory.SetPadding(1, 1, 1, 1)
	inventory.SetWrapAround(false)
	inventory.SetHover(true)
	inventory.ShowSecondaryText(false)

	inventory.SetScrollBarColor(fg)

	inventory.SetTitleColor(fg)
	inventory.SetMainTextColor(fg)
	inventory.SetSecondaryTextColor(fg)

	inventory.SetBorderColor(fg)
	inventory.SetBorderColorFocused(borderFgFocus)

	inventory.SetBackgroundColor(bg)

	inventory.SetShortcutColor(fg)

	inventory.SetSelectedTextColor(bg)
	inventory.SetSelectedBackgroundColor(fg)
	inventory.SetForceSelectedTextColor(true)

	inventory.SetStyle(u.uiTheme.defaultStyle)
	inventory.listWidth = u.settings.RightPanelWidth - 2 // cover the right panel
	inventory.SetItems(items)

	inventory.SetCloseHandler(func() {
		u.popPanel(panelName)
	})
	u.addModal(panelName, inventory)

	u.lockFocusToPrimitive(inventory)

	originalInputCapture := inventory.GetInputCapture()
	inventory.SetInputCapture(u.directionalWrapperWithoutAlphabet(originalInputCapture))

	return inventory
}

// OpenInventoryForManagement opens the inventory: a letter uses or equips, shift+letter drops, ctrl+letter
// examines and space opens the context menu of the highlighted item. getItems is asked again after each action.
func (u *UI) OpenInventoryForManagement(getItems func() []InventoryItem, isEquipped func(InventoryItem) bool, use, drop, examine func(InventoryItem), contextMenu func(item InventoryItem, done func())) *TextInventory {
	inv := u.openInventory(getItems())
	inv.SetTitle("Inventory")
	inv.SetEquippedTest(isEquipped)
	inv.SetContextMenu(contextMenu)
	refresh := func() { inv.SetItems(getItems()) }
	inv.SetDefaultSelection(func(item InventoryItem) {
		use(item)
		refresh()
	})
	inv.SetShiftSelection(func(item InventoryItem) {
		drop(item)
		refresh()
	})
	inv.SetControlSelection(examine)

	inv.SetCloseOnControlSelection(false)
	inv.SetCloseOnShiftSelection(false)
	return inv
}

func (u *UI) OpenInventoryForSelection(itemStacks []InventoryItem, prompt string, onSelected func(item InventoryItem)) *TextInventory {
	inv := u.openInventory(itemStacks)
	inv.SetSelectionMode()
	inv.SetTitle(prompt)
	inv.SetDefaultSelection(onSelected)
	inv.SetCloseOnSelection(true)
	return inv
}

type TextInventory struct {
	*cview.List
	items                []InventoryItem
	defaultSelection     func(item InventoryItem)
	shiftSelection       func(item InventoryItem)
	controlSelection     func(item InventoryItem)
	contextMenu          func(item InventoryItem, done func())
	listWidth            int
	listHeight           int
	closeHandler         func()
	isEquipped           func(item InventoryItem) bool
	style                tcell.Style
	closeOnSelect        bool
	closeOnShiftSelect   bool
	closeOnControlSelect bool
	ourTitle             string
	selectionOnly        bool
	afterClose           func()
	infoLines            []string
	footer               func() string
	screenHeight         int
	screenWidth          int
}

func (i *TextInventory) SetContextMenu(contextMenu func(item InventoryItem, done func())) {
	i.contextMenu = contextMenu
}

// SetFooter sets a line shown below the items, e.g. a summary of the pack.
func (i *TextInventory) SetFooter(footer func() string) {
	i.footer = footer
	i.updateListItems()
}

func (i *TextInventory) Draw(screen tcell.Screen) {
	x, y, w, h := i.GetRect()
	i.drawOutside(screen, x, y, w, h)
	i.List.Draw(screen)
}

// drawOutside is the box around the list: the items sit at the top, the key hints below them.
func (i *TextInventory) drawOutside(screen tcell.Screen, x int, y int, width int, height int) {
	fg, _, _ := i.style.Decompose()
	runes := []rune{cview.Borders.HorizontalFocus, cview.Borders.VerticalFocus, cview.Borders.TopLeftFocus, cview.Borders.TopRightFocus, cview.Borders.BottomRightFocus, cview.Borders.BottomLeftFocus}
	drawBackgroundAndBorderWithTitle(screen, x, y, width, height, i.ourTitle, i.style, runes)

	firstInfoLine := y + 1 + i.listHeight
	for idx, line := range i.infoLines {
		cview.Print(screen, []byte(line), x+1, firstInfoLine+idx, width-2, cview.AlignLeft, fg)
	}
}

func NewTextInventory(screenWidth int, screenHeight int) *TextInventory {
	list := cview.NewList()
	list.SetBorder(false)
	t := &TextInventory{
		List:         list,
		items:        []InventoryItem{},
		screenWidth:  screenWidth,
		screenHeight: screenHeight,
	}
	list.SetBackgroundTransparent(true)
	list.SetInputCapture(t.handleInput)
	return t
}

func (i *TextInventory) SetStyle(style tcell.Style) {
	i.style = style
}

func (i *TextInventory) SetTitle(title string) {
	i.ourTitle = title
	i.listWidth = max(i.listWidth, len(title))
}

func (i *TextInventory) SetDefaultSelection(onSelect func(item InventoryItem)) {
	i.defaultSelection = onSelect
}

func (i *TextInventory) SetShiftSelection(onSelect func(item InventoryItem)) {
	i.shiftSelection = onSelect
}

func (i *TextInventory) SetControlSelection(onSelect func(item InventoryItem)) {
	i.controlSelection = onSelect
}

func (i *TextInventory) SetItems(invItem []InventoryItem) {
	i.items = invItem
	if len(invItem) == 0 {
		i.Close()
		return
	}
	i.updateListItems()
}

func (i *TextInventory) updateListItems() {
	currentItem := i.GetCurrentItemIndex()
	labels := make([]fxtools.TableRow, len(i.items))
	for lineIndex, invItem := range i.items {
		labels[lineIndex] = fxtools.NewTableRow(InventoryLabel(invItem, i.isEquipped != nil && i.isEquipped(invItem)), invItem.RightColumn())
	}
	itemLabels := fxtools.TableLayoutLastRight(labels)

	var width int
	for _, line := range itemLabels {
		width = max(width, cview.TaggedStringWidth(line))
	}

	i.listWidth = max(i.listWidth, width)
	i.listHeight = min(len(i.items), max(1, i.screenHeight-8))

	i.Clear()

	var equipAndUseRunes []rune
	var inspectRunes []rune
	var dropRunes []rune

	for lineIndex, invItem := range i.items {
		line := itemLabels[lineIndex]
		if cview.TaggedStringWidth(line) < i.listWidth {
			line = RightPadColored(line, i.listWidth)
		}
		i.AddItem(cview.NewListItem(line))

		shortcut := invItem.Shortcut()
		if invItem.IsUsable() {
			equipAndUseRunes = append(equipAndUseRunes, shortcut)
		}
		inspectRunes = append(inspectRunes, shortcut)
		dropRunes = append(dropRunes, shortcut)
	}

	var infoLines []string
	if i.footer != nil {
		infoLines = append(infoLines, i.footer())
	}

	if !i.selectionOnly {
		if len(equipAndUseRunes) > 0 {
			infoLines = append(infoLines, cview.Escape("[letter] (Un)Equip / Use"))
		}
		if len(inspectRunes) > 0 {
			infoLines = append(infoLines, cview.Escape("[<CTRL> + letter] Examine"))
		}
		if len(dropRunes) > 0 {
			infoLines = append(infoLines, cview.Escape("[<SHFT> + letter] Drop"))
		}
		if i.contextMenu != nil {
			infoLines = append(infoLines, cview.Escape("[Space] More"))
		}
	}
	for _, line := range infoLines {
		i.listWidth = max(i.listWidth, cview.TaggedStringWidth(line)-2)
	}

	i.infoLines = infoLines
	widthNeeded := i.listWidth + 2

	heightNeeded := i.listHeight + len(infoLines) + 2

	rightX := i.screenWidth - widthNeeded
	i.SetRect(rightX, 0, widthNeeded, heightNeeded)

	if currentItem >= 0 && currentItem < len(i.items) {
		i.SetCurrentItem(currentItem)
	}
}

func (i *TextInventory) SetEquippedTest(isEquipped func(item InventoryItem) bool) {
	i.isEquipped = isEquipped
}

func (i *TextInventory) SetCloseHandler(escapeHandler func()) {
	i.closeHandler = escapeHandler
}

func (i *TextInventory) previousItem() {
	i.SetCurrentItem(i.GetCurrentItemIndex() - 1)
}

func (i *TextInventory) nextItem() {
	i.SetCurrentItem(i.GetCurrentItemIndex() + 1)
}

func (i *TextInventory) handleInput(event *tcell.EventKey) *tcell.EventKey {
	if i.closeHandler != nil && event.Key() == tcell.KeyEscape {
		i.Close()
		return nil
	}

	if event.Key() == tcell.KeyUp {
		i.previousItem()
		return nil
	} else if event.Key() == tcell.KeyDown {
		i.nextItem()
		return nil
	}

	if event.Key() == tcell.KeyEnter {
		currentIndex := i.GetCurrentItemIndex()
		if i.defaultSelection != nil && currentIndex >= 0 && currentIndex < len(i.items) {
			if i.closeOnSelect {
				i.Close()
			}
			i.defaultSelection(i.items[currentIndex])
			return nil
		}
	}

	if event.Key() == tcell.KeyRune && event.Rune() == ' ' {
		currentIndex := i.GetCurrentItemIndex()
		if i.contextMenu != nil && currentIndex >= 0 && currentIndex < len(i.items) {
			i.contextMenu(i.items[currentIndex], func() {
				i.updateListItems()
			})
			return nil
		}
	}

	event = parseControlCodes(event)

	runeReceived := event.Rune()

	modCtrl := event.Modifiers() == tcell.ModAlt || event.Modifiers() == tcell.ModCtrl || event.Modifiers() == tcell.ModMeta

	if modCtrl && runeReceived < 97 { // 1 == a, 2 == b, etc
		runeReceived = runeReceived + 96
	}

	modShift := unicode.IsUpper(runeReceived)

	if modShift {
		runeReceived = unicode.ToLower(runeReceived)
	}

	for _, invItem := range i.items {
		if runeReceived == invItem.Shortcut() {
			if modShift {
				if i.shiftSelection != nil {
					if i.closeOnShiftSelect {
						i.Close()
					}
					i.shiftSelection(invItem)
				}
			} else if modCtrl {
				if i.controlSelection != nil {
					if i.closeOnControlSelect {
						i.Close()
					}
					i.controlSelection(invItem)
				}
			} else if i.defaultSelection != nil {
				if i.closeOnSelect {
					i.Close()
				}
				i.defaultSelection(invItem)
			}
			return nil
		}
	}
	return event
}

func parseControlCodes(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() >= tcell.KeyCtrlA && event.Key() <= tcell.KeyCtrlZ {
		return tcell.NewEventKey(tcell.KeyRune, rune('a'+event.Key()-tcell.KeyCtrlA), tcell.ModCtrl)
	}
	return event
}

func (i *TextInventory) SetCloseOnSelection(value bool) {
	i.closeOnSelect = value
}

func (i *TextInventory) SetCloseOnShiftSelection(value bool) {
	i.closeOnShiftSelect = value
}

func (i *TextInventory) SetCloseOnControlSelection(value bool) {
	i.closeOnControlSelect = value
}

func (i *TextInventory) SetSelectionMode() {
	i.selectionOnly = true
}

func (i *TextInventory) Close() {
	if i.closeHandler != nil {
		i.closeHandler()
		i.closeHandler = nil
	}
	if i.afterClose != nil {
		i.afterClose()
	}
}

func (i *TextInventory) SetAfterClose(afterClose func()) {
	i.afterClose = afterClose
}
