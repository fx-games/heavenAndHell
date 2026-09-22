package common

type Broker struct {
	beforeHooks        map[ActionName][]ActionHook
	afterHooks         map[ActionName][]ActionHook
	runningAnimations  []Animation
	movementAnimations map[*Actor]*MovementAnimation
	queuedActions      [][]Action
	context            ActionContext
	ticks              uint64
	presenter          Presenter
	events             []Event // emitted by the actions executing right now, presented when each one finishes
	depth              int     // how many Executes are running
}

// NewBroker creates a new broker
// AND will inject a reference to itself into the context
func NewBroker(context ActionContext) *Broker {
	b := &Broker{
		beforeHooks:        make(map[ActionName][]ActionHook),
		afterHooks:         make(map[ActionName][]ActionHook),
		runningAnimations:  make([]Animation, 0),
		movementAnimations: make(map[*Actor]*MovementAnimation),
		queuedActions:      make([][]Action, 0),
		presenter:          Present,
	}
	context.ActionBroker = b
	b.context = context
	return b
}
func (d *Broker) Tick() {
	d.ticks++
	if len(d.movementAnimations) > 0 {
		for actor, animation := range d.movementAnimations {
			if animation.IsDone() {
				delete(d.movementAnimations, actor)
			} else {
				animation.Tick()
			}
		}
	}
	for i := len(d.runningAnimations) - 1; i >= 0; i-- {
		if d.runningAnimations[i].IsDone() {
			d.runningAnimations = append(d.runningAnimations[:i], d.runningAnimations[i+1:]...)
		} else {
			d.runningAnimations[i].Tick()
		}
	}

	d.checkForQueuedActions()
}

func (d *Broker) checkForQueuedActions() {
	animationsRunning := len(d.runningAnimations) > 0 || len(d.movementAnimations) > 0
	if !animationsRunning && len(d.queuedActions) > 0 {
		actions := d.queuedActions[0]
		d.queuedActions = d.queuedActions[1:]
		for _, action := range actions {
			d.Execute(action)
		}
	}
}

func (d *Broker) Execute(action Action) {
	if action == nil {
		return
	}

	d.endTargetingOnPlayerAttack(action)

	var chainedActions []Action
	d.callBeforeHooks(action)

	mark := len(d.events)
	d.depth++
	chainedActions = action.Execute(d.context)
	d.depth--
	animation := d.present(d.events[mark:])
	d.events = d.events[:mark]
	if len(animation) > 0 {
		d.startAnimations(animation)
		d.syncToProjectiles(animation, chainedActions)
	}

	for _, chainedAction := range chainedActions {
		d.Execute(chainedAction)
	}

	d.callAfterHooks(action)
}

// Emit reports what just happened. During an Execute it is shown when that action finishes, like the action's
// own animations; outside of one, right away.
func (d *Broker) Emit(ev Event) {
	if d.depth > 0 {
		d.events = append(d.events, ev)
		return
	}
	d.startAnimations(d.present([]Event{ev}))
}

func (d *Broker) present(events []Event) []Animation {
	var animations []Animation
	for _, ev := range events {
		if d.presenter != nil {
			animations = append(animations, d.presenter(d, ev)...)
		}
	}
	return animations
}

func (d *Broker) QueueForExecutionAtTheSameTime(actions []Action) {
	if len(actions) == 0 {
		return
	}
	d.queuedActions = append(d.queuedActions, actions)
	d.checkForQueuedActions()

}
func (d *Broker) QueueForExecution(action Action) {
	d.QueueForExecutionAtTheSameTime([]Action{action})
}
func (d *Broker) Draw(renderer TileRenderer) {
	for _, animation := range d.movementAnimations {
		animation.Draw(renderer)
	}
	for _, animation := range d.runningAnimations {
		animation.Draw(renderer)
	}
}

func (d *Broker) callAfterHooks(action Action) {
	if hooks, ok := d.afterHooks[action.Name()]; ok {
		for _, hook := range hooks {
			chained := hook.After(action, d.context)
			for _, chainedAction := range chained {
				d.Execute(chainedAction)
			}
		}
	}
}

func (d *Broker) callBeforeHooks(action Action) {
	if hooks, ok := d.beforeHooks[action.Name()]; ok {
		for _, hook := range hooks {
			chained := hook.Before(action, d.context)
			for _, chainedAction := range chained {
				d.Execute(chainedAction)
			}

		}
	}
}

func (d *Broker) RegisterHook(hook ActionHook) {
	if hook.IsAfter() {
		d.registerHook(hook, d.afterHooks)
	}
	if hook.IsBefore() {
		d.registerHook(hook, d.beforeHooks)
	}
}

