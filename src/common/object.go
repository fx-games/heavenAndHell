package common

import (
	"fmt"
	"image/color"
	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
	"strings"
)

// Prop is a D1 dungeon object (decision 12), operated by bumping into it.
type Prop struct {
	O            *rules.Object
	f            *Factory
	pos          geometry.Point
	Disarmed     bool // the rogue disarmed its trap (TryDisarm), kept in saves
	PreSpawnType int  // -1, or the monster type a sarcophagus/barrel raises when opened (rules.Object's Var2/Var4 slot,
	// resolved to a type at placement time since the monster stays hidden off-map until then)
}

// The object display table -- names, light radii, which are looted, which are wall lights and what each one
// sounds like -- lives in assets/data/objects.rec and is reached through rules.ObjectDisplays.
func (f *Factory) objectDisplay(kind int) rules.ObjectDisplay {
	return f.Generator.D.ObjectDisplays[kind]
}

// ID is the prop's OBJ_ name.
func (o *Prop) ID() string { return o.f.Generator.D.Objects[o.O.Kind].Name }

func (o *Prop) Name() string {
	if player := o.f.State.GetPlayer(); o.O.TrapFlag && player != nil && player.hero != nil && player.hero.Rules().HasClassFlag(player.hero.Class, defs.ClassTrapSense) {
		return "Trapped " + o.name() // GetObjectStr: only a rogue's trap sense sees it
	}
	return o.name()
}

func (o *Prop) name() string {
	if k := o.O.Kind; k == rules.ObjSHRINEL || k == rules.ObjSHRINER {
		return o.f.Generator.D.ShrineNames[o.O.Var1] + " Shrine"
	}
	if o.O.Kind == rules.ObjSTORYBOOK && o.O.Var3 >= 0 && o.O.Var3 < len(o.f.Generator.D.StoryBookNames) {
		return o.f.Generator.D.StoryBookNames[o.O.Var3]
	}
	if o.O.Kind == rules.ObjBOOK2L && o.f.State.onSetLevelOf(o.f.State.GetMap(), rules.QBetrayer) {
		return "Book of Vileness"
	}
	return o.f.objectDisplay(o.O.Kind).Name
}

// IsUnlootedContainer is what auto explore's Open Containers option bumps.
func (o *Prop) IsUnlootedContainer() bool {
	return o.O.Active && o.f.objectDisplay(o.O.Kind).Container
}

func (o *Prop) broken() bool {
	return !o.O.Active && (o.O.Kind == rules.ObjBARREL || o.O.Kind == rules.ObjBARRELEX)
}

func isCrux(kind int) bool { return kind >= rules.ObjCRUX1 && kind <= rules.ObjCRUX3 }

func (o *Prop) iconName() string {
	name := defs.PropIconID(o.f.Generator.D.Objects[o.O.Kind].Name)
	if !o.O.Active && o.O.Kind != rules.ObjBLOODFTN && o.O.Kind != rules.ObjPURIFYINGFTN { // used objects have their own icon
		return name + ".used"
	}
	return name
}

func (o *Prop) Icon(uint64) int32 {
	if o.broken() {
		return defs.Icon("prop.debris")
	}
	return defs.Icon(o.iconName())
}

// UsedTint grays out a used/opened/searched object whose ".used" icon is the same tile as its fresh one.
var UsedTint = color.RGBA{R: 120, G: 120, B: 120, A: 255}

// TintColor is UsedTint when the used icon can't be told apart from the fresh one, white otherwise.
func (o *Prop) TintColor() color.RGBA {
	if name, used := strings.CutSuffix(o.iconName(), ".used"); used && !o.broken() && defs.Icon(name) == defs.Icon(name+".used") {
		return UsedTint
	}
	return defs.White
}

func (o *Prop) MapPosition() geometry.Point     { return o.pos }
func (o *Prop) SetMapPosition(p geometry.Point) { o.pos = p }
func (o *Prop) IsWalkable(*Actor) bool {
	return o.broken() || o.O.Kind == rules.ObjMCIRCLE1 || o.O.Kind == rules.ObjMCIRCLE2
}
func (o *Prop) IsTransparent() bool           { return true }
func (o *Prop) IsPassableForProjectile() bool { return true }
func (o *Prop) Turn()                         {}
func (o *Prop) ActorWalkedOn(actor *Actor) {
	if actor == o.f.State.GetPlayer() && (o.O.Kind == rules.ObjMCIRCLE1 || o.O.Kind == rules.ObjMCIRCLE2) {
		o.f.magicCircle(o, actor)
	}
}

