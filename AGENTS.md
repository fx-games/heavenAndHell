# AGENTS.md

Design decisions and deferred scope for the D1 rebuild: `D1_PLAN.md`.

## Reference material (`reference/`, gitignored, never commit)

| Path | What it is | Use it for |
|---|---|---|
| `reference/devilution-109` | [diasurgical/devilution](https://github.com/diasurgical/devilution): bin-exact reconstruction of Diablo 1.09 (C++, `Source/*.cpp`, `enums.h`, `structs.h`, `defs.h`) | **Ground truth.** Formulas, constants, order of `random_()` calls. Ignore `#ifdef HELLFIRE` / `SPAWN` branches. |
| `reference/devilution/2020_03_31` | [galaxyhaxz/devilution](https://github.com/galaxyhaxz/devilution): readable rewrite with the original table files in `Source/Data/Excel/xl_*.txt` | Readable second opinion; source of our `files/assets/data/d1/xl_*.txt` (copied verbatim). `2018_03_14` is an older snapshot, skip it. |
| `reference/devilutionX` | [diasurgical/devilutionX](https://github.com/diasurgical/devilutionX): modern, maintained port | Bug-fixed logic in `Source/` (`levels/`, `monsters/`, `items/`, `quests/`...), data tables as TSV in `assets/txtdata/`, and **test fixtures** in `test/` (see below). It fixes vanilla bugs and adds QoL; when it disagrees with 1.09, 1.09 wins unless `D1_PLAN.md` says otherwise. |
| `reference/DIABDAT.MPQ` | Diablo's own data archive (bring your own copy, ~500 MB) | Level data the sources don't contain: set pieces (`Levels\L1Data\SklKng1.DUN`...), `.TIL`/`.SOL` tile tables. Read with `d1.DiabDat()` / `mustReadMPQ` (`d1/mpq.go`, looks in `.`, `reference/`, `../reference/`). |

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
```

## Icons (tileset `files/assets/classic_roguelike.png`, 8×8)

Three independent rec files under `files/assets/data/icons/`, each read fresh on every game start (no rebuild
needed to see a remap take effect — just restart the game):

| File | Covers | H&H Go names | D1 names |
|---|---|---|---|
| `actors.rec` | monsters, unique monsters, player classes, corpses, companions, D1 npcs | `IconCharacter*`, `IconMonster*`, `IconUnique*`, `IconCorpse*`, `IconRogue*` | `monster.<MT_ID>` (with a `name:` the remapper shows), `unique.<Name>`, `npc.<name>` |
| `items.rec` | weapons, armor, potions, scrolls, rings... | `IconItem*` and the "Dark"/gold cosmetic set (`IconCloakDark`...) | `item.ITYPE_*` / `item.IMISC_*` (IMISC wins); every items.rec record has a `name:` the remapper shows |
| `tiles.rec` | terrain, doors, decals, effects, dungeon objects | everything else (`IconGrass`, `IconEffect*`, `IconDecalBlood*`...) | `prop.OBJ_*` (+ `.used` for a few), each with a `name:` the remapper shows |

The H&H names are `defs.IconXxx` package vars (`defs/icons_actors.go` / `icons_items.go` / `icons_tiles.go`,
generated 1:1 from the old `defs/definedIcons.go` consts — same names, now loaded from the rec file instead of
compiled in), read directly. The D1 names have no Go identifier; look them up with `defs.Icon("enemy.Zombie")`
(panics on a typo) or `defs.IconOK(name)` when the name is optional (`common/d1_item.go`'s IMISC fallback).

Remap visually with `./remap_actors.sh`, `./remap_items.sh`, `./remap_tiles.sh` (each opens the remapper on its
rec file: click a name on the left, a tile on the right, `s` saves, `F10` quits). Adding a new D1 object/monster
family/item type needs one new `internal_name:`/`icon:` record in the right rec file — remap it same as any other.
Tinting is reserved for D1 monster variants, water tiles (`IconWater`, tinted by `State.GetWaterColor`: light green in
the foul Poisoned Water Supply, blue otherwise) and one fallback: a used D1 object whose `prop.OBJ_*.used` icon is the
same tile as its fresh `prop.OBJ_*` icon is drawn grayed out (`common.D1UsedTint`); remap `.used` to a different tile
to drop the tint. Anything else that needs to look different (uniques, magic/unique items) gets its own icon record.
Remapper: `remapper` on `$PATH`, source `~/Projects/remapper`.

## Audio (`files/assets/`)

Sound effects are **events**, not folders: `files/assets/data/sounds.rec` has one record per event the game plays
(`internal_name` = the cue passed to `PlayCue`, e.g. `open_door` or `monster.MT_NZOMBIE.hit`; `name` = list label;
`tile` = icon name; one `sound:` per file relative to `files/assets/`, one picked at random at run time). Loaded by
`audio.Player.RegisterSoundEvents` on start; missing files are dropped (and `diablo_*` ones when
`DiabloAudioEnabled` is off), so a fresh clone without the Diablo folders still runs. Assign files visually with
`./remap_sounds.sh` (`sfxmapper`, source `~/Projects/remapper/sfxmapper`, install with
`go build -o ~/bin/sfxmapper ./sfxmapper`). A new sound event = one `PlayCue("<name>")` call + one record here.

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
  find files/assets/diablo_music files/assets/diablo_sfx -iname '*.wav' -print0 | \
    xargs -0 -I{} ffmpeg -i {} -ac 2 -c:a vorbis -strict -2 -qscale:a 3 {}.ogg
  ```
  (`-ac 2`: ffmpeg's Vorbis encoder needs stereo, some of D1's sfx are mono.) `diablo_music/`'s 6 tracks
  (`dtowne`/`dlvla`/`dlvlb`/`dlvlc`/`dlvld`/`dintro`) must be renamed to `town`/`cathedral`/`catacombs`/`caves`/
  `hell`/`intro` after conversion — that's what supplies `d1AreaMusic`'s cues. `diablo_sfx/`'s ~900 files
  (`misc/`, `monsters/`, `items/`, `warrior/`, `rogue/`, `sorceror/`, `towners/`, `narrator/`, `animals/`) are
  only used through the files `sounds.rec` points at.
- `voice/` — voice lines (`RegisterVoices`), doesn't exist yet; folder absence is fine, same as `ambience/`.

## Running

- Game: `go run -tags development .` (assets from `files/`, `config.rec` from the repo root)
- D1 unit tests: `go test ./d1`
- Headless smoke test: `go run -tags development . playthrough` (must print `PLAYTHROUGH OK`)
- Print a generated D1 level: `go run . dun_gen <dungeon level> [seed]`
