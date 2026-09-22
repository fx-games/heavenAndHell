# Handover: engine/content split

You are picking up a large refactor in progress. Read this file, then
`ENGINE_SPLIT.md` (the architecture contract — it is authoritative and records
every decision and why).

## Where the work is

- **Branch: `engine-split`.** All work lives here. Nothing is merged to `d1-rebuild` or `main`.
- `d1-rebuild` and `main` are **untouched**. Do not merge to them without asking.
- `main` is an old version of the Heaven&Hell game. It is the SOURCE you read old
  content and code from (`git show main:<path>`), never a base to work on.
- Sessions work in a worktree branch and fast-forward `engine-split` to it when the
  gates are green (the user asked for that; last one: `worktree-agent-a223e92bbd2677d04`).
  Commit small and often: the user asked for commits in between, not one big one at the end.

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
   run. It is the behaviour-equivalence proof. A refactor must not change its
   output at all (`md5 -q` two runs, and against the run before your change). A
   deliberate gameplay change (the user's list below) changes it: diff the output
   and check every changed line is the intended effect, then say so in the commit.
   Current baseline, after the levels refactor: `160cc8b42230c81723947c7622e44d46`.
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
| 13 | One vendor subsystem: a stores.rec record names its `npc`s, `stock` strategy, `buys` rule, menu `service`s and a `when` condition. D1's shop switches are gone |
| 14 | H&H items equippable and complete: the item loader derives type/slot from `category`/`weapon_type`/`armor_type`; `equip_modifier` becomes item powers; use actions, light, ranged, throw, hidden, charges attach from the record. Starting kits and gold are classes.rec data for both mods |
| 15 | H&H class skills (effects/targeted.rec, untargeted.rec from main), Lucifer's death + gift, the four evidence notes, the hell town (`town:` records in levels.rec), main's enchantments as an H&H affix table |
| 16 | H&H monsters as their records say: general AI for records without a D1 brain, to-hit from dexterity, full HP, skills, gear, speech on sight, death lines, tags |
| 17 | D1's town quest talk (TalkToTowner) is `on_approach` triggers in D1's quests.rec |
| 18 | The fallback room: no mod / missing mod plays the engine's own one room, one weapon, one rat from `files/assets` |
| 19 | Each mod owns its music and sound exclusively; the engine has no audio. Saves are per mod (`saves/<mod>/`) and refuse another mod's game |
| 20 | Shrine gold costs, later reverted (D1 stays true to the original) |

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
- The playthrough depended on the time of day (the Solar shrine reads the clock):
  `common.Clock`, pinned by the playthrough.
- A monster record without animation frames cost 0 AUT per action and spun the
  turn loop forever; it now acts at its speed.

- Mod picker: with more than one directory under `files/mods/`, the start menu has a "Mod: <name>" entry and is
  always shown. Picking a mod writes `Mod:` into `config.rec` (`common.SaveMod`) and restarts the process
  (`game/save.go` `restart`). It restarts because icons, sprites, sounds and definitions are all loaded once per
  process.
- Sound audit: D1's registration, area music, call sites and quest lines (now `hero_sound`/`sound` in
  `quests.rec`) match d1-rebuild. H&H now matches main (the 7drl branch is merged into main):
  `MonsterSoundCue` falls back to `monster.<internal name>`, the generic kill path plays death sounds, and
  `heavy_door` is now the engine's `enter_dungeon`. H&H has no music, same as main. `close_door` is defined
  but never played, same as main.

## The user's rules for mods (stated in the last session, binding)

- Each mod brings its own assets: tilesheet, icons, levels, town map, NPCs,
  quests, items, skills, spells. Nothing a mod needs lives in engine assets.
- Every mod uses the same engine subsystems and interfaces. One unified
  subsystem per concern, **no mod-specific implementations**: if D1 and H&H
  both need something, it is one engine feature both configure by data.
