# Diablo 1 turn-based rebuild — plan

Goal: play through the complete Diablo 1 game as a turn-based variant.
From D1: loot, enemies, dungeon generation. Kept from H&H: graphics, UI, time management.

## Decisions

1. **World structure: D1 replaces H&H.** Tristram + 16 levels (Cathedral 1–4, Catacombs 5–8, Caves 9–12, Hell 13–16), Diablo on 16. H&H story (Meat Jack, Zedek, Lucifer, hell town, 12-level layout) is removed.
2. **Quests: critical path + boss quests.**
   - Critical: Archbishop Lazarus (15, own level) → Diablo (16).
   - Boss: The Butcher (2), Skeleton King Leoric (3, own level), Zhar the Mad (8), Warlord of Blood (13).
3. **Dungeon gen: port D1 layout algorithms (L1–L4) down to the floor/wall/door grid.** Fill `dungen.DungeonMap`, render with existing tilemaps. Skip isometric tile-fix/shadow/dirt passes. 1 D1 cell → 2×2 cells (80×80 levels). Set pieces (Leoric, Lazarus, Warlord, Diablo) hand-rebuilt as LDtk templates (`.DUN` files live in the MPQ, not the source). Go `math/rand`, not D1's RNG.
4. **Combat: D1 formulas inside H&H's turn/energy system.** D1 to-hit (Dex/clvl/item to-hit vs AC), D1 damage (weapon min–max + Str/level bonus + dmg% affixes), damage types physical/fire/lightning/magic, monster resist/immunity from `xl_monst`, player resists capped 75%, shield block chance. Attack-speed affixes → action time cost. H&H armor-as-reduction, armor penetration, weapon quality factor removed.
5. **Character: D1 sheet.** Warrior/Rogue/Sorcerer with D1 starting stats, gear, stat caps. Str/Mag/Dex/Vit. D1 XP table to clvl 50, 5 points/level, D1 per-class HP/mana growth. Item Str/Mag/Dex requirements enforced. H&H classes, class skills, Int removed.
6. **Spells: all 26 SP spells with D1 spell levels.** Books raise spell level; D1 per-level damage/mana; scrolls & staves (charges) cast at D1 levels. Turn-based: missiles resolve instantly along a line (`fire_bresen` pattern); AoE/chain instant; Fire Wall/Flame Wave/Guardian/Golem are per-turn actors/objects in the energy system; Mana Shield/Infravision/Stone Curse are status effects; Town Portal = two-way town transition (replaces `recall`). Excluded: monster-only/unused, class skills, MP-only (Resurrect, Heal Other). Unused H&H effects deleted unless monsters need them.
7. **Monster AI: one Go function per D1 AI type (24),** built on H&H primitives (Dijkstra toward/away, melee, ranged via Q6 missiles, status effects). Per-monster stats from `xl_monst`; speed → action time cost. H&H's 10%-skill-roll AI and rec-defined enemy skills removed.
8. **Sprites: one sheet icon per D1 graphic family (34), variants by tint.** Uniques = base icon, gold tint. Items: one icon per D1 item class (~30), tint by quality (white/blue/gold). Two small mapping tables (family→icon, item class→icon); first-guess icons, user corrects. ASCII mode likewise.
9. **Durability & identification, D1-style.** Weapons lose durability on hit, armor when hit; 0 = broken (no effect) until repaired at Griswold (existing SetToMax). Durability affixes real. Magic/unique items drop unidentified (base name, blue/gold); identify via Cain (free), Identify scroll or spell; unidentified items still work with hidden effects.
10. **Town: keep H&H Town map, swap in D1 NPCs.** Griswold (replaces Shekel + Abel: basic + premium stock, repair), Adria (replaces Kirioth: witch stock, staff recharge), Pepin (replaces Mary: free full heal on talk, healer stock), add Cain (identify) and Wirt (1 premium item, 50 gold to look). D1 store generation (`stores.cpp`, clvl-scaled, restock on return from dungeon), D1 buy/sell prices and buy rules. Town entrances → levels 5/9/13, unlocked once reached; cathedral stairs → level 1. `TownHell` removed.
11. **Saving: one D1-style slot + permadeath option.** Autosave on every level change and on quit. Start menu: Continue / New game (class choice + permadeath toggle). Death: Load last save / New game; with permadeath the save is deleted. Save = small snapshot (player sheet, inventory/equipment, quest flags, spell levels, per-level seed + killed monsters + changed items/objects), levels rebuilt from seed. Riskiest plumbing → build early.
12. **Dungeon objects on `BumpObject`.** Containers with D1 drop rules: chests (S/M/L), sarcophagi, barrels (exploding = fire AoE), bookcases (scrolls/books), weapon & armor racks, decapitated bodies. D1 single-player shrines (26 types, SP subset) with D1 effects + level ranges. Fountains & cauldron with D1 effects. Placement per D1 `InitObjects`. Doors stay H&H doors.
13. **Speeds from D1 frame counts, rounded to tiers.** Monster move speed from walk frames×rate, attack speed from attack frames, relative to player baseline. Tiers: slow ×0.5 / normal ×1 / fast ×1.5 / very fast ×2 (one tunable table in data). Player attack speed from D1 class×weapon frames (`PlrGFXAnimLens`); attack-speed affixes step up a tier.
14. **Data: D1 TSV tables verbatim in `files/assets/data/d1/`, parsed with `encoding/csv` (tab).** `xl_monst`, `xl_umon`, `xl_item`, `xl_uitem`, `xl_ipre`, `xl_isuf`, `xl_spell`, `xl_obj` (+ `xl_quest` for reference). Enum names → Go constants via `map[string]`. H&H enemy/item/effect/class rec files + loaders removed (`recfile` stays for config). Non-table values (formulas, class stats, speed tiers, icon mapping) in Go or one small rec file.
15. **Critical path, D1 sequence with one shortcut.** Staff of Lazarus on a stand on level 15 → bump Cain while carrying it (one fixed text line, sets flag, removes staff; no dialogue system) → red portal (transition object) on 15 → Lazarus set level (LDtk) → Lazarus death spawns return portal + opens stairs down on 15 → level 16 Diablo set piece with its 4 levers (BumpObjects opening wall sections; exception to "levers deferred") → Diablo death → existing end screen (`endMessage.txt`, reworded), save marked completed. Other boss quests have no triggers: boss sits in its set piece; Leoric's level via a marked staircase on 3.
16. **Done = smoke test + debug run + real run.** `go run . playthrough` (next to `dun_gen`/`loot`), for several seeds: generate town, levels 1–16 + set levels; D1 tables load; stairs up/down reachable (existing pathfinding); quest bosses spawn; level-16 levers make Diablo reachable; loot + store stock are valid D1 items; Lazarus → portal → stairs → Diablo flags step through; fail loudly. Debug menu: go to level N, god mode, gold/XP, set quest flags. Done when smoke test passes + one debug run town → Diablo death → end screen + one honest normal run through the Cathedral.
17. **Build order: Cathedral slice first, then widen.** Branch `d1-rebuild`, one commit per milestone, `playthrough` grows each milestone.

