package textui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
	"github.com/memmaker/go/textiles"
)

type MenuItem struct {
	Name       string
	Action     func()
	CloseMenus bool
}

// ShortCutFromIndex is a-z, then A-Z.
func ShortCutFromIndex(itemIndex int) rune {
	shortcut := rune(97 + itemIndex)
	if itemIndex > 25 {
		shortcut = rune(39 + itemIndex)
	}
	return shortcut
}

// Groundwork

func (u *UI) OpenMenuWithTitle(title string, actions []MenuItem) *cview.List {
	return u.OpenMenuWithTitleAndClose(title, actions, nil)
}

func (u *UI) OpenMenuWithTitleAndClose(title string, actions []MenuItem, onClose func()) *cview.List {
	menu := u.openSimpleMenu(actions, onClose)
	u.setModalTitle(menu, title)
	return menu
}

// setModalTitle sets a centered modal's title and widens the modal to fit it.
func (u *UI) setModalTitle(modal cview.Primitive, title string) {
	modal.(interface{ SetTitle(string) }).SetTitle(title)
	x, y, w, h := modal.GetRect()
	if needed := cview.TaggedStringWidth(title) + 4; needed > w {
		screenW, _ := u.screenSize()
		w = min(needed, screenW)
		x = (screenW - w) / 2
		modal.SetRect(x, y, w, h)
	}
}

func (u *UI) OpenMenu(actions []MenuItem) *cview.List {
	return u.openSimpleMenu(actions, nil)
}

// OpenMenuNotClosable is a menu that ESC and right clicks don't close.
func (u *UI) OpenMenuNotClosable(title string, actions []MenuItem) *cview.List {
	list := u.openSimpleMenuNotClosable("menu", actions)
	list.SetInputCapture(u.directionalWrapperWithoutAlphabet(nil))
	u.setModalTitle(list, title)
	return list
}

func (u *UI) openSimpleMenu(menuItems []MenuItem, onClose func()) *cview.List {
	panelName := "menu"
	list := u.openSimpleMenuNotClosable(panelName, menuItems)
	originalCapture := list.GetInputCapture()
	list.SetInputCapture(u.directionalWrapperWithoutAlphabet(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			u.popPanel(panelName)
			if onClose != nil {
				onClose()
			}
			return nil
		}
		if originalCapture != nil {
			return originalCapture(event)
		}
		return event
	}))
	list.SetMouseCapture(func(action cview.MouseAction, event *tcell.EventMouse) (cview.MouseAction, *tcell.EventMouse) {
		if action == cview.MouseRightClick {
			u.popPanel(panelName)
			if onClose != nil {
				onClose()
			}
			return action, nil
		}
		return action, event
	})
	return list
}

func (u *UI) openSimpleMenuNotClosable(panelName string, menuItems []MenuItem) *cview.List {
	list, longestItem := u.createSimpleMenu(panelName, menuItems)

	u.makeCenteredModal(panelName, list, longestItem, len(menuItems))

	return list
}

func (u *UI) createSimpleMenu(panelName string, menuItems []MenuItem) (*cview.List, int) {
	list := cview.NewList()
	u.applyListStyle(list)
	list.SetSelectedFunc(func(index int, listItem *cview.ListItem) {
		item := menuItems[index]
		if item.CloseMenus {
			u.popPanel(panelName)
		}
		if item.Action != nil {
			item.Action()
		}
	})

	longestItem := setListItemsFromMenuItems(list, menuItems)
	return list, longestItem
}

func setListItemsFromMenuItems(list *cview.List, menuItems []MenuItem) int {
	list.Clear()
	longestItem := 0
	for index, a := range menuItems {
		action := a
		name := action.Name
		if name == "" {
			name = " " // an empty item is a spacer; cview would draw a divider line across the border
		}
		listItem := cview.NewListItem(name)
		if action.Name != "" {
			listItem.SetShortcut(ShortCutFromIndex(index))
		}
		list.AddItem(listItem)
		itemLength := cview.TaggedStringWidth(action.Name) + 4
		longestItem = max(longestItem, itemLength)
	}
	return longestItem
}