func (o *Prop) ActorBumpedInto(actor *Actor) {
	if actor == o.f.State.GetPlayer() && actor.hero != nil {
		o.f.operateObject(o, actor)
	}
}

// UsesTurn is true when bumping into the object operates it, which takes the hero a turn; decoration and walkable
// debris don't.
func (o *Prop) UsesTurn(actor *Actor) bool {
	return actor.hero != nil && o.f.Generator.D.Objects[o.O.Kind].SelFlag != 0 && !o.IsWalkable(actor)
}

func doorAt(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point) (*Door, bool) {
	if !m.Contains(p) {
		return nil, false
	}
	obj, _ := m.TryGetObjectAt(p)
	door, ok := obj.(*Door)
	return door, ok
}

func objectAt(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point) *Prop {
	if !m.Contains(p) {
		return nil
	}
	obj, _ := m.TryGetObjectAt(p)
	o, _ := obj.(*Prop)
	return o
}

// lightRadius is D1's decorative light radius for braziers vs. torches (used by glowAt).
func lightRadius(icon int32) int {
	if icon == defs.IconWallTorch {
		return 8
	}
	return 5
}

// wallLight is a decorative light: a solid brazier on the floor, or (on a wall face) a torch decal. Not
// interactive, like D1's own oLightFlag-only objects.
type wallLight struct {
	pos     geometry.Point
	icon    int32
	onFloor bool
}

func (o *wallLight) Name() string                    { return "" }
func (o *wallLight) ID() string                      { return "" }
func (o *wallLight) MapPosition() geometry.Point     { return o.pos }
func (o *wallLight) SetMapPosition(p geometry.Point) { o.pos = p }
func (o *wallLight) Icon(uint64) int32               { return o.icon }
func (o *wallLight) IsWalkable(*Actor) bool          { return !o.onFloor }
func (o *wallLight) IsTransparent() bool             { return true }
func (o *wallLight) IsPassableForProjectile() bool   { return true }
func (o *wallLight) Turn()                           {}
func (o *wallLight) ActorWalkedOn(*Actor)            {}
func (o *wallLight) ActorBumpedInto(*Actor)          {}

// placeObject renders one of D1's generated objects: a wall light as a decal or decorative fixture, a
// pre-spawned sarcophagus/barrel skeleton resolved to its monster type from the level's (still-hidden) monster
// list, everything else as an interactive Prop.
func placeObject(f *Factory, m *gridmap.GridMap[*Actor, *Item, Object], lvl *dungen.Level, o *rules.Object, p geometry.Point) {
	if lvl.CellSolid(p.X, p.Y) && f.Generator.D.Objects[o.Kind].SelFlag == 0 && !f.objectDisplay(o.Kind).WallLight { // hangs on a wall
		m.AddDecal(p, defs.Icon(defs.PropIconID(f.Generator.D.Objects[o.Kind].Name)))
		return
	}
	if f.objectDisplay(o.Kind).WallLight {
		icon := defs.IconBrazier
		onFloor := !lvl.CellSolid(p.X, p.Y)
		if o.Kind != rules.ObjL1LIGHT {
			icon = defs.IconWallTorch
		}
		if onFloor {
			if !m.CanPlaceActorHere(p) || m.IsItemAt(p) {
				return
			}
			m.AddObject(&wallLight{pos: p, icon: icon, onFloor: true}, p)
		} else {
			m.AddDecal(p, icon)
		}
		return
	}
	preSpawn := -1
	slot := -1
	switch o.Kind {
	case rules.ObjSARC:
		slot = o.Var2
	case rules.ObjBARREL, rules.ObjBARRELEX:
		slot = o.Var4
	}
	if slot >= 0 && slot < len(lvl.Monsters) && lvl.Monsters[slot] != nil {
		preSpawn = lvl.Monsters[slot].Type
	}
	if o.AnimDisabled { // a second armor stand or weapon rack of its kind stands empty (_oSelFlag 0)
		o.Active = false
	}
	m.AddObject(&Prop{O: o, f: f, pos: p, PreSpawnType: preSpawn}, p)
}

