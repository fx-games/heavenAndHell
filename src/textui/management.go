package textui

import (
	"github.com/memmaker/go/cview"
)

func (u *UI) popPanel(panelName string) {
	u.pages.RemovePanel(panelName)
	delete(u.modals, panelName)
	u.popFocus()
}

func (u *UI) popFocus() {
	if len(u.focusStack) == 0 {
		u.resetFocusToMain()
		return
	}
	popped := u.focusStack[len(u.focusStack)-1]
	u.focusStack = u.focusStack[:len(u.focusStack)-1]
	u.application.SetBeforeFocusFunc(nil)
	u.application.SetFocus(popped.Primitive)
	u.application.SetBeforeFocusFunc(popped.BeforeFocus)
}

func (u *UI) pushFocus() {
	focus := u.application.GetFocus()
	if focus == nil {
		return
	}
	for _, f := range u.focusStack {
		if f.Primitive == focus {
			return
		}
	}

	u.focusStack = append(u.focusStack, FocusInfo{
		Primitive:   focus,
		BeforeFocus: u.application.GetBeforeFocusFunc(),
	})
}

func (u *UI) defaultFocusHandler(p cview.Primitive) bool {
	return p == u.mainGrid || p == u.mapWindow
}

func (u *UI) makeCenteredModal(panelName string, modal InputPrimitive, w, h int) {
	w = w + 2
	h = h + 2
	screenW, screenH := u.screenSize()
	if h > screenH {
		h = screenH
		w = w + 1 // scrollbar
	}
	if w > screenW {
		w = screenW
	}
	x, y := (screenW-w)/2, (screenH-h)/2
	modal.SetRect(x, y, w, h)
	u.addModal(panelName, modal)
	u.lockFocusToPrimitive(modal)
}

func (u *UI) addModal(panelName string, p cview.Primitive) {
	if u.modals[panelName] { // replacing an open panel of the same name keeps its focus entry
		u.pages.RemovePanel(panelName)
		u.popFocus()
	}
	u.pages.AddPanel(panelName, p, false, true)
	u.modals[panelName] = true
}

// CloseAllModals removes every panel over the main screen.
func (u *UI) CloseAllModals() {
	for name := range u.modals {
		u.pages.RemovePanel(name)
	}
	clear(u.modals)
	u.resetFocusToMain()
}

func (u *UI) resetFocusToMain() {
	u.focusStack = nil
	u.application.SetBeforeFocusFunc(nil)
	u.application.SetFocus(u.mainGrid)
	u.pages.ShowPanel("main")
	u.application.SetBeforeFocusFunc(u.defaultFocusHandler)
}

func (u *UI) lockFocusToPrimitive(p cview.Primitive) {
	u.pushFocus()
	u.application.SetBeforeFocusFunc(nil)
	u.application.SetFocus(p)
	u.application.SetBeforeFocusFunc(func(p cview.Primitive) bool { return false })
}

func (u *UI) applyStylingToUI() {
	u.uiTheme.SetBorders(&cview.Borders)
	u.uiTheme.SetStyles(&cview.Styles)

	fg := u.uiTheme.GetUIColorForTcell(UIColorUIForeground)
	bg := u.uiTheme.GetUIColorForTcell(UIColorUIBackground)

	u.statusBar.SetBackgroundColor(u.uiTheme.GetUIColorForTcell(UIColorStatusBarBackground))
	u.statusBar.SetTextColor(u.uiTheme.GetUIColorForTcell(UIColorStatusBarForeground))

	u.messageLabel.SetTextColor(fg)
	u.messageLabel.SetBackgroundColor(bg)
	u.messageLabel.SetBorderColor(fg)
	u.messageLabel.SetScrollBarColor(fg)
	u.messageLabel.SetDynamicColors(true)

	u.rightPanel.SetTextColor(fg)
	u.rightPanel.SetBorderColor(fg)
	u.rightPanel.SetBackgroundColor(bg)
	u.rightPanel.SetDynamicColors(true)
	u.rightPanel.SetTextAlign(cview.AlignLeft)

	u.lowerRightPanel.SetTextColor(fg)
	u.lowerRightPanel.SetBorderColor(fg)
	u.lowerRightPanel.SetBackgroundColor(bg)
	u.lowerRightPanel.SetDynamicColors(true)
	u.lowerRightPanel.SetTextAlign(cview.AlignLeft)

	u.mapOverlay.SetDefaultColors(tcellColorToRGBA(bg), tcellColorToRGBA(fg))
}

func (u *UI) applyListStyle(list *cview.List) {
	fg := u.uiTheme.GetUIColorForTcell(UIColorUIForeground)
	bg := u.uiTheme.GetUIColorForTcell(UIColorUIBackground)

	borderFgFocus := u.uiTheme.GetUIColorForTcell(UIColorBorderForegroundFocus)

	list.SetBorder(true)
	list.SetWrapAround(true)
	list.SetHover(true)
	list.ShowSecondaryText(false)

	list.SetScrollBarColor(fg)

	list.SetTitleColor(fg)
	list.SetMainTextColor(fg)
	list.SetSecondaryTextColor(fg)

	list.SetBorderColor(fg)
	list.SetBorderColorFocused(borderFgFocus)

	list.SetBackgroundColor(bg)

	list.SetShortcutColor(fg)

	list.SetSelectedTextColor(bg)
	list.SetSelectedBackgroundColor(fg)
	list.SetForceSelectedTextColor(true)
}