// Dialogues

func (u *UI) AskForConfirmation(title, message string, choice func(didConfirm bool)) {
	u.pushFocus()

	modal := cview.NewModal()
	modal.SetText(message)
	modal.AddButtons([]string{"Cancel", "Confirm"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		u.pages.RemovePanel("confirm")
		delete(u.modals, "confirm")
		u.popFocus()
		if choice != nil {
			choice(buttonLabel == "Confirm")
		}
	})
	modal.GetFrame().SetTitle(title)
	u.addModal("confirm", modal)

	// force focus on the modal
	u.application.SetBeforeFocusFunc(nil)
	u.application.SetFocus(modal.GetForm())
	// deny any focus change
	u.application.SetBeforeFocusFunc(func(p cview.Primitive) bool {
		if p == modal || p == modal.GetForm() {
			return true
		}
		x, y, w, h := p.GetRect()
		return modal.InRect(x, y) && modal.InRect(x+w, y+h)
	})

	for i := 0; i < modal.GetForm().GetButtonCount(); i++ {
		button := modal.GetForm().GetButton(i)
		button.SetInputCapture(u.directionalWrapper(button.GetInputCapture()))
	}
}

func (u *UI) OpenChoiceDialogue(title string, msg string, buttons []string, done func(index int, label string)) {
	u.pushFocus()

	modal := cview.NewModal()
	if title != "" {
		modal.SetTitle(title)
	}
	modal.SetText(msg)
	modal.AddButtons(buttons)
	modal.SetDoneFunc(func(index int, label string) {
		u.pages.RemovePanel("choice")
		delete(u.modals, "choice")
		u.popFocus()
		done(index, label)
	})
	u.addModal("choice", modal)

	// force focus on the modal
	u.application.SetBeforeFocusFunc(nil)
	u.application.SetFocus(modal)
	// deny any focus change
	u.application.SetBeforeFocusFunc(func(p cview.Primitive) bool {
		if p == modal {
			return true
		}
		x, y, w, h := p.GetRect()
		return modal.InRect(x, y) && modal.InRect(x+w, y+h)
	})
	for i := 0; i < modal.GetForm().GetButtonCount(); i++ {
		button := modal.GetForm().GetButton(i)
		button.SetInputCapture(u.directionalWrapper(button.GetInputCapture()))
	}
}

func (u *UI) AskForString(prompt string, prefill string, result func(entered string)) {
	cview.AskForString(u.application, u.pages, prompt, prefill, func(entered string) {
		u.application.QueueUpdateDraw(func() { result(entered) }) // after the form closed and gave back the focus
	})
}

// Text windows

func (u *UI) OpenTextWindow(description string) *cview.TextView {
	return u.openTextModal(description)
}

func (u *UI) openTextModal(description string) *cview.TextView {
	panelName := "textModal"
	textView := u.newTextModal(description)
	w, h := widthAndHeightFromString(description)
	u.makeCenteredModal(panelName, textView, w, h)
	originalInputCapture := textView.GetInputCapture()
	textView.SetMouseCapture(func(action cview.MouseAction, event *tcell.EventMouse) (cview.MouseAction, *tcell.EventMouse) {
		if action == cview.MouseLeftClick || action == cview.MouseRightClick {
			u.popPanel(panelName)
			return cview.MouseLeftClick, nil
		}
		return action, event
	})
	textView.SetInputCapture(u.directionalWrapper(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter || event.Rune() == 'e' {
			u.popPanel(panelName)
			return nil
		}
		if originalInputCapture != nil {
			return originalInputCapture(event)
		}
		return event
	}))
	return textView
}

