package common

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
	npc := d.npc
	if npc.GetVendorType() == VendorTypeGeneral { // S_StartHealer: Pepin heals you when you talk to him
		context.GameState.GetPlayer().HealCompletely()
		context.Msg("Pepin tends to your wounds.")
	}
	if options := d1VendorOptions(context, npc); len(options) > 0 {
		context.UserInterface().OpenMenu(options)
	}
	return nil
}

func (d *TalkToAction) GetAnimations() []Animation {
	return nil
}
