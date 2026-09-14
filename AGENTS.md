# AGENTS.md

## Local reference sources (not committed)

`reference/` is gitignored and holds shallow (`--depth 1`) clones for local lookup only:

- `reference/devilution` — https://github.com/galaxyhaxz/devilution
  Reverse-engineered Diablo 1 source (C++). Use `2020_03_31/Source` (the more accurate rewrite), e.g.
  `monstdat.cpp` (monsters), `itemdat.cpp` / `items.cpp` (loot), `drlg_l1..l4.cpp` (dungeon generation),
  `quests.cpp`, `objects.cpp`, `spelldat.cpp`.

Re-create with: `git clone --depth 1 https://github.com/galaxyhaxz/devilution reference/devilution`

Design decisions and deferred scope for the D1 rebuild: `D1_PLAN.md`.

## Running

- Game: `go run -tags development .` (assets from `files/`, `config.rec` from the repo root)
- D1 unit tests: `go test ./d1`
- Headless smoke test: `go run -tags development . playthrough` (must print `PLAYTHROUGH OK`)
- Print a generated D1 level: `go run . dun_gen <dungeon level> [seed]`
