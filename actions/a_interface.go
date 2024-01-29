package actions

import (
	"fmt"
	"image/color"
	"rpg/audio"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
)

type ItemActionHook interface {
	ActionHook
	SetItemContext(context *ItemContext)
	ToString() string
	IsBefore() bool
	IsAfter() bool
}
type GameItem interface {
	common.Item
	GetItemHooks() []ItemActionHook
}

type Cause interface {
	Action() ActionName
	GetActor() *common.Actor
	ToString() string
}

type WorldState interface {
	GetPlayer() *common.Actor
	GetPlayerStashInTown() *common.Inventory
	GetMap() *gridmap.GridMap[*common.Actor, common.Item, common.Object]
	PlayerTookTurn(autUsed int)
	GetVisibility() *common.Visibility
	GetFlags() *common.Flags
}
type Engine interface {
	GetWorldState() WorldState

	GetAudio() *audio.Player
	GetCamera() common.Camera

	Log(message string)
	SetPlayerDijkstraMap(dijkstraMap map[geometry.Point]int)
	GetActionBroker() *Broker
	OpenNPCMenu(npc *common.Actor)
	MapTransition(to gridmap.Transition)
	ShowFullStats()
	IsClosedDoorAt(pos geometry.Point) bool
	UpdateMiniMap()
	Say(actor *common.Actor, trigger common.ActorSpeechTrigger)
	TriggerCellEffectsDuringMovementForActor(sourceAction Action, actor *common.Actor, mapPos geometry.Point, startAnimating func() bool)
	AddBloodDecalAt(position geometry.Point)
	FlashScreen(color color.Color, delay float64)
	IsPlayerInTown() bool
	GetPlayerVisionRange() int
	AddItemToMapWithDistribution(item common.Item, pos geometry.Point)
    OnPlayerDied()
}
type Action interface {
	Cause() Cause
	Name() ActionName
	Execute(engine Engine) []Action
	GetAnimations() []common.Animation
	GetDelayInAut() int
}

func OneAction(action Action) []Action {
	return []Action{action}
}

type BaseAction struct {
	cause Cause
	name  ActionName
}

func (d *BaseAction) Cause() Cause {
	return d.cause
}

func (d *BaseAction) Name() ActionName {
	return d.name
}
func (d *BaseAction) GetAnimations() []common.Animation {
	return nil
}

func (d *BaseAction) GetDelayInAut() int {
	return 10
}

type ActionHook interface {
	ActionName() ActionName
	Before(action Action, engine Engine) []Action
	After(action Action, engine Engine) []Action
	IsBefore() bool
	IsAfter() bool
}

type ActionName string

const (
	ActionNameMeleeAttack     ActionName = "ActionNameMeleeAttack"
	ActionNameRangedAttack    ActionName = "ActionNameRangedAttack"
	ActionNameThrow    ActionName = "ActionNameThrow"
	ActionNameMove            ActionName = "ActionNameMove"
	ActionNameMoveItem            ActionName = "ActionNameMoveItem"
	ActionNameSwap            ActionName = "ActionNameSwap"
	ActionNameChangeEquipment ActionName = "ActionNameChangeEquipment"
	ActionNameChangeRessource ActionName = "ActionNameChangeHealth"
	ActionNamePickup          ActionName = "ActionNamePickup"
	ActionNameStealItem       ActionName = "ActionNameStealItem"
	ActionNameDrop            ActionName = "ActionNameDrop"
	ActionNameOpenDoor        ActionName = "ActionNameOpenDoor"
	ActionNameSpawnObject     ActionName = "ActionNameSpawnObject"
	ActionNameUseItem         ActionName = "ActionNameUseItem"
	ActionNameFireball        ActionName = "ActionNameFireball"

	ActionNameDie               ActionName = "ActionNameDie"
	ActionNameApplyStatusEffect ActionName = "ActionNameApplyStatusEffect"
	ActionNameEndTurn           ActionName = "ActionNameEndTurn"
	ActionNameTransition        ActionName = "ActionNameTransition"
)

type BaseActionHook struct {
	hookedAction ActionName
}

func (d *BaseActionHook) ActionName() ActionName {
	return d.hookedAction
}

func (d *BaseActionHook) Before(action Action, engine Engine) []Action {
	return OneAction(action)
}

func (d *BaseActionHook) After(action Action, engine Engine) []Action {
	return nil
}

func logDodged(engine Engine, attacker, defender *common.Actor) {
	state := engine.GetWorldState()
	if attacker == state.GetPlayer() {
		engine.Log(fmt.Sprintf("Your attack misses %s", defender.Name()))
	} else if defender == state.GetPlayer() {
		engine.Log(fmt.Sprintf("%s misses you", attacker.Name()))
	} else {
		engine.Log(fmt.Sprintf("%s misses %s", attacker.Name(), defender.Name()))
	}
}
func logAttack(engine Engine, attacker, defender *common.Actor, damageAmount int) {
	state := engine.GetWorldState()
	if attacker == state.GetPlayer() {
		if damageAmount > 0 {
			engine.Log(fmt.Sprintf("Your attack hits %s", defender.Name()))
		} else {
			engine.Log(fmt.Sprintf("Your attack hits %s but does no damage", defender.Name()))
		}
	} else if defender == state.GetPlayer() {
		if damageAmount > 0 {
			engine.Log(fmt.Sprintf("%s hits you", attacker.Name()))
		} else {
			engine.Log(fmt.Sprintf("%s hits you but does no damage", attacker.Name()))
		}
	} else {
		if damageAmount > 0 {
			engine.Log(fmt.Sprintf("%s hits %s", attacker.Name(), defender.Name()))
		} else {
			engine.Log(fmt.Sprintf("%s hits %s but does no damage", attacker.Name(), defender.Name()))
		}
	}
}

func OneAnimation(animation common.Animation) []common.Animation {
	return []common.Animation{animation}
}

type BaseAnimation struct {
	startThisTick func() bool
	hasStarted    bool
}

func (b *BaseAnimation) shouldRunNow() bool {
	if !b.hasStarted && b.startThisTick != nil && !b.startThisTick() {
		return false
	}
	b.hasStarted = true
	return true
}
func (b *BaseAnimation) StartWhenTrue(startThisTick func() bool) {
	b.startThisTick = startThisTick
}
