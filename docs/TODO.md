# TODO

The one todo list. Design decisions for the D1 port live in `docs/D1_PLAN.md`.

Guiding rules for everything below: **quality over quantity, laser focus, simplicity over complexity.**

## Roadmap

### Controls & UI

- Gamepad controls.

### Cleanup

- Diablo cleanup: remove D1's design flaws instead of porting them faithfully.
- Cleanup affixes: prune them, and make sure every combination a roll can produce is actually reachable
  and actually does something.
- Re-introduce *Band conventions: torches as a light resource, and the rest of the roguelike furniture D1 dropped.

### Content

- Make bosses interesting: a boss should need a different plan, not more hit points.
- NPCs with helpful dialogue besides lore.
- Fewer NPCs, each with a focus.
- Shrine overhaul.

### Combat & builds

- Make every playstyle interesting and viable, each with its own approach.
- Different attack patterns per weapon type.

### Bring back the concepts from my own game

- Starting in hell.
- Switching sides.
- Going out for revenge.
- The black horse, with movement-based combat.
- Roguelike easter eggs.
- A possessed adventurer's party as enemies.
- Cool spells and skills.

## Iconic touches (later)

- Voice lines: "The sanctity of this place has been fouled." (D1), "Stay a while, and listen." — Deckard Cain (D1).
- Music: D1 Tristram

## Triage: old lists, status unverified

Merged from the old `todo.txt` (7DRL era) and `agent-todo.txt`. Many are likely done; check, then delete or promote.

### Bugs

### UI / UX

### Content (7DRL goals)

## Ideas

### Enemy stat bands

| | HP | AC | Dmg |
|---|---|---|---|
| Starting | 3-10 | 0-1 | 1-5 |
| Mid game | 10-500 | 2-50 | 5-30 |
| End boss | 1000 | 60-100 | 30-60 |

Moria's Balrog: `{"Balrog", 0xFF1F0002L, 0x0081C743L, 0x5004, 55000L, 0, 40, 125, 13, 'B', { 75, 40}, {104, 78, 214, 0}, 100}`

### Monster skills from other games

- Moria: poison, aggravation.
- DCSS: reach (attack from 2 tiles), constrict (immobilize + DOT), barbs (moving hurts), knockback.
- Rogue: invisibility, mimic loot, steal item/gold, reduce dmg/AC, drain mana, steal life/mana, confuse, stun,
  damage equipment, breathe fire, immobilize, flying, evade.
- Brogue: flit (33% of moves random, always a move, never an attack; most fliers flit), aquatic, suicide attack
  (gas/explosion/pit), summon spectral blades, protect/haste/heal allies, slow, lightning, cloning, never sleeps,
  spell reflection, hallucination, inanimate, immune to weapons, corrosive, weakness, grabs hold, blink,
  damaging cloud aura, quick regeneration.
