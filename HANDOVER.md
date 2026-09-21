# Handover: engine/content split

You are picking up a large refactor in progress. Read this file, then
`ENGINE_SPLIT.md` (the architecture contract — it is authoritative and records
every decision and why).

## Where the work is

- **Branch: `engine-split`.** All work lives here. Nothing is merged to `d1-rebuild` or `main`.
- `d1-rebuild` and `main` are **untouched**. Do not merge to them without asking.
- `main` is an old version of the Heaven&Hell game. It is the SOURCE you read old
  content and code from (`git show main:<path>`), never a base to work on.
- The last session worked in worktree branch `worktree-agent-a223e92bbd2677d04`. If
  `engine-split` is behind it, fast-forward first: `git merge --ff-only worktree-agent-a223e92bbd2677d04`.

## The goal

One content-free engine plus mods. **One mod is one complete game** — mods do not
merge or layer. Two mods exist: `files/mods/d1/` (Diablo) and
`files/mods/heavenandhell/`. Each must reproduce its original game's behaviour.

Mods are **data only**: they declare content and configure engine features by
name. No scripting runtime. The one exception is expr-lang, approved for quest
trigger *conditions* only (never outcomes).

## Verify before and after every change

```bash
go build -tags embedded ./... && go build -tags development ./...
go test -tags embedded ./...        # packages
go test -tags development .         # ROOT package needs this tag
go run -tags development . playthrough 1
```

Three gates, all non-negotiable:

1. **`d1/testdata/levels.golden` must stay byte-identical.** NEVER regenerate it.
   It hashes tiles, flags AND every monster, object and item as full struct
   dumps, over 6 seeds x 20 levels. It is the safety net for every D1 change.
   The single exception, already recorded in the contract: a deliberate
   generator simplification, in its own labelled commit, one at a time.
2. **`playthrough 1` must print PLAYTHROUGH OK** and be byte-identical run to
   run. It is the behaviour-equivalence proof.
3. Both build tags green, no new test failures.

## Done

| Phase | What |
|---|---|
| 0 | Mod overlay FS (`files/mod.go`): `assets/x` resolves `mods/<active>/x` first |
| 2 | H&H effect engine restored from `main` (18 untargeted + 14 targeted + 4 item hooks) |
| 3 | D1 tables extracted to `mods/d1/*.rec` |
| 3.5 | Engine de-modded: no mod names in engine code, icons load after mod selection |
| 4 | Generators are registered strategies + 3 mod knobs; level->generator is data |
| 5/5b | D1 spells unified onto the H&H effect model; fallback switch 22 -> 8 cases |
| 6 | ONE monster/item loader, two mods. D1's native `xl_monst`/`xl_item` DELETED |
| 7 | ONE vendor record covering D1's filter model and H&H's list model |
| 8 | Quest triggers declarative: named event + expr condition + named outcome |
| 9 | `dungen/` restored as engine strategies behind a deliberately narrow seam |
| 10 | H&H's 12 levels generate, walk, and populate from its own tables |
| 11 | H&H playable except trading: hero table, town doors, quests + dialogue, own sheet/icons (see ENGINE_SPLIT.md phase 8) |
| 12 | Every mod owns its content: D1's sheet, icons and town map moved into `mods/d1`; all dialogue is `texts.rec` |

Also fixed along the way (all were real, pre-existing bugs):
- Shop menus crashed on any empty list (`items[0]` before the empty check).
- Map entry gave every monster a free turn: `RemoveEnergy` calls `BeforeTurn()`
  as a side effect, so walking downstairs healed the level and ticked debuffs.
  Split into `ResetEnergy` (no turn) vs `RemoveEnergy` (actor acted).
- The hero never staggered from damage arriving as a `ChangeResourceAction`.
- `LoadGame` asked the directory for its newest slot, not the running game's.
- The explored bitmap used a hardcoded `d1.LevelSize` stride, so any
  differently-sized level lost most of its cells on load.
- D1's quest availability table leaked into other mods by quest index.
- Every H&H monster was level 0: ~40% of kills panicked rolling a drop.
- H&H uniques spawned at random and never in their rooms.
- `build_release.sh` never copied `files/mods`: a non-embedded release had no game.

## The user's rules for mods (stated in the last session, binding)

- Each mod brings its own assets: tilesheet, icons, levels, town map, NPCs,
  quests, items, skills, spells. Nothing a mod needs lives in engine assets.
- Every mod uses the same engine subsystems and interfaces. One unified
  subsystem per concern, **no mod-specific implementations**: if D1 and H&H
  both need something, it is one engine feature both configure by data.
- All dialogue text is in rec format.

## Remaining

