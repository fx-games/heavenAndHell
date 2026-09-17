package common

import (
	"rpg/audio"
	"rpg/defs"
	"rpg/geometry"
	"rpg/util"
)

type ActionName string

const (
	ActionNameMeleeAttack     ActionName = "ActionNameMeleeAttack"
	ActionNameRangedAttack    ActionName = "ActionNameRangedAttack"
	ActionNameThrow           ActionName = "ActionNameThrow"
	ActionNameMove            ActionName = "ActionNameMove"
	ActionNameMoveItem        ActionName = "ActionNameMoveItem"
	ActionNameSwap            ActionName = "ActionNameSwap"
	ActionNameChangeEquipment ActionName = "ActionNameChangeEquipment"
	ActionNameChangeResource  ActionName = "ActionNameChangeHealth"
	ActionNamePickup          ActionName = "ActionNamePickup"
	ActionNameDrop            ActionName = "ActionNameDrop"
	ActionNameOpenDoor        ActionName = "ActionNameOpenDoor"
	ActionNameUseItem         ActionName = "ActionNameUseItem"
	ActionNameFireball        ActionName = "ActionNameFireball"
	ActionNameMonsterMissile  ActionName = "ActionNameMonsterMissile"

	ActionNameDie        ActionName = "ActionNameDie"
	ActionNameEndTurn    ActionName = "ActionNameEndTurn"
	ActionNameTransition ActionName = "ActionNameTransition"
	ActionNameTalkTo     ActionName = "ActionNameTalkToNpc"
)

type Animator interface {
	CancelAllAnimations()
	Tick()
	Draw(renderer TileRenderer)
}
type DebugActions struct {
	KillLevel      func()
	OpenDebugMenu  func()
	GoOneLevelDown func()
	GoOneLevelUp   func()
}

type UserInterface interface {
	// Queries
	GetVisibleMap() geometry.Rect
	IsPlayerForcingAggression() bool
	HasOverlays() bool

	// Features ParseColorCodedText(colorCodedText string) []util.ColoredTextPart

	// Logging
	ShowLogMessage(message string)

	// More generic
	Cancel()
	PopAll()
	ShowGameOverMenu()
	QuitGame()
	OpenMenu(options []util.MenuItem)
	OpenUncancelableMenu(options []util.MenuItem) // like OpenMenu, but ESC/cancel does nothing (e.g. the start menu)
	OpenTextWindow(msg []util.LabelText)

	CancelTargeting()
	ShowFullStats()
	UpdateStatsLabel()

	// Specialized
	OnMapChanged()
	OpenNewGameScreen(start func(class defs.LoadoutDefinition, permadeath bool, name string), back func())

	// Action specific
	BeginTargeting(rules TargetRules, onHit *HitEffect)

	// Vendor interactions
	OpenShopMenu(npc *Actor)
	OpenShopMenuFiltered(npc *Actor, filter func(*Item) bool)
	BeginPlayerSellingTo(vendor *Actor)

	OpenListInventoryForSelection(filter func(element *Item) bool, selected func(itemKey rune))
	OpenItemList(items []*TextualStack)                                     // an item selection rendered like the inventory list
	OpenServiceMenu(items []*Item, price func(*Item) int, pick func(*Item)) // a paid service laid out like the shop menus
	UpdateMiniMap()
	OpenPlayerStashInTown()
	OnPlayerTurnTaken()
	SetPlayerMessage(message func(message string))
	SetDebugActions(actions func() DebugActions)
	FadeToBlack()
	FadeFromBlack()

	// Confirm opens a Yes/No prompt asking message; onConfirm runs only if the player picks Yes.
	Confirm(message string, onConfirm func())
}
type ActionContext struct {
	GameState     *State
	UserInterface func() UserInterface
	AudioPlayer   *audio.Player
	Data          defs.DataContainer
	ActionBroker  *Broker
	Msg           func(message string)
}

type ActionHook interface {
	ActionName() ActionName
	Before(action Action, context ActionContext) []Action
	After(action Action, context ActionContext) []Action
	IsBefore() bool
	IsAfter() bool
}

type ItemActionHook interface {
	ActionHook
	SetItemContext(context *ItemContext)
	ToString() string
}

type Cause interface {
	Action() ActionName
	GetActor() *Actor
	ToString() string
}

type Action interface {
	Cause() Cause
	Name() ActionName
	Execute(context ActionContext) []Action
	GetAnimations() []Animation
	GetDelayInAut() int
}

func OneAction(action Action) []Action {
	return []Action{action}
}
