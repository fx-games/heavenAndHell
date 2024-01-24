package actions

import "fmt"

type OnDropHook struct {
	ItemHook
	hookDescription string
}

func (d OnDropHook) ToString() string {
	return fmt.Sprintf("On Drop: %s", d.hookDescription)
}

func (d OnDropHook) Name() string {
	return "OnDropHook"
}

func NewMainOnDropHook() *OnDropHook {
	return &OnDropHook{
		ItemHook: ItemHook{
			BaseActionHook: BaseActionHook{
				hookedAction: ActionNameDrop,
			},
			beforeHook: func(itemContext *ItemContext, action Action, engine Engine) []Action {
				dropAction := action.(*DropAction)
				dropper := dropAction.GetActor()
				droppedItem := dropAction.GetItem()
				droppedItem.Show()
				if dropper.GetEquipment().IsEquipped(droppedItem) {
					gameItem := droppedItem.(GameItem)
					unequipAction := NewUnequipItemAction(action.Cause(), dropper, gameItem)
					return []Action{unequipAction}
				}
				return nil
			},
		},
		hookDescription: "Unequip if equipped",
	}
}

func NewOnDropHook(hookDescription string, afterHook func(*ItemContext, Action, Engine) []Action) *OnDropHook {
	return &OnDropHook{
		ItemHook: ItemHook{
			BaseActionHook: BaseActionHook{
				hookedAction: ActionNameRangedAttack,
			},
			afterHook: afterHook,
		},
		hookDescription: hookDescription,
	}
}