// operateObject ports OperateObject/BreakObject for the player.
// ponytail: operating takes no time, like H&H's other bump objects.
func (f *Factory) operateObject(o *Prop, player *Actor) {
	obj, gen, st := o.O, f.Generator, player.GetStats()
	if gen.D.Objects[obj.Kind].SelFlag == 0 { // decoration: nothing to operate
		return
	}
	dlvl := f.State.CurrentLevel().Depth
	ilvl, hell := f.State.ItemLevel(dlvl), f.Levels.LevelType(dlvl) == rules.DTypeHell
	drop := func(it *rules.Item) {
		if it != nil {
			f.State.AddItemToMapWithDistribution(f.NewRolledItem(it), o.pos)
		}
	}
	msg := f.Msg
	switch obj.Kind { // fountains that can be used again and again
	case rules.ObjBLOODFTN, rules.ObjPURIFYINGFTN:
		f.PlaySfx("LS_FOUNTAIN", o.pos)
		res := st.GetResource(stats.Health)
		if obj.Kind == rules.ObjPURIFYINGFTN {
			res = st.GetResource(stats.Mana)
		}
		if res.IsAtMax() {
			msg(fmt.Sprintf("You drink from the %s, but nothing happens.", o.Name()))
			return
		}
		res.Increase(1)
		msg(fmt.Sprintf("You drink from the %s.", o.Name()))
		return
	}
	if f.operateQuestObject(o, player) {
		return
	}
	if obj.Kind == rules.ObjSTORYBOOK { // OperateStoryBook: the narrator reads it, as often as the hero likes
		obj.Active = false
		f.PlaySfx("IS_ISCROL", o.pos)
		f.say(o.Name(), fmt.Sprintf("TEXT_BOOK%d%d", obj.Var1+1, obj.Var2+1))
		return
	}
	if !obj.Active {
		return
	}
	obj.Active = false
	f.PlaySfx(f.objectDisplay(obj.Kind).Sfx, o.pos)
	if trapTargets[obj.Kind] { // Obj_Trap notices the used object on the next tick
		defer f.springWallTraps(f.State.GetMap(), obj.X, obj.Y)
	}
	if f.objectDisplay(obj.Kind).Container { // looting that yields no item and no monster still says so
		m := f.State.GetMap()
		items, actors := len(m.Items()), len(m.Actors())
		defer func() {
			if len(m.Items()) == items && len(m.Actors()) == actors {
				msg(fmt.Sprintf("The %s is empty.", o.Name()))
			}
		}()
	}
	switch obj.Kind {
	case rules.ObjCHEST1, rules.ObjCHEST2, rules.ObjCHEST3, rules.ObjTCHEST1, rules.ObjTCHEST2, rules.ObjTCHEST3:
		setLevel := f.State.setQuest(f.State.GetMap().GetName()) >= 0
		for j := 0; j < obj.Var1; j++ {
			if setLevel { // OperateChest: a quest level's chest only holds good items
				drop(gen.FloorItem(ilvl, true, hell))
			} else if obj.Var2 != 0 {
				drop(gen.FloorItem(ilvl, false, hell))
			} else {
				drop(gen.UsefulItem(ilvl))
			}
		}
		f.springChestTrap(o, player)
	case rules.ObjSARC:
		if obj.Var1 <= 2 {
			drop(gen.FloorItem(ilvl, false, hell))
		}
		if obj.Var1 >= 8 {
			f.skeleton(o.PreSpawnType, o.pos, player)
		}
	case rules.ObjBARREL, rules.ObjBARRELEX:
		f.breakBarrel(o, player)
	case rules.ObjCRUX1, rules.ObjCRUX2, rules.ObjCRUX3:
		f.breakCrux(o)
	case rules.ObjSHRINEL, rules.ObjSHRINER:
		f.shrine(player, obj.Var1, o.pos)
	case rules.ObjGOATSHRINE, rules.ObjCAULDRON:
		f.shrine(player, gen.RandomShrine(dlvl), o.pos)
	case rules.ObjSKELBOOK, rules.ObjBOOKSTAND:
		if rng.Intn(5) != 0 {
			drop(gen.TypeItem(ilvl, rules.ITypeMisc, "IMISC_SCROLL", false))
		} else {
			drop(gen.TypeItem(ilvl, rules.ITypeMisc, "IMISC_BOOK", false))
		}
	case rules.ObjBOOKCASEL, rules.ObjBOOKCASER:
		drop(gen.TypeItem(ilvl, rules.ITypeMisc, "IMISC_BOOK", false))
	case rules.ObjDECAP:
		drop(gen.FloorItem(ilvl, false, hell))
	case rules.ObjSLAINHERO: // OperateSlainHero: a class's own reward, and the hero's line about it
		switch book, item := f.Generator.D.SlainHeroReward(player.hero.Class); {
		case book != 0:
			drop(gen.SpellBook(ilvl, book))
		case item != nil:
			drop(gen.MagicItemWithCurs(ilvl, item.Type, item.Curs))
		}
		f.PlayHeroSound(9)
	case rules.ObjARMORSTAND, rules.ObjWARARMOR:
		unique := rng.Intn(2) != 0
		switch {
		case dlvl <= 5:
			drop(gen.TypeItem(ilvl, rules.ITypeLArmor, "", true))
		case dlvl <= 9:
			drop(gen.TypeItem(ilvl, rules.ITypeMArmor, "", unique))
		case dlvl <= 12:
			drop(gen.TypeItem(ilvl, rules.ITypeHArmor, "", false))
		default:
			drop(gen.TypeItem(ilvl, rules.ITypeHArmor, "", true))
		}
	case rules.ObjWEAPONRACK, rules.ObjWARWEAP:
		t := [4]int{rules.ITypeSword, rules.ITypeAxe, rules.ITypeBow, rules.ITypeMace}[rng.Intn(4)]
		drop(gen.TypeItem(ilvl, t, "", dlvl > 4))
	case rules.ObjMURKYFTN:
		player.AddStatusEffect(StatusInfravision, rules.InfravisionTurns(2*((dlvl-1)/4+1)))
		msg("You drink from the Murky Pool. You can see in the dark.")
	case rules.ObjLAZSTAND:
		drop(gen.PlainItem(gen.D.Item(rules.ItemLazStaff)))
		msg("The Staff of Lazarus falls from its stand.")
	case rules.ObjLEVER, rules.ObjSWITCHSKL:
		msg("You pull the lever. Something rumbles in the distance.")
		f.State.QuestsChanged(f.State.GetMap())
	case rules.ObjTEARFTN:
		from := rng.Intn(4)
		to := rng.Intn(3)
		if to >= from {
			to++
		}
		f.modifyStat(player, from, -1)
		f.modifyStat(player, to, 1)
		msg("You drink from the Fountain of Tears.")
	}
}

