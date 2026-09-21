# Levels: one way to declare, one way to query

Decided in a grilling session (2026-09-21). Supersedes the old mix of `dlvl:`/`map:`/`town:` records, quest
`set_map` fields, depth-suffixed map names and `UpdateD1Quests`.

## Declaring

Every level — town, dungeon depth, quest level, side level — is one record in the mod's `data/levels.rec`.
Quests only refer to levels; they never declare one.

Common fields, owned by the engine:

| field | | meaning |
|---|---|---|
| `map` | required | the level's id: clean, opaque, never parsed (`town`, `cathedral_3`, `leoric_tomb`) |
| `name` | | display name |
| `depth` | required | difficulty and progress only: seeds, monster/item levels, "deepest", store stock. Town 0; a quest or side level takes its parent's depth. Several levels may share one |
| `generator` | required | the algorithm, one name per distinct algorithm in the engine (`d1_cathedral`, `d1_catacombs`, `d1_caves`, `d1_hell`, `d1_set`, `accretion`, `ldtk`, ...) |
| `tiles` | required | the tile set |
| `music` | | a `music.rec` event; without it the level has no music |
| `town` | | `true`: town rules (town-only spells, restocking, no monsters) |
| `start` | | `true` on exactly one record: where a new game begins |
| `exit` | repeatable | `exit: <location> -> <map id>, <arrival location>`, with an optional `open_when: <condition>` in the quest trigger language |
| `patch` | repeatable | `patch: <name> = <map file>, rect x1 y1 x2 y2`: a region stamped onto the level when a trigger applies it |

Each generator declares the extra fields it reads (D1: decorations, set pieces, `leveltype` for `d1_set`;
`accretion`: templates, rooms, doors; `ldtk`: `template`). Anything else is a load error, as are a missing
required field, an unknown generator, a duplicate `map`, an exit to a missing level or location, and zero or
several `start` records. One test loads every mod through this check.

