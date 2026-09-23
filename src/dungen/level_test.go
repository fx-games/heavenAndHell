package dungen

import (
	"fmt"
	"rpg/src/rules"
	"strings"
	"testing"
)

// TestCreateLevels builds every level and set level for several games and checks what D1 guarantees: monsters,
// objects and items on open cells, stairs on every level that has them, no two things in one cell.
func TestCreateLevels(t *testing.T) {
	d, err := rules.Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	for game := int32(1); game <= 6; game++ {
		seeds := rules.LevelSeeds(game * 7919)
		q := d.InitQuests(seeds)
		gnr := rules.NewGenerator(d, game)
		levels := make([]struct{ dlvl, sl int }, 0, 20)
		for dlvl := 1; dlvl <= 16; dlvl++ {
			levels = append(levels, struct{ dlvl, sl int }{dlvl, rules.SLNone})
		}
		levels = append(levels, struct{ dlvl, sl int }{3, rules.SLSkelKing}, struct{ dlvl, sl int }{6, rules.SLBoneChamb},
			struct{ dlvl, sl int }{2, rules.SLPoisonWater}, struct{ dlvl, sl int }{15, rules.SLVileBetrayer})
		for _, lv := range levels {
			lvl, err := testLevels(t).CreateLevel(gnr, &q, seeds, lv.dlvl, lv.sl, EntryMain)
			if err != nil {
				t.Fatalf("game %d: %v", game, err)
			}
			used := map[[2]int]string{}
			// set pieces start closed: D1 places their monsters and objects inside what only opens later (PDungeon)
			closed := func(x, y int) bool {
				i, j := (x-16)/2, (y-16)/2
				return i >= 0 && j >= 0 && i < 40 && j < 40 && lvl.Dungeon[i][j] != lvl.PDungeon[i][j]
			}
			doors := map[[2]int]bool{}
			for _, o := range lvl.Objects {
				doors[[2]int{o.X, o.Y}] = o.DoorFlag
			}
			occupy := func(x, y int, what string) {
				// monsters may stand in doorways (a door's piece turns solid after monsters are placed); diab3a.DUN
				// puts a Blood Knight into a wall
				// a wall torch belongs on the wall (hangTorchesOnWalls)
				if lvl.Solid(x, y) && what != "trap" && !strings.Contains(what, "TORCH") && !closed(x, y) && !doors[[2]int{x, y}] && lv.dlvl != 16 {
					t.Errorf("game %d level %d/%d: %s on a solid cell %d,%d", game, lv.dlvl, lv.sl, what, x, y)
				}
				// two objects' traps can pick the same wall: AddObject doesn't check the cell
				if prev, ok := used[[2]int{x, y}]; ok && what[:4] == prev[:4] && what != "trap" {
					t.Errorf("game %d level %d/%d: %s and %s share %d,%d", game, lv.dlvl, lv.sl, what, prev, x, y)
				}
				used[[2]int{x, y}] = what
			}
			monsters := 0
			for _, m := range lvl.Monsters {
				if m == nil || m.Reserved || m.Hidden {
					continue
				}
				monsters++
				occupy(m.X, m.Y, "monster "+d.Monsters[m.Type].Name)
			}
			for _, o := range lvl.Objects {
				kind := "object " + d.Objects[o.Kind].Name
				if o.Kind == rules.ObjTRAPL || o.Kind == rules.ObjTRAPR {
					kind = "trap"
				}
				if o.Kind == rules.ObjL1LIGHT || o.Kind >= rules.ObjTORCHL && o.Kind <= rules.ObjTORCHR2 || o.DoorFlag || o.Kind >= rules.ObjTORTURE1 && o.Kind <= rules.ObjTORTURE5 {
					continue // braziers, torches, doors and hooked bodies hang on wall pieces
				}
				occupy(o.X, o.Y, kind)
			}
			for _, it := range lvl.Items {
				occupy(it.X, it.Y, "item "+it.Item.Name)
			}
			if lv.sl == rules.SLNone {
				up, down := 0, 0
				for _, tr := range lvl.Triggers {
					switch tr.Msg {
					case TrigPrevLevel, TrigTownWarpUp:
						up++
					case TrigNextLevel:
						down++
					}
				}
				if up == 0 || down == 0 && lv.dlvl < 15 && !(q[rules.QLtBanner].Level == lv.dlvl && q[rules.QLtBanner].Active != rules.QuestNotAvail) && !(lv.dlvl == 13 && q[rules.QWarlord].Active != rules.QuestNotAvail) {
					t.Errorf("game %d level %d: %d stairs up, %d down", game, lv.dlvl, up, down)
				}
				if lv.dlvl < 16 && (monsters < 30 || len(lvl.Objects) < 10) {
					t.Errorf("game %d level %d: %d monsters, %d objects", game, lv.dlvl, monsters, len(lvl.Objects))
				}
			}
			if game == 1 {
				t.Logf("level %d/%d: %d monsters, %d objects, %d items, view %v", lv.dlvl, lv.sl, monsters, len(lvl.Objects), len(lvl.Items), lvl.View)
			}
		}
	}
}

// TestStoryObjects checks what the story books and the slain hero hand out: a narration for each book's level and
// the class rewards OperateSlainHero rolls for.
func TestStoryObjects(t *testing.T) {
	d, err := rules.Load(testFS())
	if err != nil {
		t.Fatal(err)
	}
	books, heroes := 0, 0
	for game := int32(1); game <= 6; game++ {
		seeds := rules.LevelSeeds(game * 7919)
		q := d.InitQuests(seeds)
		gnr := rules.NewGenerator(d, game)
		for _, dlvl := range []int{4, 8, 9, 12} {
			lvl, err := testLevels(t).CreateLevel(gnr, &q, seeds, dlvl, rules.SLNone, EntryMain)
			if err != nil {
				t.Fatal(err)
			}
			for _, o := range lvl.Objects {
				switch o.Kind {
				case rules.ObjSTORYBOOK:
					books++
					if o.Var1 < 0 || o.Var1 > 2 || o.Var2 != dlvl/4-1 || o.Var3 < 0 || o.Var3 > 8 || d.Texts[fmt.Sprintf("TEXT_BOOK%d%d", o.Var1+1, o.Var2+1)] == "" {
						t.Errorf("game %d level %d: story book vars %d %d %d", game, dlvl, o.Var1, o.Var2, o.Var3)
					}
				case rules.ObjSLAINHERO:
					heroes++
				}
			}
		}
	}
	if books == 0 || heroes == 0 {
		t.Errorf("%d story books, %d slain heroes", books, heroes)
	}
	gnr := rules.NewGenerator(d, 1)
	const breastPlate, longWarBow = 153, 119 // their ICURS_ graphics
	if it := gnr.MagicItemWithCurs(9, rules.ITypeHArmor, breastPlate); it.Curs != breastPlate || it.Magical == 0 {
		t.Errorf("warrior reward %+v", it.Name)
	}
	if it := gnr.MagicItemWithCurs(9, rules.ITypeBow, longWarBow); it.Curs != longWarBow || it.Magical == 0 {
		t.Errorf("rogue reward %+v", it.Name)
	}
	if it := gnr.SpellBook(9, rules.SplLightning); it.Spell != rules.SplLightning {
		t.Errorf("sorcerer reward %+v", it.Name)
	}
}
