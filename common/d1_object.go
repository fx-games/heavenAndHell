package common

import (
	"fmt"
	"image/color"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"strings"
	"time"
)

// D1Object is a D1 dungeon object (decision 12), operated by bumping into it.
type D1Object struct {
	O            *d1.Object
	f            *Factory
	pos          geometry.Point
	Disarmed     bool // the rogue disarmed its trap (TryDisarm), kept in saves
	PreSpawnType int  // -1, or the monster type a sarcophagus/barrel raises when opened (d1.Object's Var2/Var4 slot,
	// resolved to a type at placement time since the monster stays hidden off-map until then)
}

// d1ObjectNames are short display names for the object kinds decision 12 keeps (icons.rec has no display names).
var d1ObjectNames = map[int]string{
	d1.ObjCHEST1: "Small Chest", d1.ObjCHEST2: "Chest", d1.ObjCHEST3: "Large Chest",
	d1.ObjTCHEST1: "Small Chest", d1.ObjTCHEST2: "Chest", d1.ObjTCHEST3: "Large Chest",
	d1.ObjSARC: "Sarcophagus", d1.ObjBARREL: "Barrel", d1.ObjBARRELEX: "Barrel",
	d1.ObjSHRINEL: "Shrine", d1.ObjSHRINER: "Shrine", d1.ObjSKELBOOK: "Skeleton Tome",
	d1.ObjBOOKCASEL: "Bookcase", d1.ObjBOOKCASER: "Bookcase", d1.ObjBOOKSTAND: "Library Book",
	d1.ObjBLOODFTN: "Blood Fountain", d1.ObjDECAP: "Decapitated Body", d1.ObjPURIFYINGFTN: "Purifying Spring",
	d1.ObjARMORSTAND: "Armor", d1.ObjGOATSHRINE: "Goat Shrine", d1.ObjCAULDRON: "Cauldron",
	d1.ObjMURKYFTN: "Murky Pool", d1.ObjTEARFTN: "Fountain of Tears", d1.ObjWEAPONRACK: "Weapon Rack",
	d1.ObjLEVER: "Lever", d1.ObjLAZSTAND: "Staff Stand", d1.ObjSWITCHSKL: "Skull Lever", d1.ObjMUSHPATCH: "Mushroom Patch",
	d1.ObjSIGNCHEST: "Large Chest", d1.ObjBLINDBOOK: "Book of the Blind", d1.ObjBLOODBOOK: "Book of Blood",
	d1.ObjSTEELTOME: "Steel Tome", d1.ObjPEDISTAL: "Pedestal of Blood", d1.ObjBOOK2R: "Mythical Book",
	d1.ObjBOOK2L: "Ancient Tome", d1.ObjSLAINHERO: "Slain Hero", d1.ObjWARARMOR: "Armor", d1.ObjWARWEAP: "Weapon Rack",
	// decoration (D1 shows no name for these)
	d1.ObjSKFIRE: "Skull Fire", d1.ObjCANDLE1: "Candle", d1.ObjCANDLE2: "Candle", d1.ObjCANDLEO: "Candle",
	d1.ObjBOOKCANDLE: "Candle", d1.ObjSTORYCANDLE: "Candle", d1.ObjBANNERL: "Banner", d1.ObjBANNERM: "Banner",
	d1.ObjBANNERR: "Banner", d1.ObjSKPILE: "Skull Pile", d1.ObjSKSTICK1: "Skull on a Stick", d1.ObjSKSTICK2: "Skull on a Stick",
	d1.ObjSKSTICK3: "Skull on a Stick", d1.ObjSKSTICK4: "Skull on a Stick", d1.ObjSKSTICK5: "Skull on a Stick",
	d1.ObjCRUX1: "Crucified Skeleton", d1.ObjCRUX2: "Crucified Skeleton", d1.ObjCRUX3: "Crucified Skeleton",
	d1.ObjSTAND: "Stand", d1.ObjANGEL: "Angel Statue", d1.ObjBCROSS: "Burning Cross", d1.ObjTBCROSS: "Burning Cross",
	d1.ObjNUDEW2R: "Tortured Body", d1.ObjTNUDEM1: "Tortured Body", d1.ObjTNUDEM2: "Tortured Body", d1.ObjTNUDEM3: "Tortured Body",
	d1.ObjTNUDEM4: "Tortured Body", d1.ObjTNUDEW1: "Tortured Body", d1.ObjTNUDEW2: "Tortured Body", d1.ObjTNUDEW3: "Tortured Body",
	d1.ObjTORTURE1: "Hooked Body", d1.ObjTORTURE2: "Hooked Body", d1.ObjTORTURE3: "Hooked Body", d1.ObjTORTURE4: "Hooked Body",
	d1.ObjTORTURE5: "Hooked Body", d1.ObjBOOKSHELF: "Bookshelf", d1.ObjWEAPRACK: "Weapon Stand", d1.ObjARMORSTANDN: "Armor Stand",
	d1.ObjWEAPONRACKN: "Weapon Rack", d1.ObjALTBOY: "Altar", d1.ObjMCIRCLE1: "Magic Circle", d1.ObjMCIRCLE2: "Magic Circle",
	d1.ObjFLAMEHOLE: "Flame Hole", d1.ObjFLAMELVR: "Lever", d1.ObjWATER: "Water", d1.ObjBOOKLVR: "Book",
}