func (d *Broker) registerHook(hook ActionHook, hooks map[ActionName][]ActionHook) {
	if _, ok := hooks[hook.ActionName()]; !ok {
		hooks[hook.ActionName()] = make([]ActionHook, 0)
	}
	hooks[hook.ActionName()] = append(hooks[hook.ActionName()], hook)
}

func (d *Broker) RemoveBeforeHook(hook ActionHook) {
	d.removeHook(hook, d.beforeHooks)
}

func (d *Broker) RemoveAfterHook(hook ActionHook) {
	d.removeHook(hook, d.afterHooks)
}
func (d *Broker) removeHook(hook ActionHook, hooks map[ActionName][]ActionHook) {
	if _, ok := hooks[hook.ActionName()]; ok {
		for i, h := range hooks[hook.ActionName()] {
			if h == hook {
				hooks[hook.ActionName()] = append(hooks[hook.ActionName()][:i], hooks[hook.ActionName()][i+1:]...)
				return
			}
		}
	}
}
func (d *Broker) AddAnimation(animation Animation) {
	d.runningAnimations = append(d.runningAnimations, animation)
}
func (d *Broker) startAnimations(animations []Animation) {
	for _, animation := range animations {
		switch typedAnimation := animation.(type) {
		case *MovementAnimation:
			d.queueMovementAnimation(typedAnimation)
		default:
			d.holdUntilPlayerArrives(animation)
			d.runningAnimations = append(d.runningAnimations, animation)
		}
	}
}

// holdUntilPlayerArrives keeps what the world does in reply to the hero's step from playing while he is still
// walking into the cell: an arrow loosed at where he is going must not land there before he does.
func (d *Broker) holdUntilPlayerArrives(animation Animation) {
	gated, ok := animation.(interface{ StartWhenTrue(func() bool) })
	if !ok {
		return
	}
	for actor, move := range d.movementAnimations {
		if move.IsForThePlayer() && !move.IsDone() && actor != animationActor(animation) {
			gated.StartWhenTrue(move.IsDone)
			return
		}
	}
}

// animationActor is whose animation this is, or nil.
func animationActor(animation Animation) *Actor {
	if a, ok := animation.(interface{ GetActor() *Actor }); ok {
		return a.GetActor()
	}
	return nil
}

func (d *Broker) CancelAllAnimations() {
	for _, animation := range d.runningAnimations {
		animation.Cancel()
	}
	d.runningAnimations = make([]Animation, 0)
	for _, animation := range d.movementAnimations {
		animation.Cancel()
	}
	d.movementAnimations = make(map[*Actor]*MovementAnimation)
}

func (d *Broker) ClearQueuedActions(shouldBeRemoved func(action Action) bool) {
	for i := len(d.queuedActions) - 1; i >= 0; i-- {
		var actions []Action
		for _, action := range d.queuedActions[i] {
			if !shouldBeRemoved(action) {
				actions = append(actions, action)
			}
		}
		if len(actions) == 0 {
			d.queuedActions = append(d.queuedActions[:i], d.queuedActions[i+1:]...)
		} else {
			d.queuedActions[i] = actions
		}
	}
}

func (d *Broker) queueMovementAnimation(newAnimation *MovementAnimation) {
	// idea: we will replace the existing with one that moves further

	if oldAnimation, ok := d.movementAnimations[newAnimation.GetActor()]; ok {
		if newAnimation.IsForThePlayer() {
			oldAnimation.Cancel()
			d.movementAnimations[newAnimation.GetActor()] = newAnimation
		} else {
			mergedAnimation := oldAnimation.Merge(newAnimation)
			d.movementAnimations[newAnimation.GetActor()] = mergedAnimation
		}

	} else {
		d.movementAnimations[newAnimation.GetActor()] = newAnimation
	}
}

func (d *Broker) GetAnimation(filter func(anim Animation) bool) (Animation, bool) {
	for _, animation := range d.runningAnimations {
		if filter(animation) {
			return animation, true
		}
	}
	for _, animation := range d.movementAnimations {
		if filter(animation) {
			return animation, true
		}
	}
	return nil, false
}

// endTargetingOnPlayerAttack drops the UI back to normal mode whenever the player attacks or casts, however it
// was started (bump, hotkey, quick cast, auto attack, the targeter itself).
func (d *Broker) endTargetingOnPlayerAttack(action Action) {
	switch action.(type) {
	case *MeleeAttackAction, *RangedAttackAction, *ThrowAction, *SpellAction:
	default:
		return
	}
	if d.context.GameState == nil || d.context.UserInterface == nil || action.Cause() == nil {
		return
	}
	if player := d.context.GameState.GetPlayer(); player != nil && action.Cause().GetActor() == player {
		if ui := d.context.UserInterface(); ui != nil {
			ui.CancelTargeting()
		}
	}
}
