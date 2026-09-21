# Engine / content split

Goal: one content-free engine binary plus mods. **One mod is one complete game** —
mods do not merge or layer. `mods/d1/` and `mods/heavenandhell/` each reproduce
their branch's game exactly.

Mods are **data only**. They declare content and configure engine features by name;
they cannot ship new behaviour. No scripting runtime, no mod-defined generators.

## Layout

    files/assets/          engine-owned: fonts, shaders, fallback room
    files/mods/<name>/     one complete game

`files/mod.go` resolves `assets/x` to `mods/<active>/x` first, engine second.

## The engine is not split along the mod axis

Subsystems are unified, never forked per game. There is ONE dungeon generation
subsystem, ONE spell effect subsystem, ONE item subsystem. A mod never gets its
own branch of engine code and the engine never knows a mod's name.

- Generation algorithms follow a strategy pattern: each registers a NAME and a
  parameter set. A mod declares `level 3: generator=caves, params...`.
- One abstract effect system expresses every effect. A mod maps effects onto
  scrolls, books, innate skills or learnable skills; it does not define effects.

Corollary: no `d1`-shaped concepts in engine APIs, and no engine code that
resolves a path containing a mod name.

## Boundary

Engine (Go):
- dungeon generation: DRLG l1-l4, gendung, themes, rng, level assembly
- effect and action implementations, animations, item hooks
- quest *logic* / state machines
- everything that is control flow

Mod (rec/tsv):
- items, affixes, monsters, objects, stores, drops, chances, spells' numbers
- level configs naming an engine generator: `generator: drlg_l1, theme: cathedral`
- dialogue text, quest text, rewards, spawn tables, placement

## Decisions

- REVERSED: `dungen/` comes back. It was dropped on the reading that D1's
  generators supersede it, but H&H cannot be PLAYED without its own generators,
  and a mod that cannot be played is not a mod. `dungen/`'s algorithms
  (accretion, megadungeon, rogue, room) return as engine generation strategies
  registered by name, exactly like cathedral/catacombs/caves/hell. H&H also
  needs its town, NPCs, vendors and quests back.
- Still dropped: `defs/enemy.go`, `defs/item.go` (the unified `defs.Monster` /
  `defs.Item` superseded them), `defs/vendor.go` (superseded by the unified
  vendor record).
- Ported back from `main` as engine: 18 files of H&H effects/actions/animations,
  giving 18 untargeted + 14 targeted effects + 4 item hooks, reachable by name.
- Quest triggers use expr-lang expressions over the existing `common.Flags` store,
  evaluated on **named events only** (on_kill, on_enter_level, on_item_pickup, ...),
  never polled. Outcomes are a fixed set the engine implements.
  The existing hook system (`OnKillHook`, `ItemActionHook`) is the event spine;
  no new event bus.
  Done, partly: `mods/d1/data/quests.rec` holds all sixteen quests' key, name,
  log text, per-NPC gossip and set-level placement, plus triggers on four named
  events (`on_kill`, `on_item_pickup`, `on_enter_level`, `on_talk`) and nine
  named outcomes. What did NOT move, and why: the town hand-ins, the talking
  bosses' state machines, `UpdateD1Quests`/`replayD1MapChanges` and the quest
  objects all sequence side effects (take this item, then spawn that reward
  there, then reopen this room) rather than test a condition. Expressing them
  would need a scripting language, which is the thing this rule exists to
  avoid, so their logic stays Go and their text and placement come from data.
- The H&H effect model is the one true effect system: name + args + hit zone +
  targeting rules, resolvable from a string. D1 spells are re-expressed as
  mappings onto it. Missing pieces get ADDED to the H&H system in its own style,
  never forked. D1 spells must "play the same"; bit-exactness is not required.
- Content loads only AFTER mod selection. No package-level `var x = load...()`.
  This has caused three silent bugs already (zeroed class tables, two vacuous
  test suites); treat init-time content binding as a defect on sight.
