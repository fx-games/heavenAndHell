package common

import (
	"fmt"
	"math/rand"
	"rpg/d1"
	"rpg/util"
)

// d1GossipKeyByVendor maps a town vendor to its gossip file's key in d1.Data.Gossip (ported from Diablo's
// Source/Data/Text/Gossip/*.txt).
var d1GossipKeyByVendor = map[VendorType]string{
	VendorTypeWeapons: "griswold", VendorTypeWands: "adria", VendorTypeGeneral: "pepin",
	VendorTypeCain: "cain", VendorTypeWirt: "wirt", VendorTypeOgden: "ogden",
	VendorTypeDrunk: "farnham", VendorTypeBarmaid: "gillian",
}

// d1Gossip picks a random gossip text id for a town NPC (S_TalkEnter's gossipstart..gossipend: all but the
// greeting), or "" if none is available.
func d1Gossip(gossip map[string][]string, vendorType VendorType) string {
	ids := gossip[d1GossipKeyByVendor[vendorType]]
	if len(ids) < 2 {
		return ""
	}
	return ids[1+rand.Intn(len(ids)-1)]
}

// d1Greeting is the NPC's TownerTalk line (TEXT_GRISWOLD1...), played as speech when the store opens.
func d1Greeting(gossip map[string][]string, vendorType VendorType) string {
	if ids := gossip[d1GossipKeyByVendor[vendorType]]; len(ids) > 0 {
		return ids[0]
	}
	return ""
}

// d1TalkTopics ports S_TalkEnter's topic list: the active quests this NPC can discuss (d1QuestTopic), plus Gossip.
func d1TalkTopics(context ActionContext, npc *Actor) []util.MenuItem {
	var items []util.MenuItem
	state := context.GameState
	for q, name := range d1.QuestNames {
		if id := state.d1QuestTopic(npc.GetVendorType(), q); id != "" {
			items = append(items, util.MenuItem{MainText: name, Action: func() { state.factory.say(npc.Name(), id) }})
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
	if context.GameState.factory.d1TownerQuests(npc) || npc.GetVendorType() == VendorTypeDeadGuy {
		return nil
	}
	f := context.GameState.factory
	f.playD1Sound(f.D1().D.TextSfx[d1Greeting(f.D1().D.Gossip, npc.GetVendorType())])
	talkItem := util.MenuItem{MainText: fmt.Sprintf("Talk to %s", npc.Name()), Action: func() {
		context.UserInterface().OpenMenu(d1TalkTopics(context, npc))
	}}
	options := append([]util.MenuItem{talkItem}, d1VendorOptions(context, npc)...)
	context.UserInterface().OpenMenu(options)
	return nil
}

func (d *TalkToAction) GetAnimations() []Animation {
	return nil
}
