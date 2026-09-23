package game

import (
	"fmt"

	"rpg/src/common"
	"rpg/src/util"
)

// show turns what the rules asked the player to see or choose into this frontend's windows.
// A game without a frontend (some tests) shows nothing.
func show(ui common.UserInterface, r common.UIRequest) {
	if ui == nil {
		return
	}
	switch r := r.(type) {
	case common.ChooseOption:
		ui.OpenMenu(r.Options)
	case common.ChooseFromList:
		ui.OpenItemList(r.List)
	case common.ChooseInventoryItem:
		ui.OpenListInventoryForSelection(r.Filter, r.Picked)
	case common.CloseMenus:
		ui.PopAll()
	case common.ShowStats:
		ui.ShowFullStats()
	case common.ShowGameOver:
		ui.ShowGameOverMenu()
	case common.EndTargeting:
		ui.CancelTargeting()
	case common.ChooseService:
		ui.OpenServiceMenu(r.Items, r.Price, r.Pick)
	case common.OpenShop:
		if r.Filter == nil {
			ui.OpenShopMenu(r.Vendor)
		} else {
			ui.OpenShopMenuFiltered(r.Vendor, r.Filter)
		}
	case common.OpenSelling:
		ui.BeginPlayerSellingTo(r.Vendor)
	case common.ShowText:
		ui.OpenTextWindow(util.ToLabelText(r.Lines))
	case common.ShowItem:
		ui.ShowItemInfo(r.Item)
	case common.OpenStash:
		ui.OpenPlayerStashInTown()
	case common.OpenEnchantmentTransfer:
		ui.OpenTransferEnchantmentMenu()
	default:
		panic(fmt.Sprintf("show: unhandled %T", r)) // a new request type without a window is a programming error
	}
}