func (u *UI) newTextModal(description string) *cview.TextView {
	textView := cview.NewTextView()
	textView.SetWrap(true)
	textView.SetWrapWidth(0)
	textView.SetWordWrap(true)
	textView.SetBorder(true)

	textView.SetTextColor(u.uiTheme.GetUIColorForTcell(UIColorUIForeground))
	textView.SetBackgroundColor(u.uiTheme.GetUIColorForTcell(UIColorUIBackground))

	textView.SetBorderColor(u.uiTheme.GetUIColorForTcell(UIColorBorderForeground))

	u.setColoredText(textView, description)
	return textView
}

func (u *UI) ShowLog(logLines []string) {
	if len(logLines) == 0 {
		u.Print("No log entries")
		return
	}
	textView := u.openTextModal(strings.Join(logLines, "\n"))
	textView.ScrollToEnd()
	textView.SetTitle("Log Messages")
}

// Containers and vendors

// ShowGiveAndTakeContainer shows two item lists side by side; picking an item moves it to the other side.
// The pickup command takes everything from the right side.
func (u *UI) ShowGiveAndTakeContainer(leftName string, leftLabels []string, rightName string, rightLabels []string, transferToRight func(index int), transferToLeft func(index int), takeAll func()) {
	leftPanel := "leftModal"
	rightPanel := "rightModal"
	var leftMenuItems []MenuItem
	var rightMenuItems []MenuItem

	closeContainer := func() {
		u.pages.RemovePanel(leftPanel)
		u.pages.RemovePanel(rightPanel)
		delete(u.modals, leftPanel)
		delete(u.modals, rightPanel)
		u.popFocus()
	}

	for index, label := range leftLabels {
		leftMenuItems = append(leftMenuItems, MenuItem{
			Name: label,
			Action: func() {
				closeContainer()
				transferToRight(index)
			},
		})
	}
	for index, label := range rightLabels {
		rightMenuItems = append(rightMenuItems, MenuItem{
			Name: label,
			Action: func() {
				closeContainer()
				transferToLeft(index)
			},
		})
	}

	leftMenu, longestItemLeft := u.createSimpleMenu(leftPanel, leftMenuItems)
	longestItemLeft = max(longestItemLeft, len(leftName))
	leftWidth := longestItemLeft + 2

	leftMenu.SetTitle(leftName)
	leftMenu.SetSelectedFocusOnly(true)

	if len(rightLabels) > 0 {
		keyForTakeAll := u.GetKeysForCommandAsString(KeyLayerMain, "pickup")
		u.Print(fmt.Sprintf("Press %s to take all items", keyForTakeAll))
	}

	rightMenu, longestItemRight := u.createSimpleMenu(rightPanel, rightMenuItems)
	longestItemRight = max(longestItemRight, len(rightName))
	rightWidth := longestItemRight + 2
	rightMenu.SetTitle(rightName)
	rightMenu.ShowFocus(true)
	rightMenu.SetSelectedFocusOnly(true)

	screenWidth, screenHeight := u.screenSize()

	height := min(screenHeight-4, max(len(leftLabels)+2, len(rightLabels)+2))

	leftHasScrollbar := len(leftLabels) > height-2
	rightHasScrollbar := len(rightLabels) > height-2

	centerGap := 4
	equalWidth := max(leftWidth, rightWidth)
	if screenWidth <= (equalWidth*2)+centerGap+4 {
		centerGap = 0
	}
	leftWidth = equalWidth
	rightWidth = equalWidth

	if leftHasScrollbar {
		leftWidth += 2
	}

	if rightHasScrollbar {
		rightWidth += 2
	}

	halfCenterGap := centerGap / 2
	centerScreen := screenWidth / 2

	leftListStart := centerScreen - leftWidth - halfCenterGap
	rightListStart := leftListStart + leftWidth + centerGap
	leftMenu.ShowFocus(true)
	mouseCapture := func(action cview.MouseAction, event *tcell.EventMouse) (cview.MouseAction, *tcell.EventMouse) {
		if action == cview.MouseRightClick {
			if !leftMenu.InRect(event.Position()) && !rightMenu.InRect(event.Position()) {
				u.application.QueueUpdateDraw(closeContainer)
				return action, nil
			}
		}
		if action == cview.MouseMove {
			if leftMenu.InRect(event.Position()) {
				u.application.SetFocus(leftMenu)
			} else if rightMenu.InRect(event.Position()) {
				u.application.SetFocus(rightMenu)
			}
		}
		return action, event
	}
	inputCapture := func(other *cview.List) func(event *tcell.EventKey) *tcell.EventKey {
		return func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				closeContainer()
				return nil
			}
			command := u.getCommandForKey(toUIKey(event))
			if command == "pickup" && takeAll != nil && len(rightLabels) > 0 {
				closeContainer()
				takeAll()
				return nil
			} else if command == "west" || command == "east" {
				u.application.SetFocus(other)
				return nil
			} else if command == "wait" {
				closeContainer()
				return nil
			} else if command == "north" {
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			} else if command == "south" {
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			}
			return event
		}
	}
	leftMenu.SetMouseCapture(mouseCapture)
	leftMenu.SetInputCapture(inputCapture(rightMenu))
	rightMenu.SetMouseCapture(mouseCapture)
	rightMenu.SetInputCapture(inputCapture(leftMenu))

	// place both lists vertically centered, but side by side
	leftMenu.SetRect(leftListStart, 2, leftWidth, height)
	rightMenu.SetRect(rightListStart, 2, rightWidth, height)

	u.pages.AddPanel(leftPanel, leftMenu, false, true)
	u.pages.AddPanel(rightPanel, rightMenu, false, true)
	u.modals[leftPanel] = true
	u.modals[rightPanel] = true

	u.pushFocus()

	u.application.SetBeforeFocusFunc(nil)

	if len(rightLabels) > 0 {
		u.application.SetFocus(rightMenu)
	} else {
		u.application.SetFocus(leftMenu)
	}

	u.application.SetBeforeFocusFunc(func(p cview.Primitive) bool {
		return p == leftMenu || p == rightMenu
	})
}

