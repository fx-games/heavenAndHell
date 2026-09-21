# Diablo 1 turn-based rebuild — plan

Goal: play through the complete Diablo 1 game as a turn-based variant.
From D1: loot, enemies, dungeon generation. Kept from H&H: graphics, UI, time management.

## Decisions

1. **World structure: D1 replaces H&H.** Tristram + 16 levels (Cathedral 1–4, Catacombs 5–8, Caves 9–12, Hell 13–16), Diablo on 16. H&H story (Meat Jack, Zedek, Lucifer, hell town, 12-level layout) is removed.
2. **Quests: every D1 single-player quest** (see "All quests" below), including the critical path Lazarus (15, own level) → Diablo (16).
3. **Dungeon gen: byte-exact port of D1 1.09's generator** (see "Exact dungeon generation" below), down to the floor/wall/door grid. Set pieces and set levels load from the real `DIABDAT.MPQ`. Skip isometric shadow/dirt passes.
4. **Combat: D1 formulas inside H&H's turn/energy system.** D1 to-hit (Dex/clvl/item to-hit vs AC), D1 damage (weapon min–max + Str/level bonus + dmg% affixes), damage types physical/fire/lightning/magic, monster resist/immunity from `xl_monst`, player resists capped 75%, shield block chance. Attack-speed affixes → action time cost. H&H armor-as-reduction, armor penetration, weapon quality factor removed.
5. **Character: D1 sheet.** Warrior/Rogue/Sorcerer with D1 starting stats, gear, stat caps. Str/Mag/Dex/Vit. D1 XP table to clvl 50, 5 points/level, D1 per-class HP/mana growth. Item Str/Mag/Dex requirements enforced. H&H classes, class skills, Int removed.
6. **Spells: all 26 SP spells with D1 spell levels.** Books raise spell level; D1 per-level damage/mana; scrolls & staves (charges) cast at D1 levels. Turn-based: missiles resolve instantly along a line (`fire_bresen` pattern); AoE/chain instant; Fire Wall/Flame Wave/Guardian/Golem are per-turn actors/objects in the energy system; Mana Shield/Infravision/Stone Curse are status effects; Town Portal = two-way town transition (replaces `recall`). Excluded: monster-only/unused, class skills, MP-only (Resurrect, Heal Other). Unused H&H effects deleted unless monsters need them.
7. **Monster AI: one Go function per D1 AI type (24),** built on H&H primitives (Dijkstra toward/away, melee, ranged via Q6 missiles, status effects). Per-monster stats from `xl_monst`; speed → action time cost. H&H's 10%-skill-roll AI and rec-defined enemy skills removed.
8. **Sprites: one sheet icon per D1 graphic family (34), variants by tint.** Tinting is only for monster variants; everything else gets its own remappable icon: uniques `unique.<Name>`, magic/unique items `IconItemMagic*`/`IconItemUnique*`, used objects `prop.OBJ_*.used`. Two small mapping tables (family→icon, item class→icon); first-guess icons, user corrects. ASCII mode likewise.
9. **Durability & identification, D1-style.** Durability wears like D1 (WeaponDur/ArmorDur/ShieldDur, incl. StartPlrHit's flinch threshold and the 3-in-4 / 1-in-10 rolls); at 0 an item is destroyed like in D1, uniques too. Durability affixes real. Magic/unique items drop unidentified (base name, blue/gold); identify via Cain (100 gold flat, matching D1's `AddStoreHoldId`), Identify scroll or spell; like D1's CalcPlrItemVals/CalcSelfItems, unidentified magic items give only their base damage and armor, no affix bonuses, until identified.
10. **Town: keep H&H Town map, swap in D1 NPCs.** Griswold (replaces Shekel + Abel: basic + premium stock, repair), Adria (replaces Kirioth: witch stock, staff recharge), Pepin (replaces Mary: free full heal on talk, healer stock), add Cain (identify) and Wirt (1 premium item, 50 gold to look). D1 store generation (`stores.cpp`, clvl-scaled, restock on return from dungeon), D1 buy/sell prices and buy rules. Town entrances → levels 5/9/13, unlocked once reached; cathedral stairs → level 1. `TownHell` removed.
11. **Saving: one D1-style slot + permadeath option.** Autosave on every level change and on quit. Start menu: Continue / New game (class choice + permadeath toggle). Death: Load last save / New game; with permadeath the save is deleted. Save = small snapshot (player sheet, inventory/equipment, quest flags, spell levels, per-level seed + killed monsters + changed items/objects), levels rebuilt from seed. Riskiest plumbing → build early.
12. **Dungeon objects on `BumpObject`.** Containers with D1 drop rules: chests (S/M/L), sarcophagi, barrels (exploding = fire AoE), bookcases (scrolls/books), weapon & armor racks, decapitated bodies. D1 single-player shrines (26 types, SP subset) with D1 effects + level ranges. Fountains & cauldron with D1 effects. Placement per D1 `InitObjects`. Doors stay H&H doors.
13. **One D1 clock: RefWalkTicks (8) ticks = one 10-AUT turn.** Every action costs its animation's ticks via `d1TicksToAUT`, so D1's walk/attack ratio holds (a sword swing is two steps): the hero's step is 10, hero attacks come from D1 class×weapon frames (`PlrGFXAnimLens`) minus PM_DoAttack's frame skips for the attack-speed affixes (melee only, bows skip none), monster walks and attacks from their frames×(rate+1), casts from the class's spell frames, hit recovery from its ticks. 1 AUT is 0.8 ticks, so every frame count gets its own cost.
14. **Data: D1 TSV tables verbatim in `files/assets/data/d1/`, parsed with `encoding/csv` (tab).** `xl_monst`, `xl_umon`, `xl_item`, `xl_uitem`, `xl_ipre`, `xl_isuf`, `xl_spell`, `xl_obj` (+ `xl_quest` for reference). Enum names → Go constants via `map[string]`. H&H enemy/item/effect/class rec files + loaders removed (`recfile` stays for config). Non-table values (formulas, class stats, speed tiers, icon mapping) in Go or one small rec file.
15. **Critical path as D1 progresses it** (see "All quests" below): Staff of Lazarus → Cain → red portal on 15 → Lazarus' Lair → stairs down on 15 → level 16, where D1's skull levers open the sealed quads → Diablo death → end screen (`endMessage.txt`, reworded).
16. **Done = smoke test + debug run + real run.** `go run . playthrough` (next to `dun_gen`/`loot`), for several seeds: generate town, levels 1–16 + set levels; D1 tables load; stairs up/down reachable (existing pathfinding); quest bosses spawn; level-16 levers make Diablo reachable; loot + store stock are valid D1 items; Lazarus → portal → stairs → Diablo flags step through; fail loudly. Debug menu: go to level N, god mode, gold/XP, set quest flags. Done when smoke test passes + one debug run town → Diablo death → end screen + one honest normal run through the Cathedral.
17. **Build order: Cathedral slice first, then widen.** Branch `d1-rebuild`, one commit per milestone, `playthrough` grows each milestone.

