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

- **fixed — CheckMonsterHit is not ported** (`monster.cpp` 6658, called from `PlrHitMonst`, `MonsterMHit`,
  `M_TryM2MHit`). 1.09: a blow at a dormant gargoyle (`MFLAG_ALLOW_SPECIAL`) wakes it and does no damage; a
  counselor/magistrate/cabalist/advocate that isn't `MGOAL_NORMAL` (fading, retreating, circling) cannot be hit.
  Ours: both take the blow normally. `common/d1_hero.go` resolveD1Attack.
- **fixed — untouchable monsters.** `PlrHitMonst` returns FALSE for an Illusion Weaver in `MGOAL_RETREAT` and any
  monster in `MM_CHARGE`. Ours hits them.
- **fixed — Black Death's max-HP drain** (`M_TryH2HHit`): every landed hit by `MT_YZOMBIE` takes 1 from the
  hero's max HP and base max HP, permanently, while both are above 1. Not implemented at all.
- **fixed — life steal stacks.** `PlrHitMonst` takes 3% *or* 5% (the 5% flag overrides the 3%), plus the random
  steal. `d1.Hero.HitLeech` adds 3% and 5% together when an item has both.

## Traps

- **fixed — trap missiles only hit the hero.** 1.09's `CheckMissileCol` for a sourceless missile also hits
  monsters (`MonsterTrapHit`), and a golem for monster missiles. Traps now hit monsters; still open: a monster's
  missile hitting the golem, and the fire ring's lingering walls burning monsters.

## Monster AI (f25226a)

- **fixed — the golem's wake-up.** `MAI_Golum` wakes the 5x5 block around the golem through `monster[_menemy]`
  instead of `monster[_menemy-1]`, so it wakes the *next* monster in the table. We do the same by SpawnIndex (the
  level slot); monsters spawned later have none and are skipped.
- **open — darkness.** `MAI_Sneak` idles where `dLight == lightmax`. We have no light map: a cell counts as dark
  outside the hero's light radius and any lit room.
- **fixed — combat: experience at the cap.** `AddPlrExperience` sets level 50 outright once experience reaches
  `ExpLvlsTbl[49]`, without `NextPlrLevel` (no stat points, life or mana for those levels), and gives a dead
  hero nothing. `d1.Hero.AddExperience` levels up normally.

## Shrines (`OperateShrine`)

Compared all 26: Mysterious, Hidden, Gloomy, Weird, Stone, Religious, Enchanted, Thaumaturgic, Fascinating,
Cryptic, Eldritch, Eerie, Divine, Holy, Sacred, Abandoned, Creepy, Quiet, Secluded, Ornate and Glimmering match
(Spooky and Tainted are multiplayer only).

- **fixed — Magical.** 1.09 casts the mana shield at spell level `2 * leveltype` (`AddMissile(..., MIS_MANASHIELD,
  ..., 2 * leveltype)`), so its damage reduction comes from that level. Ours raises the hero's mana shield status,
  which then reduces by the hero's own Mana Shield level (0 when unknown).
- **by design — Spiritual.** 1.09 fills every empty cell of the 40-cell inventory grid with a gold pile; this
  engine's pack has stacks, not a grid, so ours adds one pile per free stack.

## Spells

Matched: the damage of Firebolt, Holy Bolt, Charged Bolt, Fireball, Elemental, Nova, Apocalypse, Flash, Fire
Wall, Healing and Infravision (`d1/spells.go` against the `Add*` functions), the Guardian's firebolts at the hero's
own Firebolt level, Phasing / the Holy shrine (`AddRndTeleport`), the mana shield having no timer.

- **fixed — Mana Shield's reduction.** 1.09's `MI_Manashield` takes a third off the life lost once the spell
  level is above 0 (`diff += diff / -3`). Hellfire instead divides by `3·min(slvl, 7)`. `d1.ManaShieldCost`
  applies both, one after the other, so it matches neither game.
