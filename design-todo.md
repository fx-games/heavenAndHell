# Design TODO

Issues from the game design review against `REVIEW.md`, in priority order. Check an item off when it lands.

## 1. Monster weaknesses (criterion 4: tactics over auto attack)

- [x] Every family has a `class` (hero.rec now has D1's sword/mace bonuses) and a resistance: skeletons want
      maces, caves swords and no fire, hell's fiends are fire immune (lightning or steel), human bosses resist
      by armour or faith. Threats as tags: knockback, turns_on_attacker. Fire on the ground honours immunity.
- [x] `poison_immune` tag; H&H's undead shrug off Black Clouds and Poison.
- [x] Every H&H family fights under a D1 brain now (health doubled for D1's halving): skeletons as D1's
      skeletons, cavelings, fallen and jackals as packs that flee when one dies, bears, emus and horned
      fiends charge, bats hit and run, goatmen circle, the thief stalks unseen. Skill-driven ones keep no ai.

## 2. Measurable progression (criterion 2)

- [x] `playthrough` is hard-wired to D1; let it run H&H so level/depth/char-level can be compared to D1's golden run.
  → `go run ./src autoplay <mod> [seed] [runs]`: a bot plays class 0 down the dungeon, one row per depth. First
  read (seeds 1–2): D1 clears ~135 monsters on depth 1 in ~900 turns and reaches clvl 6 by depth 3; H&H ~60–120 in
  2–4k turns, clvl 3–4 by depth 3 (slower XP, more walking). The bot keeps its starting gear, so both die on 3–4.
  Found on the way: the possessed thief (mlvl 11) and the thief (mlvl 21) spawned from depth 1; now 6 and 12.
- [ ] Autoplay bot: equip found gear, buy potions in town, cast spells, so its depth curve means more than relative pacing.
- [x] Variety: the type draw had 4 candidates at every depth (D1: 15-24). Now 6-15, from 42 monsters in 11
      families borrowed from D1 (sprite, brain and numbers; H&H names), see the end of H&H's enemies.rec:
      | depth | family (D1 source) | role |
      |---|---|---|
      | 1-5 | The Forsaken (zombies) | plague victims the church walled in; slow, poison immune |
      | 1-6 | Carrion hounds (scavengers) | eat corpses to heal |
      | 3-8 | Bat demons (blink/gloom/familiar) | hit and run |
      | 5-8 | Penitents (stalker/unseen) | the Inquisition's invisible killers; humans |
      | 5-9 | Stone saints (gargoyles) | church statues that wake |
      | 5-12 | Gluttons (overlord family) | slow, hard hitting |
      | 6-14 | Acid beasts | spit from afar, acid blood |
      | 7-12 | Chargers (horned demons) | circle, then charge |
      | 8-11 | Magma demons | fire immune casters |
      | 11-16 | Vipers (snakes) | lunge through gaps |
      | 12-16 | Templars (black knights) | the church's knights; humans |
      | 13-16 | Inquisitors (counselors) | the church's mages, blink and cast; humans |
- [ ] Playtest the borrowed families' balance and remap odd sprite picks (frost charger, mud runner, penitent).
- [x] ~~`skeleton_warrior` stops at `max_dlvl: 2`~~: it is the depth 1-2 dagger variant, fine.
- [x] ~~`fallen_shaman` 889 exp from depth 1~~: misread, it is `dlvl: 9`, quest-placed only; on par with a goatman.
- [ ] Depths 17-20 plus the second act: fine only if the total stays within D1's 16 ±4.

## 3. Mid-game quests (criterion 3)

- [x] Sister Mary's "The lost pilgrims" (caves 10, fetch; early clue). Proposed, not chosen: Where is Jim? (Abel,
      caves 11), Seal of the Church (Kirioth, hell 14 boss), Consecrate the Dead (item use, needs engine hook).
- [ ] Act 1 still thin at depths 11-15.
- [ ] Nearly all are "kill X"; add fetch/lever/rule-changing set pieces like the evidence hunt.

## 4. Bosses

- [x] Each boss fights under one D1 brain (see the "Boss mechanic" comments in H&H's enemies.rec):
      Meat Jack eats corpses to heal (scavenger), Zedek raises skeletons from afar (Skeleton King), Lucifer
      keeps away and calls Apocalypse (Diablo), the Rogue is unseen outside your light (stalker) and his dog
      charges (horned demon), the adventurers are a party (knight wall, barbarian charger, archer, priest who
      shields the rest), Shekel lobs poison behind two hired templars (acid beast), Kirioth blinks and casts
      behind two inquisitors (counselor). Zedek's tomb and Lucifer's hall got candles: with a radius 1 torch
      a boss in the dark only notices you next to it. spawn_monster now brings a boss's group_spawn escort.
- [ ] Playtest the boss fights; the priest's shield and Meat Jack's corpse eating are untested in play.

## 5. Classes (criterion 1)

- [x] Starting skills: Warrior starts with `crush`, Mage with `black_clouds` (the old ones stay learnable).
- [ ] Archer uses `voice: WARR`; ids still `rogue`/`sorcerer`.
- [x] Class skills repair/disarm/recharge are D1's leftovers, not H&H's. → removed; H&H classes have no class skill.

## 6. Focus (criterion 5)

- [x] Carried light: kept (decision 2026-09-24).
- [ ] TODO.md's "Bring back" list (black horse, switching sides, easter eggs...) must each pass criterion 5 first.
