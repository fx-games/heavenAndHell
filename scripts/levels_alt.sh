#!/bin/zsh
# Browse the roguelike stand-ins for D1's level generators (table in src/cmd/levelcmp/alt.go) in the terminal, one char per map cell.
# 1-4: cathedral, catacombs, caves, hell; the same key or space rerolls; q quits. Compare with scripts/levels_d1.sh.
cd "${0:A:h}/.." && go run ./src/cmd/levelcmp -alt "$@"
