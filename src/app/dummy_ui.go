package app

import (
	"image"
	"image/color"
	"log/slog"
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/util"
)

type DummyUI struct {
}

func (d DummyUI) FadeToBlack() {

}

func (d DummyUI) FadeFromBlack() {

}

func (d DummyUI) Confirm(message string, onConfirm func()) {
	onConfirm()
}

func (d DummyUI) SetDebugActions(actions func() common.DebugActions) {

}

func (d DummyUI) ToggleMapInfo()                   {}
func (d DummyUI) FlashScreen(color.Color, float64) {}
func (d DummyUI) ToggleFogOfWar()                  {}
func (d DummyUI) CycleRenderingMode()              {}
func (d DummyUI) SetFont(string)                   {}

func (d DummyUI) GetVisibleMap() geometry.Rect {
	//TODO implement me
	return geometry.NewRect(0, 0, 0, 0)
}

func (d DummyUI) IsPlayerForcingAggression() bool {
	//TODO implement me
	return false
}

func (d DummyUI) HasOverlays() bool {
	//TODO implement me
	return true
}

func (d DummyUI) ShowLogMessage(message string) {
	//TODO implement me
	slog.Debug("log message", "text", message)
}

func (d DummyUI) Cancel() {
	//TODO implement me

}

func (d DummyUI) PopAll() {
	//TODO implement me

}

func (d DummyUI) QuitGame() {}

func (d DummyUI) ShowGameOverMenu() {
	//TODO implement me

}

func (d DummyUI) OpenMenu(options []util.MenuItem) {
	//TODO implement me

}

func (d DummyUI) OpenUncancelableMenu(options []util.MenuItem) {
	//TODO implement me

}

func (d DummyUI) OpenSettingsMenu(back func()) {
	//TODO implement me

}

func (d DummyUI) ShowItemInfo(item *common.Item) {}

func (d DummyUI) OpenInfoWindow(title string, sections []common.SheetSection) {}

// OpenSaveSlots can't ask anyone, so it answers with the slot a player asked to replace one would most likely
// pick: the first free one, or the least recently played game. Headless runs never stall on the question.
func (d DummyUI) OpenSaveSlots(title string, slots []common.SaveSlot, pick func(slot int), back func()) {
	choice := common.SaveSlot{}
	for _, slot := range slots {
		if !slot.Used {
			pick(slot.Slot)
			return
		}
		if choice.Slot == 0 || slot.SavedAt.Before(choice.SavedAt) {
			choice = slot
		}
	}
	if choice.Slot != 0 {
		pick(choice.Slot)
	}
}

func (d DummyUI) CaptureThumbnail() image.Image { return nil }

func (d DummyUI) OpenTextWindow(msg []util.LabelText) {
	//TODO implement me

}

func (d DummyUI) CancelTargeting() {
	//TODO implement me

}

func (d DummyUI) ShowFullStats() {
	//TODO implement me

}

func (d DummyUI) UpdateStatsLabel() {
	//TODO implement me

}

func (d DummyUI) OnMapChanged() {
	//TODO implement me

}

func (d DummyUI) OpenNewGameScreen(start func(class defs.LoadoutDefinition, permadeath bool, name string), back func()) {
	//TODO implement me

}

func (d DummyUI) BeginTargeting(rules common.TargetRules, onHit *common.HitEffect) {
	//TODO implement me

}

func (d DummyUI) OpenShopMenu(npc *common.Actor) {
	//TODO implement me

}

func (d DummyUI) OpenShopMenuFiltered(npc *common.Actor, filter func(*common.Item) bool) {
	//TODO implement me

}

func (d DummyUI) BeginPlayerSellingTo(vendor *common.Actor) {
	//TODO implement me

}

func (d DummyUI) OpenItemList(items []*common.TextualStack) {}

func (d DummyUI) OpenServiceMenu(items []*common.Item, price func(*common.Item) int, pick func(*common.Item)) {
}

func (d DummyUI) OpenListInventoryForSelection(filter func(element *common.Item) bool, selected func(itemKey rune)) {
	//TODO implement me

}

func (d DummyUI) UpdateMiniMap() {
	//TODO implement me

}

func (d DummyUI) RefreshIconBars() {}

func (d DummyUI) OnPlayerTurnTaken() {
	//TODO implement me

}

func (d DummyUI) SetPlayerMessage(message func(message string)) {
	//TODO implement me

}

func NewDummyUI() *DummyUI {
	return &DummyUI{}
}

func (d DummyUI) OpenPlayerStashInTown() {}

func (d DummyUI) OpenTransferEnchantmentMenu() {}
