package common

import (
	"fmt"
	"math/rand"
	"rpg/src/util"
)

// d1GossipSeeds is each townsperson's _tSeed: rolled on first talk, dropped when the hero reaches the dungeon.
var d1GossipSeeds = map[VendorType]int64{}

// d1Gossip picks a gossip text id for a town NPC (S_TalkEnter's gossipstart..gossipend: all but the greeting),
// seeded by the NPC's _tSeed so it repeats until the hero leaves town, or "" if none is available.
func d1Gossip(gossip map[string][]string, vendorType VendorType) string {
	ids := gossip[string(vendorType)]
	if len(ids) < 2 {
		return ""
	}
	seed, ok := d1GossipSeeds[vendorType]
	if !ok {
		seed = rand.Int63()
		d1GossipSeeds[vendorType] = seed
	}
	return ids[1+rand.New(rand.NewSource(seed)).Intn(len(ids)-1)]
}

// d1Greeting is the NPC's TownerTalk line (TEXT_GRISWOLD1...), played as speech when the store opens.
func d1Greeting(gossip map[string][]string, vendorType VendorType) string {
	if ids := gossip[string(vendorType)]; len(ids) > 0 {
		return ids[0]
	}
	return ""
}

// d1TalkTopics ports S_TalkEnter's topic list: the active quests this NPC can discuss (d1QuestTopic), plus Gossip.
func d1TalkTopics(context ActionContext, npc *Actor) []util.MenuItem {
	var items []util.MenuItem
	state := context.GameState
	for q, def := range state.QuestDefs() {
		if id := state.d1QuestTopic(npc.GetVendorType(), q); id != "" {
			items = append(items, util.MenuItem{MainText: def.Name, Action: func() { state.factory.say(npc.Name(), id) }})
		}
	}
	items = append(items, util.MenuItem{MainText: "Gossip", Action: func() {
		if id := d1Gossip(state.factory.D1().D.Gossip, npc.GetVendorType()); id != "" {
			state.factory.say(npc.Name(), id)
		}
	}})
	return items
}

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
	f := context.GameState.factory
	f.talkAt = npc.MapPosition()
	// Walking up to an NPC plays its on_approach triggers (TalkToTowner's quest talk): if one speaks, that is the
	// whole visit. The dying townsman never has a menu.
	if f.fireQuestTriggers(EventApproach, npc.GetInternalName(), npc.Name()) || npc.GetVendorType() == VendorTypeDeadGuy {
		return nil
	}
	// Whatever the mod's on_talk triggers say for this NPC is the conversation; only an NPC with nothing
	// scripted falls back to the quest topics and gossip menu.
	talk := func() {
		if !f.fireQuestTriggers(EventTalk, npc.GetInternalName(), npc.Name()) {
			context.UserInterface().OpenMenu(d1TalkTopics(context, npc))
		}
	}
	f.playD1Sound(f.D1().D.TextSfx[d1Greeting(f.D1().D.Gossip, npc.GetVendorType())])
	if npc.GetVendorType() != "" {
		PlayUICue("store_enter")
	}
	talkItem := util.MenuItem{MainText: fmt.Sprintf("Talk to %s", npc.Name()), Action: talk}
	options := append([]util.MenuItem{talkItem}, d1VendorOptions(context, npc)...)
	context.UserInterface().OpenMenu(options)
	return nil
}

func (d *TalkToAction) GetAnimations() []Animation {
	return nil
}
