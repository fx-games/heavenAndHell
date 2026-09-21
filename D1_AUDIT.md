# D1 mod vs Diablo 1.09 — audit

Every known difference between the D1 mod and `reference/devilution-109/Source` (the non-HELLFIRE side). The D1
mod is meant to be 1.09 1:1; the one allowed difference is real-time → turn-based (see D1_PLAN.md "Defaults").
Each entry: what 1.09 does, what we do, where. **Status**: open / fixed (commit) / by design (turn-based).

Already matched and verified: level generation, monster/object/item placement and loot rolls (all 120 golden
dumps identical to 1.09's), stores (b224ec8), traps' burst and nova (7fcd007), every MAI_* (f25226a).

## By design (turn-based)

- **RNG sequence.** D1 rolls one shared seed per tick; our AI and combat draw from the game's own sources, so
  the same seed plays out differently. Probabilities and decision logic are 1.09's, the roll order is not.
- **Player/monster mode.** D1 only lets the hero block while standing or attacking (`M_TryH2HHit`); a turn has
  no mode, so the hero always may block.
- **Missiles resolve instantly** along their path (decision 6); D1's per-tick flight is collapsed into one turn.

## Combat

- **open — CheckMonsterHit is not ported** (`monster.cpp` 6658, called from `PlrHitMonst`, `MonsterMHit`,
  `M_TryM2MHit`). 1.09: a blow at a dormant gargoyle (`MFLAG_ALLOW_SPECIAL`) wakes it and does no damage; a
  counselor/magistrate/cabalist/advocate that isn't `MGOAL_NORMAL` (fading, retreating, circling) cannot be hit.
  Ours: both take the blow normally. `common/d1_hero.go` resolveD1Attack.
- **open — untouchable monsters.** `PlrHitMonst` returns FALSE for an Illusion Weaver in `MGOAL_RETREAT` and any
  monster in `MM_CHARGE`. Ours hits them.
- **open — Black Death's max-HP drain** (`M_TryH2HHit`): every landed hit by `MT_YZOMBIE` takes 1 from the
  hero's max HP and base max HP, permanently, while both are above 1. Not implemented at all.
- **open — life steal stacks.** `PlrHitMonst` takes 3% *or* 5% (the 5% flag overrides the 3%), plus the random
  steal. `d1.Hero.HitLeech` adds 3% and 5% together when an item has both.

## Traps

- **open — trap missiles only hit the hero.** 1.09's `CheckMissileCol` for a sourceless missile also hits
  monsters (`MonsterTrapHit`), and a golem for monster missiles. `common/d1_trap.go`, `common/d1_mmissile.go`.

## Monster AI (f25226a)

- **open — the golem's wake-up.** `MAI_Golum` wakes the 5x5 block around the golem through `monster[_menemy]`
  instead of `monster[_menemy-1]`, so it wakes the *next* monster in the table. We wake the ones standing there;
  our monsters have no table index to reproduce the bug with.
- **open — darkness.** `MAI_Sneak` idles where `dLight == lightmax`. We have no light map: a cell counts as dark
  outside the hero's light radius and any lit room.
- **open — combat: experience at the cap.** `AddPlrExperience` sets level 50 outright once experience reaches
  `ExpLvlsTbl[49]`, without `NextPlrLevel` (no stat points, life or mana for those levels), and gives a dead
  hero nothing. `d1.Hero.AddExperience` levels up normally.

## Shrines (`OperateShrine`)

Compared all 26: Mysterious, Hidden, Gloomy, Weird, Stone, Religious, Enchanted, Thaumaturgic, Fascinating,
Cryptic, Eldritch, Eerie, Divine, Holy, Sacred, Abandoned, Creepy, Quiet, Secluded, Ornate and Glimmering match
(Spooky and Tainted are multiplayer only).

- **open — Magical.** 1.09 casts the mana shield at spell level `2 * leveltype` (`AddMissile(..., MIS_MANASHIELD,
  ..., 2 * leveltype)`), so its damage reduction comes from that level. Ours raises the hero's mana shield status,
  which then reduces by the hero's own Mana Shield level (0 when unknown).
- **by design — Spiritual.** 1.09 fills every empty cell of the 40-cell inventory grid with a gold pile; this
  engine's pack has stacks, not a grid, so ours adds one pile per free stack.

## Spells

Matched: the damage of Firebolt, Holy Bolt, Charged Bolt, Fireball, Elemental, Nova, Apocalypse, Flash, Fire
Wall, Healing and Infravision (`d1/spells.go` against the `Add*` functions), the Guardian's firebolts at the hero's
own Firebolt level, Phasing / the Holy shrine (`AddRndTeleport`), the mana shield having no timer.

- **open — Mana Shield's reduction.** 1.09's `MI_Manashield` takes a third off the life lost once the spell
  level is above 0 (`diff += diff / -3`). Hellfire instead divides by `3·min(slvl, 7)`. `d1.ManaShieldCost`
  applies both, one after the other, so it matches neither game.
- **open — Chain Lightning.** `MI_Chain` sends one lightning bolt (`MIS_LIGHTCTRL`) at the target, and one more at
  every monster within `slvl + 2` cells of the caster (CrawlTable rings `1 .. slvl+2`, capped at 18). Ours is a
  beam that jumps up to 6 times within 7 cells (`hit_zone: chain_aoe(6|7)`).
- **open — Guardian's aim.** `MI_Guardian` fires every 16 ticks at the first monster its scan meets: each of the
  23 `vCrawlTable` rays, from its 6th point inwards, each point in four mirrored quadrants, on a clear line
  (`Sentfire`). So it reaches 6 cells and prefers far targets in a fixed order. Ours shoots the closest monster
  within 15 cells.
- **open — Stone Curse.** `AddStone` petrifies the first monster in the six CrawlTable rings around the target
  cell (never Diablo, never one fading or charging), for `min(slvl + 6, 15) · 16` ticks. Ours only curses a
  monster on the target cell, for a flat 12 turns (`apply_status(stone|12)`), and Diablo can be cursed.
- **open — Apocalypse's box.** `AddApoca`/`MI_Apoca` cover x−8 … x+7 and y−8 … y+7 (the upper bound is
  exclusive), with no line-of-sight test (Hellfire added one). Our `box_aoe(8)` is −8 … +8.
- **open — Flame Wave's shape.** `MI_Wave` lays a straight wall of `MIS_FIREMOVE` flames across the cast
  direction, `1 + 2·(slvl/2 + 2)` cells wide (each side stops at the first cell missiles can't cross), and the
  flames roll forward. Ours is a cone that widens to 3 cells either side over 8 (`hit_zone: cone_aoe(8|3)`).

## Not yet audited

Lightning's and Inferno's per-cell burn, Blood Star, Bone Spirit, Telekinesis, Teleport, Town Portal, Golem,
Identify; objects (chests, barrels, sarcophagi, bookcases, racks, decapitated bodies, crucified skeletons,
levers, books, fountains, cauldron, goat shrines, pedestal); the 16 quests; monster special attacks and unique
monster traits; monster drops and gold; durability, repair and identification; the hero's death; town
dialogue.
