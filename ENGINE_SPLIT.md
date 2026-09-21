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
