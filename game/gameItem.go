package game

import (
	"fmt"
	"rpg/actions"
	"rpg/common"
	"rpg/stats"
	"rpg/util"
)

type GameItem struct {
	*common.BaseItem
	forcedTooltip     []util.LabelText
	itemHooks         []actions.ItemActionHook
	targetedAction    *common.TargetedAction
	untargetedAction  *common.UntargetedAction
	actionCharges     int
	destroyAfterUse   bool
	nameOnlyInTooltip bool
}



func (i *GameItem) SetCharges(charges int) {
	i.actionCharges = charges
}

func (i *GameItem) SetUnlimitedCharges() {
	i.actionCharges = -1
}
func (i *GameItem) GetCharges() int {
	return i.actionCharges
}

func (i *GameItem) HasAction() bool {
	return i.targetedAction != nil || i.untargetedAction != nil
}

func (i *GameItem) GetTargetedAction() *common.TargetedAction {
	return i.targetedAction
}
func (i *GameItem) GetUntargetedAction() *common.UntargetedAction {
	return i.untargetedAction
}
func (i *GameItem) GetItemHooks() []actions.ItemActionHook {
	return i.itemHooks
}
func (i *GameItem) GetTooltipLines() []util.LabelText {
	info := []util.LabelText{common.LabelTextFromString(i.Name())}

	if i.nameOnlyInTooltip {
		return info
	}

	if len(i.forcedTooltip) > 0 {
		return append(info, i.forcedTooltip...)
	}

	if i.IsWeapon() {
		handedNess := "One-Handed"
		if i.SlotName() == common.SlotNameTwoHandedWeapon {
			handedNess = "Two-Handed"
		}
		mrType := "melee"
		useText := []util.LabelText{common.LabelTextFromString("Move into an enemy to attack")}
		if i.IsRangedWeapon() {
			mrType = "ranged"
			useText = []util.LabelText{common.LabelTextFromString("Press \"f\" to aim and fire"), common.LabelTextFromString("Press \"q\" for quickfire")}
		}
		info = append(info, common.LabelTextFromString(fmt.Sprintf("%s %s weapon", handedNess, mrType)))
		info = append(info, common.LabelTextFromString(""))
		info = append(info, useText...)
		info = append(info, common.LabelTextFromString(""))
		info = append(info, i.GetWeaponStatsAsString()...)
	}

	if i.IsArmor() {
		info = append(info, i.GetArmorStatsAsString()...)
	}

	if len(i.GetEquippedModifiers()) > 0 {
		for _, m := range i.GetEquippedModifiers() {
			info = append(info, common.LabelTextFromString(fmt.Sprintf("%s", m.ToString())))
		}
	}
	if len(i.GetItemHooks()) > 0 {
		info = append(info, common.LabelTextFromString(""), common.LabelTextFromString("Effects"))
		for _, h := range i.GetItemHooks() {
			info = append(info, common.LabelTextFromString(fmt.Sprintf("%s", h.ToString())))
		}
	}
	if i.HasAction() { // wands

		if i.HasInfiniteCharges() {
			//info = append(info, common.LabelTextFromString(fmt.Sprintf("Has infinite charges")))
			if i.targetedAction != nil {
				info = append(info, common.LabelTextFromString(fmt.Sprintf("%s", i.targetedAction.Name())))
				info = append(info, common.LabelTextFromString(fmt.Sprintf("%s", i.targetedAction.GetDescription())))
			}
			if i.untargetedAction != nil {
				info = append(info, common.LabelTextFromString(fmt.Sprintf("%s", i.untargetedAction.Name())))
				info = append(info, common.LabelTextFromString(fmt.Sprintf("%s", i.untargetedAction.GetDescription())))
			}
		} else {
			if i.SelfDestructs() {
				info = append(info, common.LabelTextFromString(fmt.Sprintf("Is destroyed after use")))
			}

			if i.targetedAction != nil {
				info = append(info, common.LabelTextFromString(fmt.Sprintf("%s (x%d)", i.targetedAction.Name(), i.GetCharges())))
				info = append(info, common.LabelTextFromString(fmt.Sprintf("%s", i.targetedAction.GetDescription())))
			}
			if i.untargetedAction != nil {
				info = append(info, common.LabelTextFromString(fmt.Sprintf("%s (x%d)", i.untargetedAction.Name(), i.GetCharges())))
				info = append(info, common.LabelTextFromString(fmt.Sprintf("%s", i.untargetedAction.GetDescription())))
			}
		}

	}
	return info
}
func (i *GameItem) GetAllEnchantments() []Enchantment {
	var result []Enchantment
	for _, m := range i.GetEquippedModifiers() {
		result = append(result, m)
	}
	for _, h := range i.GetItemHooks() {
		result = append(result, h)
	}
	return result
}

func (i *GameItem) RemoveEnchantment(enchantment Enchantment) {
	switch e := enchantment.(type) {
	case *stats.Modifier:
		i.RemoveEquippedModifier(e)
	case actions.ItemActionHook:
		i.RemoveItemHook(e)
	}
}

func (i *GameItem) AddEnchantment(enchantment Enchantment) {
	switch e := enchantment.(type) {
	case *stats.Modifier:
		i.BaseItem.AddEquippedModifier(e)
	case actions.ItemActionHook:
		i.AddItemhook(e)
	}
}

func (i *GameItem) RemoveItemHook(hookToRemove actions.ItemActionHook) {
	for index, hook := range i.itemHooks {
		if hook == hookToRemove {
			i.itemHooks = append(i.itemHooks[:index], i.itemHooks[index+1:]...)
			return
		}
	}
}

func (i *GameItem) AddItemhook(hookToAdd actions.ItemActionHook) {
	i.itemHooks = append(i.itemHooks, hookToAdd)
}

func (i *GameItem) IsEquippable() bool {
	return i.SlotName() != common.SlotNameNotEquippable
}

func (i *GameItem) HasTargetedAction() bool {
	return i.targetedAction != nil
}

func (i *GameItem) HasUntargetedAction() bool {
	return i.untargetedAction != nil
}

func (i *GameItem) UseCharge() {
	if i.actionCharges > 0 {
		i.actionCharges--
	}
}

func (i *GameItem) HasInfiniteCharges() bool {
	return i.actionCharges == -1
}

func (i *GameItem) SelfDestructs() bool {
	return i.destroyAfterUse
}

func (i *GameItem) SetItemHooks(hooks []actions.ItemActionHook) {
	i.itemHooks = hooks
}

func (i *GameItem) ShowNameOnlyInTooltip() {
	i.nameOnlyInTooltip = true
}
func (i *GameItem) SetTooltip(forcedTooltip []util.LabelText) {
	i.forcedTooltip = forcedTooltip
}

func (i *GameItem) HasChargesLeft() bool {
	return i.actionCharges != 0
}


func flatFunc(i int) func() int {
	return func() int {
		return i
	}
}
