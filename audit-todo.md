# Audit todo

What the September 2026 repository audit found that has not been done yet, ordered by value. Done so far and
not repeated here: the bug fixes (seeded RNG, gossip seeds in State, atomic config writes, mod-data parse
warnings, the Bless default args), dependency updates and the deprecated Ebiten APIs, unified logging (log/slog,
`HH_LOG`), the d1 rule tables on `*d1.Data`, rules asking for windows through `UIRequest`, and actions reporting
`Event`s instead of building animations.

## 1. Finish separating rules from presentation (S2 follow-up)

**Where it stands.** No action builds an animation: actions `Emit` events (`common/events.go`) and `Present`
(`common/present.go`) turns them into animations and sounds. The "show this when…" callbacks are gone too: the
presenter decides for itself when a blow may be shown (`Broker.whenShown`: once the missile arrives, else once
the victim is drawn where it was hit), and it owns `visualGate`, `shownHealth` and `moveHold`. The presenter
rolls its own dice (`Broker.fx`) for what only affects the screen; where blood lands stays a rules roll, because
it is saved with the map, while when it appears is the presenter's.

What is still open:

- **Headless runs still animate.** Tests tick through animations nobody draws. `Broker.presenter` can now be
  nil; wire that up for tests and see what breaks (it should only be pacing, see below).
- **Presentation state on the model.** `Actor.visualGate`, `moveHold`, `shownHealth`, `DrawPosition`,
  `State.dying`, `State.worldTexture`, `State.animator`, `State.glowCache`. Only the presenter writes the first
  three now. *Suggestion:* move them into a view model the presenter owns, keyed by actor.
- **The broker paces the game by animations.** `checkForQueuedActions` runs queued actions only when no
  animation is running, so a UI pacing decision sits in the rules' scheduler. *Suggestion:* a `Pacer` interface
  the frontend implements (`Busy() bool`), nil meaning never busy. Needed before a nil presenter is useful.
- **Sounds are still played by the rules.** Melee swings, bow twangs, door and item cues, `PlayUICue` on death.
  *Suggestion:* the events are already there (`MeleeSwing`, `Shot`, `Died` would be new) - move each cue to the
  presenter as its event is added.

## 2. Rules still querying the UI (S3 follow-up)

- `IsPlayerForcingAggression()` is read by two untargeted effects (`def_magic_untargeted.go`) through
  `Factory.UI`. It is input state. *Suggestion:* the input layer puts it on the action (a `Forced bool` on the
  cast), and `Factory.UI` goes away.
- `ActionContext.UserInterface` and `Factory.UI` still exist for that and for `CancelTargeting` in the broker.
  With the query gone, drop both and let `ActionContext.Show` be the rules' only way to the frontend.
- `UserInterface` has about 48 methods, and debug toggles sit next to gameplay prompts. *Suggestion:* split it
  into small interfaces owned by their callers (`Prompter`, `Notifier`, `DebugView`); the gui and console
  implementations stay as they are.

## 3. Split `common` (S1)

`common` is about 21.6k lines in 109 files, and its file prefixes are packages in disguise: `act_*`
(actions), `anim_*` + `present.go` (presentation), `d1_*` (the Diablo rules), `def_*` (effects), `base_*` /
`o_*` (model), `a_*` / `z_*` (plumbing and hooks). *Suggestion:* move out in this order: `anim_*` + `present.go` → `fx` (depends on the model, not the other way
round), then `d1_*` → `rules/d1`, then `act_*` → `rules`. Each step is mechanical once nothing points back.
`docs/ENGINE_SPLIT.md` already sets the direction (no d1-shaped concepts in engine APIs); this is the package
side of it.

## 4. D1 knowledge in the engine

- 118 hardcoded `"AI_*"` / `"MT_*"` / `"MC_*"` strings in `common` and `game` (`d1Talkers`,
  `d1NeverFightsWithoutQuest`, `d1FallenRunDist`, `d1MonsterClassNames`, …). Each is a monster property.
  *Suggestion:* a column in the mod's `monsters.rec` (`talks: true`, `never_fights_without_quest: true`,
  `run_distance: 7`), read into `d1.MonsterData`.
- `Actor.hero`, `Actor.d1Monster` and `Item.d1` are optional D1 parts guarded by about 100 nil checks.
  *Suggestion:* no big-bang change. Stop adding new ones: new features take a small interface
  (`type Leveler interface{ Level() int }`), so an actor without the D1 part simply doesn't implement it.

## 5. God objects (S4)

Method counts: `Factory` 157, `UIController` 152, `Actor` 121, `Item` 120, `State` 115, `GameInstance` 73.
`Factory` works as a service locator (State, broker, generator, RNG, UI) and `State` points back to it.
*Suggestion:* split by what changes together, not by size. `Factory`'s spell registries (`GetAllUntargetedEffects`
and friends) → an `effects` value the factory holds; its D1 object and trap code → `rules/d1` (item 3).
`UIController`: the shop, inventory and character sheet each become their own type holding a pointer to the
controller. Don't split `Actor` / `Item` before item 4.

## 6. Tests

Cross-package coverage is about 46%: `d1` 88%, `dungen` 88%, `common` 56%, `game` 43%, `gridmap` 45%, gui
and console under 10%.

- The fixed-seed playthrough is the best regression check there is (it caught nothing in this audit because
  every refactor kept it byte-identical, which is the point). *Suggestion:* make it a test: record its summary
  lines in `testdata/playthrough_seed1.txt` and diff against them. Right now it is only run by hand
  (`go run . playthrough 1`).
- `deadcode` misses methods of generic types (`GridMap`'s `TryGetDownedActorAt`, `SaveToDisk`,
  `RandomPosAround` and more were dead). Use `staticcheck -checks U1000` in CI as well.
- Knockback has no test; its damage gating changed from "animation done" to "drawn where it landed".

## 7. Smaller items

- **Check text rendering by eye.** The move to `text/v2` keeps the baseline where v1 put it (`drawTTF`), but
  nothing tests pixels. Look at the log, tooltips, shop and the colour-coded lines once.
- **Unused generators.** `rogue` and `megadungeon` (`dungen/`, about 700 lines with tests) are registered but no
  mod's `levels.rec` uses them. Keep them if they are there for modders; otherwise delete them.
- **Sound variety uses the global RNG.** `audio.Sound.Next` picks random variants with `math/rand`'s global
  source. It's harmless (not game state) but inconsistent; use the presenter's `Broker.fx` instead.
- **Config write failure panics.** `Configuration.SaveToRecFile` panics if the write fails (now atomic). Return
  the error and log it; losing a settings change is not worth a crash.
- **New indirect dependencies.** `text/v2` pulled in `go-text/typesetting` and `srwiley/rasterx`. They're pure Go
  (the cgo-free build still works), but the binary is bigger. Nothing to do; just know it.
- **`d1GossipSeeds` pattern.** Other package-level state in `common`: `UICues` (the audio player), `Clock` (the
  Solar shrine's hour), `questConditions`, `itemNameTemplates`. The last two are caches of mod data, so they
  belong on the mod's data, like `d1.Data` now. `UICues` and `Clock` are test seams and fine as they are.
