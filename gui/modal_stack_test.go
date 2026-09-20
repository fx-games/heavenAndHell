package gui

import (
	"rpg/common"
	"rpg/util"
	"testing"
)

// A choice that opens a window keeps its menu underneath, so closing the window goes back to it; a choice that
// opens nothing closes the whole chain; a menu that rebuilds itself via reopen doesn't stack a copy.
func TestModalStackGoesBack(t *testing.T) {
	u := &UIController{context: common.UIContext{GameState: func() *common.State { return &common.State{} }}}
	menu := func(items ...util.MenuItem) *BasicMenu {
		return &BasicMenu{uiController: u, items: items, closeOnSelect: true, cancelable: true}
	}
	var child *BasicMenu
	var refresh func()
	refresh = func() { u.PushWidget(menu(util.MenuItem{MainText: "toggle", Action: func() { u.reopen(refresh) }})) }
	parent := menu(util.MenuItem{MainText: "open", Action: func() {
		child = menu(util.MenuItem{MainText: "done"})
		u.PushWidget(child)
	}}, util.MenuItem{MainText: "settings", Action: refresh})
	u.PushWidget(parent)

	parent.OnCommand(PlayerCommandConfirm)
	if len(u.modals) != 2 || u.GetTopModal() != child {
		t.Fatalf("opening a child: stack %v", u.modals)
	}
	child.OnCommand(PlayerCommandCancel)
	if u.dropClosedWidgets() != parent || len(u.modals) != 1 {
		t.Fatalf("closing the child should go back to the parent: stack %v", u.modals)
	}

	parent.OnCommand(PlayerCommandConfirm)
	child.OnCommand(PlayerCommandConfirm) // "done" opens nothing
	if u.IsUIModalVisible() {
		t.Fatalf("a final choice should close the chain: stack %v", u.modals)
	}

	u.PushWidget(parent)
	parent.selectedIndex = 1
	parent.OnCommand(PlayerCommandConfirm)
	u.GetTopModal().OnCommand(PlayerCommandConfirm) // toggle rebuilds the settings menu in place
	if len(u.modals) != 2 {
		t.Fatalf("reopen should replace the menu, not stack it: %d widgets", len(u.modals))
	}

	// a slot screen stays under the overwrite confirmation it opens, and "No" comes back to it
	u.PopAll()
	u.PushWidget(parent)
	slots := &SaveSlotScreen{u: u, slots: []common.SaveSlot{{Slot: 1}}, back: func() {}}
	slots.pick = func(int) {
		u.PushWidget(menu(util.MenuItem{MainText: "Yes"}, util.MenuItem{MainText: "No", Action: u.PopWidget}))
	}
	u.PushWidget(slots)
	slots.choose(0)
	confirm := u.GetTopModal().(*BasicMenu)
	confirm.selectedIndex = 1
	confirm.OnCommand(PlayerCommandConfirm)
	if u.dropClosedWidgets() != slots || len(u.modals) != 2 {
		t.Fatalf("\"No\" should go back to the slot screen: stack %v", u.modals)
	}
	slots.choose(0)
	u.GetTopModal().OnCommand(PlayerCommandConfirm) // "Yes" writes and ends the chain
	if u.dropClosedWidgets() != nil {
		t.Fatalf("\"Yes\" should close everything: stack %v", u.modals)
	}
}