## Milestones

1. [x] **Cathedral slice** — D1 table loading, character sheet, D1 combat, L1 generator, levels 1–4 with D1 monsters + AIs, loot drops (affixes, uniques), speed tiers, sprite mapping; `playthrough` for 1–4.
   Notes: sprite icons/tints are first guesses (`common/d1_monster.go`, `common/d1_item.go`). Old H&H content still feeds town shops and levels 5+, so its deletion moved to milestones 3 and 6. Shortcuts are marked `ponytail:` in code.
2. [x] **Saving** — snapshot save, Continue/New game, permadeath toggle, load on death.
   Notes: five save slots under `saves/` in the working directory (`saves/slot<N>.json` plus `slot<N>.png`, `game/save.go`); levels rebuild from `levelSeed` + `LevelSnapshot`. Not saved: non-D1 (old H&H) items. Explored cells, monster AI state, corpses, disarmed traps, the golem and Mana Shield were added later (see "Gap fixes"), slots after that (see "Save slots").
3. [x] **Town** — Griswold, Adria, Pepin, Cain, Wirt; D1 store gen; identify, durability, repair; town entrances unlock; delete H&H items/vendor defs.
   Notes: `maps.ldtk` is the town's single source: every townsperson, sign, door and entrance (a `Location` with `TargetMap`) is placed there, code adds nothing to the town map but store stock and the town portal; entrances below the cathedral stay shut until their level was reached (`z_onMove.go`). Abel's spot became Cain, Wirt stands near the cathedral entrance. Griswold's premium items share his BUY list. Deliberate differences from D1 (per decisions 9/10): broken uniques stay until repaired instead of being destroyed (Cain's identify price now matches D1 exactly: 100 gold flat). H&H's town stash is gone (D1 has none). The Hell gate (level 13) comes with milestone 6.
