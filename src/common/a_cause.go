package common

import (
	"fmt"
)

type NamedCause struct {
	name ActionName
}

func (c NamedCause) ToString() string {
	return string(c.name)
}

func (c NamedCause) Action() ActionName {
	return c.name
}
func (c NamedCause) GetActor() *Actor {
	return nil
}
func AbstractCause(s ActionName) NamedCause {
	return NamedCause{name: s}
}

type ActorCause struct {
	actionName ActionName
	actor      *Actor
}

func (c ActorCause) ToString() string {
	return fmt.Sprintf("%s by %s", c.actionName, c.actor.Name())
}
func NewActorCause(actionDescription ActionName, actor *Actor) ActorCause {
	return ActorCause{
		actionName: actionDescription,
		actor:      actor,
	}
}
func (c ActorCause) Action() ActionName {
	return c.actionName
}

func (c ActorCause) GetActor() *Actor {
	return c.actor
}
