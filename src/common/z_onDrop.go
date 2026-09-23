package common

type OnDropHook struct {
	FuncHook
}

func (d OnDropHook) Name() string {
	return "OnDropHook"
}

func NewMainOnDropHook() *OnDropHook {
	return &OnDropHook{
		FuncHook: FuncHook{
			HookedAction: ActionNameDrop,
			BeforeHook: func(action Action, ctx ActionContext) []Action {
				dropAction := action.(*DropAction)
				dropper := dropAction.GetActor()
				droppedItem := dropAction.GetItem()
				droppedItem.Show()
				if dropper.GetEquipment().IsEquipped(droppedItem) {
					gameItem := droppedItem
					unequipAction := NewUnequipItemAction(action.Cause(), dropper, gameItem)
					return []Action{unequipAction}
				}
				return nil
			},
		},
	}
}
