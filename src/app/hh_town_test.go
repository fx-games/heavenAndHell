package app

import (
	"os"
	"path/filepath"
	"rpg/files"
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/game"
	"rpg/src/gridmap"
	"rpg/src/hhm"
	"rpg/src/rules"
	"rpg/src/stats"
	"rpg/src/util"
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
		for name, lvl := range class.StartSpells {
			if lvl > 0 && !skills[name] {
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
		if s.GetMap().GetName() != "town" {
			t.Fatalf("%s starts on %q, not in town", class.Name, s.GetMap().GetName())
		}
		for _, door := range []string{"to_cathedral", "to_catacombs", "to_caves"} {
			s.MapTransition(gridmap.Transition{TargetMap: "town", TargetLocation: "spawn"})
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
			if want := (gridmap.Transition{TargetMap: "town", TargetLocation: door}); !ok || up != want {
				t.Errorf("%s: the way back from %s leads to %+v, want %+v", class.Name, d.GetName(), up, want)
			}
		}
	}
}

func newHeavenAndHellGame(t *testing.T, ui common.UserInterface) *game.GameInstance {
	t.Helper()
	return newModGame(t, "heavenandhell", ui)
}

// newModGame starts a game instance on a mod ("" is none: the engine's own fallback content) and puts the test
// binary back on the default mod afterwards.
func newModGame(t *testing.T, mod string, ui common.UserInterface) *game.GameInstance {
	t.Helper()
	files.SelectMod(mod)
	_ = defs.LoadIcons()
	t.Cleanup(func() {
		files.SelectMod(testMod)
		if err := defs.LoadIcons(); err != nil {
			t.Fatal(err)
		}
	})
	cfg := common.NewConfigurationFromRecFile(common.ConfigFile)
	g := game.NewGameInstance(common.StartInfo{ScreenSize: cfg.WindowSize, TileScale: 4, MapDir: hhm.Dir,
		SpriteSheet: cfg.SpriteSheet, FontInfos: common.FontInfo{TTFFile: cfg.FontFile(), TTFSize: cfg.DefaultFontSize},
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
	town := gridmap.Transition{TargetMap: "town", TargetLocation: "spawn"}

	windows := 1 // how many text windows one talk opens
	talkIn := func(where gridmap.Transition, npc, wantText string) {
		t.Helper()
		s.MapTransition(where)
		var who *common.Actor
		for _, a := range s.GetMap().Actors() {
			if a.GetInternalName() == npc {
				who = a
			}
		}
		if who == nil {
			t.Fatalf("no %s on %s", npc, where.TargetMap)
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
		if len(ui.texts) != windows || !strings.Contains(ui.texts[0], wantText) {
			t.Fatalf("talking to %s: got %q, want %d window(s), the first saying %q", npc, ui.texts, windows, wantText)
		}
	}
	talk := func(npc, wantText string) { t.Helper(); talkIn(town, npc, wantText) }
	kill := func(depthOrMap, internalName string) {
		t.Helper()
		if depthOrMap == "town" {
			s.MapTransition(town)
		} else {
			s.MapTransition(gridmap.Transition{TargetMap: depthOrMap, TargetLocation: "ladder_up"})
		}
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
	expect("poisoned_water", rules.QuestInit)
	talk("kirioth", "I am Kirioth")
	expect("meat_jack", rules.QuestActive)
	talk("kirioth", "still waiting")

	kill(levelAt(s, 2), "meat_jack")
	items, exp := len(player.GetInventory().Items()), player.Hero().Exp
	talk("kirioth", "")
	expect("meat_jack", rules.QuestDone)
	if len(player.GetInventory().Items()) != items+1 || player.Hero().Exp != exp+1000 {
		t.Errorf("Meat Jack's reward: %d -> %d items, %d -> %d exp", items, len(player.GetInventory().Items()), exp, player.Hero().Exp)
	}

	talk("abel", "rivers")
	expect("poisoned_water", rules.QuestActive)
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 5), TargetLocation: "ladder_up"})
	down, ok := s.GetMap().GetTransitionAt(s.GetMap().GetNamedLocation("to_poisoned_water_supply"))
	if !ok || down.TargetMap != "poisoned_water_supply" {
		t.Fatalf("level 5 has no way to the poisoned water supply: %+v", down)
	}
	kill("poisoned_water_supply", "")
	talk("abel", "")
	expect("poisoned_water", rules.QuestDone)

	talk("shekel", "Zedek")
	expect("skeleton_king", rules.QuestActive)
	kill(levelAt(s, 8), "king_zedek")
	talk("shekel", "")
	expect("skeleton_king", rules.QuestDone)

	talk("kirioth", "")
	expect("lucifer", rules.QuestActive)
	for dlvl := 5; dlvl <= 16 && !s.GetFlags().IsSet("killed:lucifer"); dlvl++ {
		s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, dlvl), TargetLocation: "ladder_up"})
		for _, a := range slices.Clone(s.GetMap().Actors()) {
			if a.GetInternalName() == "lucifer" {
				g.GetActionBroker().Execute(common.NewDieAction(cause, a))
			}
		}
	}
	var gift *common.Item
	for _, it := range s.Map("town").Items() {
		if it.Name() == "Lucifer's gift" && it.HasUntargetedAction() {
			gift = it
		}
	}
	if gift == nil {
		t.Fatal("Lucifer fell and left no gift in town")
	}
	ui.texts = nil
	gift.GetUntargetedAction().Invoke(player, "test", false)
	s.ItemUsed(gift)
	expect("evidence", rules.QuestActive)
	if len(ui.texts) != 1 || !strings.Contains(ui.texts[0], "among the seeing") {
		t.Errorf("drinking the gift: got %q, want Lucifer's note", ui.texts)
	}
	lucifersNote := false
	for _, it := range s.GetMap().Items() {
		lucifersNote = lucifersNote || strings.Contains(it.Name(), "Morningstar") && it.HasUntargetedAction()
	}
	if !lucifersNote {
		t.Error("Lucifer's note was not left at the hero's feet")
	}
	if !s.GetFlags().IsSet(string(common.FlagHasTrueSight)) {
		t.Error("the gift did not open the hero's eyes")
	}
	// the last piece of evidence waits on Caves 12 for a hero who is looking for it
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 12), TargetLocation: "ladder_up"})
	for note, where := range map[string]string{"Purity report": "poisoned_water_supply",
		"King Zedek's last will": levelAt(s, 8), "Kirioth's schemes": "town",
		"From K. to S.": levelAt(s, 12)} {
		m := s.Map(where)
		found := false
		for _, it := range m.Items() {
			found = found || it.Name() == note && it.IsHidden() && it.HasUntargetedAction()
		}
		if !found {
			t.Errorf("no hidden %q on %s", note, where)
		}
	}
	talk("kirioth", "")
	expect("lucifer", rules.QuestDone)
	talk("kirioth", "Thanks again") // not mute once his quests are done

	// Picking up the four pieces of evidence, in any order, ends the quest once: Lucifer finds the hero, as his
	// dying words promised, and tells them what the notes mean.
	pickUp := func(where, note string) {
		t.Helper()
		if where == "town" {
			s.MapTransition(town)
		} else {
			s.MapTransition(gridmap.Transition{TargetMap: where, TargetLocation: "ladder_up"})
		}
		m := s.GetMap()
		for _, it := range m.Items() {
			if it.Name() == note {
				m.RemoveItem(it)
				m.AddItem(it, player.MapPosition())
				g.GetActionBroker().Execute(common.NewPickUpAction(cause, player))
				return
			}
		}
		t.Fatalf("no %q on %s", note, where)
	}
	exp = player.Hero().Exp
	pickUp("town", "Kirioth's schemes")
	pickUp(levelAt(s, 12), "From K. to S.")
	pickUp("poisoned_water_supply", "Purity report")
	expect("evidence", rules.QuestActive)
	ui.texts = nil
	pickUp(levelAt(s, 8), "King Zedek's last will")
	expect("evidence", rules.QuestDone)
	if len(ui.texts) != 1 || !strings.Contains(ui.texts[0], "mankind itself") {
		t.Errorf("the last piece of evidence: got %q, want Lucifer's rant", ui.texts)
	}
	if player.Hero().Exp != exp+1000 {
		t.Errorf("the evidence quest's reward: %d -> %d exp", exp, player.Hero().Exp)
	}
	ui.texts = nil
	for _, it := range player.GetInventory().Items() {
		if it.Name() == "Purity report" {
			player.GetInventory().Remove(it)
			s.GetMap().AddItem(it, player.MapPosition())
		}
	}
	g.GetActionBroker().Execute(common.NewPickUpAction(cause, player))
	if len(ui.texts) != 0 {
		t.Errorf("picking a note up again replayed %q", ui.texts)
	}

	// The second act: Lucifer, in the town in hell, sends the hero after the town's true villains.
	hellTown := gridmap.Transition{TargetMap: "hell_town", TargetLocation: "to_caves"}
	lucifer := func(want string) { t.Helper(); talkIn(hellTown, "lucifer_townsman", want) }
	lucifer("settle the town's accounts")
	for _, q := range []string{"adventurers", "skeleton_army", "shekel"} {
		expect(q, rules.QuestActive)
	}
	lucifer("debts are still open")

	for _, adventurer := range []string{"adventurer_knight", "adventurer_priest", "adventurer_barbarian", "adventurer_archer"} {
		kill(levelAt(s, 17), adventurer)
	}
	kill(levelAt(s, 20), "evil_shekel")

	crownIn := func() *common.Item {
		for _, it := range player.GetInventory().Items() {
			if it.Name() == "King Zedek's crown" {
				return it
			}
		}
		return nil
	}
	pickUp("town", "King Zedek's crown")
	s.ItemUsed(crownIn()) // anywhere but his tomb, it is only a crown
	if crownIn() == nil {
		t.Fatal("the crown was buried in town")
	}
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 8), TargetLocation: "ladder_up"})
	s.ItemUsed(crownIn())
	if crownIn() != nil {
		t.Fatal("the crown was not buried in King Zedek's tomb")
	}

	exp = player.Hero().Exp
	windows = 4
	lucifer("Four fewer heroes") // all three debts settle in one talk, each in its own window, then Kirioth's offer
	windows = 1
	for _, q := range []string{"adventurers", "skeleton_army", "shekel"} {
		expect(q, rules.QuestDone)
	}
	if player.Hero().Exp != exp+6000 {
		t.Errorf("the second act's rewards: %d -> %d exp", exp, player.Hero().Exp)
	}
	expect("kirioth", rules.QuestActive)
	kill("town", "evil_kirioth")
	expect("kirioth", rules.QuestDone)
	for _, a := range s.Map("town").Actors() {
		if n := a.GetInternalName(); n == "shekel" || n == "kirioth" {
			t.Errorf("%s still stands in town after his true self died", n)
		}
	}
	lucifer("It is done")
	talk("lucifer_townsman", "It is done") // and he has come up to town

	// The town is rebuilt from its drawing on load; the quests' changes to who lives there must survive that.
	old := game.SaveDir
	t.Cleanup(func() { game.SaveDir = old })
	game.SaveDir = filepath.Join(t.TempDir(), "saves")
	g.SaveToSlot(1)
	if err := g.LoadSlot(1); err != nil {
		t.Fatal(err)
	}
	s = g.GetWorldState()
	player = s.GetPlayer()
	cause = common.NewActorCause("test", player)
	var townsfolk []string
	for _, a := range s.Map("town").Actors() {
		townsfolk = append(townsfolk, a.GetInternalName())
	}
	if slices.Contains(townsfolk, "kirioth") || slices.Contains(townsfolk, "shekel") || !slices.Contains(townsfolk, "lucifer_townsman") {
		t.Errorf("after loading, town holds %v", townsfolk)
	}

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
		if buys := common.VendorBuys(who, sword); buys != (npc == "mary") {
			t.Errorf("%s buys the hero's %s: %v", npc, sword.Name(), buys)
		}
	}
}

