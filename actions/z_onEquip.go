package actions

type OnEquipHook struct {
	BaseActionHook
}

func (d OnEquipHook) IsBefore() bool {
	return true
}

func (d OnEquipHook) IsAfter() bool {
	return true
}

func (d OnEquipHook) Name() string {
	return "OnEquipHook"
}

func NewMainOnEquipHook() *OnEquipHook {
	return &OnEquipHook{
		BaseActionHook: BaseActionHook{
			hookedAction: ActionNameChangeEquipment,
		},
	}
}
func (d OnEquipHook) Before(action Action, engine Engine) []Action {
	changeEquipmentAction := action.(*ChangeEquipmentAction)

	actor := changeEquipmentAction.GetActor()
	item := changeEquipmentAction.GetItem()

	if changeEquipmentAction.GetMode() == EquipmentChangeModeUnequip {
		return nil
	}

	// we only care about equipping stuff here
	// we want to identify the equipment that needs to unequipped
	// and issue the unequip actions for them

	equipment := actor.GetEquipment()

	blockingItems := equipment.GetItemsToReplace(item)

	var actions []Action
	for _, blockingItem := range blockingItems {
		unequipAction := NewUnequipItemAction(action.Cause(), actor, blockingItem.(GameItem))
		actions = append(actions, unequipAction)
	}

	return actions
}

func (d OnEquipHook) After(action Action, engine Engine) []Action {
	changeEquipmentAction := action.(*ChangeEquipmentAction)

	actor := changeEquipmentAction.GetActor()
	item := changeEquipmentAction.GetItem()

	actorStats := actor.GetStats()

	isEquipAction := changeEquipmentAction.GetMode() == EquipmentChangeModeEquip
	isUnequipAction := changeEquipmentAction.GetMode() == EquipmentChangeModeUnequip

	for _, modifier := range item.GetEquippedModifiers() {
		if isEquipAction {
			actorStats.AddModifier(modifier)
		} else if isUnequipAction {
			actorStats.RemoveModifier(modifier)
		}
	}

	for _, itemHook := range item.GetItemHooks() {
		itemHook.SetItemContext(&ItemContext{
			wearer: actor,
			item:   item,
		})

		broker := engine.GetActionBroker()

		if isEquipAction {
			if itemHook.IsBefore() {
				broker.RegisterHook(itemHook)
			}
			if itemHook.IsAfter() {
				broker.RegisterHook(itemHook)
			}
		} else if isUnequipAction {
			if itemHook.IsBefore() {
				broker.RemoveBeforeHook(itemHook)
			}
			if itemHook.IsAfter() {
				broker.RemoveAfterHook(itemHook)
			}
		}
	}

	engine.ShowFullStats() // problem, this gets overridden by updatestats called by the endturnaction

	if item.IsModifyingLight() {
		return OneAction(NewUpdateFoVAction(action.Cause(), actor))
	}
	//actor := changeEquipmentAction.GetAttacker()
	/*if actor == state.GetPlayer() {
	      return OneAction(NewEndPlayerTurnAction(d, changeEquipmentAction.GetDelayInAut()))
	  }
	*/
	return nil
}