type VendorItem struct {
	Name         string // with color tags
	Price        int
	TooExpensive bool
	Buy          func()
	Inspect      func()
}

// OpenVendorMenu lists items with right aligned prices; the examine command inspects the highlighted item.
func (u *UI) OpenVendorMenu(title string, itemsForSale []VendorItem, onClose func()) {
	var menuItems []MenuItem
	var tableRows []fxtools.TableRow
	for _, item := range itemsForSale {
		tableRows = append(tableRows, fxtools.NewTableRow(item.Name, fmt.Sprintf("%d gold", item.Price)))
	}
	rendered := fxtools.TableLayoutLastRight(tableRows)
	for index, line := range rendered {
		item := itemsForSale[index]
		if item.TooExpensive {
			line = fmt.Sprintf("%s%s[-]", textiles.RGBAToFgColorCode(u.uiTheme.GetColorByName("dark_gray_1")), string(cview.StripTags([]byte(line), true, true)))
		}
		menuItems = append(menuItems, MenuItem{
			Name: line,
			Action: func() {
				if item.TooExpensive {
					u.Print("You can't afford that.")
					return
				}
				item.Buy()
			},
			CloseMenus: !item.TooExpensive,
		})
	}
	menu := u.openSimpleMenu(menuItems, onClose)
	u.setModalTitle(menu, title)
	origCapture := menu.GetInputCapture()
	menu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := u.getCommandForKey(toUIKey(event))
		if command == "examine" {
			index := menu.GetCurrentItemIndex()
			if index >= 0 && index < len(itemsForSale) && itemsForSale[index].Inspect != nil {
				itemsForSale[index].Inspect()
				return nil
			}
		}
		if origCapture != nil {
			return origCapture(event)
		}
		return event
	})
}