// barrelSfx is BreakBarrel's sound: a barrel breaking or blowing up.
func barrelSfx(kind int) string {
	if kind == rules.ObjBARRELEX {
		return "IS_BARLFIRE"
	}
	return "IS_BARREL"
}

// SpawnSkeleton ports M_SpawnSkel for the Skeleton King: one of the level's skeletons rises next to pos.
func (f *Factory) SpawnSkeleton(pos geometry.Point) {
	mt := f.Generator.D.MonsterByTag("raised_by_default")
	for _, a := range f.State.GetMap().Actors() {
		if mon := a.monster; mon != nil && mon.Unique == nil && mon.Data.Has("raisable") {
			mt = mon.Type
			break
		}
	}
	f.skeleton(mt, pos, f.State.GetPlayer())
}

func (f *Factory) skeleton(mt int, pos geometry.Point, player *Actor) {
	m := f.State.GetMap()
	if mt < 0 {
		return
	}
	cells := freeCellsNear(m, pos, 1, m.CanPlaceActorHere)
	if len(cells) == 0 {
		return
	}
	a := f.NewMonster(mt)
	a.monster.SpawnIndex = -1 // saved in LevelSnapshot.Spawned
	a.SetDrawPosition(cells[0].ToPointF())
	m.AddActor(a, cells[0])
	f.State.OnActorSpawned(a)
	f.ActionBroker.Execute(NewUpdateFoVAction(AbstractCause("Skeleton raised"), player)) // tab targets it right away
	alert(a, player)
}

