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

- Dropped, not ported: `dungen/`, `defs/enemy.go`, `defs/vendor.go`, `defs/item.go`.
  The D1 equivalents supersede them.
- Ported back from `main` as engine: 18 files of H&H effects/actions/animations,
  giving 18 untargeted + 14 targeted effects + 4 item hooks, reachable by name.
- Quest triggers use expr-lang expressions over the existing `common.Flags` store,
  evaluated on **named events only** (on_kill, on_enter_level, on_item_pickup, ...),
  never polled. Outcomes are a fixed set the engine implements.
  The existing hook system (`OnKillHook`, `ItemActionHook`) is the event spine;
  no new event bus.
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

## The playthrough is nondeterministic - and hides two real save bugs

Seeding both time-seeded RNGs (instance source + `common.d1Rng`, pinned by
`playthrough <seed>`) was NOT enough: the same seed still produces different
outcomes run to run. Something below the RNGs varies - Go map iteration order
is the prime suspect, since the failures cluster in save/load.

The residual variance surfaces two genuine defects, both intermittent:

1. `load failed: open .../slot0.json: no such file or directory` - a load asks
   for slot 0 when the run saved into slot 1, so `g.slot` is not what the
   loader thinks it is.
2. `load forgot explored cells (1253 -> 305)` / `(564 -> 150)` - the explored
   bitmap comes back short, so a loaded game forgets most of the map.

Both are in the same area as the already-fixed monster-status-on-load bug, so
a common cause is plausible. Until they are fixed, ONE green playthrough run
is weak evidence: run it several times.

## The playthrough smoke test now passes

`go run -tags development . playthrough` reports PLAYTHROUGH OK as of the
phase 5 merge plus the hero-flinch fix. Keep it that way: it is the proof that
a mod still behaves like its branch. Run it before merging any phase.

**It is flaky, roughly one run in two**, and has been since that commit
(`fc2e91b`, verified by running it there six times: three failures). It always
fails the same check:

    PLAYTHROUGH FAILED: doppelganger: 200 blows at <monster> split off no copy

`checkDoppelganger` picks the first eligible monster on the level and demands it
replicate within 200 blows; whether that monster replicates is an unseeded roll.
Run it more than once before believing either result. Fixing it means seeding
that roll or picking a monster that must replicate — not regenerating anything.

Historical note, since it shaped several decisions:

`go run -tags development . playthrough` runs a scripted end-to-end game and
asserts behaviours. It FAILS on `d1-rebuild` too, with several independent
broken checks, and reports a different one depending on ordering: doppelganger
(replicate), hit recovery staggering, Hellfire Lightning Wall. It is not a
regression from the split.

It is the natural proof of "the mods behave like their branches", so it gets
repaired near the end, once the effect system is unified - several of its
failures look like missing effects, which phase 5 is restoring anyway.

## Where the second mod stands

`mods/heavenandhell/` exists and its monsters, items and icons load through the
same code as `mods/d1/`'s — `defs.ReadMonsters`, `defs.ReadItems`,
`defs.LoadIcons`, one loader each, no branch on the mod's name. `d1.MonsterData`
and `d1.ItemData` are aliases for `defs.Monster` and `defs.Item`; Diablo's
`xl_monst.txt` and `xl_item.txt` are gone, converted into the unified `.rec`
schema, with `levels.golden` unchanged as the proof.

What that convergence cost, for the next one:

- `health`/`damage`/`weapon_damage` are `util.Dice` and `armor_class` a
  `stats.Interval`. D1's min..max and H&H's XdY are the same value written two
  ways ("4-7" is 1d4+3), and `ParseDice` already read both.
- A `.rec` field is trimmed, so a value cannot end in a space. D1's "Book of "
  relied on one; the separator moved into `bookSpell`, where it always belonged.
- Diablo's `NULL` sentinel is not content. Columns that held it are simply
  absent from the records.
- `MonstAvailTbl`, a Go literal running parallel to the monster table, is now an
  `available` field on the record it belonged to.

### Not converged, and why

Stores, effects and dialogue are still one game's each. Each needs engine work
first, and faking it would mean a second loader:

- **Stores.** D1 stocks a vendor by running a Go predicate over the item table
  (`storeIdx`, `RndSmithItem` and friends); H&H lists stock per store in
  `stores.rec`. Converging means either a filter expression in content — which
  this document rules out — or making D1's vendors declarative. The latter is
  the real job: a vendor record of level range, purse, slot count and an item
  filter expressed in the columns items already have.
- **Effects.** `NewUntargetedActionFromName` resolves straight from the registry;
  `defs.ReadTargetedDefinitions`/`ReadUntargetedDefinitions` exist but nothing
  calls them. Giving effects a content layer means giving D1's spells one too,
  or the mods diverge again.
- **Dialogue.** The formats converge cheaply (H&H's prose files become `TEXT_*`
  blocks `loadTexts` already reads), but no H&H NPC, town or quest is content on
  this branch, so nothing would speak the lines.

The engine's `Icon*` list is the other unfinished edge: it is a fixed set of Go
vars naming D1 and Hellfire's content (six class icons for classes H&H does not
have, dungeon furniture it does not draw). A mod's classes should come from its
own `classes.rec` rather than from that list.

## Open hazard: icons fail silently

`defs.Icon*` are package vars filled by `defs.LoadIcons()` after mod selection.
Nothing forces that call, and a name a mod's `icons/*.rec` does not define
resolves to 0 rather than erroring. Four bugs in this project have already come
from content binding silently to nothing.

This becomes load-bearing the moment a SECOND mod ships its own icon names, so
extracting the H&H content must also make a missing icon name loud: fail the
load naming every icon the mod did not supply, rather than rendering tile 0.

Done: `LoadIcons` returns an error listing every name the mod did not define.
It earned itself immediately — it is what produced the 15-name gap above.

## Verified behaviour of a missing mod (phase 1 input)

With `Mod: nosuchmod`, the engine today dies with:

    open assets/data/xl_monst.txt: no such file or directory

The overlay falls through to `assets/`, which holds no game content, and the
first content read fails with a path the player cannot act on. That is the
current "no mod" state, and it is what the fallback room replaces: one
hardcoded room, one weapon, one enemy, plus a clear message naming the mod
that could not be found.

Papercut found alongside it: the config parser reads `records[0]` only. Today
`config.rec` is ONE record holding every setting (MiniMapColor included, so
nothing is being dropped) followed by a trailing blank line. But appending a
setting at the end of the file - the obvious thing for a player to do - lands
after that blank line, becomes a second record, and is silently ignored.
Worth warning when a later record holds keys, rather than dropping them.

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
