package common

type EndTurnAction struct {
	BaseAction
	autUsed int
}

func NewEndPlayerTurnAction(source Cause, autUsed int) *EndTurnAction {
	return &EndTurnAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameEndTurn,
		},
		autUsed: autUsed,
	}
}

// hmmm, this can get triggered while action chains and their animations are still running..
// basically we want to schedule this action to be executed after all the animations are done or cancelled
func (d *EndTurnAction) Execute(context ActionContext) []Action {
	// THIS WILL TRIGGER ALL THE AI ACTIONS
	state := context.GameState
	state.PlayerTookTurn(d.autUsed)

	return nil
}

func (d *EndTurnAction) GetAnimations() []Animation {
	return nil
}