// A fight on H&H's first level, through the real attack and turn actions: the monster fights back with its own
// numbers, the hero hits with the weapon it started with, and the kill pays experience.
func TestHeavenAndHellCombat(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	player := s.GetPlayer()
	player.Hero().MaxHPBase += 500 << 6 // survive long enough to see both sides land blows
	common.SyncHero(player)
	player.GetStats().GetResource(stats.Health).SetToMax()
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 1), TargetLocation: "ladder_up"})
	m := s.GetMap()
	var foe *common.Actor
	for _, a := range m.Actors() {
		if a != player && a.Monster() != nil && len(m.GetFreeCardinalNeighbors(a.MapPosition())) > 0 {
			foe = a
			break
		}
	}
	if foe == nil {
		t.Fatal("no monster on level 1")
	}
	m.MoveActor(player, m.GetFreeCardinalNeighbors(foe.MapPosition())[0])
	broker, cause := g.GetActionBroker(), common.NewActorCause("test", player)
	exp, full := player.Hero().Exp, player.GetStats().GetResource(stats.Health).Value()
	broker.Execute(common.NewUpdateFoVAction(cause, player))
	for turn := 0; turn < 30; turn++ { // let it have its go first
		broker.Execute(common.NewEndPlayerTurnAction(cause, 10))
	}
	hurt := player.GetStats().GetResource(stats.Health).Value() < full
	for turn := 0; turn < 200 && foe.IsAlive(); turn++ {
		player.GetStats().GetResource(stats.Health).SetToMax() // a dead hero earns nothing (AddPlrExperience)
		broker.Execute(common.NewMeleeAttackAction(cause, player, foe))
		broker.Execute(common.NewEndPlayerTurnAction(cause, 10))
	}
	if foe.IsAlive() {
		t.Fatalf("200 swings and %s still stands", foe.Name())
	}
	if player.Hero().Exp <= exp {
		t.Errorf("killing %s paid no experience", foe.Name())
	}
	if !hurt {
		t.Errorf("%s never hit back", foe.Name())
	}
}

