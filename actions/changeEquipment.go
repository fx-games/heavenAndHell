package actions

import (
	"fmt"
	"rpg/common"
)

type ChangeEquipmentAction struct {
	BaseAction
	actor *common.Actor
	mode  EquipmentChangeMode
	item  GameItem
}

type EquipmentChangeMode bool

const (
	EquipmentChangeModeEquip   EquipmentChangeMode = true
	EquipmentChangeModeUnequip EquipmentChangeMode = false
)

func NewEquipItemAction(source Cause, actor *common.Actor, item GameItem) *ChangeEquipmentAction {
	return &ChangeEquipmentAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameChangeEquipment,
		},
		actor: actor,
		mode:  EquipmentChangeModeEquip,
		item:  item,
	}
}

func NewUnequipItemAction(source Cause, actor *common.Actor, item GameItem) *ChangeEquipmentAction {
	return &ChangeEquipmentAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameChangeEquipment,
		},
		actor: actor,
		mode:  EquipmentChangeModeUnequip,
		item:  item,
	}
}
func (d *ChangeEquipmentAction) Execute(engine Engine) []Action {

	actor := d.actor
	equipment := actor.GetEquipment()
	item := d.item
	var message string

	if d.mode == EquipmentChangeModeEquip {
		equipment.Equip(item)

		name := actor.Name()
		verb := "equips"
		if engine.GetWorldState().GetPlayer() == actor {
			name = "You"
			verb = "equip"
		}
		message = fmt.Sprintf("%s %s %s", name, verb, item.Name())
	} else if d.mode == EquipmentChangeModeUnequip {
		equipment.UnEquip(item)

		name := actor.Name()
		verb := "unequips"
		if engine.GetWorldState().GetPlayer() == actor {
			name = "You"
			verb = "unequip"
		}
		message = fmt.Sprintf("%s %s %s", name, verb, item.Name())
	}

	engine.Log(message)

	return nil
}

func (d *ChangeEquipmentAction) GetAnimations() []common.Animation {
	return nil
}

func (d *ChangeEquipmentAction) GetMode() EquipmentChangeMode {
	return d.mode
}

func (d *ChangeEquipmentAction) GetItem() GameItem {
	return d.item
}

func (d *ChangeEquipmentAction) GetActor() *common.Actor {
	return d.actor
}
