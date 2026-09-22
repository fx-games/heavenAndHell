package common

type SwapPositionsAction struct {
	BaseAction
	actorOne          *Actor
	actorTwo          *Actor
	isPlayer          bool
	animationDelay    float64
	animationDisabled bool
}

func NewSwapPositionsAction(source Cause, actorOne, actorTwo *Actor, isPlayer bool) *SwapPositionsAction {
	return &SwapPositionsAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameSwap,
		},
		actorOne:       actorOne,
		actorTwo:       actorTwo,
		isPlayer:       isPlayer,
		animationDelay: 0.2,
	}
}

func (d *SwapPositionsAction) SetAnimationDelay(delay float64) {
	d.animationDelay = delay
}
func (d *SwapPositionsAction) Execute(context ActionContext) []Action {
	state := context.GameState
	gameMap := state.GetMap() // read state

	gameMap.SwapPositions(d.actorOne, d.actorTwo) // write state

	context.ActionBroker.Emit(Swapped{One: d.actorOne, Two: d.actorTwo, Delay: d.animationDelay, ByPlayer: d.isPlayer,
		Quiet: d.animationDisabled})
	return nil
}

func (d *SwapPositionsAction) GetActors() []*Actor {
	return []*Actor{d.actorOne, d.actorTwo}
}

func (d *SwapPositionsAction) DisableAnimation() {
	d.animationDisabled = true
}
