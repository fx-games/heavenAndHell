package common

import (
	"math/rand"
	"rpg/d1"
	"rpg/defs"
	"rpg/stats"
	"rpg/util"
)

// Will turn definitions into actual game objects
type Factory struct {
	definitions  defs.DataContainer
	actionBroker ActionBroker
	randomSource *rand.Rand
	state        *State
	gui          func() UserInterface

	giveAndEquipItemToActor func(actor *Actor, item *Item)
	getTimeKeeper           func() *TimeKeeper
	logError                func(msg string)
	queueAfterTurn          func(func())
	d1                      *d1.Generator
}

type FactoryContext struct {
	Definitions  defs.DataContainer
	ActionBroker ActionBroker
	RandomSource *rand.Rand
	State        *State

	GetTimeKeeper func() *TimeKeeper

	GiveAndEquipItemToActor func(actor *Actor, item *Item)
	Msg                     func(msg string)
	D1                      *d1.Generator
}

func NewFactory(context FactoryContext) *Factory {
	return &Factory{
		definitions:             context.Definitions,
		randomSource:            context.RandomSource,
		state:                   context.State,
		giveAndEquipItemToActor: context.GiveAndEquipItemToActor,
		actionBroker:            context.ActionBroker,
		getTimeKeeper:           context.GetTimeKeeper,
		logError:                context.Msg,
		d1:                      context.D1,
	}
}

func (g *Factory) D1() *d1.Generator { return g.d1 }

func (g *Factory) NewSign(inscription []string) *BumpObject {
	return NewBumpObject("a sign", defs.IconSign, func(actor *Actor, startAnimating func() bool) {
		g.gui().OpenTextWindow(util.ToLabelText(inscription))
	})
}

func (g *Factory) SetQueueAfterTurn(afterTurn func(call func())) {
	g.queueAfterTurn = afterTurn
}

func (g *Factory) SetUserInterface(ui func() UserInterface) {
	g.gui = ui
}

func (g *Factory) NewPoisonedEffect(turnsLeft int, damagePerRound int) *ParametrizedStatusEffect {
	return NewParametrizedStatusEffect(StatusDamageOverTime, func(a *Actor) {
		g.actionBroker.Execute(NewChangeResourceAction(AbstractCause("poison"), a, stats.Health, float64(-damagePerRound)))
	}, turnsLeft)
}