// Everything H&H's records declare, the engine has to be able to build: every item with the action, slot and
// modifiers it names, every monster with its skills, gear and lines, every named skill. A name the engine cannot
// resolve is content silently lost, which is the one thing this split must not do.
func TestHeavenAndHellContentResolves(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	f := g.GetFactory()
	data := f.Generator.D
	equippable := map[defs.ItemCategory]bool{defs.ItemCategoryWeapon: true, defs.ItemCategoryArmor: true,
		defs.ItemCategoryRing: true, defs.ItemCategoryAmulet: true, defs.ItemCategoryLightSource: true}
	known := map[string]bool{"strength": true, "dexterity": true, "intelligence": true, "health": true, "mana": true,
		"damage": true, "armor_class": true, "light_radius": true, "attack_speed": true, "steal_health": true,
		"steal_mana": true, "speed": true, "move_speed": true}
	for i := range data.Items {
		b := &data.Items[i]
		it := f.NewRolledItem(f.Generator.PlainItem(b))
		if b.UntargetedAction != "" && !it.HasUntargetedAction() {
			t.Errorf("item %s: untargeted_action %q does not resolve", b.InternalName, b.UntargetedAction)
		}
		if b.TargetedAction != "" && !it.HasTargetedAction() {
			t.Errorf("item %s: targeted_action %q does not resolve", b.InternalName, b.TargetedAction)
		}
		if equippable[b.InventoryCategory] && !it.IsEquippable() && it.SlotName() == defs.SlotNameNotEquippable {
			t.Errorf("item %s: a %v that cannot be equipped", b.InternalName, b.InventoryCategory)
		}
		for _, m := range b.Modifiers {
			if name, _, _ := strings.Cut(m, "("); !known[name] {
				t.Errorf("item %s: equip_modifier %q has no engine meaning", b.InternalName, m)
			}
		}
	}
	for _, mon := range data.Monsters {
		a := f.NewMonsterByID(mon.ID)
		if got, want := len(a.GetTargetedSkills())+len(a.GetUntargetedSkills()), len(mon.TargetedSkills)+len(mon.UntargetedSkills); got != want {
			t.Errorf("monster %s: %d of its %d skills resolve", mon.ID, got, want)
		}
		if got := len(a.GetEquipment().AllItems()); got != len(mon.Equipment) {
			t.Errorf("monster %s: wears %d of its %d items", mon.ID, got, len(mon.Equipment))
		}
		if len(mon.SpeechOnSight) != len(a.SpeechOnSight()) {
			t.Errorf("monster %s: lost its lines", mon.ID)
		}
	}
	magic := 0
	for i := 0; i < 400; i++ {
		if it := f.Generator.FloorItem(1+i%12, true, false); it != nil && it.Magical != 0 && strings.Contains(it.IName, " of ") {
			magic++
		}
	}
	if magic == 0 {
		t.Error("400 floor items and not one magic: H&H's enchantments do not roll")
	}
	for name := range data.TargetedDefs {
		if f.NewTargetedActionFromName(name) == nil {
			t.Errorf("targeted.rec %s does not build", name)
		}
	}
	for name := range data.UntargetedDefs {
		if f.NewUntargetedActionFromName(name) == nil {
			t.Errorf("untargeted.rec %s does not build", name)
		}
	}
}

