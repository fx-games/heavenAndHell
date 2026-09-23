# AGENTS.md

Design decisions and deferred scope for the D1 rebuild: `docs/D1_PLAN.md`.

## Reference material (`reference/`, gitignored, never commit)

| Path | What it is | Use it for |
|---|---|---|
| `reference/devilution-109` | [diasurgical/devilution](https://github.com/diasurgical/devilution): bin-exact reconstruction of Diablo 1.09 (C++, `Source/*.cpp`, `enums.h`, `structs.h`, `defs.h`) | **Ground truth.** Formulas, constants, order of `random_()` calls. Ignore `#ifdef HELLFIRE` / `SPAWN` branches, except for the six player classes: Hellfire's Monk, Bard and Barbarian are in (`docs/D1_PLAN.md`), and their tables come from devilutionX. |
| `reference/devilution/2020_03_31` | [galaxyhaxz/devilution](https://github.com/galaxyhaxz/devilution): readable rewrite with the original table files in `Source/Data/Excel/xl_*.txt` | Readable second opinion; source of our `files/mods/d1/data/xl_*.txt` (data rows verbatim except item powers, renamed from `IPL_` to snake_case (`to_hit`, `thorns`); `##` comment headers added). `2018_03_14` is an older snapshot, skip it. |
| `reference/devilutionX` | [diasurgical/devilutionX](https://github.com/diasurgical/devilutionX): modern, maintained port | Bug-fixed logic in `Source/` (`levels/`, `monsters/`, `items/`, `quests/`...), data tables as TSV in `assets/txtdata/`, and **test fixtures** in `test/` (see below). It fixes vanilla bugs and adds QoL; when it disagrees with 1.09, 1.09 wins unless `docs/D1_PLAN.md` says otherwise. |
| `reference/DIABDAT.MPQ` | Diablo's own data archive (bring your own copy, ~500 MB) | Level data the sources don't contain: set pieces (`Levels\L1Data\SklKng1.DUN`...), `.TIL`/`.SOL` tile tables. Read with `mpq.DiabDat()` (`src/cmd/internal/mpq`, looks in `.`, `reference/`, `../reference/`). |
| `reference/diablo-hellfire` | [pvpgn/diablo-hellfire](https://github.com/pvpgn/diablo-hellfire): Synergistic Software's own Hellfire source (VC++ 6, `src/*.CPP`, uppercase filenames) | **Ground truth for Hellfire**, the way `devilution-109` is for D1. `ITEMDAT.CPP`, `MONSTDAT.CPP`, `SPELLDAT.CPP`, `MISDAT.CPP` and `OBJDAT.CPP` hold the real tables with the original comments; `PLAYER.CPP` the class tables. Hellfire's own levels (the Nest and the Crypt), its quests, towners and the monsters that only spawn there are **cut** from this game (`docs/D1_PLAN.md`); its items, affixes, spells, classes and store behaviour are in and always on. |
| `reference/HELLFIRE.MPQ`, `HFVOICE.MPQ`, `HFMONK.MPQ`, `HFMUSIC.MPQ` | Hellfire's four archives, straight off the expansion's disc (`hdiutil attach Hellfire.iso`, they sit in the root). `hfvoice` holds the expansion's speech at full quality and `hfmonk` the Monk's lines (neither is in `hellfire.mpq`), `hfmusic` the expansion's tracks | Hellfire's item/spell sprites for `cmd/d1sprites`. Its audio is **not** used at all (no `diablo_sfx/`, see Audio), and neither are its items or classes: those are numbers in the tables above, and this game draws them from its own tileset through `files/mods/<mod>/data/icons/*.rec`. Opens with the same `OpenMPQ` as DIABDAT (`src/cmd/internal/mpq`). |

### How to port with them

1. **Find the function in 1.09** (`grep -rn 'void DRLG_L1Floor' reference/devilution-109/Source`), port it 1:1 into the engine
   under the same name in Go style (`DRLG_L1Floor` → `drlgL1Floor`) and say which D1 function it ports in the doc comment.
   Keep D1's RNG call order, or seeded results drift.
2. **Cross-check** with galaxyhaxz for readability and devilutionX for known bugs, then decide per `docs/D1_PLAN.md`.
3. **Take data, not code, where there's a table.** Values come from `files/mods/d1/data/xl_*.txt`
   (`src/rules/data.go`); the level generators' minisets and lookup tables live in `files/mods/d1/data/levels/generator.txt` (read by `g.mini`/`g.table`/`g.rows`); anything in the MPQ is read
   from the MPQ, not copied into the repo.
4. **Verify against DevilutionX fixtures**, which were produced by the real game logic:
   - `test/fixtures/diablo/<dlvl>-<seed>.dun`: the level D1 builds from that seed; `test/drlg_l*_test.cpp` shows the
     quest flags and expected entry positions. Ported in `src/dungen/drlg_test.go` / `src/dungen/drlg_fixtures_test.go`.
   - `test/fixtures/levels/l*data/*.dun`: set pieces, used to check our MPQ reader (`src/cmd/d1import/mpq_test.go`).
   - Other `test/*_test.cpp` (items, stores, player, missiles, random, path) hold expected values worth copying into Go
     tests when porting that area.
5. Tests that need `reference/` must `t.Skip` when it's missing, so `go test ./...` still passes on a fresh clone.

Re-create the folder (DIABDAT.MPQ comes from your own Diablo install):

```
git clone --depth 1 https://github.com/diasurgical/devilution reference/devilution-109
git clone --depth 1 https://github.com/galaxyhaxz/devilution reference/devilution
git clone --depth 1 https://github.com/diasurgical/devilutionX reference/devilutionX
git clone --depth 1 https://github.com/pvpgn/diablo-hellfire reference/diablo-hellfire
```

## Icons (tileset `files/mods/<mod>/classic_roguelike.png`, 8×8)

Four independent rec files under `files/mods/<mod>/data/icons/`, each read fresh on every game start (no rebuild
needed to see a remap take effect — just restart the game). One ID convention throughout: `kind.snake_case_name`
— `class.warrior_male`, `monster.nzombie`, `unique.gharbad_the_weak`, `npc.wirt`, `corpse.default` (actors.rec);
`item.falchion` (+ `.magic`/`.unique` per weapon/armor family), `item.heal` (potions/scrolls, by IMISC_*) (items.rec);
`tile.grass`, `prop.chest1` (+ `.used`), `decal.blood1`, `effect.fire_small` (tiles.rec); `spell.firebolt`
(skills.rec). IDs built from D1's own enum names go through `defs.IconID(kind, raw)`, which strips the enum's
prefix (`MT_`/`OBJ_`/`SPL_`/`ITYPE_`/`IMISC_`) and lowercases the rest, so game code never hand-builds these
strings. Weapons and armor are broken down by family (falchion, dagger, broad sword, cap, kite shield...) instead
of one icon per D1 item type, each with its own `.magic`/`.unique` variant; the remapper's list sorts by `name:`,
so every family's `name:` is prefixed with its category ("Sword: Falchion", "Light Armor: Cape") to keep
same-category items clustered together when picking icons.

| File | Covers |
|---|---|
| `actors.rec` | monsters, uniques, player classes, corpses, D1 npcs |
| `items.rec` | weapons (by family), armor, potions, scrolls, rings, quest items... |
| `tiles.rec` | terrain, doors, decals, effects, dungeon objects |
| `skills.rec` | class skill and spell icons |

The handful of always-present H&H icons (`IconGrass`, `IconCharacterWarriorMale`, ...) are still `defs.IconXxx`
package vars (`src/defs/icons_actors.go` / `icons_tiles.go`), read directly; their rec key now just follows the same
convention. Everything else has no Go identifier; look it up with `defs.Icon(defs.IconID("monster", m.ID))`
(panics on a typo) or `defs.IconOK(id)` when the name is optional (`common/item.go`'s IMISC fallback).
Dungeon objects are `prop.<name>` (`defs.PropIconID`). Objects our top-down view can't tell apart (same sprite
under another ID: trapped/quest chests, Warlord's racks; or another isometric angle: left/right shrines,
bookcases, banners, crux skeletons, lectern books) are records with `same_as: prop.<other>` instead of `icon:`;
give one its own `icon:` to tell it apart. `src/app/prop_folds_test.go` checks every prop record is some object's.

Remap visually with `scripts/remap_actors.sh`, `scripts/remap_items.sh`, `scripts/remap_tiles.sh`, `scripts/remap_skills.sh` (each opens
the remapper on its rec file: click a name on the left, a tile on the right, `s` saves, `F10` quits). Adding a new
D1 object/monster family/item type needs one new `internal_name:`/`icon:`/`name:` record in the right rec file —
remap it same as any other. Tinting is reserved for D1 monster variants, water tiles (`IconWater`, tinted by
`State.WaterColorOf`: light green in the foul Poisoned Water Supply, blue otherwise, shallows lighter) and one fallback: a used D1
object whose `prop.<name>.used` icon is the same tile as its fresh `prop.<name>` icon is drawn grayed out
(`common.UsedTint`); remap `.used` to a different tile to drop the tint. Anything else that needs to look
different (uniques, magic/unique items) gets its own icon record. Remapper: `remapper` on `$PATH`, source
`~/Projects/remapper`. It shows an entry's `reference_image:` (a D1 sprite under the gitignored `reference/`) beside
the atlas; `go run ./src/cmd/d1sprites` writes those fields into all four rec files.

## Skills (`src/rules/skills.rec`)

Class skills are spells: D1's `SPL_` slots come first, then the engine's named skills from the embedded
`src/rules/skills.rec` (Arrow Rain, Crush, Black Clouds...), usable by any mod by id alone. A mod's `spells.rec`
record with the same id replaces the engine's. They share the spell machinery: levels (saved by id), mana, books
that raise them, scrolls/staffs that cast them (`spell: arrow_rain`, at level 1 + item level/5), gear bonuses
(`equip_modifier: skill(arrow_rain|2)`). A class lists what it can learn in `start_spells` (`arrow_rain=0` is
learnable, not known) and gains `skill_points` per level up to `max_skill_level`. The effects are in
`src/common/def_skills.go`; each gets the skill level as its last argument. Icons: `spell.<id>` in `icons/skills.rec`.

## Audio (`files/assets/`)

Every cue the game plays comes from a rec file under `files/assets/data/`, read fresh on every start (no rebuild to
see a remap take effect) and assigned visually with `sfxmapper` (source `~/Projects/remapper/sfxmapper`, install
with `go build -o ~/bin/sfxmapper ./sfxmapper`): click a record on the left, browse `files/assets/` on the right,
"Add", `s` saves, `F10` quits. A record with no `sound:` is simply silent — nothing breaks, `HasCue` says no and
callers fall back.

| Rec file | Script | What it holds | Registered by |
|---|---|---|---|
| `data/sounds.rec` | `scripts/remap_sounds.sh` | sound effects, one record per event: `internal_name` = the cue `PlayCue` takes (`open_door`), D1's effects as `sfx.<_sfx_id>` (`sfx.IS_CAST2`, `sfx.PS_SWING`), monsters as `monster.<id>.attack\|hit\|death\|special`. Several `sound:` lines = one picked at random | `RegisterSoundEvents` |
| `data/voices.rec` | `scripts/remap_voices.sh` | D1's speech, one record per `_sfx_id` (`TSFX_BMAID1`, `PS_WARR8`, `USFX_GARBUD1`). `text:` is what the original line said, so you know what to record in its place | `RegisterVoiceEvents` (voice volume) |
| `data/music.rec` | `scripts/remap_music.sh` | the five area themes `town`/`cathedral`/`catacombs`/`caves`/`hell` that `d1AreaMusic` (`game/town.go`) switches between | `RegisterMusicEvents` (looped, listed in the Music Player) |

Every rec above is the mod's own (`files/mods/<mod>/data/`); the scripts take the mod, default `d1`
(`scripts/remap_sounds.sh heavenandhell`), and the file browser shows only that mod's folder.

The engine's own cues, each silent unless the mod's `sounds.rec` has it: `menu_move`, `menu_select` (both menus;
D1: `IS_TITLEMOV`/`IS_TITLSLCT`), `store_enter`, `vendor_buy`, `vendor_sell`, `stairs_down`, `stairs_up`,
`enter_dungeon`, `identify_bad|ego|art`, `status.<effect>` and `status_recover` (the hero's), `low_health`,
`item_break`, `quaff`, `use_item`, `use_staff`, `disarm`, `item_equip`, `item_unequip`, `player_death`,
`game_over`, `you_win`, `teleport`, `earthquake`, `melee_attack`, `bow_fire`, `open_door`, `close_door`,
`level_up`, and `monster.<id>.attack|hit|death|special`. The shared D1 code plays D1's ids as `sfx.<id>`, whatever
the mod. A new sound event = one `PlayCue("<name>")` call + one record in `sounds.rec`. D1's effects go through
`playSfxAt` / `Factory.PlaySfx` with D1's distance fade and pan (`common/sound.go`), its speech through
`playSound`; `TestSfxHaveSoundEvents` and `TestSpeechHaveVoiceEvents` fail when a record is missing.

Heaven & Hell's folders:

- `sfx/`, `ang_sfx/` — this game's own effects and the Dubtrain Angband Sound Pack (CC BY-NC, credited in
  `docs/LEGAL.md`). The pack's unconverted `.wav` originals sit in `files/assets/CC-BY-DASP/` (gitignored) next to its
  `sound.cfg`, which says what each sample was meant for; a sample we use is converted flat into
  `ang_sfx/<name>.ogg`.
- `music/` — our own tracks (`town.ogg`, wired up in `music.rec`).
- `ambience/` — extra tracks and ambient loops, registered by directory (`RegisterMusic`) and picked manually from
  the pause menu's Music Player; `music.rec` wires some of them as area themes.
- `voice/` — voice lines registered by directory (`RegisterVoices`), doesn't exist yet; folder absence is fine.

Each mod's audio is its own, and the two never mix:

- **D1 plays only Diablo 1.09's original audio**: `files/mods/d1/diablo_sfx/` and `diablo_music/` (gitignored,
  bring your own, like `DIABDAT.MPQ`; converted copies sit in `reference/cut_diablo_audio/`, the old MPQ importer is
  in git at `2d2eb5c^:cmd/d1import/sfx.go`). Every `sfx.<id>` and speech record points at the file 1.09's `sgSFX`
  gives that id (`effects.cpp`), monsters at their `monstdat` set, `music.rec` at the five themes. Without those
  folders D1 is silent. Its engine cues (`melee_attack`, `open_door`...) have no file: D1's code plays D1's ids.
  Hellfire's audio is not used.
- **Heaven & Hell plays anything but Diablo's**: its own `sfx/`, the Angband pack in `ang_sfx/`, `ambience/` and
  `music/`, all in `files/mods/heavenandhell/`.

Convert new files with `oggenc -Q -q 4` (not `ffmpeg -c:a vorbis`: its experimental Vorbis encoder produces audible
static on speech). `go:embed mods` embeds whatever is on disk: build a release for others from a checkout without
`diablo_*`.

## ASCII mode (`console/`, `textui/`)

`go run ./src ascii` plays the game in the terminal, with the same features as the tile UI.

- `textui/` is the terminal UI framework, taken from contractor's `ui_console` (`~/Projects/contractor`) and built on
  `github.com/memmaker/go` (`cview`, `textiles`, `fxtools`): panels and focus handling, menus and dialogs, the text
  inventory, key maps, map drawing and targeting. It knows nothing about this game — keep game logic and graphics out.
- `console/` implements `common.UserInterface` on top of it: game glyphs and colors, commands, shops, stash, inventory.
- Data in each mod's `data/ascii/`: `keymap.txt` (key -> command, two layers: `% main` and `% targeting`),
  `ui.rec` (theme colors by palette name and border runes), `palette.rec` (contractor's palette), `glyphs.rec`
  (object glyphs by object id) and `commands.rec` (the help's command names and columns).
  A new command = one line in `keymap.txt` + one record in `commands.rec` + one entry in `setupCommands`
  (`src/console/commands.go`).
- Menu contents and shop transactions both UIs share live in `src/common/ui_shared.go`, so the two can't drift apart.
- Keys and mouse buttons match the tile UI (`src/gui/input.go`), including Shift to attack what isn't hostile, the arrow
  keys scrolling the map, `x` centering it and right clicking an inventory line to drop it. No terminal equivalent:
  `m`/`n` zoom, `F8` rendering mode, `F11` fullscreen, `F12` screen flash. Terminal-only additions: `v` looks around
  with the keyboard (the tile UI hovers with the mouse), `?` examines an item in a shop, and in the inventory
  `<CTRL>`+letter examines, `<SHIFT>`+letter drops.
- Not ported: contractor's own animator (this game's animations come from `common`, drawn through an
  animation canvas in `src/console/game_map.go`) and its rec-file themes beyond `ui.rec`.

## Running

- Game: `go run ./src` (assets from `files/`, `config.rec` from the repo root). A mod can fix a setting in
  its `data/mod.rec` (`common.ModSetting`): the mod's value wins and the menu shows it locked. D1 fixes
  `AutosaveOnLevelChange: false`, as 1.09 only saves on "Save Game" and on quitting
- ASCII mode: `go run ./src ascii` (needs a real terminal, F1 lists the keys)
- Rules and level generation tests: `go test ./src/rules ./src/dungen`
- Headless smoke test: `go run ./src playthrough` (must print `PLAYTHROUGH OK`)
- Its seed-1 output is a golden file: `TestPlaythroughSeed1` diffs against `src/app/testdata/playthrough_seed1.txt`
  (skipped with `-short`). A deliberate gameplay change regenerates it: `go run ./src playthrough 1 > src/app/testdata/playthrough_seed1.txt`
- Dead code: `go run honnef.co/go/tools/cmd/staticcheck@latest -checks U1000 ./...` (also finds dead methods of
  generic types, which `deadcode` misses)
- Print a generated D1 level: `go run ./src dun_gen <dungeon level> [seed]`
- Import level data: `go run ./src/cmd/d1import`
- Sprite references for picking icons: `go run ./src/cmd/d1sprites` writes `reference/d1_{monsters,items,skills,tiles}.pdf` (one per icons/*.rec, each sprite labelled with its current icon) and every sprite as a PNG under `reference/d1_sprites/`: monsters and uniques in their TRN recolors, items, spell and skill icons, objects and each level type's megatiles