- All dialogue text is in rec format.
- Each mod owns its music and sound effects exclusively.
- Save games store the mod they need and are only accessible while that mod is loaded.
- "We want all the content": everything H&H had on `main` must reach the game.
  `TestHeavenAndHellContentResolves` builds every H&H item, monster and named skill
  and fails on anything a record declares that the engine cannot build.

## Remaining

The plan (user, 2026-09-21):
- **D1 mod:** feature parity with the original Diablo, as close to 1:1 as possible, following the source in
  `reference/`. The earlier gameplay changes (no Black Death max-HP drain, fixed shop stock across reloads, no
  item destruction, skill trees) are dropped, because they depart from the original.
- **H&H mod:** show every feature that main and the 7drl branch (merged into main) already had for H&H.

1. **D1 parity, in progress.** Done: the D1 mod is 1.09 (b224ec8; all 120 golden levels identical to
   14a2d58^'s pre-Hellfire golden with the display passes off), shrines' permanent costs back, items break
   as in D1, potions always drink, the golem dies on a level change, traps' fire-arrow burst and nova as in
   1.09 (7fcd007), and every monster AI compared against monster.cpp (f25226a). Hellfire is opt-in engine
   content (see D1_PLAN.md "Defaults"). The engine faces east/west only - don't add 8-way facing (user).
   Next: the full system-by-system audit against `reference/devilution-109` - list every difference in
   D1_AUDIT.md first, then fix with the user. Known so far: trap missiles don't hit monsters
   (MonsterTrapHit); the golem's wake-up doesn't reproduce D1's off-by-one monster index; the Hidden's
   darkness test uses the hero's light radius, not D1's light map. Oracle for anything generator-side:
   regenerate `levels.golden` with `hangTorchesOnWalls`/`moveMonstersOffObjects` off and ` DamAcFlags:0`
   stripped from the item dump, and compare with `git show 14a2d58^:d1/testdata/levels.golden`.
2. **H&H leftovers**: main's on-hit item *hooks* (the other half of its
   enchantments; `common/def_itemhooks.go` has them) are not rolled. An affix
   power cannot carry one without a field on `d1.Item`, and `d1.Item` is dumped
   into `levels.golden` - adding a field changes that dump. Put the hook on the
   game item (`common.Item.SetItemHooks`) from a record field instead. Kirioth
   says nothing after Lucifer (main had no text for it either). The evidence
   notes carry main's placeholder text ("Are you sure about this?").
3. `UpdateD1Quests` (set-piece map changes) stays engine-side on purpose: it is
   D1's level family's business, like its generators (the seam in ENGINE_SPLIT.md).
4. One intermittent failure of the root package under `-tags embedded` was seen
   once and did not reproduce in 25 runs of the new tests or 4 of the package.
   If it recurs, capture the failing test's name before rerunning.

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
  `newHeavenAndHellGame` in `hh_town_test.go` (`newModGame(t, mod, ui)` for any
  mod, "" for the fallback room). Walking `Actors()` while killing skips every
  other actor: clone the slice.
- **Edit `maps.ldtk` as text, never by re-dumping the JSON**: `json.dumps`
  reformats the whole file (135k changed lines). Clone an entity's text block,
  and match block ends on the exact indentation - a looser match cut inside an
  inner brace and produced invalid JSON once.
- **A clock read breaks reproducibility** like a map iteration does. Any new
  wall-clock dependence goes through `common.Clock`.
- **Bisecting in zsh**: `for f in $FILES` does not split words in zsh; use
  `bash -c` or the run reverts nothing and "proves" nothing (cost an hour).
  Set work aside in a WIP commit first, never with bare `git stash`.
- **A mod lacking a file falls back to the engine's** `files/assets` copy, which
  is now the fallback game's content (hero.rec, classes.rec, items.rec,
  enemies.rec, icons, sheet, map). Both mods ship every one of those; a new mod
  that forgets one gets the fallback's silently.
- **The vendor and quest vocabularies are the engine's API**: `defs.VendorStocks`,
  `VendorBuyRules`, `VendorServices`; quest events in `common.QuestEvents`, outcomes
  in `QuestOutcomeExists`. Extend them there, and the loaders reject unknown names.

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
