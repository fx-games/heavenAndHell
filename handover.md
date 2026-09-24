# Handover: switch D1 to the stand-in level generators

Branch `feat/d1-alt-generators` (off `feat/level-strategies-map-parts`), nothing done on it yet.
Last commit `947dbfa` holds everything below under "Done". Read `AGENTS.md` and `docs/D1_PLAN.md` first.
Rules from the user's memory: one engine, mods are data (no mod-specific layers); never rely on rec entry
order, key by id. Style: ponytail (shortest working diff, deletion over addition).

## Decision (user, 2026-09-23/24)

Seed-exact 1.09 levels don't matter (players can't pick seeds). Goal: D1's level *experience* without the
isometric cruft (megatiles, set pieces in megatile ids, wall/shadow fix-up passes). Replace D1's ported
generators with the stand-ins prototyped in `src/cmd/levelcmp`, then delete the megatile machinery.

## Done (commit 947dbfa)

- `files/mods/d1/maps/l<n>_megatiles.hhm` reduced to distinct 2x2 parts (36/25/51/16); id -> part lookup in
  `files/mods/d1/data/levels/generator.txt` `[table l<n>MegatileParts]` (read in `src/dungen/leveldata.go`).
- `tools/mapedit` Set pieces mode draws megatiles with the game's tiles (`piece_canvas.go` `partTiles`/`drawMega`).
- `src/cmd/levelcmp` (`main.go`, `alt.go` knob table `altTypes`, `passes.go`, `levelcmp_test.go`) +
  `scripts/levels_d1.sh` / `scripts/levels_alt.sh`: side-by-side ASCII viewer, keys 1-4 type, space reroll.
  Stand-ins: cathedral = bsp + D1 spine/chambers + arcades + pillars + loops; catacombs = megadungeon
  (`inner_room_chance` 0.45); caves = drunkard DLA + noise lakes + rivers + fords as lava; hell = cathedral
  generator on a 19x19 quadrant mirrored. Final `connect` pass guarantees connectivity; `TestAltConnected`
  checks 200 seeds per type (connected floor, doors floor/floor + wall/wall).
- dungen additions: `MegaDungeonConfig.InnerRoomChance`, megadungeon dead-end fill treats doors as floor (bug
  fix), `NoiseConfig` `Rivers`/`RiverWidth`/`RiverMeander`, `ford()` in `lakes.go`.
- Checks green at 947dbfa: `go test ./src/...`, mapedit tests, playthrough golden unchanged.

## To do (the switch)

1. Move the levelcmp -alt pipeline into `src/dungen` as Layouts/LevelStrategies configured from
   `files/mods/d1/data/levels.rec` (altTypes knobs -> rec fields; existing defaults unchanged so H&H stays
   identical). Passes become generic, config-enabled dungen passes. Keep short.
2. Every decoration the passes paint (cathedral pillar, arcade pillar, hell post, lava, ford, ...) gets its own
   `tile.<name>` record in `files/mods/d1/data/icons/tiles.rec` so the remapper lists it.
3. Convert set pieces + quest levels (`files/mods/d1/data/levels/l1..l4/*.txt`) into `.hhm` maps with entities,
   resolving megatile ids via the megatile sheets + `l<n>MegatileParts` BEFORE deleting those (one-off
   converter, don't commit). Place like D1:
   - cathedral (skngdo, rnd6, banner2): into a spine chamber (`l1SelectChamber`, `drlg_l1.go`)
   - catacombs (`l2InitSetPiece`, `drlg_quests.go`): reserved room of the piece's size (cf. accretion `require_room`)
   - caves (anvil): `l3PlaceAnvil`'s search (piece + 1 border all rock), then connect
   - hell: warlord in a reserved first room; Diablo quads diab1..4 at mirrored spots (`l4LoadDiabQuads`)
   - quest set levels (`setmaps.go`: sklkng, bonecha, foulwatr, vile) load whole; lever swaps (pdungeon,
     `objChangeMapResync`, `src/common/level_patches.go`) need the alternate layout as a second .hhm/layer.
   Keep triggers, entry/view positions, object ranges, monsters working.
4. Delete: `drlg_l1..l4.go`, `drlg_l1_tiles.go`, megatile-only parts of `drlg_quests.go`/`setmaps.go`/
   `leveldata*.go`/`gendung.go`, `strategy_drlg.go`, `generator.txt`, megatile sheets, set piece txt/til/sol,
   D1-seed tests, mapedit Set pieces mode (`piece_*.go`, `dun.go`, F2, README), `src/cmd/d1import` if only set
   pieces, levelcmp `-d1`. Update AGENTS.md + D1_PLAN.md. Run staticcheck U1000. Estimated ~7.5-8k Go lines out,
   ~600 in.
5. Tests: connectivity/doors per type (keep 200-seed test), every quest piece placed + reachable, each set level
   loads and lever swap works. Regenerate playthrough golden only after `go run ./src playthrough` prints
   `PLAYTHROUGH OK`; check the diff is level changes only.
6. Verify: `go build ./...`, `go vet`, `go test ./...`, `cd tools/mapedit && go build . && go test ./...`
   (building overwrites the tracked `tools/mapedit/mapedit` binary; `git checkout` it).

Work in phases, keep the tree building. If something forces a mod-specific layer, stop and ask the user.
Commit on this branch; messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