4. [x] **Objects** — containers, shrines, fountains.
   Notes: `d1/objects.go` ports InitThemes/InitObjects/CreateThemeRooms (Cathedral theme rooms only; L2–L4 themes come with milestone 6); `common/d1_object.go` operates them. Fixing theme-room detection also corrected the Cathedral grid (it was one cell off D1's walls). Operating an object takes the hero a turn (decoration and debris don't). Broken barrels leave walkable debris. Mana Shield/Nova shrines and the Murky Pool's infravision do nothing yet (milestone 5). (Back then any wall counted as a "trap wall"; the exact generator reads D1's own trap bit from the `.SOL` tables.) Object icons are first guesses.
5. [x] **Spells** — all 26, books/scrolls/staves.
   Notes: formulas in `d1/spells.go`, effects in `common/d1_spell.go`. Press `e` for the spell menu; known spells also sit in the skill bar. Each cast resolves at once on the grid: bolts fly until they hit (a miss flies on), Lightning/Chain/Nova/Flame Wave are rays, Flash/Inferno/fire walls roll D1's per-tick hits in one go, Fire Wall and Guardian are timed objects. Charged Bolts follow MI_Cbolt's sub-tile path (d1.ChargedBoltFlight), drawn tile to tile. Golem: an ally monster; monsters attack whichever of player and golem is closer. Town Portal: one portal, placed next to the town spawn, closed by walking back through it; saved. Staves cast from the pack (D1 needs them in hand). Adria's books ask 20% more Magic per level the hero has in their spell (WitchBookLevel, added after milestone 7); floor books don't. Monster spellcasters come with milestone 6. Fixed on the way: skeletons from sarcophagi never acted; H&H's ClearQueuedActions crashed on map change.
6. [x] **Catacombs, Caves, Hell** — L2–L4 generators, remaining AIs, levels 1–16; delete H&H enemies/effects/story generator.
   Notes: `d1/drlg_l2.go`–`drlg_l4.go` port the generators, `d1/gendung.go` holds what gendung.cpp shares; `go run . dun_gen <dlvl> [seed]` prints a level. Skipped because they don't change what's walkable: hell shadows/floor variants. Levels 5, 9 and 13 have D1's town warp; the town got a Hell gate (`to_hell`).
   Monster AIs: all single-player types except the quest bosses' (milestone 7). `AI_FIREMAN` isn't ported: Incinerators are unavailable in single player. Monster missiles (`common/d1_mmissile.go`) port PlayerMHit (magic to-hit, resistances, block, 1 HP minimum per tick) and resolve at once like the player's spells; flames and acid puddles linger as hazards; charges (horned demons, glooms, snakes) rush along the line and knock back. Like D1, monster missiles only hurt the player, not the golem. Simplifications: circling AIs treat D1's same-room check as always true, monsters don't open doors while circling (MonstCheckDoors), gargoyles heal one step per turn.
   Removed: H&H level generator (`dungen`), level templates and TownHell from `maps.ldtk`, enemy/effect rec files and their factories, dialogue, story mode/quest log, Lucifer/hell-town code (~16k lines, mostly level templates). Fixed on the way: save/load could lose a monster whose saved cell was still taken.
