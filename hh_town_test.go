package main

import (
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/files"
	"rpg/game"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
	"slices"
	"strings"
	"testing"
)

// A mod is done when you can play it. For Heaven & Hell that starts with every class starting a real game in its
// own town, and each of the town's three doors leading down and back to the same door - the path main's
// setupTownTransitions hardcoded in Go, and the mod now declares as data.
func TestHeavenAndHellTownIsPlayable(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	for _, class := range g.GetGameData().ClassDefinitions {
		g.NewGame(class, false, "")
		s := g.GetWorldState()
		skills := map[string]bool{}
		for _, a := range s.GetPlayer().GetTargetedSkills() {
			skills[a.GetInternalName()] = true
		}
		for _, a := range s.GetPlayer().GetUntargetedSkills() {
			skills[a.GetInternalName()] = true
		}
		for _, name := range append(append([]string{}, class.TargetedSkills...), class.UntargetedSkills...) {
			if n, _, _ := strings.Cut(name, "("); !skills[n] {
				t.Errorf("%s lacks its class skill %s (has %v)", class.Name, name, skills)
			}
		}
		armed := false
		for _, it := range s.GetPlayer().GetEquipment().AllItems() {
			armed = armed || it.IsWeapon()
		}
		if !armed || s.GetPlayer().HeroSheet().MaxDam <= 1 {
			t.Errorf("%s starts unarmed (max damage %d)", class.Name, s.GetPlayer().HeroSheet().MaxDam)
		}
		if s.GetMap().GetName() != string(defs.LevelNameTown) {
			t.Fatalf("%s starts on %q, not in town", class.Name, s.GetMap().GetName())
		}
		for _, door := range []string{"to_cathedral", "to_catacombs", "to_caves"} {
			s.MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "spawn"})
			down, ok := s.GetMap().GetTransitionAt(s.GetMap().GetNamedLocation(door))
			if !ok {
				t.Errorf("%s: the town's %s leads nowhere", class.Name, door)
				continue
			}
			s.MapTransition(down)
			d := s.GetMap()
			if d.GetName() != down.TargetMap {
				t.Errorf("%s: %s should lead to %s, got %s", class.Name, door, down.TargetMap, d.GetName())
				continue
			}
			up, ok := d.GetTransitionAt(d.GetNamedLocation(down.TargetLocation))
			if want := (gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: door}); !ok || up != want {
				t.Errorf("%s: the way back from %s leads to %+v, want %+v", class.Name, d.GetName(), up, want)
			}
		}
	}
}

func newHeavenAndHellGame(t *testing.T, ui common.UserInterface) *game.GameInstance {
	t.Helper()
	files.SelectMod("heavenandhell")
	_ = defs.LoadIcons()
	t.Cleanup(func() {
		files.SelectMod(common.DefaultMod)
		if err := defs.LoadIcons(); err != nil {
			t.Fatal(err)
		}
	})
	cfg := common.NewConfigurationFromRecFile(common.ConfigFile)
	g := game.NewGameInstance(common.StartInfo{ScreenSize: cfg.WindowSize, TileScale: 4, LDTKMapFile: "assets/maps.ldtk",
		SpriteSheet: cfg.SpriteSheet, FontInfos: common.FontInfo{TTFFile: cfg.DefaultFont, TTFSize: cfg.DefaultFontSize},
		UserConfiguration: &cfg, FileSystem: files.GetFileSystem()}, ui)
	g.GetAudio().SetDisablePlayback(true)
	return g
}

// talkUI keeps the menus and text windows the game opens, so a test can pick "Talk to" and read the reply.
type talkUI struct {
	*DummyUI
	menus [][]util.MenuItem
	texts []string
}

func (r *talkUI) OpenMenu(options []util.MenuItem) { r.menus = append(r.menus, options) }
func (r *talkUI) OpenTextWindow(msg []util.LabelText) {
	var lines []string
	for _, l := range msg {
		lines = append(lines, l.String())
	}
	r.texts = append(r.texts, strings.Join(lines, " "))
}

