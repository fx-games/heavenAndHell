package gui

import (
	"fmt"
	"rpg/common"
	"rpg/util"
)

func (u *UIController) OpenTransferEnchantmentMenu() {
	// step one, choose an item
	state := u.context.GameState()
	player := state.GetPlayer()
	playerInventory := player.GetInventory()
	msg := u.msg
	if playerInventory.IsEmpty() {
		msg(fmt.Sprintf("You have no items"))
		return
	}
	itemsWithEnchantments := make([]*common.Item, 0)
	for _, item := range playerInventory.Items() {
		gItem := item
		if len(gItem.GetAllEnchantments()) > 0 {
			itemsWithEnchantments = append(itemsWithEnchantments, item)
		}
	}
	if len(itemsWithEnchantments) == 0 {
		msg(fmt.Sprintf("You have no items with enchantments"))
		return
	}
	var menuItems []util.MenuItem
	for _, item := range itemsWithEnchantments {
		gItem := item
		menuItems = append(menuItems, util.MenuItem{
			MainText: gItem.Name(),
			Action: func() {
				u.openChooseEnchantmentMenu(gItem)
			},
		})
	}

	u.OpenMenu(menuItems)
}

func (u *UIController) openChooseEnchantmentMenu(source *common.Item) {
	var menuItems []util.MenuItem
	for _, e := range source.GetAllEnchantments() {
		enchantment := e
		menuItems = append(menuItems, util.MenuItem{
			MainText: enchantment.ToString(),
			Action: func() {
				u.openChooseTransferTargetMenu(source, enchantment)
			},
		})
	}

	u.OpenMenu(menuItems)
}

func (u *UIController) openChooseTransferTargetMenu(source *common.Item, enchantment common.Enchantment) {
	state := u.context.GameState()
	player := state.GetPlayer()
	playerInventory := player.GetInventory()
	if playerInventory.IsEmpty() {
		return
	}
	var menuItems []util.MenuItem
	for _, item := range playerInventory.Items() {
		gItem := item
		if gItem == source {
			continue
		}

		menuItems = append(menuItems, util.MenuItem{
			MainText: gItem.Name(),
			Action: func() {
				u.transferEnchantment(source, gItem, enchantment)
			},
		})
	}

	u.OpenMenu(menuItems)
}

func (u *UIController) transferEnchantment(source *common.Item, item *common.Item, enchantment common.Enchantment) {
	msg := u.msg
	source.RemoveEnchantment(enchantment)
	msg(fmt.Sprintf("%s lost %s", source.Name(), enchantment.ToString()))
	item.AddEnchantment(enchantment)
	msg(fmt.Sprintf("%s gained %s", item.Name(), enchantment.ToString()))
	u.PopAll()
}