- Dungeon generation output must stay byte-identical: `d1/testdata/levels.golden`
  is the referee. A golden diff is a bug, never a regeneration.
  Its scope is wide: `hashLevel` hashes tiles, flags, AND every monster, object
  and item as a full struct dump, over 6 game seeds x 20 levels. So migrating D1
  content to the unified schema is checkable, not faith-based - identical structs
  out means identical content in.
  The ONE exception: deliberate simplification of the generators is allowed to
  change output, in its own labelled commit, one simplification at a time, with
  the before/after summarised. Refactors never change it.

## Both games must be PLAYABLE, not merely loadable

A mod is done when you can play it, not when its files parse. For H&H that
means: generators (`dungen/`), a town, NPCs, vendors and quests. Content
without the means to reach it is not a finished mod.

## The generator seam: narrow on purpose

`dungeon/` holds ONE registry and a deliberately tiny interface:

    type Config any
    type Strategy interface { Parse(recfile.Record) (Config, error) }

The engine's only generic business with a generator is "does this name resolve,
and do the knobs the mod declared for it parse" - so a typo in `levels.rec`
fails when the mod loads, not on the stairs down. Past that, D1's and H&H's
families share nothing that is not a lie: their intersection (bare terrain)
throws away everything D1 produces (SOL, set pieces, quest triggers, monsters,
objects), and their union forces `ltype`, megatile fills and quest set pieces
into a type H&H would have to carry and never use.

So level building stays on each family's own side (`d1.CreateLevel`,
`dungen.Build`) and each recovers its own type from `Config` with one type
assertion. That assertion IS the seam, not a workaround for it. Widening this
interface to the union of both families is the failure mode to resist.

`rogue.go` did not come back: it places nine rooms and never connects them, so
every level it builds fails the walkability check. Unfinished code, not a
generator.

## One content schema

The H&H `.rec` schema is the unified content model, for the same reason the H&H
effect model is: it is already declarative, names effects by string, and uses
dice notation. D1's native `xl_*.txt` tables are an IMPORT SOURCE, not a second
content model.

    Diablo's own files --[cmd/d1import, offline dev tool]--> mods/d1/**.rec
    mods/<any>/**.rec  --[engine, one loader]-->              game

`cmd/d1import` and `cmd/d1sprites` are offline developer tools and may know
about Diablo's formats. The ENGINE may not: it reads one schema, whichever mod
supplied it.

## The playthrough is the equivalence proof - keep it deterministic

`go run -tags development . playthrough [seed]` is reproducible: the same seed
gives byte-identical output. Verified 3 seeds x 3 runs, plus the fixing agent's
4 seeds x 5 runs. Run it before merging anything.

Four sources of nondeterminism had to die for that, and they are worth knowing
because the same mistakes are easy to make again:

1. The engine rolls most dice on the PACKAGE-LEVEL `math/rand`, not the game's
   own source. Since Go 1.20 that seeds itself and `rand.Seed` is a silent
   no-op - `//go:debug randseednop=0` restores it for reproducible runs only.
2. `gridmap.GetFreeCellsForDistribution` popped its open list out of a map and
   sorted unstably, so placement varied. Now a FIFO flood with coordinate ties.
3. `ChangeResourceAction` rolled inside an animation callback (frame-timed) off
   a `time.Now()` source.
4. `Equipment.AllItems()` ranged the slots MAP, so save order and which item a
   durability roll reached first varied. Walked in name order now.

Still open, deliberately: `common/d1_object.go` branches on `time.Now().Hour()`,
so a run can differ across an hour boundary.

Two real save defects were hiding behind that variance, now fixed and pinned by
tests: `LoadGame` asked the directory for its newest slot instead of loading the
running game's own slot, and the explored bitmap was written and read at a
hardcoded `d1.LevelSize` stride rather than the map's width, so any level of
another size lost most of its cells. The stride now travels with the bitmap.

## Phases

0. mod overlay filesystem — done
1. fallback room: one hardcoded room, weapon, enemy, no mod loaded
2. port H&H effect engine back from `main`
3. extract D1 tables to `mods/d1/`
4. extract H&H content to `mods/heavenandhell/`
5. quests + dialogue declarative (expr triggers)
6. per-mod level configs
7. delete hardcoded paths, both mods boot

Gameplay changes (shrines, monster abilities, shop seeding, durability, skill trees)
land after the boundary exists, not before.