There is no `type:`. D1's leveltype is a parameter of D1's generators (the cathedral algorithm is leveltype 1;
`d1_set` takes `leveltype:`, 1.09's `_qlvltype`). The engine has no notion of level type.

## Links

Every link is an explicit `exit`, both directions. Generators only place named locations (`stairs_up`,
`stairs_down`, `warp`, `altar`, ...); the record says where they lead. An exit whose `open_when` is false has
no transition. Quests never invent links, they satisfy conditions.

## Quests and map changes

A quest names its level (`level: cathedral_3`). Everything `UpdateD1Quests` did is declarative:
- gated exits: `open_when` on the exit;
- map changes (ObjChangeMap: Leoric's lever, the Bone Chamber book, the Pedestal, Snotspill): `patch` records,
  applied by `do: apply_patch(<map>, <patch>)`; applied patches are saved and re-applied on arrival;
- items placed on arrival (ResyncQuests): `on_enter_level` triggers with `do: place_item(<item>, <location>)`,
  guarded by a flag.
`UpdateD1Quests` is deleted.

## Querying

- `state.Level(id)` returns the parsed record. It is the only way to ask about a level.
- `state.Map(id)` / `state.SetMap(id, m)`: one store of built maps; a map's name is its id.
- No depth → level lookup: callers name ids. "Deepest" is the maximum depth over visited ids.
- The town portal leads to the deepest town visited (ties: the last visited).

Deleted: `LevelFromName`, `LevelNameFromDepth`, `IsTownLevel`/`SetTowns`/`LevelNameTown`, `d1DungeonLevel`,
`setLevelOf`/`onSetLevelOf`, `sideLevels`, `levelConfig`, `D1LevelType`, `d1AreaMusic`, the town/dungeon map
stores.

## Seeds

A generator derives its own seed from `(gameSeed, record)`. D1's generators keep `glSeedTbl[depth]` (the
golden dumps stay byte-identical); others use `hash(gameSeed, map id)`.

## Mods and saves

- No engine fallback game: a mod must exist, load and start, or the game stops with an error naming it.
- The default mod is `heavenandhell`.
- Old saves are not migrated: they stop loading.

## Order

1. Schema + loader + validation, clean ids in both mods, `Level(id)`, one map store; delete the name/depth helpers.
2. Explicit exits; generators only place locations; `open_when`.
3. Quests declarative (`level:`, patches, `place_item`); delete `UpdateD1Quests`.
4. Music and tiles from the record; `d1_set` with `leveltype`.
5. No fallback game; default mod `heavenandhell`.

Every step keeps the game playable and the 120 D1 golden dumps byte-identical.

## Status (handover) — 2026-09-21

Steps 1 and 2 are being done together (they are coupled: the stairs used to be linked by depth name).
Uncommitted work on branch `engine-split`; nothing of it is committed yet.

Done (compiles: `go build ./...` fails only in `playthrough.go` and tests, see below):
- `dungeon/dungeon.go`: new `Level` (Map, Name, Depth, Generator, Config, Tiles, Music, Town, Start, Exits) and
  `Exit` (Location, Target, Arrival, OpenWhen, Tile). `LoadLevels(recs)` validates required fields, unknown
  fields (a generator accepts what its Parse reads through `Field`/`FieldsNamed`/`Int`, tracked in `read`), unique
  ids, one start, exit targets. `LoadTowns`, `LoadSideLevels`, `Transition`, `UpTarget` are gone.
- `d1/strategy.go`: generators renamed `d1_cathedral|d1_catacombs|d1_caves|d1_hell`; new `d1_set`
  (`SetLevelParams{Quest}`; set level number and leveltype come from 1.09's quest table). D1's own config table
  is keyed by `depth:` and skips other generators.
- `common/levels.go` (new): `State.Level(id)`, `CurrentLevel`, `StartLevel`, `Levels`, one map store
  `Map/SetMap/HasMap/Maps`, `Visit` (deepest level + deepest town), `Deepest`, `DeepestLevel`, `DeepestTown`,
  `SetVisited`, `IsPlayerInTown`, `wireExits` (lays open exits as transitions; idempotent; runs on build, on
  every arrival in `MapTransition`, and in `UpdateD1Quests`), `exitTiles`.
- `common/map_gen.go`: levels by id; `GenerateMap(id)` dispatches on Config type (ldtk, d1.LevelParams,
  d1.SetLevelParams, dungen), then `wireExits`; `ldtk` generator registered (`template:`); open_when compiled
  at load (`compileQuestCondition` in quest_triggers.go). `LevelFromName` deleted.
- `common/map_load.go`: `loadLDTKLevel(lvl, template)` replaces `NewGridMapFromLDTK`; LDtk TargetMap is ignored.
- `common/map_gen_d1.go`: `generateD1Map(level, set)`; `placeD1Triggers` / `placeD1SetLevelReturn` only name
  locations (ladder_up, ladder_down, warp, quest key for quest entrances; lair: ladder_up, exit).
- `common/map_gen_dungen.go`: `addDungenStairs` places one stairwell per declared exit (ladder_up at the
  entrance, ladder_down far, others furthest); tiles from the record, required.
- `defs`: `LevelNameFromDepth`, `IsTownLevel`, `SetTowns`, `LevelNameTown`, `D1LevelName`, `D1Tilemap` deleted.
  `defs/quests.go`: quests have `level:`; `set_map/set_entrance/set_name/set_level` gone.
- Quest helpers: `d1SetQuest(id)`, `onSetLevelOf`, `questMap(q)` read d1_set records; `questLevel(q)`,
  `questOn` use quests' `level:`. `setLevelOf`, `SetLevels`, `D1SetLevel` gone. `UpdateD1Quests` no longer
  makes links (exits with open_when do); it still does quest vars, item spawns and map changes (step 3).
- Town portal: `D1Portal.Town` = deepest town at opening. Recall spell: to deepest level / deepest town.
- Music (step 4 part): `playLevelMusic` plays `Level.Music`, stops when empty; `d1AreaMusic` gone;
  `audio.Player.StopMusic` added. `D1LevelType` still exists (reads Config) — delete in step 4.
- Saves: `saveGame.DeepestLevel/DeepestTown`; old saves are not migrated (decided).
- Data: both mods' `levels.rec` rewritten (generated by script; D1 21 levels, H&H 15); D1 `quests.rec` has
  `level:` per quest, on_enter_level subjects are ids; H&H's on_enter_level subject too. D1 `maps.ldtk`
  TargetMap/TargetLocation values nulled.

Still to do for steps 1+2:
- H&H `maps.ldtk`: null the 5 TargetMap/TargetLocation values (its JSON formatting differs from D1's).
- `common/map_gen.go` LoadLevels: validate `tiles:` (a defs tile set, or `drawn` for ldtk).
- `playthrough.go` (dev tool) and tests: replace `defs.LevelNameFromDepth(n)` / `LevelNameTown` /
  `common.D1TownGates` / old map names with ids (`cathedral_1`, `town`, ...). A tests-only helper
  `levelAt(state, depth)` in package main is fine (first non-town level with that depth).
- Run `go build ./...`, `go test -tags embedded ./common ./d1 ./dungen ./dungeon`, `go test -tags development .`;
  the D1 golden dumps must stay byte-identical. Play both mods briefly (town gates, quest entrances, Lazarus).
- Update AGENTS.md mentions of levels.rec / LevelNameFromDepth, then commit as one "levels: one way" commit.

Then steps 3 (patches + place_item, delete UpdateD1Quests), 4 (delete D1LevelType), 5 (no fallback game,
default mod heavenandhell).