// Heaven & Hell's four quests, played through the town's own dialogue in main's order: Kirioth sends you after
// Meat Jack, Abel after the poison in the water once he is dead, Shekel after King Zedek once it runs clean,
// and Kirioth after Lucifer once Zedek falls. Each step is a real talk and a real kill.
func TestHeavenAndHellQuestsPlayThrough(t *testing.T) {
	ui := &talkUI{DummyUI: NewDummyUI()}
	g := newHeavenAndHellGame(t, ui)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	player := s.GetPlayer()
	cause := common.NewActorCause("test", player)
	town := gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "spawn"}

	talk := func(npc, wantText string) {
		t.Helper()
		s.MapTransition(town)
		var who *common.Actor
		for _, a := range s.GetMap().Actors() {
			if a.GetInternalName() == npc {
				who = a
			}
		}
		if who == nil {
			t.Fatalf("no %s in town", npc)
		}
		ui.menus, ui.texts = nil, nil
		g.GetActionBroker().Execute(common.NewTalkToAction(cause, who))
		if len(ui.menus) == 0 {
			t.Fatalf("%s opens no menu", npc)
		}
		for _, item := range ui.menus[0] {
			if strings.HasPrefix(item.MainText, "Talk to") {
				item.Action()
			}
		}
		if len(ui.texts) != 1 || !strings.Contains(ui.texts[0], wantText) {
			t.Fatalf("talking to %s: got %q, want one window saying %q", npc, ui.texts, wantText)
		}
	}
	kill := func(depthOrMap, internalName string) {
		t.Helper()
		s.MapTransition(gridmap.Transition{TargetMap: depthOrMap, TargetLocation: "ladder_up"})
		found := false
		for _, a := range slices.Clone(s.GetMap().Actors()) { // dying removes from the list being walked
			if a != player && a.IsAlive() && (internalName == "" || a.GetInternalName() == internalName) {
				g.GetActionBroker().Execute(common.NewDieAction(cause, a))
				found = true
			}
		}
		if !found {
			t.Fatalf("no %q on %s", internalName, depthOrMap)
		}
	}
	quest := func(key string) int { return s.QuestActive(s.QuestByKey(key)) }
	expect := func(key string, want int) {
		t.Helper()
		if got := quest(key); got != want {
			t.Fatalf("quest %s is %d, want %d", key, got, want)
		}
	}

	talk("mary", "Welcome")
	talk("abel", "")
	expect("poisoned_water", d1.QuestInit)
	talk("kirioth", "I am Kirioth")
	expect("meat_jack", d1.QuestActive)
	talk("kirioth", "still waiting")

	kill(defs.LevelNameFromDepth(2), "meat_jack")
	items, exp := len(player.GetInventory().Items()), player.Hero().Exp
	talk("kirioth", "")
	expect("meat_jack", d1.QuestDone)
	if len(player.GetInventory().Items()) != items+1 || player.Hero().Exp != exp+1000 {
		t.Errorf("Meat Jack's reward: %d -> %d items, %d -> %d exp", items, len(player.GetInventory().Items()), exp, player.Hero().Exp)
	}

	talk("abel", "rivers")
	expect("poisoned_water", d1.QuestActive)
	s.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(3), TargetLocation: "ladder_up"})
	down, ok := s.GetMap().GetTransitionAt(s.GetMap().GetNamedLocation("to_poisoned_water_supply"))
	if !ok || down.TargetMap != "poisoned_water_supply" {
		t.Fatalf("level 3 has no way to the poisoned water supply: %+v", down)
	}
	kill("poisoned_water_supply", "")
	talk("abel", "")
	expect("poisoned_water", d1.QuestDone)

	talk("shekel", "Zedek")
	expect("skeleton_king", d1.QuestActive)
	kill(defs.LevelNameFromDepth(4), "king_zedek")
	talk("shekel", "")
	expect("skeleton_king", d1.QuestDone)

	talk("kirioth", "")
	expect("lucifer", d1.QuestActive)
	for dlvl := 5; dlvl <= 12 && !s.GetFlags().IsSet("killed:lucifer"); dlvl++ {
		s.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl), TargetLocation: "ladder_up"})
		for _, a := range slices.Clone(s.GetMap().Actors()) {
			if a.GetInternalName() == "lucifer" {
				g.GetActionBroker().Execute(common.NewDieAction(cause, a))
			}
		}
	}
	gift := false
	for _, it := range s.GetTownMap(string(defs.LevelNameTown)).Items() {
		gift = gift || it.Name() == "Lucifer's gift" && it.HasUntargetedAction()
	}
	if !gift {
		t.Error("Lucifer fell and left no gift in town")
	}
	talk("kirioth", "")
	expect("lucifer", d1.QuestDone)
}

// H&H's four shops, as main had them: stocked from their own lists, each with its own menu, Mary buying from the
// hero and Kirioth healing whoever walks in.
func TestHeavenAndHellShops(t *testing.T) {
	ui := &talkUI{DummyUI: NewDummyUI()}
	g := newHeavenAndHellGame(t, ui)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	player := s.GetPlayer()
	want := map[string][]string{
		"mary":    {"Buy items", "Sell items"},
		"abel":    {"Buy items", "Open your stash"},
		"shekel":  {"Buy items", "Repair items"},
		"kirioth": {"Buy items", "Transfer an enchantment"},
	}
	for npc, labels := range want {
		var who *common.Actor
		for _, a := range s.GetMap().Actors() {
			if a.GetInternalName() == npc {
				who = a
			}
		}
		if who == nil {
			t.Fatalf("no %s in town", npc)
		}
		if who.GetInventory().IsEmpty() {
			t.Errorf("%s has no stock", npc)
		}
		if npc == "kirioth" {
			usable := false
			for _, it := range who.GetInventory().Items() {
				usable = usable || it.HasUntargetedAction()
			}
			if !usable {
				t.Errorf("none of Kirioth's potions does anything")
			}
		}
		for _, it := range who.GetInventory().Items() {
			if it.BuyPrice() <= 0 {
				t.Errorf("%s sells %s for %d", npc, it.Name(), it.BuyPrice())
			}
		}
		player.GetStats().GetResource(stats.Health).Decrease(1)
		ui.menus = nil
		g.GetActionBroker().Execute(common.NewTalkToAction(common.NewActorCause("test", player), who))
		var got []string
		for _, item := range ui.menus[0][1:] { // after "Talk to"
			got = append(got, item.MainText)
		}
		if !slices.Equal(got, labels) {
			t.Errorf("%s offers %q, want %q", npc, got, labels)
		}
		healed := player.GetStats().GetResource(stats.Health).IsAtMax()
		if healed != (npc == "kirioth") {
			t.Errorf("%s: hero healed=%v", npc, healed)
		}
		sword := player.GetEquipment().AllItems()[0]
		if buys := common.D1VendorBuys(who, sword); buys != (npc == "mary") {
			t.Errorf("%s buys the hero's %s: %v", npc, sword.Name(), buys)
		}
	}
}
