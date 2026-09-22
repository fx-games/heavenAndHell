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

Each generator declares the extra fields it reads (D1: decorations, set pieces, `quest` for `d1_set`;
`accretion`: templates, rooms, doors; `ldtk`: `template`). Anything else is a load error, as are a missing
required field, an unknown generator, a duplicate `map`, an exit to a missing level or location, and zero or
several `start` records. One test loads every mod through this check.

There is no `type:`. D1's leveltype belongs to D1's generators (the cathedral algorithm is leveltype 1; `d1_set`
names its quest, and 1.09's quest table gives the set level and its `_qlvltype`). The engine has no notion of
level type.

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
4. Music and tiles from the record; delete `D1LevelType`.
5. No fallback game; default mod `heavenandhell`.

Every step keeps the game playable and the 120 D1 golden dumps byte-identical.

## Status — 2026-09-22

Steps 1 and 2 are done and committed ("levels: one way"): every level is a levels.rec record with explicit
exits, `State.Level(id)` and one map store, generators only place locations, `open_when` gates exits. The
120 D1 golden dumps are byte-identical; `playthrough 1` passes on the d1 mod (it now pins that mod itself, as
does the root test package, instead of following config.rec).

Next: steps 3 (patches + place_item, delete `UpdateD1Quests`), 4 (delete `D1LevelType`), 5 (no fallback game,
default mod heavenandhell; `TestFallbackRoom` is skipped until then).