## Milestones

1. [x] **Cathedral slice** — D1 table loading, character sheet, D1 combat, L1 generator, levels 1–4 with D1 monsters + AIs, loot drops (affixes, uniques), speed tiers, sprite mapping; `playthrough` for 1–4.
   Notes: sprite icons/tints are first guesses (`common/d1_monster.go`, `common/d1_item.go`). Old H&H content still feeds town shops and levels 5+, so its deletion moved to milestones 3 and 6. Shortcuts are marked `ponytail:` in code.
2. [ ] **Saving** — snapshot save, Continue/New game, permadeath toggle, load on death.
3. [ ] **Town** — Griswold, Adria, Pepin, Cain, Wirt; D1 store gen; identify, durability, repair; town entrances unlock; delete H&H items/vendor defs.
4. [ ] **Objects** — containers, shrines, fountains.
5. [ ] **Spells** — all 26, books/scrolls/staves.
6. [ ] **Catacombs, Caves, Hell** — L2–L4 generators, remaining AIs, levels 1–16; delete H&H enemies/effects/story generator.
7. [ ] **Quests & ending** — set pieces (Butcher, Leoric, Zhar, Warlord, Lazarus, Diablo), critical-path triggers, end screen, debug shortcuts, full `playthrough`.

## Defaults (not grilled)

- Hellfire content, multiplayer-only items/spells/shrines, and difficulty levels (Nightmare/Hell) out of scope.
- UI stays: H&H stack inventory, hotkeys, menus; no D1 grid inventory, belt, automap.
- Unrelated H&H subsystems (`osr`, `console` ASCII mode, `creator`, window-client IPC) left untouched.

## Deferred (later)

- Combat: hit recovery / stun lock (maybe: big hit costs target energy).
- Class skills: Warrior repair, Rogue disarm trap, Sorcerer recharge staff (need durability/traps).
- Town: Ogden, Farnham, Gillian; all gossip/NPC dialogue; rebuilding Tristram's D1 layout in LDtk.
- Objects: traps (trapped chests, arrow/fire-wall traps, levers — disarm deferred), quest-only objects of deferred quests, decorative objects (candles, banners, torture racks, crucifixes).
- Difficulty levels (Nightmare/Hell).
- Quests: Magic Rock, Black Mushroom, Gharbad the Weak, Ogden's Sign, Halls of the Blind, Valor, Anvil of Fury, Lachdanan, Chamber of Bone, Poisoned Water Supply.
