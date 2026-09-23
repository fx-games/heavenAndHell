#!/bin/zsh
# Browse the d1 mod's dungeon layouts (levels.rec: layout and passes, set pieces left out) in the terminal, one char
# per map cell. 1-4: cathedral, catacombs, caves, hell; the same key or space rerolls; q quits.
cd "${0:A:h}/.." && go run ./src/cmd/levelcmp "$@"
