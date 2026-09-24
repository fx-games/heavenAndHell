# REVIEW.md: Game Design Review Guidelines

Use these guidelines when reviewing the game design of this project.

## Vision

We are building a graphical roguelike on a turn-based Diablo core. In roguelike terms it is a Diablo variant, written
without the original source code. It follows most of Diablo's design and fixes its biggest pain points to make the
game more fun.

The repository has two mods:

- **D1** (Diablo 1 1.09) is the baseline. Its game design is proven but still has flaws.
- **H&H** (Of Heaven and Hell) is our game.

**H&H must never be worse or more boring than D1.** It is only worth making if it is better.

Goal: a game that plays better than a turn-based Diablo 1 while following in its footsteps.

## Review criteria

For the game to feel good, it needs the following:

1. **Distinct classes with very different gameplay.** Classes do not have to be balanced against each other. They can
   act as implicit difficulty levels.
2. **Sensible progression.** Progression should broadly work the way it does in D1. A player should predictably reach a
   certain character level by the end, after about as many dungeon levels as D1 (±4). Monster placement and item
   availability should progress along the same lines and make sense.
3. **Quests that break up the hack 'n' slash loop.** Quests should be in the spirit of D1's, but tell a different
   story: humans are the real evil.
4. **A difficulty that is neither too hard nor too easy.** Baseline for "too easy": the game must not be winnable using
   only auto attack or auto cast. The player should need tactical decisions, exploitation of monster weaknesses and
   clever use of resources to gain the upper hand.
5. **A focused design, not a kitchen sink.** Every new feature has to answer: "Is this really needed? What purpose does
   it serve? Does it make the game substantially better and more fun?"