7. [x] **Quests & ending** — set pieces (Butcher, Leoric, Zhar, Warlord, Lazarus, Diablo), critical-path triggers, end screen, debug shortcuts, full `playthrough`.
   Notes: `common/d1_quest.go` holds the quest flags, the quest log (`r`) and `UpdateD1Quests`, which opens whatever the flags unlock each time a level is entered or a flag changes. Boss AIs: the Skeleton King raises skeletons, Zhar and Lazarus fight like counselors, Red Vex and Black Jade like succubi, the Warlord like a skeleton, Diablo casts Apocalypse. Diablo's death shows the reworded `endMessage.txt`; the end screen is a text window, the game carries on behind it, and "completed" is only the Diablo quest flag saved with the next autosave. Debug menu: go to level (including set levels), god mode, complete quest.
   Terrain: cave lava lakes, rivers and wooden fences use the tiles' separate walkable/transparent flags (`CellLava`, `CellFence`); the monsters' line check sees over them. Lava (159) and fence (63) icons are first guesses.

## After milestone 7 (playtest fixes)

- **Hit recovery** (`common/d1_hero.go`): the hero flinches only where D1 calls StartPlrHit (`D1StartPlrHit`): monster melee and arrows, missiles and traps not resisted (PlayerMHit skips it when the resistance is above 0), exploding barrels, and the forced flinch of a rhino's charge and of knockback monsters (Diablo, Hellboar, Biclops, Na-Krul, who also push the hero a cell). The threshold uses the damage before mana shield, as D1's shield refunds HP afterwards; a killing blow and self-inflicted damage (Rage, drain life, burning crosses) don't flinch. Each flinch wears armor 3 in 4 times (PM_DoGotHit). A hit of at least the hero's character level (a Barbarian 1.25×) staggers them for their class's hit animation (Warrior 6, Rogue 7, Sorcerer 8 ticks; fast/faster/fastest recovery take off 1/2/3). The lost time passes as extra world time after the hero's turn; a hard hit meanwhile restarts it instead of adding to it (capped at 50 in a row). Monsters go through `d1HitRecovery`, hooked into ChangeResourceAction: a monster hit for at least its level + 3 (any hit for the Sneak family) loses energy: its hit animation minus the rest of its attack animation after mAFNum, because our monsters' blows land when their action ends. Ticks convert at RefAttackTicks (16 ticks = 10 AUT).
- **Monster regeneration** (`d1MonsterRegen` in `game/ai_d1.go`): ProcessMonsters' mLevel/2 (mLevel on level 1) per tick in 1/64 HP, scaled by each action's cost; stopped for good by a no-heal weapon hit (`D1Monster.NoHeal`, saved). The hero has no natural regeneration, as in D1.
- **Item effects** (`resolveD1Attack` in `common/d1_hero.go`): fire/lightning weapons explode on every swing's target, hit or miss (magic to-hit, plain elemental damage); fire/lightning arrows burst where they hit (arrow to-hit and damage bonuses); knockback pushes a surviving monster one cell away on melee and arrow hits; drain life takes 4/64 HP per tick outside town; the infravision affix works like the spell. D1 stops monster healing on melee hits with the "can't heal" bit (0x100) and on fire-arrow hits; the bit named for monsters is never read. Still without effect: fast block, random arrow speed. Half trap damage works since traps fire (see "Traps, decoration and Lazarus' lair").
- "You are staggered!" when a hit starts the hero's recovery.
- Light (render only): D1 heroes always carry H&H's torch light (warm color, fire flicker) without an item. Catacomb wall torches hang as decals on wall faces above floor, Cathedral braziers stand as solid decorative objects on the floor in front of a wall face where the row further out stays open (1 in 10 / 1 in 20 faces, since D1's dungeon pieces don't exist on our grid) and glow within D1's light radius (5 / 8) where they have a clear line (`d1GlowAt` in `common/base_state.go`). Vision stays the hero's light radius, as in D1. Icons are first guesses. Iconic sound and music ideas: `iconic-todo.md`.
- Saves store at least 1 HP for living actors (regeneration leaves monsters alive below 1 HP).
- The level-up sound plays once per new character level; loot is drawn above corpses; looting a container that gives nothing says it's empty.
- Checked against D1 and unchanged: starting HP/mana, to-hit, damage, block, monster HP (halved in single player), action speeds (decision 13; walk animations have no delay, so frames are ticks).