- **fixed — Chain Lightning.** `MI_Chain` sends one lightning bolt (`MIS_LIGHTCTRL`) at the target, and one more at
  every monster within `slvl + 2` cells of the caster (CrawlTable rings `1 .. slvl+2`, capped at 18). Ours is a
  beam that jumps up to 6 times within 7 cells (`hit_zone: chain_aoe(6|7)`).
- **fixed — Guardian's aim.** `MI_Guardian` fires every 16 ticks at the first monster its scan meets: each of the
  23 `vCrawlTable` rays, from its 6th point inwards, each point in four mirrored quadrants, on a clear line
  (`Sentfire`). So it reaches 6 cells and prefers far targets in a fixed order. Ours shoots the closest monster
  within 15 cells.
- **fixed — Stone Curse.** `AddStone` petrifies the first monster in the six CrawlTable rings around the target
  cell (never Diablo, never one fading or charging), for `min(slvl + 6, 15) · 16` ticks. Ours only curses a
  monster on the target cell, for a flat 12 turns (`apply_status(stone|12)`), and Diablo can be cursed.
- **fixed — Apocalypse's box.** `AddApoca`/`MI_Apoca` cover x−8 … x+7 and y−8 … y+7 (the upper bound is
  exclusive), with no line-of-sight test (Hellfire added one). Our `box_aoe(8)` is −8 … +8.
- **fixed — Flame Wave's shape.** `MI_Wave` lays a straight wall of `MIS_FIREMOVE` flames across the cast
  direction, `1 + 2·(slvl/2 + 2)` cells wide (each side stops at the first cell missiles can't cross), and the
  flames roll forward. Ours is a cone that widens to 3 cells either side over 8 (`hit_zone: cone_aoe(8|3)`).

- **fixed — Inferno's burn length.** `AddFlame` gives the flame n cells out (n = 0, 1, 2) `20 + 5n` ticks, and rolls
  each flame's damage on its own. Ours burns all three cells for 20 ticks (`InfernoTicks(0)`).
- **fixed — bolt reach.** 1.09's Firebolt, Lightning, Blood Star and Bone Spirit fly 256 ticks, i.e. until a wall
  stops them. Ours stop after 20 cells (`bolt_aoe(20)`, `ray_aoe(20)`).
- **fixed — Bone Spirit's homing.** `MI_Bonespirit` flies to the target cell; if it gets there without hitting,
  it turns towards the closest monster within 19 cells in line (`FindClosest`, CrawlTable order), or on in the
  hero's direction if there is none. Ours stops at the target cell.
- **fixed — Teleport's landing.** `AddTeleport` lands on the first free cell (not solid, no monster, object or
  player) of the six CrawlTable rings around the target, and fails (no mana spent) when there is none. Ours
  moves onto the target cell or not at all.
- **fixed — Town Portal's spot.** `AddTown` opens the portal at the target cell, or the first cell of the six
  CrawlTable rings around it free of missiles, solids, missile-blocking pieces, objects, players and triggers.
  Ours opens it next to the hero.
- **fixed — Golem's recast.** `AddGolem` kills the old golem (`M_StartKill`: death, corpse) and spends the mana even
  when `MI_Golem` then finds no free cell. Ours removes the old golem silently, and a cast with nowhere to put
  the new one does nothing and costs nothing.
- **fixed — Telekinesis on monsters.** `DoTelekinesis` never pushes a talking monster (`M_Talker`, `mtalkmsg`).
  `M_GetKnockback` moves it back against its own facing and puts it in hit recovery. Ours pushes away from the
  caster (the same thing while it faces the hero), with no hit recovery, and pushes talkers too.
- **fixed — CrawlTable order.** `freeNear` walks 1.09's own CrawlTable (`d1.CrawlRings`).
- Matched: Lightning's per-cell damage and burn (`slvl/2 + 6` ticks), Blood Star (damage, 5 HP, magic),
  Bone Spirit's third (6 HP), the golem's numbers (`SpawnGolum`), Identify.

