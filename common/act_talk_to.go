package common

import (
	"fmt"
	"rpg/stats"
	"rpg/util"
)

type TalkToAction struct {
	BaseAction
	npc *Actor
}

func NewTalkToAction(source Cause, npc *Actor) *TalkToAction {
	return &TalkToAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameTalkTo,
		},
		npc: npc,
	}
}
func (d *TalkToAction) Execute(context ActionContext) []Action {
	state := context.GameState
	gui := context.UserInterface()
	npc := d.npc
	vendorType := npc.GetVendorType()
	msg := context.Msg

	if vendorType == VendorTypeWands {
		state.GetPlayer().HealCompletely()
		msg(fmt.Sprintf("You feel refreshed"))
	}

	options := getVendorOptions(context, npc)
	if len(options) == 0 {
		return nil
	}

	gui.OpenMenu(options)
	return nil
}

func (d *TalkToAction) GetAnimations() []Animation {
	return nil
}

func getVendorOptions(context ActionContext, vendor *Actor) []util.MenuItem {
	vendorType := vendor.GetVendorType()
	var menuItems []util.MenuItem

	switch vendorType {
	case VendorTypeWands:
		menuItems = getWandVendorOptions(context, vendor)
	case VendorTypeWeapons:
		menuItems = getWeaponVendorOptions(context, vendor)
	case VendorTypeArmor:
		menuItems = getArmorVendorOptions(context, vendor)
	case VendorTypeGeneral:
		menuItems = getGeneralVendorOptions(context, vendor)
	}
	return menuItems
}
func getWandVendorOptions(context ActionContext, vendor *Actor) []util.MenuItem {
	gui := context.UserInterface()
	state := context.GameState
	vendorOptions := []util.MenuItem{
		{
			MainText: "BUY",
			Action: func() {
				gui.OpenShopMenu(vendor)
			},
		},
		{
			MainText: "TRANSFER",
			Action:   gui.OpenTransferEnchantmentMenu,
		},
	}

	if state.IsStoryModeEnabled() {
		return append(getNPCTalkOption(context, vendor), vendorOptions...)
	} else {
		return vendorOptions
	}
}

func getGeneralVendorOptions(context ActionContext, vendor *Actor) []util.MenuItem {
	state := context.GameState
	gui := context.UserInterface()
	vendorOptions := []util.MenuItem{
		{
			MainText: "BUY",
			Action: func() {
				gui.OpenShopMenu(vendor)
			},
		},
		{
			MainText: "SELL",
			Action: func() {
				gui.BeginPlayerSellingTo(vendor)
			},
		},
	}

	if state.IsStoryModeEnabled() {
		return append(getNPCTalkOption(context, vendor), vendorOptions...)
	} else {
		return vendorOptions
	}
}

func getWeaponVendorOptions(context ActionContext, vendor *Actor) []util.MenuItem {
	state := context.GameState
	gui := context.UserInterface()
	vendorOptions := []util.MenuItem{
		{
			MainText: "BUY",
			Action: func() {
				gui.OpenShopMenu(vendor)
			},
		},
		{
			MainText: "REPAIR",
			Action: func() {
				//gui.BeginPlayerRepairing(vendor)
				//TODO
				player := state.GetPlayer()
				player.GetInventory().RepairAll()
				player.GetStats().DecrementBy(stats.Gold, 10)
			},
		},
	}

	if state.IsStoryModeEnabled() {
		return append(getNPCTalkOption(context, vendor), vendorOptions...)
	} else {
		return vendorOptions
	}
}

func getArmorVendorOptions(context ActionContext, vendor *Actor) []util.MenuItem {
	state := context.GameState
	gui := context.UserInterface()
	vendorOptions := []util.MenuItem{
		{
			MainText: "BUY",
			Action: func() {
				gui.OpenShopMenu(vendor)
			},
		},
		{
			MainText: "STASH",
			Action: func() {
				gui.OpenPlayerStashInTown()
			},
		},
	}

	if state.IsStoryModeEnabled() {
		return append(getNPCTalkOption(context, vendor), vendorOptions...)
	} else {
		return vendorOptions
	}
}

func getNPCTalkOption(context ActionContext, vendor *Actor) []util.MenuItem {
	state := context.GameState
	gui := context.UserInterface()
	talkOption := []util.MenuItem{
		{
			MainText: "TALK",
			Action: func() {
				textForNPC, onShowFunc := state.GetTextForNamedNPC(vendor)
				gui.OpenTextWindow(textForNPC)
				if onShowFunc != nil {
					onShowFunc()
				}
			},
		},
	}
	return talkOption
}