// d1ObjectLights are the light radii of the decorative objects that glow (ProcessObjects' Obj_Light).
var d1ObjectLights = map[int]int{d1.ObjSKFIRE: 5, d1.ObjCANDLE2: 5, d1.ObjBOOKCANDLE: 5, d1.ObjSTORYCANDLE: 3,
	d1.ObjBCROSS: 10, d1.ObjTBCROSS: 10}

// d1StoryBookNames ports StoryBookName, indexed by a story book's Var3.
var d1StoryBookNames = [...]string{"The Great Conflict", "The Wages of Sin are War", "The Tale of the Horadrim",
	"The Dark Exile", "The Sin War", "The Binding of the Three", "The Realms Beyond", "Tale of the Three", "The Black King",
	"Journal: The Ensorcellment", "Journal: The Meeting", "Journal: The Tirade", "Journal: His Power Grows",
	"Journal: NA-KRUL", "Journal: The End", "A Spellbook"} // Hellfire's Crypt journals and Na-Krul's books

// d1NakrulBookText is the first of Na-Krul's three spell books' texts (Var8 6-8) by the reader's class.
var d1NakrulBookText = [d1.NumClasses]string{d1.Warrior: "TEXT_BOOKA", d1.Rogue: "TEXT_RBOOKA", d1.Sorcerer: "TEXT_MBOOKA",
	d1.Monk: "TEXT_OBOOKA", d1.Bard: "TEXT_BBOOKA", d1.Barbarian: "TEXT_BOOKA"}

// d1Containers are the objects that are looted: opening or breaking them may give nothing.
var d1Containers = map[int]bool{d1.ObjCHEST1: true, d1.ObjCHEST2: true, d1.ObjCHEST3: true,
	d1.ObjTCHEST1: true, d1.ObjTCHEST2: true, d1.ObjTCHEST3: true, d1.ObjSARC: true, d1.ObjBARREL: true,
	d1.ObjSKELBOOK: true, d1.ObjBOOKCASEL: true, d1.ObjBOOKCASER: true, d1.ObjBOOKSTAND: true, d1.ObjDECAP: true,
	d1.ObjARMORSTAND: true, d1.ObjWEAPONRACK: true, d1.ObjWARARMOR: true, d1.ObjWARWEAP: true}

func (o *D1Object) Name() string {
	if player := o.f.State.GetPlayer(); o.O.TrapFlag && player != nil && player.hero != nil && d1.HasClassFlag(player.hero.Class, d1.ClassTrapSense) {
		return "Trapped " + o.name() // GetObjectStr: only a rogue's trap sense sees it
	}
	return o.name()
}

func (o *D1Object) name() string {
	if k := o.O.Kind; k == d1.ObjSHRINEL || k == d1.ObjSHRINER {
		return d1.ShrineNames[o.O.Var1] + " Shrine"
	}
	if o.O.Kind == d1.ObjSTORYBOOK && o.O.Var3 >= 0 && o.O.Var3 < len(d1StoryBookNames) {
		return d1StoryBookNames[o.O.Var3]
	}
	if o.O.Kind == d1.ObjBOOK2L && o.f.State.GetMap().GetName() == D1LazarusLevel {
		return "Book of Vileness"
	}
	return d1ObjectNames[o.O.Kind]
}

// IsUnlootedContainer is what auto explore's Open Containers option bumps.
func (o *D1Object) IsUnlootedContainer() bool { return o.O.Active && d1Containers[o.O.Kind] }

func (o *D1Object) broken() bool {
	return !o.O.Active && (o.O.Kind == d1.ObjBARREL || o.O.Kind == d1.ObjBARRELEX)
}

func isD1Crux(kind int) bool { return kind >= d1.ObjCRUX1 && kind <= d1.ObjCRUX3 }

func (o *D1Object) iconName() string {
	name := defs.PropIconID(o.f.Generator.D.Objects[o.O.Kind].Name)
	if !o.O.Active && o.O.Kind != d1.ObjBLOODFTN && o.O.Kind != d1.ObjPURIFYINGFTN { // used objects have their own icon
		return name + ".used"
	}
	return name
}

func (o *D1Object) Icon(uint64) int32 {
	if o.broken() {
		return defs.Icon("prop.debris")
	}
	return defs.Icon(o.iconName())
}

// D1UsedTint grays out a used/opened/searched object whose ".used" icon is the same tile as its fresh one.
var D1UsedTint = color.RGBA{R: 120, G: 120, B: 120, A: 255}

// TintColor is D1UsedTint when the used icon can't be told apart from the fresh one, white otherwise.
func (o *D1Object) TintColor() color.RGBA {
	if name, used := strings.CutSuffix(o.iconName(), ".used"); used && !o.broken() && defs.Icon(name) == defs.Icon(name+".used") {
		return D1UsedTint
	}
	return defs.White
}

