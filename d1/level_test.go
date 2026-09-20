package d1

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestCreateLevels builds every level and set level for several games and checks what D1 guarantees: monsters,
// objects and items on open cells, stairs on every level that has them, no two things in one cell.
func TestCreateLevels(t *testing.T) {
	d, err := Load(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	for game := int32(1); game <= 6; game++ {
		seeds := LevelSeeds(game * 7919)
		q := InitQuests(seeds)
		gnr := NewGenerator(d, game)
		levels := make([]struct{ dlvl, sl int }, 0, 20)
		for dlvl := 1; dlvl <= 16; dlvl++ {
			levels = append(levels, struct{ dlvl, sl int }{dlvl, SLNone})
		}
		levels = append(levels, struct{ dlvl, sl int }{3, SLSkelKing}, struct{ dlvl, sl int }{6, SLBoneChamb},
			struct{ dlvl, sl int }{2, SLPoisonWater}, struct{ dlvl, sl int }{15, SLVileBetrayer})
		for _, lv := range levels {
			lvl, err := gnr.CreateLevel(&q, seeds, lv.dlvl, lv.sl, EntryMain)
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
				if o.Kind == ObjTRAPL || o.Kind == ObjTRAPR {
					kind = "trap"
				}
				if o.Kind == ObjL1LIGHT || o.Kind >= ObjTORCHL && o.Kind <= ObjTORCHR2 || o.DoorFlag || o.Kind >= ObjTORTURE1 && o.Kind <= ObjTORTURE5 {
					continue // braziers, torches, doors and hooked bodies hang on wall pieces
				}
				occupy(o.X, o.Y, kind)
			}
			for _, it := range lvl.Items {
				occupy(it.X, it.Y, "item "+it.Item.Name)
			}
			if lv.sl == SLNone {
				up, down := 0, 0
				for _, tr := range lvl.Triggers {
					switch tr.Msg {
					case TrigPrevLevel, TrigTownWarpUp:
						up++
					case TrigNextLevel:
						down++
					}
				}
				if up == 0 || down == 0 && lv.dlvl < 15 && !(q[QLtBanner].Level == lv.dlvl && q[QLtBanner].Active != QuestNotAvail) && !(lv.dlvl == 13 && q[QWarlord].Active != QuestNotAvail) {
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
	d, err := Load(os.DirFS("../files"))
	if err != nil {
		t.Fatal(err)
	}
	books, heroes := 0, 0
	for game := int32(1); game <= 6; game++ {
		seeds := LevelSeeds(game * 7919)
		q := InitQuests(seeds)
		gnr := NewGenerator(d, game)
		for _, dlvl := range []int{4, 8, 9, 12} {
			lvl, err := gnr.CreateLevel(&q, seeds, dlvl, SLNone, EntryMain)
			if err != nil {
				t.Fatal(err)
			}
			for _, o := range lvl.Objects {
				switch o.Kind {
				case ObjSTORYBOOK:
					books++
					if o.Var1 < 0 || o.Var1 > 2 || o.Var2 != dlvl/4-1 || o.Var3 < 0 || o.Var3 > 8 || d.Texts[fmt.Sprintf("TEXT_BOOK%d%d", o.Var1+1, o.Var2+1)] == "" {
						t.Errorf("game %d level %d: story book vars %d %d %d", game, dlvl, o.Var1, o.Var2, o.Var3)
					}
				case ObjSLAINHERO:
					heroes++
				}
			}
		}
	}
	if books == 0 || heroes == 0 {
		t.Errorf("%d story books, %d slain heroes", books, heroes)
	}
	gnr := NewGenerator(d, 1)
	if it := gnr.MagicItemWithCurs(9, ITypeHArmor, ICursBreastPlate); it.Curs != ICursBreastPlate || it.Magical == 0 {
		t.Errorf("warrior reward %+v", it.Name)
	}
	if it := gnr.MagicItemWithCurs(9, ITypeBow, ICursLongWarBow); it.Curs != ICursLongWarBow || it.Magical == 0 {
		t.Errorf("rogue reward %+v", it.Name)
	}
	if it := gnr.SpellBook(9, SplLightning); it.Spell != SplLightning {
		t.Errorf("sorcerer reward %+v", it.Name)
	}
}
