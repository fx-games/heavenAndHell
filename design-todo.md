# Design TODO

Issues from the game design review against `REVIEW.md`, in priority order. Check an item off when it lands.

## 1. Monster weaknesses (criterion 4: tactics over auto attack)

- [x] Every family has a `class` (hero.rec now has D1's sword/mace bonuses) and a resistance: skeletons want
      maces, caves swords and no fire, hell's fiends are fire immune (lightning or steel), human bosses resist
      by armour or faith. Threats as tags: knockback, turns_on_attacker. Fire on the ground honours immunity.
- [x] `poison_immune` tag; H&H's undead shrug off Black Clouds and Poison.
- [ ] Only 5 of 44 monsters have a skill; `ai` stays empty (D1 brains halve stated health, so adopting one is a
      per-monster rebalance). Give each family one behaviour of its own.

## 2. Measurable progression (criterion 2)

- [ ] `playthrough` is hard-wired to D1; let it run H&H so level/depth/char-level can be compared to D1's golden run.
- [ ] Variety: a level's type draw has 4 candidates at every depth (plus the two thieves as 10% loners); D1 has
      15-24 on depths 2-14. Every cathedral level offers the same rat/skeleton/bat mix. Needs new families.
- [x] ~~`skeleton_warrior` stops at `max_dlvl: 2`~~: it is the depth 1-2 dagger variant, fine.
- [x] ~~`fallen_shaman` 889 exp from depth 1~~: misread, it is `dlvl: 9`, quest-placed only; on par with a goatman.
- [ ] Depths 17-20 plus the second act: fine only if the total stays within D1's 16 ±4.

## 3. Mid-game quests (criterion 3)

- [ ] Act 1 has four quests (D1: ~16); depths 9-15 have none.
- [ ] Nearly all are "kill X"; add fetch/lever/rule-changing set pieces like the evidence hunt.

## 4. Bosses

- [ ] Lucifer, Zedek, Shekel, Kirioth: each needs one mechanic that demands a different plan, not more HP.

## 5. Classes (criterion 1)

- [ ] Starting skills are weak identities: Warrior `poison_target`, Mage `heal_health` (flat 20, useless later).
- [ ] Archer uses `voice: WARR`; ids still `rogue`/`sorcerer`.
- [ ] Class skills repair/disarm/recharge are D1's leftovers, not H&H's.

## 6. Focus (criterion 5)

- [ ] Carried light: meaningful only if light is scarce; otherwise cut it.
- [ ] TODO.md's "Bring back" list (black horse, switching sides, easter eggs...) must each pass criterion 5 first.