func (o *D1Object) MapPosition() geometry.Point     { return o.pos }
func (o *D1Object) SetMapPosition(p geometry.Point) { o.pos = p }
func (o *D1Object) IsWalkable(*Actor) bool {
	return o.broken() || o.O.Kind == d1.ObjMCIRCLE1 || o.O.Kind == d1.ObjMCIRCLE2
}
func (o *D1Object) IsTransparent() bool           { return true }
func (o *D1Object) IsPassableForProjectile() bool { return true }
func (o *D1Object) Turn()                         {}
func (o *D1Object) ActorWalkedOn(actor *Actor, _ func() bool) {
	if actor == o.f.State.GetPlayer() && (o.O.Kind == d1.ObjMCIRCLE1 || o.O.Kind == d1.ObjMCIRCLE2) {
		o.f.d1MagicCircle(o, actor)
	}
}

func (o *D1Object) ActorBumpedInto(actor *Actor, _ func() bool) {
	if actor == o.f.State.GetPlayer() && actor.hero != nil {
		o.f.operateD1Object(o, actor)
	}
}

// UsesTurn is true when bumping into the object operates it, which takes the hero a turn; decoration and walkable
// debris don't.
func (o *D1Object) UsesTurn(actor *Actor) bool {
	return actor.hero != nil && o.f.Generator.D.Objects[o.O.Kind].SelFlag != 0 && !o.IsWalkable(actor)
}

func d1DoorAt(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point) (*Door, bool) {
	if !m.Contains(p) || !m.IsObjectAt(p) {
		return nil, false
	}
	door, ok := m.ObjectAt(p).(*Door)
	return door, ok
}

func d1ObjectAt(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point) *D1Object {
	if !m.Contains(p) || !m.IsObjectAt(p) {
		return nil
	}
	o, _ := m.ObjectAt(p).(*D1Object)
	return o
}

// d1WallLightKinds are the decorative lights D1 places directly (an exact replacement for the old random-oneIn
// heuristic): Cathedral braziers standing on the floor, Catacomb torches mounted on a wall face.
var d1WallLightKinds = map[int]bool{d1.ObjL1LIGHT: true, d1.ObjTORCHL: true, d1.ObjTORCHR: true,
	d1.ObjTORCHL2: true, d1.ObjTORCHR2: true}

// d1LightRadius is D1's decorative light radius for braziers vs. torches (used by d1GlowAt).
func d1LightRadius(icon int32) int {
	if icon == defs.IconWallTorch {
		return 8
	}
	return 5
}

// d1WallLight is a decorative light: a solid brazier on the floor, or (on a wall face) a torch decal. Not
// interactive, like D1's own oLightFlag-only objects.
type d1WallLight struct {
	pos     geometry.Point
	icon    int32
	onFloor bool
}

func (o *d1WallLight) Name() string                        { return "" }
func (o *d1WallLight) MapPosition() geometry.Point         { return o.pos }
func (o *d1WallLight) SetMapPosition(p geometry.Point)     { o.pos = p }
func (o *d1WallLight) Icon(uint64) int32                   { return o.icon }
func (o *d1WallLight) IsWalkable(*Actor) bool              { return !o.onFloor }
func (o *d1WallLight) IsTransparent() bool                 { return true }
func (o *d1WallLight) IsPassableForProjectile() bool       { return true }
func (o *d1WallLight) Turn()                               {}
func (o *d1WallLight) ActorWalkedOn(*Actor, func() bool)   {}
func (o *d1WallLight) ActorBumpedInto(*Actor, func() bool) {}

// d1PlaceObject renders one of D1's generated objects: a wall light as a decal or decorative fixture, a
// pre-spawned sarcophagus/barrel skeleton resolved to its monster type from the level's (still-hidden) monster
// list, everything else as an interactive D1Object.
func d1PlaceObject(f *Factory, m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level, o *d1.Object, p geometry.Point) {
	if lvl.CellSolid(p.X, p.Y) && f.Generator.D.Objects[o.Kind].SelFlag == 0 && !d1WallLightKinds[o.Kind] { // hangs on a wall
		m.AddDecal(p, defs.Icon(defs.PropIconID(f.Generator.D.Objects[o.Kind].Name)))
		return
	}
	if d1WallLightKinds[o.Kind] {
		icon := defs.IconBrazier
		onFloor := !lvl.CellSolid(p.X, p.Y)
		if o.Kind != d1.ObjL1LIGHT {
			icon = defs.IconWallTorch
		}
		if onFloor {
			if !m.CanPlaceActorHere(p) || m.IsItemAt(p) {
				return
			}
			m.AddObject(&d1WallLight{pos: p, icon: icon, onFloor: true}, p)
		} else {
			m.AddDecal(p, icon)
		}
		return
	}
	preSpawn := -1
	slot := -1
	switch o.Kind {
	case d1.ObjSARC:
		slot = o.Var2
	case d1.ObjBARREL, d1.ObjBARRELEX:
		slot = o.Var4
	}
	if slot >= 0 && slot < len(lvl.Monsters) && lvl.Monsters[slot] != nil {
		preSpawn = lvl.Monsters[slot].Type
	}
	if o.AnimDisabled { // a second armor stand or weapon rack of its kind stands empty (_oSelFlag 0)
		o.Active = false
	}
	m.AddObject(&D1Object{O: o, f: f, pos: p, PreSpawnType: preSpawn}, p)
}