// SpawnRandomUnique places a random unique monster next to pos, for the "Spawn random unique" debug command.
// nil if there's nowhere free to put it.
func (f *Factory) SpawnRandomUnique(pos geometry.Point) *Actor {
	m := f.State.GetMap()
	cells := freeCellsNear(m, pos, 2, m.CanPlaceActorHere)
	if len(cells) == 0 {
		return nil
	}
	var pickable []int
	for i, um := range f.Generator.D.UniqueMonsters {
		if !f.Generator.D.MonsterAIs[um.AI].NeedsQuest {
			pickable = append(pickable, i)
		}
	}
	if len(pickable) == 0 { // a mod without fighting uniques
		return nil
	}
	a := f.NewUniqueMonster(pickable[rng.Intn(len(pickable))])
	a.monster.SpawnIndex = -1 // saved in LevelSnapshot.Spawned
	a.SetDrawPosition(cells[0].ToPointF())
	m.AddActor(a, cells[0])
	f.State.OnActorSpawned(a)
	return a
}

// IsBarrel reports an intact barrel at p. A ranged attack that lands on the cell breaks it (hitObject), so a
// left-click on one with a bow fires at it instead of walking over.
func IsBarrel(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point) bool {
	o := objectAt(m, p)
	return o != nil && o.O.Active && (o.O.Kind == rules.ObjBARREL || o.O.Kind == rules.ObjBARRELEX)
}

// hitObject breaks an intact barrel or crucified skeleton at p (_oBreak 1): anything that hits its cell does,
// like D1's missile collisions (BreakObject).
func (f *Factory) hitObject(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point, attacker *Actor) {
	o := objectAt(m, p)
	switch {
	case o == nil || !o.O.Active:
	case o.O.Kind == rules.ObjBARREL || o.O.Kind == rules.ObjBARRELEX:
		f.breakBarrel(o, attacker)
	case isCrux(o.O.Kind) && m == f.State.GetMap():
		f.breakCrux(o)
	}
}

// breakCrux ports BreakCrux: once every crucified skeleton of its group is down, its part of the map opens.
func (f *Factory) breakCrux(o *Prop) {
	o.O.Active = false
	f.PlaySfx("IS_LEVER", o.pos)
	if m := f.State.GetMap(); objectGroupUsed(m, isCrux, o.O.Var8) {
		f.Msg("Something rumbles in the distance.")
		f.State.QuestsChanged(m) // levels.rec's "group" patch opens its range
	}
}

// blastAction shows an exploding barrel's fire; the damage is queued as separate actions.
type blastAction struct {
	BaseAction
	cells []geometry.Point // the fire, shown when the queue gets to it
}

func (a *blastAction) Execute(ctx ActionContext) []Action {
	for _, p := range a.cells {
		ctx.ActionBroker.Emit(Flash{At: p, Seconds: 0.5, Icon: defs.IconEffectFireBig})
	}
	return nil
}

