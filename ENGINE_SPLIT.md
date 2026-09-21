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

## Known broken: the playthrough smoke test

`go run -tags development . playthrough` runs a scripted end-to-end game and
asserts behaviours. It FAILS on `d1-rebuild` too, with several independent
broken checks, and reports a different one depending on ordering: doppelganger
(replicate), hit recovery staggering, Hellfire Lightning Wall. It is not a
regression from the split.

It is the natural proof of "the mods behave like their branches", so it gets
repaired near the end, once the effect system is unified - several of its
failures look like missing effects, which phase 5 is restoring anyway.

## Open hazard: icons fail silently

`defs.Icon*` are package vars filled by `defs.LoadIcons()` after mod selection.
Nothing forces that call, and a name a mod's `icons/*.rec` does not define
resolves to 0 rather than erroring. Four bugs in this project have already come
from content binding silently to nothing.

This becomes load-bearing the moment a SECOND mod ships its own icon names, so
extracting the H&H content must also make a missing icon name loud: fail the
load naming every icon the mod did not supply, rather than rendering tile 0.

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
