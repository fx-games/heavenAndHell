# AGENTS.md

Design decisions and deferred scope for the D1 rebuild: `D1_PLAN.md`.

## Reference material (`reference/`, gitignored, never commit)

| Path | What it is | Use it for |
|---|---|---|
| `reference/devilution-109` | [diasurgical/devilution](https://github.com/diasurgical/devilution): bin-exact reconstruction of Diablo 1.09 (C++, `Source/*.cpp`, `enums.h`, `structs.h`, `defs.h`) | **Ground truth.** Formulas, constants, order of `random_()` calls. Ignore `#ifdef HELLFIRE` / `SPAWN` branches, except for the six player classes: Hellfire's Monk, Bard and Barbarian are in (`D1_PLAN.md`), and their tables come from devilutionX. |
| `reference/devilution/2020_03_31` | [galaxyhaxz/devilution](https://github.com/galaxyhaxz/devilution): readable rewrite with the original table files in `Source/Data/Excel/xl_*.txt` | Readable second opinion; source of our `files/assets/data/d1/xl_*.txt` (copied verbatim). `2018_03_14` is an older snapshot, skip it. |
| `reference/devilutionX` | [diasurgical/devilutionX](https://github.com/diasurgical/devilutionX): modern, maintained port | Bug-fixed logic in `Source/` (`levels/`, `monsters/`, `items/`, `quests/`...), data tables as TSV in `assets/txtdata/`, and **test fixtures** in `test/` (see below). It fixes vanilla bugs and adds QoL; when it disagrees with 1.09, 1.09 wins unless `D1_PLAN.md` says otherwise. |
| `reference/DIABDAT.MPQ` | Diablo's own data archive (bring your own copy, ~500 MB) | Level data the sources don't contain: set pieces (`Levels\L1Data\SklKng1.DUN`...), `.TIL`/`.SOL` tile tables. Read with `d1.DiabDat()` / `mustReadMPQ` (`d1/mpq.go`, looks in `.`, `reference/`, `../reference/`). |
| `reference/diablo-hellfire` | [pvpgn/diablo-hellfire](https://github.com/pvpgn/diablo-hellfire): Synergistic Software's own Hellfire source (VC++ 6, `src/*.CPP`, uppercase filenames) | **Ground truth for Hellfire**, the way `devilution-109` is for D1. `ITEMDAT.CPP`, `MONSTDAT.CPP`, `SPELLDAT.CPP`, `MISDAT.CPP` and `OBJDAT.CPP` hold the real tables with the original comments; `PLAYER.CPP` the class tables. The Nest generator is `DRLG_L6` in `DRLG_L3.CPP` and the Crypt's is `DRLG_L5` in `DRLG_L1.CPP`. Note Hellfire numbers its own levels 17-24 internally as 32-35 (Nest) and 21-24 (Crypt) in places — check `MonstAvailTbl` / `DRLG` callers before trusting a number. |
| `reference/HELLFIRE.MPQ`, `HFVOICE.MPQ`, `HFMONK.MPQ`, `HFMUSIC.MPQ` | Hellfire's four archives, straight off the expansion's disc (`hdiutil attach Hellfire.iso`, they sit in the root). `hfvoice` holds the expansion's speech at full quality and `hfmonk` the Monk's lines (neither is in `hellfire.mpq`), `hfmusic` the Nest's and the Crypt's tracks | Only what the sources don't contain: the Crypt's and the Nest's tile tables (`NLevels\L5Data\L5.SOL/.TIL/.MIN`, `NLevels\L6Data\L6.*`) and `NLevels\TownData\*`, i.e. the two dungeon types, if they ever get built. **Not** needed for Hellfire's items, monsters or classes: those are numbers in the tables above, and this game draws them from its own tileset through `files/assets/data/icons/*.rec`. Opens with the same `OpenMPQ` as DIABDAT (`cmd/internal/mpq`). |

### How to port with them

1. **Find the function in 1.09** (`grep -rn 'void DRLG_L1Floor' reference/devilution-109/Source`), port it 1:1 into `d1/`
   under the same name in Go style (`DRLG_L1Floor` → `drlgL1Floor`) and say which D1 function it ports in the doc comment.
   Keep D1's RNG call order, or seeded results drift.
2. **Cross-check** with galaxyhaxz for readability and devilutionX for known bugs, then decide per `D1_PLAN.md`.
3. **Take data, not code, where there's a table.** Values come from `files/assets/data/d1/xl_*.txt`
   (`d1/data.go`); big static arrays that aren't in a table go into a `d1/*_data.go` file; anything in the MPQ is read
   from the MPQ, not copied into the repo.
4. **Verify against DevilutionX fixtures**, which were produced by the real game logic:
   - `test/fixtures/diablo/<dlvl>-<seed>.dun`: the level D1 builds from that seed; `test/drlg_l*_test.cpp` shows the
     quest flags and expected entry positions. Ported in `d1/drlg_test.go` / `d1/drlg_fixtures_test.go`.
   - `test/fixtures/levels/l*data/*.dun`: set pieces, used to check our MPQ reader (`d1/mpq_test.go`).
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

## Icons (tileset `files/assets/classic_roguelike.png`, 8×8)

Four independent rec files under `files/assets/data/icons/`, each read fresh on every game start (no rebuild
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
| `actors.rec` | monsters (D1's and Hellfire's), uniques, player classes, corpses, D1 npcs |
| `items.rec` | weapons (by family), armor, potions, scrolls, rings, quest items... |
| `tiles.rec` | terrain, doors, decals, effects, dungeon objects |
| `skills.rec` | class skill and spell icons |

The handful of always-present H&H icons (`IconGrass`, `IconCharacterWarriorMale`, ...) are still `defs.IconXxx`
package vars (`defs/icons_actors.go` / `icons_tiles.go`), read directly; their rec key now just follows the same
convention. Everything else has no Go identifier; look it up with `defs.Icon(defs.IconID("monster", m.ID))`
(panics on a typo) or `defs.IconOK(id)` when the name is optional (`common/d1_item.go`'s IMISC fallback).
D1 objects go through `defs.PropIconID`, which folds objects our top-down view can't tell apart (same sprite
under another ID: trapped/quest chests, Warlord's racks; or another isometric angle: left/right shrines,
bookcases, banners, crux skeletons, lectern books) into one `prop.*` record via `propFolds` in `defs/icons.go`;
`prop_folds_test.go` keeps it and `tiles.rec` in sync.

Remap visually with `./remap_actors.sh`, `./remap_items.sh`, `./remap_tiles.sh`, `./remap_skills.sh` (each opens
the remapper on its rec file: click a name on the left, a tile on the right, `s` saves, `F10` quits). Adding a new
D1 object/monster family/item type needs one new `internal_name:`/`icon:`/`name:` record in the right rec file —
remap it same as any other. Tinting is reserved for D1 monster variants, water tiles (`IconWater`, tinted by
`State.GetWaterColor`: light green in the foul Poisoned Water Supply, blue otherwise) and one fallback: a used D1
object whose `prop.<name>.used` icon is the same tile as its fresh `prop.<name>` icon is drawn grayed out
(`common.D1UsedTint`); remap `.used` to a different tile to drop the tint. Anything else that needs to look
different (uniques, magic/unique items) gets its own icon record. Remapper: `remapper` on `$PATH`, source
`~/Projects/remapper`. It shows an entry's `reference_image:` (a D1 sprite under the gitignored `reference/`) beside
the atlas; `go run -tags development ./cmd/d1sprites` writes those fields into all four rec files.

## Audio (`files/assets/`)

Sound effects are **events**, not folders: `files/assets/data/sounds.rec` has one record per event the game plays
(`internal_name` = the cue passed to `PlayCue`, e.g. `open_door` or `monster.nzombie.hit`; `name` = list label;
`tile` = icon name; one `sound:` per file relative to `files/assets/`, one picked at random at run time). Loaded by
`audio.Player.RegisterSoundEvents` on start; missing files are dropped (and `diablo_*` ones when
`DiabloAudioEnabled` is off), so a fresh clone without the Diablo folders still runs. Assign files visually with
`./remap_sounds.sh` (`sfxmapper`, source `~/Projects/remapper/sfxmapper`, install with
`go build -o ~/bin/sfxmapper ./sfxmapper`). A new sound event = one `PlayCue("<name>")` call + one record here.
D1's sound effects are events named `sfx.<_sfx_id>` (`sfx.IS_CAST2`, `sfx.PS_SWING`; RndSFX's variants are the
event's extra files), played through `playD1SfxAt` / `Factory.PlayD1Sfx` with D1's distance fade and pan
(`common/d1_sound.go`); monster sounds are `monster.<id>.attack|hit|death|special`. A new D1 sound = one call + one
record; `TestD1SfxHaveSoundEvents` fails when the record is missing.

- `sfx/`, `ang_sfx/` — this game's own and the Angband-sourced effects, referenced from `sounds.rec`.
- `ambience/` — background music tracks and ambient loops, shown in the pause menu's Music Player
  (`RegisterMusic`); `d1AreaMusic` (`game/town_d1.go`) needs cues named exactly `town`/`cathedral`/`catacombs`/
  `caves`/`hell`, which `ambience/` doesn't provide (they're `calm_music_*`/`battle_music_*`/`amb_*`) — so D1's
  auto area music only plays once `diablo_music/` supplies those names (below); `ambience/` tracks are still
  picked manually from the Music Player.
- `diablo_music/`, `diablo_sfx/` — **not in git** (`.gitignore`; bring your own copy of Diablo, like
  `reference/DIABDAT.MPQ`). Convert D1's own `.wav` files to `.ogg` (the engine only decodes Vorbis) and drop
  them in; they're picked up automatically next run, no code change:
  ```
  brew install vorbis-tools   # oggenc, once
  find files/assets/diablo_music files/assets/diablo_sfx -iname '*.wav' -print0 | \
    xargs -0 -I{} oggenc -Q -q 4 -o {}.ogg {}
  ```
  Use `oggenc`, not `ffmpeg -c:a vorbis`: ffmpeg's own Vorbis encoder is its unmaintained experimental one
  (needs `-strict -2` to even run) and produces audibly corrupted output — a wall of static — on some of D1's
  speech files; `oggenc` is the Xiph reference encoder and doesn't have that problem. It also keeps mono
  sources mono (no `-ac 2` needed).

  `go run ./cmd/d1import -sfx` pulls those `.wav` files straight out of every archive in `reference/` into
  `files/assets/diablo_sfx/`, in the layout `sounds.rec` expects, so you only have to run the ffmpeg line above
  afterwards. It also drops the Nest's and the Crypt's music into `diablo_music/` as `nest.wav` / `crypt.wav`,
  already named the way `d1AreaMusic` wants them. It takes everything `sounds.rec` points at plus `d1.SfxFiles`'
  speech plus all of Hellfire's own sounds, and quietly skips whatever your archives don't have — no MPQ is
  required. Where a sound is in several archives the better copy wins: `hfvoice` first, then `hfmonk`, `hfmusic`,
  `hellfire`, and DIABDAT last. Without `hfmonk`/`hfvoice` the Monk simply has no lines; the Bard and the
  Barbarian always borrow the Rogue's and the Warrior's, which is what Hellfire itself does. `diablo_music/`'s 6 tracks
  (`dtowne`/`dlvla`/`dlvlb`/`dlvlc`/`dlvld`/`dintro`) must be renamed to `town`/`cathedral`/`catacombs`/`caves`/
  `hell`/`intro` after conversion — that's what supplies `d1AreaMusic`'s cues. `diablo_sfx/`'s ~900 files
  (`misc/`, `monsters/`, `items/`, `warrior/`, `rogue/`, `sorceror/`, `towners/`, `narrator/`, `animals/`) are
  only used through the files `sounds.rec` points at.
- `voice/` — voice lines (`RegisterVoices`), doesn't exist yet; folder absence is fine, same as `ambience/`.

## ASCII mode (`console/`, `textui/`)

`go run -tags development . ascii` plays the game in the terminal, with the same features as the tile UI.

- `textui/` is the terminal UI framework, taken from contractor's `ui_console` (`~/Projects/contractor`) and built on
  `github.com/memmaker/go` (`cview`, `textiles`, `fxtools`): panels and focus handling, menus and dialogs, the text
  inventory, key maps, map drawing and targeting. It knows nothing about this game — keep game logic and graphics out.
- `console/` implements `common.UserInterface` on top of it: game glyphs and colors, commands, shops, stash, inventory.
- Data in `files/assets/data/ascii/`: `keymap.txt` (key -> command, two layers: `% main` and `% targeting`),
  `ui.rec` (theme colors by palette name and border runes) and `palette.rec` (contractor's palette).
  A new command = one line in `keymap.txt` + one entry in `setupCommands` (`console/commands.go`).
- Menu contents and shop transactions both UIs share live in `common/ui_shared.go`, so the two can't drift apart.
- Keys and mouse buttons match the tile UI (`gui/input.go`), including Shift to attack what isn't hostile, the arrow
  keys scrolling the map, `x` centering it and right clicking an inventory line to drop it. No terminal equivalent:
  `m`/`n` zoom, `F8` rendering mode, `F11` fullscreen, `F12` screen flash. Terminal-only additions: `v` looks around
  with the keyboard (the tile UI hovers with the mouse), `?` examines an item in a shop, and in the inventory
  `<CTRL>`+letter examines, `<SHIFT>`+letter drops.
- Not ported: contractor's own animator (this game's animations come from `common`, drawn through an
  animation canvas in `console/game_map.go`) and its rec-file themes beyond `ui.rec`.

## Running

- Game: `go run -tags development .` (assets from `files/`, `config.rec` from the repo root)
- ASCII mode: `go run -tags development . ascii` (needs a real terminal, F1 lists the keys)
- D1 unit tests: `go test ./d1`
- Headless smoke test: `go run -tags development . playthrough` (must print `PLAYTHROUGH OK`)
- Print a generated D1 level: `go run . dun_gen <dungeon level> [seed]`
- Import level data: `go run ./cmd/d1import` (adds the Crypt's and the Nest's tile tables when HELLFIRE.MPQ is there)
- Extract sounds: `go run ./cmd/d1import -sfx`, then convert with the ffmpeg line above
- Sprite references for picking icons: `go run -tags development ./cmd/d1sprites` writes `reference/d1_{monsters,items,skills,tiles}.pdf` (one per icons/*.rec, each sprite labelled with its current icon) and every sprite as a PNG under `reference/d1_sprites/`: monsters and uniques in their TRN recolors, items, spell and skill icons, objects and each level type's megatiles