## Exact dungeon generation

A byte-exact port of D1 1.09's own generator: real `DIABDAT.MPQ` read by a from-scratch MPQ/PKWARE reader (`d1/mpq.go`), D1's own Borland LCG RNG (`d1/rng.go`), and `Generator.CreateLevel` (`d1/level.go`) driving the same `CreateDungeon`/`InitMonsters`/`InitObjects`/`InitItems`/`CreateThemeRooms` pipeline D1 uses, verified against DevilutionX's fixture tests (`d1/drlg_fixtures_test.go`) and against real quest set levels loaded straight from the MPQ. `common/map_gen_d1.go`, `common/d1_object.go` and `common/d1_quest.go` place monsters/objects/items/triggers at the exact coordinates D1's generator chose. Unreachable stairs found via `go run -tags development . playthrough`, all fixed:

- **Quest rooms D1 opens with a trigger we don't port** (`resyncQuests` in `d1/drlg_quests.go`, run after population): D1 keeps them sealed in `dungeon` and swaps in `pdungeon` later. With talks skipped (decision 15) they start open: Ogden's sign on 4 (Banner2.DUN walls in the stairs down until the banner is handed over, ResyncQuests' qvar1 3), the Warlord's hall on 13 (Steel Tome, OperateBookLever; the sealed look was D1-correct data, not a SOL misread). Lazarus, Red Vex and Black Jade are placed in the lair (SetMapMonsters' SL_VILEBETRAYER block); the lair itself now starts sealed, as in D1 (see "Traps, decoration and Lazarus' lair").
- **Caves lava/fence:** `addBasicTiles` painted the walkable (SOL 0) pieces of lava/fence megatiles as lava/fence and the solid ones as walls; now lava/fence = solid but see-through pieces.
- **Level 15:** `ladder_down` sits on L4PENTA2's stairs piece (portal +3,+3) so UpdateD1Quests can open it.
- **Level 16:** D1's own skull levers (AddDiabObjs) open the sealed quads group by group (ObjChangeMap); the random 3×3 seal + 4 levers is gone.

## All quests (after the exact generator)

Every D1 single-player quest runs as D1 1.09 progresses it (`common/d1_quest.go`):
- **State:** InitQuests' availability still comes from the seed; `_qactive`/`_qlog`/`_qvar1`/`_qvar2` live in the saved flags (`quest:<key>`, `.active`, `.log`, `.v1`, `.v2`). The quest log lists the active, logged quests with their `_qdmsg` text. Quest, book and gossip texts are D1's own (`files/assets/data/d1/text`, `gossip`).
- **Town (TalkToTowner):** Ogden (Leoric, Ogden's Sign) and the Wounded Townsman (Butcher) joined the town; Griswold (Magic Rock, Anvil of Fury), Adria (Black Mushroom), Pepin (Poisoned Water, the brain), Cain (the staff, Diablo) take quest items and hand out rewards at their feet. A townsperson with a quest line says it instead of opening the store. "Talk to" lists Qtalklist's topics for active, logged quests.
- **Talking bosses:** bumping Gharbad, Zhar, Snotspill, Lachdanan, Red Vex/Black Jade talks (TalktoMonster, M_DoTalk); the Warlord and Lazarus speak once seen. Their AIs port the talk states (a line changes once the hero is out of sight, they fight after their last line); the state is rebuilt from the quest vars when a level is. Bookcases on Zhar's level make him talk and attack.
- **Map changes:** `d1.Level.ChangeMap` ports ObjChangeMap on a built map and `MapGenerator.changeD1Map` redraws the cells (with D1's doors). `UpdateD1Quests` re-applies what used levers, book levers, the Mythical Book, the Pedestal of Blood and Snotspill opened, so a rebuilt level looks the same. Like D1, level 4's stairs down stay behind Snotspill's gang until Ogden's Sign is done, and level 13's lead through the Warlord's hall, opened by the Steel Tome.
- **Set levels:** King Leoric's Tomb, the Chamber of Bone (Ancient Tome: +1 Guardian), the Poisoned Water Supply (done once its monsters are dead) and Lazarus' Lair, entered from their quest spots (CheckQuests).
- **Items:** Fungal Tome (level 9) and Golden Elixir (level 15) appear on arrival (ResyncQuests), the brain drops from the next ordinary monster once Adria has the mushroom, picking up quest items advances their quests (CheckQuestItem).
- Deviations: DRLG_MRectTrans stays skipped: it only marks rooms (dTransVal), which our grid doesn't draw and the circling AIs treat as always the same room.
- `go run -tags development . playthrough` walks every quest line with all quests made available (`questLines`).

## Gap fixes

- **Class skills** (`common/d1_skills.go`, formulas in `d1/stores.go`), first in the skill list and free of mana: the Warrior's Item Repair (RepairItem: refills durability, costs maximum durability, can wear an item away), the Sorcerer's Staff Recharge (RechargeItem: costs a maximum charge per round), the Rogue's Trap Disarm (TryDisarm: 2×Dex − 5×dlvl on an adjacent object; clears its trap flag and the wall traps aimed at it; rogues see "Trapped …"). Disarming stops the traps from firing (see below). Like ACTION_DISARM, doors (rogues see "a trapped door") can be disarmed too, and the object is operated right after the roll: a failed disarm opens the door or chest and sets its trap off; barrels and crucified skeletons are struck instead. Disarmed objects and doors are saved. Simplification: D1's disarm on an open door would close it; H&H doors don't close.
- **Town:** Farnham and Gillian stand at the spawn (D1's tavern) with their gossip and Qtalklist topics. Fixed on the way: Adria used Farnham's Qtalklist row, and every townsperson lost their Black Mushroom topic once Adria had the mushroom (D1 only drops hers).
- **Story objects:** story books on 4, 8 and 12 read the narrator's text (TEXT_BOOKxy) as often as the hero likes; the slain hero on 9 gives a magic breast plate, long war bow or Lightning book by class (CreateMagicArmor/Weapon, CreateSpellBook). Story candles stay decoration.
- **Terrain:** cave islands, cracks and the Hall of Heroes fix were already part of the exact generator. The Cathedral's arches, fences, pillars and lamps are see-through `IconArch`/`IconGrate`/`IconPillar`/`IconLamp` cells where D1's sight passes them (`d1CathedralSeeThrough`, like the Caves lava/fence); every other solid piece is an opaque wall, even where D1's SOL doesn't block light (stairs surrounds, catacomb/hell wall variants), since a see-through cell drawn as a plain wall lets you see and shoot into what looks like a closed room. The Poisoned Water Supply shows its lava pieces as water tiles, tinted light green until the quest is done and blue after (D1's L3pfoul/L3pwater palettes, `State.GetWaterColor`); water is the one tint besides monster variants and used objects.
- **Saves:** each level snapshot keeps monster AI state (`D1MonsterState`: goals, talk squelch, direction, dormant gargoyles, NoHeal), corpses (for scavengers), explored cells and disarmed traps. Old saves still load.
- **Golem and Mana Shield**: the golem dies on a level change and leaves its corpse (RemovePlrMissiles); the mana shield stays up, as in Hellfire.
- **Quests:** the Wounded Townsman says his line once, then the hero's PS_WARR8 line plays; Ogden's Sign is checked inside Leoric's check and quests only get `_qvar1` 1 when they start from QUEST_INIT, like 1.09. D1's `_tMsgSaid` is reset on every talk, so our one-line-per-talk returns already match it. Speech plays with every quest, book and gossip text and each townsperson's greeting, plus the hero's quest lines (kills, quest items, mushroom patch, sign chest) and IS_QUESTDN (`d1.SfxFiles`, generated from 1.09's sound table; silent until `voices.rec` has files). The third blood stone lights the hall with Blood2.DUN's torches (LoadMapObjs).

## Save slots

- **Five slots** (`game/save.go`): `saves/slot<N>.json` with the screenshot the game was last saved at beside it
  as `slot<N>.png`. `ListSaveSlots` reads each file's header only, so the screen shows who is in a slot, their
  level, the deepest dungeon they reached and when it was last played (`common.SaveSlot`) without loading a game.
  A pre-slots `savegame.json` moves into slot 1 the first time the slots are listed, and only when `SaveDir` is
  the real one, so a test never walks off with the player's game.
- **Nothing is overwritten unasked** (`confirmOverwrite`): an empty slot and the running game's own slot are
  written straight away, any other slot asks first. A new game takes the first free slot; with all five taken it
  opens the slot screen and asks which hero to replace. Autosaving keeps writing into the slot the running game
  came from, which is what "Continue" (the most recently played slot) reloads.
- **Not in permadeath**: a permadeath game still autosaves into its slot and still loses it on death, but the
  Save/Load entries are left out of the pause menu - being able to reload is the one thing permadeath forbids.
- **The screenshot** is the last frame with no menu over it, scaled to 176px wide and refreshed every 20 ticks
  (`UIController.captureThumbnail`), saved as PNG next to the slot. The terminal UI has no framebuffer and so no
  screenshots; its slot screen is a menu of one-line summaries.

## Traps, decoration and Lazarus' lair

- **Traps** (`common/d1_trap.go`): the wall traps AddObjTraps hides next to doors, chests, sarcophagi and levers fire once when their object is opened or used (Obj_Trap: arrow, fire bolt or lightning by depth, aimed at the hero next to the object); trapped chests shoot an arrow, a fire arrow or a nova at the hero who opens them (OperateChest). Missiles from no monster use PlayerMHit's own rolls: currlevel-based damage, 100 − AC/2 to-hit for arrows, 40 for magic, half damage with "absorbs half trap damage". Fired and disarmed wall traps (and so their targets' trap flags) and open doors are saved (`LevelSnapshot.Traps`, `Doors`). Simplifications: a fire arrow's burst and a nova hit once. Flame holes and their levers aren't placed by any single-player level.
- **Decoration:** every object D1 places is on the map now (`common/d1_object.go`): candles, banners, skull piles and sticks, crucified skeletons, angel statues, burning crosses, tortured and hooked bodies, bookshelves, weapon/armor stands, the lair's altar and magic circles. Solid like in D1; the ones hanging on walls are decals. Skull fires, candles and burning crosses glow (Obj_Light radii 3/5/10). Burning crosses burn the hero standing just north of them (Obj_BCrossDamage). Crucified skeletons break like barrels, and once all four in King Leoric's Tomb are down his side room opens (BreakCrux). The second armor stand/weapon rack of a level stands empty, as in D1; the Warlord's armor and weapons are looted like them. Icons are first guesses.
- **Lazarus' lair** as 1.09 plays it: the hero arrives on the central circle with the halls walled off; standing on the circle in front of a Book of Vileness and reading it teleports them into the sealed room behind it and opens it; once both are read the central circle opens the way (`_qvar1` 4) and teleports them in front of Lazarus, who speaks and opens Red Vex's and Black Jade's rooms (`_qvar1` 6). His red portal home opens at D1's (35, 32). Monsters D1 placed inside walls wait off the map until a map change opens their cell (`MapGenerator.walledIn`). 1.09 resets `_qvar1` to 4 each time the hero steps on the central circle again (its BUGFIX note); that isn't repeated.

## Defaults (not grilled)

- **Hellfire is always on, and has no toggle.** Everything of the expansion this game keeps is merged into the
  default code paths: one item table in Hellfire's `AllItemsList` order, its affixes, spells, classes, stores,
  shrines and chest traps, and its Cathedral stairs. There is no "Hellfire content" checkbox, no `D1Only` save
  field and no mid-game upgrade; a save from before the cut loads as full content.
- Hellfire's six-class roster is in: Monk, Bard and Barbarian, with their skills (Search, Identify, Rage), their
  attribute tables (devilutionX `txtdata/classes/<class>/attributes.tsv`, which also reproduces D1 1.09's) and
  their class traits (critical strike, trap sense, iron skin, natural resistance, the Monk's bare-handed block).
  The Bard dual wields (`SlotNameOneHandedWeapon` resolves to whichever hand is free, like `SlotNameRing`).
- **Hellfire's items** are in: the thirteen it drops into D1's empty table slots 35-47, the oils, runes, Scroll of
  Search and Bovine Plate appended past 156 in `xl_item.txt`, 20 uniques, 3 prefixes and 3 suffixes. `Load` lays the
  table out in Hellfire's own AllItemsList order (`hellfireItemOrder`: oils after Full Rejuvenation, Search after
  Healing, Bovine Plate in Lightforge's slot, runes last, the Short Staff of Mana), which every item roll walks.
- **Hellfire's levels, quests, towners and monsters are cut.** The Nest (17-20) and the Crypt (21-24) are gone
  (`DRLG_L6`/`DRLG_L5` and their tile tables), so the dungeon stops at 16; with them went Farmer's Orchard,
  Grave Matters, The Defiler, Na-Krul, the Cornerstone of the World, The Jersey's Jersey and the Little Girl, the
  towners Lester, the Complete Nut and Celia, their quest items, and all 26 of Hellfire's monsters and its three
  new unique monsters — every one of those only spawned on 17-24, and all seven of their AIs turn out to be
  `MAI_Ranged`/`MAI_RoundRanged` with a differently coloured flare, so nothing behavioural was lost. Hellfire's
  quest-item rows stay in `xl_item.txt` (all `IDROP_NEVER`, so unreachable) rather than renumber the table.
- **Hellfire's spells** are in, in Hellfire's spell table order (37-51, Search moved to 46): Mana, the Magi, the
  Jester, Lightning Wall, Immolation, Warp, Reflect, Berserk and Ring of Fire, with Hellfire's book and staff rolls
  (MAX_SPELLS 52, staves always carry a spell, Nova and Apocalypse books) and its fire wall that burns the hero.
  Doppelganger's weapons split ordinary monsters (AddDoppelganger). The five runes are placeable **traps**
  (Rune of Lightning as DevilutionX's lightning wall; the fire blast and the wall hurt the hero too); the Go and
  data identifiers keep D1's own `SPL_RUNE*` / `IMISC_RUNE*` names.
- **Hellfire's stores** are in (`d1/stores.go`): bigger stocks and value caps, 15 premium items rotating three a
  level and picked for the hero's class, stats and best gear, Wirt's class-aware item at 25% off, Adria's up to three
  guaranteed books, no oils at Adria but oils and plain staves at Griswold, uniques recharged, elixirs only below
  the class maximum, and Pepin healing as you walk in.
- **Still out**: town cows, the stores' 48-line list cap and belt selling (H&H has neither),
  multiplayer-only items/spells/shrines and difficulty levels (Nightmare/Hell).
- **Actor descriptions** (right click, `common.ActorInfoSections`) are titled blocks of label/value rows, laid
  out like the character sheet and drawn by the same code (`gui/info_window.go`, `drawSectionBlock`): what the
  creature is, its numbers, what it resists, how it behaves, and the matchup forecast with the rules behind it.
  The terminal UI flattens the same sections (`common.SectionLines`). H&H's old wall of combat prose for a
  non-D1 hostile (a provoked townsperson) is gone along with its formulas; such an actor now shows its numbers
  and gear, and no forecast.
- **Potions** are drunk even at full health or mana, as in D1.
- UI stays: H&H stack inventory, hotkeys, menus; no D1 grid inventory, belt, automap.
- Unrelated H&H subsystems (`osr`, `console` ASCII mode, `creator`, window-client IPC) left untouched.

## Deferred (later)

- Town: rebuilding Tristram's D1 layout in LDtk.
- Difficulty levels (Nightmare/Hell).