// The town in hell, main's second town: once Lucifer is dead, Hell 16 and the Dungeon of Doom's first level (17) both lead there and back, and its
// four shopkeepers keep the same stock lists as their kind upstairs.
func TestHeavenAndHellHellTown(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, 17), TargetLocation: "ladder_up"})
	if there, ok := s.GetMap().GetTransitionAt(s.GetMap().GetNamedLocation("to_hell_town")); ok {
		t.Fatalf("the hell town is open before Lucifer fell: %+v", there)
	}
	s.GetFlags().Set("killed:lucifer", true)
	for dlvl, door := range map[int]string{16: "to_caves", 17: "to_dungeon"} {
		s.MapTransition(gridmap.Transition{TargetMap: levelAt(s, dlvl), TargetLocation: "ladder_up"})
		d := s.GetMap()
		there, ok := d.GetTransitionAt(d.GetNamedLocation("to_hell_town"))
		if !ok || there.TargetMap != "hell_town" || there.TargetLocation != door {
			t.Fatalf("level %d's way to the hell town: %+v", dlvl, there)
		}
		s.MapTransition(there)
		town := s.GetMap()
		if town.GetName() != "hell_town" {
			t.Fatalf("level %d led to %s", dlvl, town.GetName())
		}
		back, ok := town.GetTransitionAt(town.GetNamedLocation(door))
		if !ok || back.TargetMap != levelAt(s, dlvl) || back.TargetLocation != "to_hell_town" {
			t.Errorf("the hell town's %s leads to %+v", door, back)
		}
	}
	for _, npc := range []string{"azazel", "samael", "hellfire", "grumpy"} {
		stocked := false
		for _, a := range s.GetMap().Actors() {
			stocked = stocked || a.GetInternalName() == npc && !a.GetInventory().IsEmpty()
		}
		if !stocked {
			t.Errorf("%s has nothing to sell", npc)
		}
	}
}