// breakBarrel ports BreakBarrel: a normal barrel holds an item or a skeleton, an exploding one burns its 3×3 area
// and sets off exploding barrels next to it.
func (f *Factory) breakBarrel(o *Prop, player *Actor) {
	obj, gen, m := o.O, f.Generator, f.State.GetMap()
	obj.Active = false // the debris stays, walkable
	f.PlaySfx(barrelSfx(obj.Kind), o.pos)
	if obj.Kind == rules.ObjBARREL {
		dlvl := f.State.CurrentLevel().Depth
		ilvl, hell := f.State.ItemLevel(dlvl), f.Levels.LevelType(dlvl) == rules.DTypeHell
		if obj.Var2 <= 1 {
			it := gen.UsefulItem(ilvl)
			if obj.Var3 != 0 {
				it = gen.FloorItem(ilvl, false, hell)
			}
			if it != nil {
				f.State.AddItemToMapWithDistribution(f.NewRolledItem(it), o.pos)
			}
		}
		if obj.Var2 >= 8 {
			f.skeleton(o.PreSpawnType, o.pos, player)
		}
		return
	}
	cause := NewActorCause("Exploding barrel", player)
	blast := &blastAction{BaseAction: BaseAction{cause: cause, name: "ActionNameBarrelExplosion"}}
	for y := o.pos.Y - 1; y <= o.pos.Y+1; y++ {
		for x := o.pos.X - 1; x <= o.pos.X+1; x++ {
			if p := (geometry.Point{X: x, Y: y}); m.Contains(p) && m.IsTransparent(p) {
				blast.cells = append(blast.cells, p)
			}
		}
	}
	f.ActionBroker.QueueForExecution(blast)
	for y := o.pos.Y - 1; y <= o.pos.Y+1; y++ {
		for x := o.pos.X - 1; x <= o.pos.X+1; x++ {
			p := geometry.Point{X: x, Y: y}
			if a, ok := m.TryGetActorAt(p); ok && a.IsAlive() {
				dam := 0
				switch {
				case a.hero != nil && rng.Intn(100) < 40: // PlayerMHit with a fire missile from no monster
					dam64 := rules.Fixed(8) + rng.Intn(rules.Fixed(8)+1)
					if fr := a.HeroSheet().FR; fr > 0 { // PlayerMHit: a resisted hit doesn't flinch
						dam64 -= dam64 * fr / 100
					} else {
						StartPlrHit(f, a, dam64, rules.Whole(dam64), false)
					}
					dam = rules.Whole(dam64)
				case a.monster != nil: // MonsterTrapHit
					mon := a.monster
					if mon.MagicRes&rules.ImmuneFire != 0 || rng.Intn(100) >= min(max(90-mon.AC, 5), 95) {
						break
					}
					dam = rng.Intn(4) + 1
					if mon.MagicRes&rules.ResistFire != 0 {
						dam >>= 2
					}
					alert(a, player)
				}
				if dam > 0 {
					f.ActionBroker.QueueForExecution(NewChangeResourceAction(cause, a, stats.Health, -float64(dam)))
				}
			}
			if other := objectAt(m, p); other != nil && other.O.Kind == rules.ObjBARRELEX && other.O.Active {
				f.breakBarrel(other, player)
			}
		}
	}
}

// modifyStat ports ModifyPlr{Str,Mag,Dex,Vit} and CheckStats.
func (f *Factory) modifyStat(player *Actor, attr, n int) {
	dhp, dmana := player.hero.AddBaseStat(attr, n)
	SyncHero(player)
	player.GetStats().GetResource(stats.Health).Increase(float64(rules.Whole(dhp)))
	player.GetStats().GetResource(stats.Mana).Increase(float64(rules.Whole(dmana)))
}

// ownedItems lists the player's D1 items: equipped first, then the pack.
func ownedItems(player *Actor) (equipped, pack []*Item) {
	eq, inv := player.GetEquipment(), player.GetInventory()
	for _, it := range eq.AllItems() {
		if it.roll != nil {
			equipped = append(equipped, it)
		}
	}
	for _, it := range inv.Items() {
		if it.roll != nil && !eq.IsEquipped(it) {
			pack = append(pack, it)
		}
	}
	return
}

func isWeapon(it *rules.Item) bool {
	switch it.Type {
	case rules.ITypeSword, rules.ITypeAxe, rules.ITypeBow, rules.ITypeMace, rules.ITypeStaff:
		return true
	}
	return false
}

func isArmor(it *rules.Item) bool {
	switch it.Type {
	case rules.ITypeShield, rules.ITypeLArmor, rules.ITypeHelm, rules.ITypeMArmor, rules.ITypeHArmor:
		return true
	}
	return false
}

// ShrineInfoText describes a shrine's effect for the right-click description popup, analogous to
// d1MonsterInfoText. false for anything else, including a goat shrine/cauldron, whose effect is rolled fresh
// each time it's touched rather than fixed at level generation.
func ShrineInfoText(o *Prop) ([]string, bool) {
	switch o.O.Kind {
	case rules.ObjSHRINEL, rules.ObjSHRINER:
		shrine := o.O.Var1
		return []string{o.f.Generator.D.ShrineNames[shrine] + " Shrine", "", o.f.Generator.D.ShrineMessages[shrine]}, true
	case rules.ObjGOATSHRINE, rules.ObjCAULDRON:
		return []string{o.Name(), "", "Its effect is chosen at random when touched."}, true
	}
	return nil, false
}