1. **Vendors, unified.** H&H's four NPCs open no shop. Design and pointers are in
   ENGINE_SPLIT.md phase 8 "Not done: d". This is the last gap to H&H playable.
   `hh_town_test.go` is where to prove it (talk -> buy -> sell).
2. **H&H leftovers from main**, not ported yet: the notes/evidence items and
   Lucifer's death text (`git show main:files/assets/dialogue/note_*.txt`,
   `lucifer_death.txt`), Kirioth's missing `kirioth_after_lucifer` text (main
   never had the file either), and H&H's own spells/skills (classes.rec names
   them; the engine gives H&H heroes none yet).
3. **Remaining engine-side D1 hardcodes** that break the no-mod-specific rule:
   D1 town quest hand-ins (`d1TownerQuests`), `UpdateD1Quests`, vendor menus.
4. **The fallback room.** With no mod, the engine should run one hardcoded room,
   one weapon, one enemy. Today a missing mod stops with a clear message
   (`mod "x" not found: expected a directory at files/mods/x`), which is the
   floor, not the goal. Engine `files/assets` no longer holds a sheet or map.
5. **The original gameplay requests**, deferred behind the port and never
   started. These are the user's own words:
   - Remove shrine effects with permanent negative outcomes. Decided:
     permanent stat/spell-level/HP/mana loss is out; temporary debuffs and gold
     loss are fine; replace with **cost-not-loss**; rename the shrines.
   - Remove monster abilities that permanently reduce player stats (Black
     Death's max-HP loss). Other drains stay. Make those monsters harder instead.
   - **Prevent shop save-scumming**: reloading must give the same stock; restock
     only on reaching a new level. Wirt's item likewise fixed per reload.
   - **Remove item destruction**: at 0 durability an item is fully inert (no
     base stats either) but repairable at max cost. Indestructible = never loses
     durability.
   - **Skill trees**: 2 points per level, D2-style tiers, 8 skills per class,
     10 ranks each, max 1 point per skill per level-up, respec in town for 1000
     gold. Skill points are ON TOP of attribute points. Save wipe is acceptable.
     Re-activate H&H spells/skills from git history.

## Traps that have already cost real time

- **Agent worktrees are created off `main`** — the wrong base, an old version of
  a different game. EVERY spawned agent must `git reset --hard engine-split`
  first and verify with `git log --oneline -1`. This has bitten every agent.
- **Content binding at package init.** `var x = load...()` runs before
  `files.SelectMod`, so content binds to nothing, silently. This has caused
  **six** bugs: zeroed class tables, two vacuously-passing test suites, icons
  resolving to tile 0, and more. Load content only after mod selection.
- **The engine rolls most dice on the package-level `math/rand`**, not the
  game's own source. Go 1.20+ seeds it itself and `rand.Seed` is a silent
  no-op; `//go:debug randseednop=0` restores it for reproducible runs.
- **Map iteration order leaks into behaviour** — placement, save order, which
  item a durability roll reaches first. Walk keys sorted. This is how
  reproducibility dies.
- **`playthrough` wipes its save dir at startup.** It is now keyed by PID, but
  before that, two concurrent runs destroyed each other's saves and produced
  symptoms identical to a real save/load bug. It caused a wrong diagnosis twice.
- **`recfile` gotchas**: `AsBool` accepts only the literal `true` (`yes` parses
  as false, silently); fields are trimmed, so a trailing space in a value is
  lost (this hit `"Book of "`); a blank line starts a NEW record and the config
  parser reads only `records[0]`, so a setting appended to the end of
  `config.rec` is ignored.
- **Effect arguments split on `|`, not `,`.** A whole batch of spell mappings
  silently resolved to nil until a test caught it.
- **Test a mod by playing it.** Every H&H bug last session (crash on class
  skill icon, town load nil deref, quest index panic, level-0 drops, uniques)
  was invisible to unit tests and fell out of one test driving a real game:
  `newHeavenAndHellGame` in `hh_town_test.go`. Walking `Actors()` while killing
  skips every other actor: clone the slice.

## Working agreements that produced good results

- Spawn agents in worktrees for large, file-disjoint chunks; do the small
  cross-cutting fixes yourself.
- Give every agent: the base-reset step, the three gates, "be lazy", and
  explicit **permission to stop at a boundary and report** rather than force an
  abstraction. The best outcomes here came from agents that stopped and said
  what resisted (the elixir rule, the town hand-ins, H&H's hero numbers).
- Demand a runnable check per phase, and verify it bites by breaking the thing
  it guards. Several "passing" tests were vacuous until mutated.
- Verify agent claims independently. Most were accurate; a few were not
  (a wrong branch name, an overstated baseline, one determinism claim that did
  not survive a merge).