// A mod owns its music and sound outright: H&H plays main's sounds, and none of Diablo's reach it.
func TestHeavenAndHellHearsOnlyItsOwnSounds(t *testing.T) {
	g := newHeavenAndHellGame(t, NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	a := g.GetAudio()
	for _, cue := range []string{"level_up", "monster.skeleton.hit", "monster.skeleton.death", "monster.bat.attack", "enter_dungeon"} {
		if !a.HasCue(cue) {
			t.Errorf("H&H's own %s is not registered", cue)
		}
	}
	for _, a := range g.GetWorldState().GetMap().Actors() {
		if a.GetInternalName() == "skeleton" && a.MonsterSoundCue("death") != "monster.skeleton.death" {
			t.Errorf("an H&H skeleton dies with %q", a.MonsterSoundCue("death"))
		}
	}
	if a.HasCue("monster.nzombie.hit") {
		t.Error("H&H has Diablo's monsters' sounds")
	}
	// the effect ids the shared D1 code plays (sfx.PS_SWING...) are H&H's to fill, with anything but Diablo's files
	for _, rec := range []string{"sounds.rec", "music.rec"} {
		data, err := os.ReadFile(filepath.Join("files", "mods", "heavenandhell", "data", rec))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "diablo_") {
			t.Errorf("H&H's %s points at Diablo's audio", rec)
		}
	}
}