// shrine ports OperateShrine for single player.
func (f *Factory) shrine(player *Actor, shrine int, pos geometry.Point) {
	h, gen, m, st := player.hero, f.Generator, f.State.GetMap(), player.GetStats()
	dlvl := f.State.CurrentLevel().Depth
	ilvl, lt := f.State.ItemLevel(dlvl), (dlvl-1)/4+1
	hp, mana := st.GetResource(stats.Health), st.GetResource(stats.Mana)
	equipped, pack := ownedItems(player)
	all := append(append([]*Item(nil), equipped...), pack...)
	learn := func(spell int) { // Fascinating, Sacred, Ornate: +2 spell levels for 10% of base mana
		h.SpellLevels[spell] = min(h.SpellLevels[spell]+2, 15)
		cut := h.MaxManaBase / 10
		h.MaxManaBase -= cut
		h.ManaBase = max(h.ManaBase-cut, 0)
		SyncHero(player)
		mana.Decrease(rules.Whole(cut))
	}
	switch shrine {
	case 0: // Mysterious
		for attr := rules.AttrStr; attr <= rules.AttrVit; attr++ {
			f.modifyStat(player, attr, -1)
		}
		f.modifyStat(player, rng.Intn(4), 6)
	case 1: // Hidden
		var durable []*Item
		for _, it := range equipped {
			if d := it.roll; d.MaxDur != 0 && d.MaxDur != 255 {
				d.Durability, d.MaxDur = min(d.Durability+10, d.MaxDur+10), d.MaxDur+10
				durable = append(durable, it)
			}
		}
		if len(durable) > 0 {
			d := durable[rng.Intn(len(durable))].roll
			d.Durability, d.MaxDur = max(d.Durability-20, 1), max(d.MaxDur-20, 1)
		}
	case 2: // Gloomy
		for _, it := range all {
			if d := it.roll; isArmor(d) {
				d.AC += 2
			} else if isWeapon(d) {
				d.MaxDam = max(d.MaxDam-1, d.MinDam)
			}
		}
	case 3: // Weird
		for _, it := range all {
			if isWeapon(it.roll) {
				it.roll.MaxDam++
			}
		}
	case 4, 11: // Magical
		raiseManaShield(player, 2*lt) // AddMissile(..., MIS_MANASHIELD, ..., 2 * leveltype)
	case 5: // Stone
		for _, it := range all {
			if it.roll.Type == rules.ITypeStaff {
				it.roll.Charges = it.roll.MaxCharges
			}
		}
	case 6: // Religious
		for _, it := range all {
			it.roll.Durability = it.roll.MaxDur
		}
	case 7: // Enchanted
		var known []int
		for s, l := range h.SpellLevels {
			if l > 0 {
				known = append(known, s)
			}
		}
		if len(known) > 1 {
			for _, s := range known {
				h.SpellLevels[s] = min(h.SpellLevels[s]+1, 15)
			}
			s := known[rng.Intn(len(known))]
			h.SpellLevels[s] = max(h.SpellLevels[s]-2, 0)
		}
	case 8: // Thaumaturgic
		for _, obj := range m.Objects() {
			if o, ok := obj.(*Prop); ok && o.O.Kind >= rules.ObjCHEST1 && o.O.Kind <= rules.ObjCHEST3 {
				o.O.Active = true
			}
		}
	case 9: // Fascinating
		learn(rules.SplFirebolt)
	case 10: // Cryptic
		f.ActionBroker.Execute(NewSpellAction(player, rules.SplNova, 2*lt, player.MapPosition()))
		mana.SetToMax()
	case 12: // Eldritch
		for _, it := range pack {
			rejuv := map[string]string{"IMISC_HEAL": "IMISC_REJUV", "IMISC_MANA": "IMISC_REJUV",
				"IMISC_FULLHEAL": "IMISC_FULLREJUV", "IMISC_FULLMANA": "IMISC_FULLREJUV"}[it.roll.MiscID]
			if rejuv == "" {
				continue
			}
			for i := range gen.D.Items {
				if b := &gen.D.Items[i]; b.Drop != "IDROP_NEVER" && b.MiscID == rejuv {
					player.GetInventory().Remove(it)
					player.GetInventory().Add(f.NewRolledItem(gen.PlainItem(b)))
					break
				}
			}
		}
	case 13: // Eerie
		f.modifyStat(player, rules.AttrMag, 2)
	case 14: // Divine
		drop := func(misc string) {
			if it := gen.TypeItem(ilvl, rules.ITypeMisc, misc, false); it != nil {
				f.State.AddItemToMapWithDistribution(f.NewRolledItem(it), pos)
			}
		}
		if 2*dlvl < 7 {
			drop("IMISC_FULLMANA")
			drop("IMISC_FULLHEAL")
		} else {
			drop("IMISC_FULLREJUV")
			drop("IMISC_FULLREJUV")
		}
		hp.SetToMax()
		mana.SetToMax()
	case 15: // Holy
		f.randomTeleport(player)
	case 16: // Sacred
		learn(rules.SplChargedBolt)
	case 17: // Spiritual
		inv := player.GetInventory()
		for n := inv.maxItemStacks - len(inv.StackedItems()); n > 0; n-- {
			st.IncrementBy(stats.Gold, 5*lt+rng.Intn(10*lt))
		}
	case 19: // Abandoned
		f.modifyStat(player, rules.AttrDex, 2)
	case 20: // Creepy
		f.modifyStat(player, rules.AttrStr, 2)
	case 21: // Quiet
		f.modifyStat(player, rules.AttrVit, 2)
	case 22: // Secluded
		m.SetAllExplored()
	case 23: // Ornate
		learn(rules.SplHolyBolt)
	case 24: // Glimmering
		for _, it := range all {
			if it.roll.Magical != 0 && !it.roll.Identified {
				it.roll.Identified = true
				f.ActionBroker.Emit(ItemIdentified{Item: it.roll})
			}
		}
	case 26: // Oily
		for _, a := range h.Rules().Class(h.Class).Oily {
			f.modifyStat(player, a[0], a[1])
		}
		f.ActionBroker.QueueForExecution(NewTrapMissileAction(pos, MisFireWall, player.MapPosition()))
	case 27: // Glowing
		gain, keep := h.Exp/1000, 0
		if h.Exp > 5000 {
			gain, keep = 5, int(float64(h.Exp)*0.95)
		}
		f.modifyStat(player, rules.AttrMag, gain)
		h.Exp = keep
	case 28: // Mendicant's
		gold := st.GetAttribute(stats.Gold) / 2
		h.AddExperience(h.Level, gold)
		st.DecrementBy(stats.Gold, gold)
	case 29: // Sparkling
		h.AddExperience(h.Level, 1000*dlvl)
		f.ActionBroker.QueueForExecution(NewTrapMissileAction(pos, MisFlashTrap, player.MapPosition()))
	case 30: // Town
		f.ActionBroker.Execute(NewSpellAction(player, rules.SplTown, 0, player.MapPosition()))
	case 31: // Shimmering
		h.ManaBase = h.MaxManaBase
		mana.SetToMax()
	case 32: // Solar
		switch hour := Clock().Hour(); {
		case hour > 20 || hour < 4:
			f.modifyStat(player, rules.AttrVit, 2)
		case hour > 18:
			f.modifyStat(player, rules.AttrMag, 2)
		case hour > 12:
			f.modifyStat(player, rules.AttrStr, 2)
		case hour > 4:
			f.modifyStat(player, rules.AttrDex, 2)
		}
	case 33: // Murphy's
		broke := false
		for _, it := range equipped {
			if d := it.roll; rng.Intn(3) == 0 && d.Durability != rules.DurIndestructible && d.Durability != 0 {
				d.Durability /= 2
				broke = true
				if d.Durability == 0 {
					itemBroke(f.ActionBroker, f.Msg, player, d)
				}
				break
			}
		}
		if !broke {
			st.DecrementBy(stats.Gold, st.GetAttribute(stats.Gold)/3)
		}
	}
	for _, it := range all {
		it.Refresh()
	}
	SyncHero(player)
	f.RefreshSpells(player)
	f.Msg(f.Generator.D.ShrineMessages[shrine])
}