// operateD1Object ports OperateObject/BreakObject for the player.
// ponytail: operating takes no time, like H&H's other bump objects.
func (f *Factory) operateD1Object(o *D1Object, player *Actor) {
	obj, gen, st := o.O, f.Generator, player.GetStats()
	if gen.D.Objects[obj.Kind].SelFlag == 0 { // decoration: nothing to operate
		return
	}
	dlvl := d1DungeonLevel(f.State)
	ilvl, hell := d1.ItemLevel(dlvl), d1.LevelType(dlvl) == d1.DTypeHell
	drop := func(it *d1.Item) {
		if it != nil {
			f.State.AddItemToMapWithDistribution(f.NewD1Item(it), o.pos)
		}
	}
	msg := f.Msg
	switch obj.Kind { // fountains that can be used again and again
	case d1.ObjBLOODFTN, d1.ObjPURIFYINGFTN:
		res := st.GetResource(stats.Health)
		if obj.Kind == d1.ObjPURIFYINGFTN {
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
	if f.operateD1QuestObject(o, player) {
		return
	}
	if obj.Kind == d1.ObjSTORYBOOK { // OperateStoryBook: the narrator reads it, as often as the hero likes
		obj.Active = false
		if !f.d1CryptJournal(obj) {
			return
		}
		switch id := fmt.Sprintf("TEXT_BOOK%d%d", obj.Var1+1, obj.Var2+1); {
		case obj.Var8 >= 6: // Na-Krul's books: A, B or C
			id = d1NakrulBookText[player.hero.Class]
			f.say(o.Name(), id[:len(id)-1]+string(rune('A'+obj.Var8-6)))
		case obj.Var2 > 300: // the Crypt's journals, TEXT_BOOK4 (317) to TEXT_BOOK8
			f.say(o.Name(), fmt.Sprintf("TEXT_BOOK%d", obj.Var2-313))
		default:
			f.say(o.Name(), id)
		}
		return
	}
	if !obj.Active {
		return
	}
	obj.Active = false
	if d1TrapTargets[obj.Kind] { // Obj_Trap notices the used object on the next tick
		defer f.springD1WallTraps(f.State.GetMap(), obj.X, obj.Y)
	}
	if d1Containers[obj.Kind] { // looting that yields no item and no monster still says so
		m := f.State.GetMap()
		items, actors := len(m.Items()), len(m.Actors())
		defer func() {
			if len(m.Items()) == items && len(m.Actors()) == actors {
				msg(fmt.Sprintf("The %s is empty.", o.Name()))
			}
		}()
	}
	switch obj.Kind {
	case d1.ObjCHEST1, d1.ObjCHEST2, d1.ObjCHEST3, d1.ObjTCHEST1, d1.ObjTCHEST2, d1.ObjTCHEST3:
		for j := 0; j < obj.Var1; j++ {
			if obj.Var2 != 0 {
				drop(gen.FloorItem(ilvl, false, hell))
			} else {
				drop(gen.UsefulItem(ilvl))
			}
		}
		f.springD1ChestTrap(o, player)
	case d1.ObjSARC:
		if obj.Var1 <= 2 {
			drop(gen.FloorItem(ilvl, false, hell))
		}
		if obj.Var1 >= 8 {
			f.d1Skeleton(o.PreSpawnType, o.pos, player)
		}
	case d1.ObjBARREL, d1.ObjBARRELEX:
		f.breakD1Barrel(o, player)
	case d1.ObjCRUX1, d1.ObjCRUX2, d1.ObjCRUX3:
		f.breakD1Crux(o)
	case d1.ObjSHRINEL, d1.ObjSHRINER:
		f.d1Shrine(player, obj.Var1, o.pos)
	case d1.ObjGOATSHRINE, d1.ObjCAULDRON:
		f.d1Shrine(player, gen.RandomShrine(dlvl), o.pos)
	case d1.ObjSKELBOOK, d1.ObjBOOKSTAND:
		if d1Rng.Intn(5) != 0 {
			drop(gen.TypeItem(ilvl, d1.ITypeMisc, "IMISC_SCROLL", false))
		} else {
			drop(gen.TypeItem(ilvl, d1.ITypeMisc, "IMISC_BOOK", false))
		}
	case d1.ObjBOOKCASEL, d1.ObjBOOKCASER:
		drop(gen.TypeItem(ilvl, d1.ITypeMisc, "IMISC_BOOK", false))
	case d1.ObjDECAP:
		drop(gen.FloorItem(ilvl, false, hell))
	case d1.ObjSLAINHERO: // OperateSlainHero: a class's own reward, and the hero's line about it
		switch player.hero.Class {
		case d1.Warrior:
			drop(gen.MagicItemWithCurs(ilvl, d1.ITypeHArmor, d1.ICursBreastPlate))
		case d1.Rogue:
			drop(gen.MagicItemWithCurs(ilvl, d1.ITypeBow, d1.ICursLongWarBow))
		case d1.Monk:
			drop(gen.MagicItemWithCurs(ilvl, d1.ITypeStaff, d1.ICursWarStaff))
		case d1.Bard:
			drop(gen.MagicItemWithCurs(ilvl, d1.ITypeSword, d1.ICursBastardSword))
		case d1.Barbarian:
			drop(gen.MagicItemWithCurs(ilvl, d1.ITypeAxe, d1.ICursBattleAxe))
		default:
			drop(gen.SpellBook(ilvl, d1.SplLightning))
		}
		f.PlayD1HeroSound(9)
	case d1.ObjARMORSTAND, d1.ObjWARARMOR:
		unique := d1Rng.Intn(2) != 0
		switch {
		case dlvl <= 5:
			drop(gen.TypeItem(ilvl, d1.ITypeLArmor, "", true))
		case dlvl <= 9:
			drop(gen.TypeItem(ilvl, d1.ITypeMArmor, "", unique))
		case dlvl <= 12:
			drop(gen.TypeItem(ilvl, d1.ITypeHArmor, "", false))
		default:
			drop(gen.TypeItem(ilvl, d1.ITypeHArmor, "", true))
		}
	case d1.ObjWEAPONRACK, d1.ObjWARWEAP:
		t := [4]int{d1.ITypeSword, d1.ITypeAxe, d1.ITypeBow, d1.ITypeMace}[d1Rng.Intn(4)]
		drop(gen.TypeItem(ilvl, t, "", dlvl > 4))
	case d1.ObjMURKYFTN:
		player.AddStatusEffect(StatusInfravision, d1.InfravisionTurns(2*((dlvl-1)/4+1)))
		msg("You drink from the Murky Pool. You can see in the dark.")
	case d1.ObjLAZSTAND:
		drop(gen.PlainItem(d1.IdiLazStaff))
		msg("The Staff of Lazarus falls from its stand.")
	case d1.ObjLEVER, d1.ObjSWITCHSKL:
		msg("You pull the lever. Something rumbles in the distance.")
		f.d1NakrulLever(f.State.GetMap())
		UpdateD1Quests(f.State, f.State.GetMap())
	case d1.ObjTEARFTN:
		from := d1Rng.Intn(4)
		to := d1Rng.Intn(3)
		if to >= from {
			to++
		}
		f.d1ModifyStat(player, from, -1)
		f.d1ModifyStat(player, to, 1)
		msg("You drink from the Fountain of Tears.")
	}
}

// SpawnD1Skeleton ports M_SpawnSkel for the Skeleton King: one of the level's skeletons rises next to pos.
func (f *Factory) SpawnD1Skeleton(pos geometry.Point) {
	mt := f.Generator.D.MonsterByID("MT_WSKELAX")
	for _, a := range f.State.GetMap().Actors() {
		if mon := a.d1Monster; mon != nil && mon.Unique == nil && strings.HasPrefix(mon.Data.Family, "Skel") {
			mt = mon.Type
			break
		}
	}
	f.d1Skeleton(mt, pos, f.State.GetPlayer())
}

func (f *Factory) d1Skeleton(mt int, pos geometry.Point, player *Actor) {
	m := f.State.GetMap()
	if mt < 0 {
		return
	}
	cells := freeCellsNear(m, pos, 1, m.CanPlaceActorHere)
	if len(cells) == 0 {
		return
	}
	a := f.NewD1Monster(mt)
	a.d1Monster.SpawnIndex = -1 // saved in LevelSnapshot.Spawned
	a.SetDrawPosition(cells[0].ToPointF())
	m.AddActor(a, cells[0])
	f.State.OnActorSpawned(a)
	f.ActionBroker.Execute(NewUpdateFoVAction(AbstractCause("Skeleton raised"), player)) // tab targets it right away
	d1Alert(a, player)
}

// SpawnRandomD1Unique places a random unique monster next to pos, for the "Spawn random unique" debug command.
// nil if there's nowhere free to put it.
// d1NeverFightsWithoutQuest are UniqueMonsterData.AI values whose brain (game/ai_d1.go's think()) only ever
// fights after its own quest has set it up first: AI_LACHDAN never fights at all (by design, ported from
// MAI_Lachdanan), and AI_SNOTSPIL/AI_LAZHELP gate their attack on quest variables (D1TalkerTurn) that a debug
// spawn never advances, so they'd stand there forever. Excluded from SpawnRandomD1Unique for that reason.
var d1NeverFightsWithoutQuest = map[string]bool{"AI_LACHDAN": true, "AI_SNOTSPIL": true, "AI_LAZHELP": true}

func (f *Factory) SpawnRandomD1Unique(pos geometry.Point) *Actor {
	m := f.State.GetMap()
	cells := freeCellsNear(m, pos, 2, m.CanPlaceActorHere)
	if len(cells) == 0 {
		return nil
	}
	var pickable []int
	for i, um := range f.Generator.D.UniqueMonsters {
		if !d1NeverFightsWithoutQuest[um.AI] {
			pickable = append(pickable, i)
		}
	}
	a := f.NewD1UniqueMonster(pickable[d1Rng.Intn(len(pickable))])
	a.d1Monster.SpawnIndex = -1 // saved in LevelSnapshot.Spawned
	a.SetDrawPosition(cells[0].ToPointF())
	m.AddActor(a, cells[0])
	f.State.OnActorSpawned(a)
	return a
}

// hitD1Object breaks an intact barrel or crucified skeleton at p (_oBreak 1): anything that hits its cell does,
// like D1's missile collisions (BreakObject).
func (f *Factory) hitD1Object(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point, attacker *Actor) {
	o := d1ObjectAt(m, p)
	switch {
	case o == nil || !o.O.Active:
	case o.O.Kind == d1.ObjBARREL || o.O.Kind == d1.ObjBARRELEX:
		f.breakD1Barrel(o, attacker)
	case isD1Crux(o.O.Kind) && m == f.State.GetMap():
		f.breakD1Crux(o)
	}
}

// breakD1Crux ports BreakCrux: once every crucified skeleton of its group is down, its part of the map opens.
func (f *Factory) breakD1Crux(o *D1Object) {
	o.O.Active = false
	if m := f.State.GetMap(); d1ObjectGroupUsed(m, isD1Crux, o.O.Var8) {
		f.Msg("Something rumbles in the distance.")
		f.State.mapGen.changeD1Map(m, o.O.Var1, o.O.Var2, o.O.Var3, o.O.Var4)
	}
}

// d1BlastAction shows an exploding barrel's fire; the damage is queued as separate actions.
type d1BlastAction struct {
	BaseAction
	animations []Animation
}

func (a *d1BlastAction) Execute(ActionContext) []Action { return nil }
func (a *d1BlastAction) GetAnimations() []Animation     { return a.animations }

// breakD1Barrel ports BreakBarrel: a normal barrel holds an item or a skeleton, an exploding one burns its 3×3 area
// and sets off exploding barrels next to it.
func (f *Factory) breakD1Barrel(o *D1Object, player *Actor) {
	obj, gen, m := o.O, f.Generator, f.State.GetMap()
	obj.Active = false // the debris stays, walkable
	if obj.Kind == d1.ObjBARREL {
		dlvl := d1DungeonLevel(f.State)
		ilvl, hell := d1.ItemLevel(dlvl), d1.LevelType(dlvl) == d1.DTypeHell
		if obj.Var2 <= 1 {
			it := gen.UsefulItem(ilvl)
			if obj.Var3 != 0 {
				it = gen.FloorItem(ilvl, false, hell)
			}
			f.State.AddItemToMapWithDistribution(f.NewD1Item(it), o.pos)
		}
		if obj.Var2 >= 8 {
			f.d1Skeleton(o.PreSpawnType, o.pos, player)
		}
		return
	}
	cause := NewActorCause("Exploding barrel", player)
	blast := &d1BlastAction{BaseAction: BaseAction{cause: cause, name: "ActionNameBarrelExplosion"}}
	for y := o.pos.Y - 1; y <= o.pos.Y+1; y++ {
		for x := o.pos.X - 1; x <= o.pos.X+1; x++ {
			if p := (geometry.Point{X: x, Y: y}); m.Contains(p) && m.IsTransparent(p) {
				blast.animations = append(blast.animations, NewHitAnimation(p, 0.5, defs.IconEffectFireBig, 2))
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
				case a.hero != nil && d1Rng.Intn(100) < 40: // PlayerMHit with a fire missile from no monster
					dam64 := 8<<6 + d1Rng.Intn(8<<6+1)
					dam64 -= dam64 * a.HeroSheet().FR / 100
					dam = dam64 >> 6
				case a.d1Monster != nil: // MonsterTrapHit
					mon := a.d1Monster
					if mon.MagicRes&d1.ImmuneFire != 0 || d1Rng.Intn(100) >= min(max(90-mon.AC, 5), 95) {
						break
					}
					dam = d1Rng.Intn(4) + 1
					if mon.MagicRes&d1.ResistFire != 0 {
						dam >>= 2
					}
					d1Alert(a, player)
				}
				if dam > 0 {
					f.ActionBroker.QueueForExecution(NewChangeResourceAction(cause, a, stats.Health, -float64(dam)))
				}
			}
			if other := d1ObjectAt(m, p); other != nil && other.O.Kind == d1.ObjBARRELEX && other.O.Active {
				f.breakD1Barrel(other, player)
			}
		}
	}
}

// d1ModifyStat ports ModifyPlr{Str,Mag,Dex,Vit} and CheckStats.
func (f *Factory) d1ModifyStat(player *Actor, attr, n int) {
	dhp, dmana := player.hero.AddBaseStat(attr, n)
	SyncHero(player)
	player.GetStats().GetResource(stats.Health).Increase(float64(dhp >> 6))
	player.GetStats().GetResource(stats.Mana).Increase(float64(dmana >> 6))
}

// d1OwnedItems lists the player's D1 items: equipped first, then the pack.
func d1OwnedItems(player *Actor) (equipped, pack []*Item) {
	eq, inv := player.GetEquipment(), player.GetInventory()
	for _, it := range eq.AllItems() {
		if it.d1 != nil {
			equipped = append(equipped, it)
		}
	}
	for _, it := range inv.Items() {
		if it.d1 != nil && !eq.IsEquipped(it) {
			pack = append(pack, it)
		}
	}
	return
}

func isD1Weapon(it *d1.Item) bool {
	switch it.Type {
	case d1.ITypeSword, d1.ITypeAxe, d1.ITypeBow, d1.ITypeMace, d1.ITypeStaff:
		return true
	}
	return false
}

func isD1Armor(it *d1.Item) bool {
	switch it.Type {
	case d1.ITypeShield, d1.ITypeLArmor, d1.ITypeHelm, d1.ITypeMArmor, d1.ITypeHArmor:
		return true
	}
	return false
}

// D1ShrineInfoText describes a shrine's effect for the right-click description popup, analogous to
// d1MonsterInfoText. false for anything else, including a goat shrine/cauldron, whose effect is rolled fresh
// each time it's touched rather than fixed at level generation.
func D1ShrineInfoText(o *D1Object) ([]string, bool) {
	switch o.O.Kind {
	case d1.ObjSHRINEL, d1.ObjSHRINER:
		shrine := o.O.Var1
		return []string{d1.ShrineNames[shrine] + " Shrine", "", d1.ShrineMessages[shrine]}, true
	case d1.ObjGOATSHRINE, d1.ObjCAULDRON:
		return []string{o.Name(), "", "Its effect is chosen at random when touched."}, true
	}
	return nil, false
}

// d1Shrine ports OperateShrine for single player.
func (f *Factory) d1Shrine(player *Actor, shrine int, pos geometry.Point) {
	h, gen, m, st := player.hero, f.Generator, f.State.GetMap(), player.GetStats()
	dlvl := d1DungeonLevel(f.State)
	ilvl, lt := d1.ItemLevel(dlvl), (dlvl-1)/4+1
	hp, mana := st.GetResource(stats.Health), st.GetResource(stats.Mana)
	equipped, pack := d1OwnedItems(player)
	all := append(append([]*Item(nil), equipped...), pack...)
	learn := func(spell int) { // Fascinating, Sacred, Ornate: +2 spell levels for 10% of base mana
		h.SpellLevels[spell] = min(h.SpellLevels[spell]+2, 15)
		cut := h.MaxManaBase / 10
		h.MaxManaBase -= cut
		h.ManaBase = max(h.ManaBase-cut, 0)
		SyncHero(player)
		mana.Decrease(cut >> 6)
	}
	switch shrine {
	case 0: // Mysterious
		for attr := d1.AttrStr; attr <= d1.AttrVit; attr++ {
			f.d1ModifyStat(player, attr, -1)
		}
		f.d1ModifyStat(player, d1Rng.Intn(4), 6)
	case 1: // Hidden
		var durable []*Item
		for _, it := range equipped {
			if d := it.d1; d.MaxDur != 0 && d.MaxDur != 255 {
				d.Durability, d.MaxDur = min(d.Durability+10, d.MaxDur+10), d.MaxDur+10
				durable = append(durable, it)
			}
		}
		if len(durable) > 0 {
			d := durable[d1Rng.Intn(len(durable))].d1
			d.Durability, d.MaxDur = max(d.Durability-20, 1), max(d.MaxDur-20, 1)
		}
	case 2: // Gloomy
		for _, it := range all {
			if d := it.d1; isD1Armor(d) {
				d.AC += 2
			} else if isD1Weapon(d) {
				d.MaxDam = max(d.MaxDam-1, d.MinDam)
			}
		}
	case 3: // Weird
		for _, it := range all {
			if isD1Weapon(it.d1) {
				it.d1.MaxDam++
			}
		}
	case 4, 11: // Magical
		player.AddStatusEffect(StatusManaShield, -1)
	case 5: // Stone
		for _, it := range all {
			if it.d1.Type == d1.ITypeStaff {
				it.d1.Charges = it.d1.MaxCharges
			}
		}
	case 6: // Religious
		for _, it := range all {
			it.d1.Durability = it.d1.MaxDur
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
			s := known[d1Rng.Intn(len(known))]
			h.SpellLevels[s] = max(h.SpellLevels[s]-2, 0)
		}
	case 8: // Thaumaturgic
		for _, obj := range m.Objects() {
			if o, ok := obj.(*D1Object); ok && o.O.Kind >= d1.ObjCHEST1 && o.O.Kind <= d1.ObjCHEST3 {
				o.O.Active = true
			}
		}
	case 9: // Fascinating
		learn(d1.SplFirebolt)
	case 10: // Cryptic
		f.ActionBroker.Execute(NewSpellAction(player, d1.SplNova, 2*lt, player.MapPosition()))
		mana.SetToMax()
	case 12: // Eldritch
		for _, it := range pack {
			rejuv := map[string]string{"IMISC_HEAL": "IMISC_REJUV", "IMISC_MANA": "IMISC_REJUV",
				"IMISC_FULLHEAL": "IMISC_FULLREJUV", "IMISC_FULLMANA": "IMISC_FULLREJUV"}[it.d1.MiscID]
			if rejuv == "" {
				continue
			}
			for idx, b := range gen.D.Items {
				if b.Drop != "IDROP_NEVER" && b.MiscID == rejuv {
					player.GetInventory().Remove(it)
					player.GetInventory().Add(f.NewD1Item(gen.PlainItem(idx)))
					break
				}
			}
		}
	case 13: // Eerie
		f.d1ModifyStat(player, d1.AttrMag, 2)
	case 14: // Divine
		drop := func(misc string) {
			if it := gen.TypeItem(ilvl, d1.ITypeMisc, misc, false); it != nil {
				f.State.AddItemToMapWithDistribution(f.NewD1Item(it), pos)
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
		f.d1RandomTeleport(player)
	case 16: // Sacred
		learn(d1.SplChargedBolt)
	case 17: // Spiritual
		inv := player.GetInventory()
		for n := inv.maxItemStacks - len(inv.StackedItems()); n > 0; n-- {
			st.IncrementBy(stats.Gold, 5*lt+d1Rng.Intn(10*lt))
		}
	case 19: // Abandoned
		f.d1ModifyStat(player, d1.AttrDex, 2)
	case 20: // Creepy
		f.d1ModifyStat(player, d1.AttrStr, 2)
	case 21: // Quiet
		f.d1ModifyStat(player, d1.AttrVit, 2)
	case 22: // Secluded
		m.SetAllExplored()
	case 23: // Ornate
		learn(d1.SplHolyBolt)
	case 24: // Glimmering
		for _, it := range all {
			if it.d1.Magical != 0 {
				it.d1.Identified = true
			}
		}
	case 26: // Oily
		for _, a := range map[int][][2]int{d1.Warrior: {{d1.AttrStr, 2}}, d1.Rogue: {{d1.AttrDex, 2}},
			d1.Sorcerer: {{d1.AttrMag, 2}}, d1.Barbarian: {{d1.AttrVit, 2}}, d1.Monk: {{d1.AttrStr, 1}, {d1.AttrDex, 1}},
			d1.Bard: {{d1.AttrDex, 1}, {d1.AttrMag, 1}}}[h.Class] {
			f.d1ModifyStat(player, a[0], a[1])
		}
		f.ActionBroker.QueueForExecution(NewTrapMissileAction(pos, MisFireWall, player.MapPosition()))
	case 27: // Glowing
		gain, keep := h.Exp/1000, 0
		if h.Exp > 5000 {
			gain, keep = 5, int(float64(h.Exp)*0.95)
		}
		f.d1ModifyStat(player, d1.AttrMag, gain)
		h.Exp = keep
	case 28: // Mendicant's
		gold := st.GetAttribute(stats.Gold) / 2
		h.AddExperience(h.Level, gold)
		st.DecrementBy(stats.Gold, gold)
	case 29: // Sparkling
		h.AddExperience(h.Level, 1000*dlvl)
		f.ActionBroker.QueueForExecution(NewTrapMissileAction(pos, MisFlashTrap, player.MapPosition()))
	case 30: // Town
		f.ActionBroker.Execute(NewSpellAction(player, d1.SplTown, 0, player.MapPosition()))
	case 31: // Shimmering
		h.ManaBase = h.MaxManaBase
		mana.SetToMax()
	case 32: // Solar
		switch hour := time.Now().Hour(); {
		case hour > 20 || hour < 4:
			f.d1ModifyStat(player, d1.AttrVit, 2)
		case hour > 18:
			f.d1ModifyStat(player, d1.AttrMag, 2)
		case hour > 12:
			f.d1ModifyStat(player, d1.AttrStr, 2)
		case hour > 4:
			f.d1ModifyStat(player, d1.AttrDex, 2)
		}
	case 33: // Murphy's
		broke := false
		for _, it := range equipped {
			if d := it.d1; d1Rng.Intn(3) == 0 && d.Durability != d1.DurIndestructible && d.Durability != 0 {
				d.Durability /= 2
				broke = true
				break
			}
		}
		if !broke {
			st.DecrementBy(stats.Gold, st.GetAttribute(stats.Gold)/3)
		}
	}
	for _, it := range all {
		it.RefreshD1()
	}
	SyncHero(player)
	f.RefreshD1Spells(player)
	f.Msg(d1.ShrineMessages[shrine])
}