## Objects (`OperateObject`)

Matched: sarcophagi, barrels and exploding barrels (`BreakBarrel`), skeleton books, bookcases, decapitated
bodies, armor stands, weapon racks, slain heroes, the four fountains, goat shrines and cauldrons
(`FindValidShrine`), crucified skeletons, levers, the Staff of Lazarus stand, story books.

- **fixed — chests on quest levels.** `OperateChest` on a set level always drops `CreateRndItem(..., onlygood =
  TRUE)`, ignoring `_oVar2`. Ours rolls as on an ordinary level.
- **fixed — doors can't be closed.** 1.09 closes an open door when it is clicked again, unless a monster, item or
  corpse is in the doorway (`OperateL*Door`). Ours never closes one.

## Quests (`quests.cpp`, `TalkToTowner`, `CheckQuestItem`, `CheckQuestKill`)

Matched: the kill and pick-up triggers, Griswold (Magic Rock, Anvil), Adria and Pepin (Black Mushroom,
Poisoned Water), Cain (Lazarus, Diablo), the dead guy, the quest objects (books, pedestal, tomes).

- **fixed — Ogden's Sign needs Leoric.** Ogden's banner lines in 1.09 only ask for level 3 visited; our triggers
  also require `avail("leoric")`. Leoric and Poisoned Water exclude each other, so in a Poisoned Water game the
  banner quest can never start or be handed in.
- **fixed — Ogden's greeting.** Talking to Ogden before ever entering level 1 plays `TEXT_INTRO`. Ours never does.

## Monsters

- **fixed — packs.** 1.09 ties a leader and its minions (`leaderflag`, `packsize`): a minion won't step 4 or more
  cells from its leader (`DirOK`); a leader with `mUnqAttr & 2` only moves when its whole pack is around it;
  leader and minions share `_msquelch`/`_lastx` (whoever is more alert tells the other, and wakes a gargoyle);
  a minion that strays gets back in line or drops out (`M_UpdateLeader`). Ours sets `Leader` and never reads it.
- **by design — New Game Plus.** (kept: it comes after the official content) Once Diablo is dead we keep the game running with "Nightmare" monsters and loot
  (`D1NewGamePlus`, `scaleD1NewGamePlus`, `D1ItemLevel`), using devilutionX's AC/to-hit bonuses and an HP bonus in
  the wrong units, and the end text talks of Gillian's donation. 1.09 plays the ending and goes back to the
  main menu; Nightmare only exists as a difficulty picked for a multiplayer game.
- Matched: unique monsters' stats (`mUnqAttr` 4/8), drops (`SpawnItem`, `RndItem` including its Resurrect/Heal
  Other off-by-one, `RndUItem`, `SpawnUnique`), the Black Mushroom brain.

## Items

- **fixed — bow wear.** 1.09 wears the bow on every shot (`PM_DoRangeAttack`: `WeaponDur(pnum, 40)`); ours only on
  a hit. Melee (1 in 30 on a hit), armour (3 in 4 per hit recovery, chest 2 in 3) and shields (1 in 10 per
  block) match.
- **fixed — repair of worthless magic items.** `AddStoreHoldRepair` leaves out an identified magic item whose
  repair would cost 0; ours charges 1.

## Hero death and saving

- **fixed — saving.** 1.09 single player saves only on "Save Game" and on quitting. The level-change autosave is
  now `AutosaveOnLevelChange` in config.rec (pause menu), and D1's `data/mod.rec` fixes it off, locked for the player.

## Town

- **fixed — gossip.** `S_TalkEnter` seeds the gossip roll with the townsperson's `_tSeed`, rolled when the town is
  loaded: everyone repeats the same line until the hero leaves town. Ours rolls a new line each time.
